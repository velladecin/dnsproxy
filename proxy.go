package main

import (
    "fmt"
    "net"
_    "sync"
    "regexp"
)

const (
    // net
    network = "udp4"
    port = 53

    // default upstream
    upstream1 = "8.8.8.8"
    upstream2 = "8.8.4.4"

    // pre-prepared queue size
    // for upstream/remote connections and empty packets
    factoryQsize = 3
)

type DnsProxy struct {
    // local DNS listener
    //Local net.PacketConn
    Listener net.PacketConn

    // stream of (new) connections to upstream DNS
    //Remote <-chan net.Conn
    upstreamConn <-chan net.Conn

    // stream of (empty) packets
    emptyPacket <-chan []byte

    // DNS packet handler
    handler func(question Packet, client net.Addr) (answer *Packet)
}

func NewDnsProxy(upstream ...string) (*DnsProxy, error) {
    var dx DnsProxy

    // TODO listen on public IP(s?)
    conn, err := net.ListenPacket(network, fmtDnsNetPoint("127.0.0.1")[0])
    if err != nil {
        return &dx, err
    }

    if len(upstream) == 0 {
        upstream = []string{upstream1, upstream2}
    }

    dx.Listener = conn
    dx.upstreamConn = upstreamFactory(fmtDnsNetPoint(upstream...))
    dx.emptyPacket = packetFactory()

    return &dx, nil
}

func (dx *DnsProxy) Handler(h func(question Packet, client net.Addr) *Packet) { dx.handler = h }
func (dx *DnsProxy) proxy_new(question Packet, client net.Addr) {
    var answer *Packet
    if dx.handler != nil {
        answer = dx.handler(question, client)
        fmt.Printf("proxy::answer %+v\n", answer)
    }
    // handler() can return nil when no conditions are met there
    // check answer and ask upstream if we have none yet (answer)
    if answer == nil {
        upstream := <-dx.upstreamConn
        defer upstream.Close()

        // Upstream write question
        _, err := upstream.Write(question)
        if err != nil {
            panic(err)
        }

        // Upstream receive answer
        p := <-dx.emptyPacket
        _, err = upstream.Read(p)
        if err != nil {
            panic(err)
        }

        answer = (*Packet)(&p)
    }

    fmt.Printf("fansw: %+v\n", answer)
    // Downstream write (back) answer
    _, err := dx.Listener.WriteTo(*answer, client)
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
        go dx.proxy_new(Packet(query), addr)
    }
}

func packetFactory() chan []byte {
    ch := make(chan []byte, factoryQsize)
    go func() {
        for {
            p := make([]byte, 512)
            ch <- p
        }
    }()

    return ch
}

func upstreamFactory(remote []string) chan net.Conn {
    ch := make(chan net.Conn, factoryQsize)
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
    dnp := make([]string, len(s))
    for i, val := range s {
        if ok, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+(\:\d+)?$`, val); !ok {
            panic("Invalid net definition: " + val)
        }

        if ok, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+\:\d+$`, val); !ok {
            val = fmt.Sprintf("%s:%d", val, port)
        }

        dnp[i] = val
    }

    return dnp

    /* ORIGINAL
    var dnp []string
    for _, val := range s {
        if ok, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+(\:\d+)?$`, val); !ok {
            panic("Invalid net definition: " + val)
        }

        if ok, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+\:\d+$`, val); !ok {
            val = fmt.Sprintf("%s:%d", val, port)
        }

        dnp = append(dnp, val)
    }

    return dnp
    */
}
