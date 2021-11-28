package main

import (
    "fmt"
    "net"
    "sync"
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
    Local net.PacketConn

    // stream of (new) connections to upstream DNS
    Remote <-chan net.Conn

    // stream of (empty) packets
    Packet <-chan []byte

    // user defined handlers
    queryHandler func(*Query)
    answerHandler func(*Query, *Answer) // TODO this is already modified Query
                                        // will we also need the original?
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

    dx.Local = conn
    dx.Remote = upstreamFactory(make(chan net.Conn, factoryQsize), remote)
    dx.Packet = packetFactory(make(chan []byte, factoryQsize))

    return &dx, nil
}

// QueryHandler is defined by user
// handleQuery is run by proxy query receiver
func (dx *DnsProxy) QueryHandler(fn func(*Query)) { dx.queryHandler = fn }
func (dx *DnsProxy) handleQuery(r *Query) {
    if dx.queryHandler == nil {
        return
    }

    dx.queryHandler(r)
}

// AnswerHandler is defined by user
// handleAnswer is run by proxy answer receiver
func (dx *DnsProxy) AnswerHandler(fn func(*Query, *Answer)) { dx.answerHandler = fn }
func (dx *DnsProxy) handleAnswer(q *Query, a *Answer) {
    if dx.answerHandler == nil {
        return
    }

    dx.answerHandler(q, a)
}

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

func (dx *DnsProxy) Accept() {
    for {
        // question receiver
        query := <-dx.Packet
        _, addr, err := dx.Local.ReadFrom(query) // blocking
        if err != nil {
            // TODO log error here and move on?
            panic(err)
        }

        // offload to free the receiver
        go dx.proxy(NewQuery(query, addr))
    }
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
