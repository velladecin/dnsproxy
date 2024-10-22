package main

import (
    "net"
    "time"
    "syscall"
    // had to link in go-<ver>/src
    // golang.org -> cmd/vendor/golang.org
    "golang.org/x/sys/unix"
    "context"
)

// TODO signals to terminate

// debug
var debug bool

// server, cache log
var sInfo, sWarn, sCrit, sDebg Logger
var cInfo, cWarn, cCrit, cDebg Logger

type Server struct {
    worker []*Proxy
}

func (s Server) Run() {
    for i, srv := range s.worker {
        go srv.Accept() 
        sInfo.Printf("Listener worker #%d accepting connections", i+1)
    }

    for {
        time.Sleep(1*time.Second)
    }
}

type Proxy struct {
    // local listener
    listener net.PacketConn

    // remote listener/dialer
    dialer <-chan string
    
    // prepares and provides empty packets
    packet <-chan []byte

    // local cache
    c *Cache

    // worker ID
    id int
}

func NewServer(config string, dbg, stdout bool) Server {
    debug = dbg

    conf, err := newCfg(config)
    if err != nil {
        panic(err)
    }

    // CLI overwrites config file
    if stdout {
        conf.serverLog = STDOUT
        conf.cacheLog = STDOUT
    }

    cInfo, cWarn, cCrit, cDebg = NewHandles(conf.serverLog)
    sInfo, sWarn, sCrit, sDebg = NewHandles(conf.cacheLog)

    sInfo.Printf("== Server Configuration ==")
    sInfo.Printf("Local host: %s", conf.localHostString())
    sInfo.Printf("Local worker: %d", conf.localWorker)
    sInfo.Printf("Remote host: %s", conf.remoteHostString())
    sInfo.Printf("Resource records: %s", conf.localRR)
    sInfo.Printf("Default domain: %s", conf.defaultDomain)
    sInfo.Printf("Server log: %s", conf.serverLog)
    sInfo.Printf("Cache log: %s", conf.cacheLog)

    // cache
    cache := NewCache(conf.localRR, conf.defaultDomain)

    // dialer
    dch := make(chan string, PACKET_PREP_Q_SIZE + (conf.localWorker-WORKER)*7)
    go func(c chan string) {
        for {
            c <- conf.remoteNetConnString()
        }
    }(dch)

    // packets (empty)
    pch := make(chan []byte, PACKET_PREP_Q_SIZE + (conf.localWorker-WORKER)*7)
    go func(c chan []byte) {
        for {
            c <- make([]byte, PACKET_SIZE)
        }
    }(pch)

    // network config
    lconf := net.ListenConfig{
        Control: func (net, addr string, c syscall.RawConn) error {
            return c.Control(func(fd uintptr) {
                // SO_REUSEADDR
                err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
                if err != nil {
                    panic(err)
                }

                // SO_REUSEPORT
                err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
                if err != nil {
                    panic(err)
                }
            })
        },
    }

    srv := Server{make([]*Proxy, conf.localWorker)}
    
    for i:=0; i<conf.localWorker; i++ {
        ll, err := lconf.ListenPacket(context.Background(), "udp4", conf.localNetConnString())
        if err != nil {
            panic(err)
        }

        sInfo.Printf("Listener worker #%d netsock bind: %s", i+1, ll.LocalAddr().String())

        srv.worker[i] = &Proxy{ll, dch, pch, cache, i+1}
    }

    return srv
}

func (p *Proxy) Accept() {

    // don't panic() in the below connections
    // client will hopefully just timeout

    for {
        query := <-p.packet
        answer := <-p.packet

        // receiver
        ql, addr, err := p.listener.ReadFrom(query) // blocking
        if err != nil {
            if ql > 0 {
                sCrit.Print("Could not read request: bytes read %d, err: %s: ", ql, err.Error())
            }

            continue
        }

        qs := Question(query[0:ql])

        sInfo.Printf("worker %d: Query id: %d, len: %d, question: %s", p.id, bytesToInt(query[:2]), ql, qs)
        if debug {
            sDebg.Printf("worker %d: Query id: %d, bytes: %+v", p.id, bytesToInt(query[:2]), query[0:ql])
        }

        // check local cache or
        // contact remote/dialer

        if a := p.c.Get(qs); a != nil {
            a.CopyRequestId(query)
            answer = a.serializePacket(answer)

            sInfo.Printf("worker %d: Resp id: %d, len: %d, answer: %s", p.id, bytesToInt(answer[:2]), len(answer), a.ResponseString())
        } else {
            dialer, err := net.Dial("udp4", <-p.dialer)
            if err != nil {
                if debug {
                    sDebg.Printf("worker %d: Failed to dial upstream: %s", p.id, err.Error())
                }

                continue
            }

            if debug {
                sDebg.Printf("worker %d: Dialing to upstream: %s", p.id, dialer.RemoteAddr().String())
            }

            // request timeout
            dialer.SetDeadline(time.Now().Add(time.Second * CONNECTION_TIMEOUT))

            al := 0
            al, err = dialer.Write(query[0:ql])
            if err != nil {
                sCrit.Printf("worker %d: Failed to write query to upstream, written: %d, error: %s", p.id, al, err.Error())
                continue
            }

            if debug {
                sDebg.Printf("worker %d: Bytes written to dialer: %d", p.id, al)
            }

            al = 0
            al, err = dialer.Read(answer)
            if err != nil {
                sCrit.Printf("worker %d: Failed to read from upstream, read: %d, error: %s", p.id, al, err.Error())
                continue
            }

            answer = answer[0:al]

            // TODO fish out answer from upstream
            sInfo.Printf("worker %d, Resp id: %d, upstream: %s, len: %d", p.id, bytesToInt(answer[:2]), dialer.RemoteAddr().String(), al)
            dialer.Close()
        }
        
        if debug {
            sDebg.Printf("worker %d: Resp id: %d, bytes: %+v", p.id, bytesToInt(answer[:2]), answer)
        }

        _, err = p.listener.WriteTo(answer, addr)
        if err != nil {
            sCrit.Printf("worker %d: Failed to write answer back to the client: %s", p.id, err.Error())
        }
    }
}
