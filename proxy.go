package main

import (
    "fmt"
    "net"
_    "sync"
)

const (
    // net
    network = "udp4"
    port = 53

    // proxy
    local = "127.0.0.1"
    remote1 = "8.8.8.8"
    remote2 = "8.8.4.4"

    // pre-prepared queue size
    // for upstream/remote connections and empty packets
    factoryQsize = 3
)

// Proxy
// local peer is always this server
// remote peers are N+1 servers

type DnsProxy struct {
    // local DNS listener
    //Local net.PacketConn
    Listener net.PacketConn

    // stream of (new) connections to upstream DNS
    //Remote <-chan net.Conn
    upstreamConn <-chan net.Conn

    // stream of (empty) packets
    emptyPacket <-chan []byte

    // handlers
    // Question handler accepts *Pskel (question) as argument and may return *Pskel (its own answer).
    // If return is nil question will be passed up to the default upstream to be answered.
    // Setting answerHandler while returning answer from questionHandler will have no effect on the answer.
    questionHandler func(*Pskel) *Pskel // question
    answerHandler func(*Pskel)          // answer
}

func NewDnsProxy(remote ...string) (*DnsProxy, error) {
    var dx DnsProxy

    conn, err := net.ListenPacket(network, fmtDnsNetPoint(local)[0])
    if err != nil {
        return &dx, err
    }

    if len(remote) == 0 {
        remote = fmtDnsNetPoint(remote1, remote2)
    }

    dx.Listener = conn
    dx.upstreamConn = upstreamFactory(make(chan net.Conn, factoryQsize), remote)
    dx.emptyPacket = packetFactory(make(chan []byte, factoryQsize))

    return &dx, nil
}

func (dx *DnsProxy) QuestionHandler(h func(*Pskel) *Pskel) { dx.questionHandler = h }
func (dx *DnsProxy) AnswerHandler(h func(*Pskel)) { dx.answerHandler = h }

/*

func (dx *DnsProxy) proxy(query *Query) {
    // handle query here
    var wg sync.WaitGroup
    wg.Add(1)

    go func(wg *sync.WaitGroup) {
        defer wg.Done()
        dx.handleQuery(query)
    }(&wg)

    upstream := <-dx.Remote
    defer upstream.Close()

    wg.Wait()

    // Upstream / Remote
    // write
    _, err := upstream.Write(query.bytes)
    if err != nil {
        panic(err)
    }

    // receive
    p := <-dx.Packet
    _, err = upstream.Read(p)
    if err != nil {
        panic(err)
    }

    // handle answer here
    answer := NewAnswer(p)
    dx.handleAnswer(query, answer)

    // Downstream / Local
    // write
    _, err = dx.Local.WriteTo(answer.bytes, query.conn)
    if err != nil {
        panic(err)
    }
}
*/

func (dx *DnsProxy) proxy_new(query []byte, client net.Addr) {
    qskel, err := NewPacketSkeleton(query)
    if err != nil {
        panic(err)
    }
    //fmt.Printf("Q: %+v\n", qskel)

    var askel *Pskel
    if dx.questionHandler != nil {
        askel = dx.questionHandler(qskel)
    }

    if askel == nil {
        upstream := <-dx.upstreamConn
        defer upstream.Close()

        // Upstream write question
        _, err = upstream.Write(query) // TODO will this be from query/skell, possibly after mod?
        if err != nil {
            panic(err)
        }

        // Upstream receive answer
        p := <-dx.emptyPacket
        _, err = upstream.Read(p)
        if err != nil {
            panic(err)
        }

        askel, err = NewPacketSkeleton(p)
        if err != nil {
            panic(err)
        }
        //fmt.Printf("A: %+v\n", askel)

        if dx.answerHandler != nil {
            dx.answerHandler(askel)
        }
    }

    // Downstream write (back) answer
    _, err = dx.Listener.WriteTo(askel.Bytes(), client) // TODO will this be from answer/skell, possibly after mod?
    if err != nil {
        panic(err)
    }
}

func (dx *DnsProxy) Accept() {
    for {
        // query receiver
        query := <-dx.emptyPacket
        _, addr, err := dx.Listener.ReadFrom(query) // blocking
        if err != nil {
            // TODO log error here and move on?
            panic(err)
        }

        // offload to free the receiver
        go dx.proxy_new(query, addr)
    }
}

func packetFactory(ch chan []byte) chan []byte {
    go func() {
        for {
            p := make([]byte, 512)
            ch <- p
        }
    }()

    return ch
}

func upstreamFactory(ch chan net.Conn, remote []string) chan net.Conn {
    go func() {
        errMax := 3
        errCount := 0
        for count:=0;; count++ {
            pos := count % len(remote)
            if pos == 0 {
                count = 0
            }

            conn, err := net.Dial(network, remote[pos])
            // catch network issues
            // is this good enough?
            if err == nil {
                errCount = 0
            } else {
                // TODO log this
                // TODO do this by upstream server and take out non-functional
                errCount++
                if errCount >= errMax {
                    panic(err)
                }

                continue
            }

            ch <- conn
        }
    }()

    return ch
}

func fmtDnsNetPoint(s ...string) []string {
    var dnp []string
    for _, val := range s {
        dnp = append(dnp, fmt.Sprintf("%s:%d", val, port))
    }

    return dnp
}
