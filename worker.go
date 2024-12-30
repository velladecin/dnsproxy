package main

import (
    "sync"
    "net"
    "context"
    "time"
)

type Worker interface {
    Start(net.ListenConfig, string, bool, *Cache, chan []byte, chan string, int) error
    ServeDNS()
    Close()
    Type() string
}

type WorkerCommon struct {
    // cache
    cache *Cache

    // predeclared empty packets
    packeter chan []byte

    // upstream dialer
    dialer chan string

    // sync
    wg sync.WaitGroup

    // proxy mode
    proxy bool

    // shutdown request
    exit chan bool

    // shutdown confirmation
    exited chan bool

    // worker id
    id int
}

//
// UDP

type WorkerUDP struct {
    // udp listener
    listener net.PacketConn

    WorkerCommon
}

func NewWorkerUDP() *WorkerUDP {
    return &WorkerUDP{}
}

func (w *WorkerUDP) Type() string {
    return "UDP"
}

func (w *WorkerUDP) Start(lc net.ListenConfig, iface string, x bool, c *Cache, p chan []byte, d chan string, id int) error {
    l, err := lc.ListenPacket(context.Background(), "udp4", iface)
    if err != nil {
        return err
    }

    w.listener = l
    w.cache = c
    w.packeter = p
    w.dialer = d
    w.proxy = x
    w.exit = make(chan bool)
    w.exited = make(chan bool)
    w.id = id

    return nil
}

func (w *WorkerUDP) ServeDNS() {
    for {
        query := <-w.packeter

        // receiver (blocking)
        ql, addr, err := w.listener.ReadFrom(query)
        if err != nil {
            select {
            case <-w.exit:
                sInfo.Printf("Listener #%d closing %s socket", w.id, w.Type())
                w.wg.Wait()
                close(w.exited)

                // jump out
                return

            default:
                sCrit.Printf("Listener #%d %s request receive error: bytes read %d, err: %s: ", w.id, w.Type(), ql, err.Error())
            }

            continue
        }

        // offload processing
        // to free up the listener
        w.wg.Add(1)
        go func(q, a []byte, c *Cache, d string, p bool, i int, l net.PacketConn, addr net.Addr) {
                answer := ProcessQuery(q, a, c, d, p, i)
                _, err := l.WriteTo(answer, addr)
                if err != nil {
                    sCrit.Printf("Listener #%d failed to write answer back to the client: %s", i, err.Error())
                }
                w.wg.Done()
        }(query[0:ql], <-w.packeter, w.cache, <-w.dialer, w.proxy, w.id, w.listener, addr)
    }
}

func (w *WorkerUDP) Close() {
    // exit request
    close(w.exit)

    // close listening socket
    w.listener.Close()

    // exit confirmation
    <-w.exited
}


//
// TCP

type WorkerTCP struct {
    // tcp listener
    listener net.Listener

    WorkerCommon
}

func NewWorkerTCP() *WorkerTCP {
    return &WorkerTCP{}
}

func (w *WorkerTCP) Type() string {
    return "TCP"
}

func (w *WorkerTCP) Start(lc net.ListenConfig, iface string, x bool, c *Cache, p chan []byte, d chan string, id int) error {
    l, err := lc.Listen(context.Background(), "tcp4", iface)
    if err != nil {
        return err
    }

    w.listener = l
    w.cache = c
    w.packeter = p
    w.dialer = d
    w.proxy = x
    w.exit = make(chan bool)
    w.exited = make(chan bool)
    w.id = id

    return nil
}

func (w WorkerTCP) ServeDNS() {
    for {
        query := <-w.packeter

        // blocking receiver
        conn, err := w.listener.Accept()
        if err != nil {
            select {
            case <-w.exit:
                sInfo.Printf("Listener #%d closing %s socket", w.id, w.Type())
                w.wg.Wait()
                close(w.exited)

                // jump out
                return

            default:
                sCrit.Printf("Listener #%d %s request receive error: %s", w.id, w.Type(), err.Error())
            }

            continue
        }

        ql, err := conn.Read(query)
        if err != nil {
            sCrit.Printf("Listener #%d %s request read error: bytes read %d, err: %s: ", w.id, w.Type(), ql, err.Error())
            continue
        }

        w.wg.Add(1)
        go func(q, a []byte, c *Cache, d string, p bool, i int, conn net.Conn) {
                answer := ProcessQuery(q, a, c, d, p, i)
                _, err := conn.Write(answer)
                if err != nil {
                    sCrit.Printf("Listner #%d failed to write answer back to the client: %s", i, err.Error())
                }
                w.wg.Done()
        }(query[0:ql], <-w.packeter, w.cache, <-w.dialer, w.proxy, w.id, conn)
    }
}

func (w *WorkerTCP) Close() {
    // exit request
    close(w.exit)

    // close listening socket
    w.listener.Close()

    // exit confirmation
    <-w.exited
}

func ProcessQuery(query, answer []byte, cache *Cache, dialer string, proxy bool, wid int) []byte {
    qs := Question(query)
    rt := RequestType(query)

    sInfo.Printf("#%d: Query id: %d, type: %s, len: %d, question: %s", wid, bytesToInt(query[:2]), RequestTypeString(rt), len(query), qs)
    if debug {
        sDebg.Printf("#%d: Query id: %d, bytes: %+v", wid, bytesToInt(query[:2]), query)
    }

    // answer length
    al := 0

    if a := cache.Get(rt, qs); a != nil {
        a.CopyRequestId(query)
        al = a.serializePacket(answer)

        sInfo.Printf("#%d: Resp id: %d, len: %d, answer: %s", wid, bytesToInt(answer[:2]), al, a.ResponseString())

        return answer[0:al]
    }

    if proxy {
        // TODO should tcp worker be calling tcp here too??
        // proxy on, dial upstream
        conn, err := net.Dial("udp4", dialer)
        if err != nil {
            if debug {
                sDebg.Printf("#%d: Failed to dial upstream: %s", wid, err.Error())
            }

            return answer
        }
        defer conn.Close()

        if debug {
            sDebg.Printf("#%d: Dialing to upstream: %s", wid, conn.RemoteAddr().String())
        }

        // upstream connection timeout
        conn.SetDeadline(time.Now().Add(time.Second * CONNECTION_TIMEOUT))

        al, err = conn.Write(query)
        if err != nil {
            sCrit.Printf("#%d: Failed to write query to upstream, written: %d, error: %s", wid, al, err.Error())
            return answer
        }

        if debug {
            sDebg.Printf("#%d: Bytes written upstream: %d", wid, al)
        }

        al = 0
        al, err = conn.Read(answer)
        if err != nil {
            sCrit.Printf("#%d: Failed to read from upstream, read: %d, error: %s", wid, al, err.Error())
            return answer
        }

        sInfo.Printf("#%d, X-ON, Resp id: %d, upstream: %s, len: %d, answer: %s", wid, bytesToInt(answer[:2]), conn.RemoteAddr().String(), al, Response(answer))
        return answer[0:al]
    }

    a := NewRefused(qs)
    a.CopyRequestId(query)
    al = a.serializePacket(answer)

    sInfo.Printf("#%d: X-OFF, Resp id: %d, len: %d, answer: %s", wid, bytesToInt(answer[:2]), al, a.ResponseString())
    
    if debug {
        sDebg.Printf("#%d: Resp id: %d, bytes: %+v", wid, bytesToInt(answer[:2]), answer[:al])
    }

    return answer[0:al]
}
