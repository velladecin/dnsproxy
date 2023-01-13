package main

import (
    "fmt"
    "net"
    "regexp"
    "strings"
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

    // proxy cache
    cache *cache

    // DNS packet handler
    handler func(query []byte, client net.Addr) (answer []byte)
}

func NewDnsProxy(upstream ...string) (*DnsProxy, error) {
    var dx DnsProxy
    conn, err := net.ListenPacket(NET, fmt.Sprintf(":%d", PORT))
    if err != nil {
        return &dx, err
    }

    if len(upstream) == 0 {
        upstream = []string{US1, US2}
    }

    dx.Listener = conn
    dx.upstreamConn = upstreamFactory(fmtDnsNetPoint(upstream...))
    dx.emptyPacket = packetFactory()
    // initialize with handler that does nothing
    // to force initialization of the struct field
    dx.handler = func (query []byte, client net.Addr) []byte {
        if dx.cache != nil {
            return dx.cache.Bytes(QueryStr(query))
        }
        return nil
    }

    return &dx, nil
}

func (dx *DnsProxy) Handler(h func(query []byte, client net.Addr)(answer []byte)) {
    dx.handler = h
}
func (dx *DnsProxy) proxy_new(query []byte, client net.Addr) {
    fmt.Printf("query: %+v\n", query)

    answer := dx.handler(query, client)

    switch len(answer) {
    case 0:
        // either handler didn't match any conditions
        // or handler has not been defined, go to upstream next
        upstream := <-dx.upstreamConn
        defer upstream.Close()

        // query upstream
        _, err := upstream.Write(query)
        if err != nil {
            panic(err)
        }
        // receive answer
        answer = <-dx.emptyPacket
        _, err = upstream.Read(answer)
        if err != nil {
            panic(err)
        }

        fmt.Printf("upstream: %+v\n", answer)
    default:
        // handler produced smth
        // update packet id
        answer[0] = query[0]
        answer[1] = query[1]
        fmt.Printf("proxy::answer %+v\n", answer)
    }

    fmt.Printf("fansw: %+v\n", answer)

    // answer back to client
    _, err := dx.Listener.WriteTo(answer, client)
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

func packetFactory() chan []byte {
    ch := make(chan []byte, FACTORY_Q_SIZE)
    go func() {
        for {
            p := make([]byte, 512)
            ch <- p
        }
    }()

    return ch
}

func upstreamFactory(remote []string) chan net.Conn {
    ch := make(chan net.Conn, FACTORY_Q_SIZE)
    go func() {
        errCount := 0
        for count:=0;; count++ {
            pos := count % len(remote)
            if pos == 0 {
                count = 0
            }

            conn, err := net.Dial(NET, remote[pos])
            if err != nil {
                // catch network issues
                // is this good enough?
                // TODO log this
                // TODO do this by upstream server and take out non-functional
                errCount++
                if errCount >= 3 {
                    panic(err)
                }
                continue
            }
            errCount = 0
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
        if parts := strings.Split(val, ":"); len(parts) == 1 {
            val = fmt.Sprintf("%s:%d", val, PORT)
        }
        dnp[i] = val
    }

    return dnp
}
