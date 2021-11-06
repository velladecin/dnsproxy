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
    requestHandler func(*Request)
    responseHandler func(*Response)
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

// RequestHandler is defined by user
// handleRequest is run by proxy request receiver
func (dx *DnsProxy) RequestHandler(fn func(*Request)) { dx.requestHandler = fn }
func (dx *DnsProxy) handleRequest(r *Request) {
    if dx.requestHandler == nil {
        return
    }

    dx.requestHandler(r)
}

// ResponseHandler is defined by user
// handleResponse is run by proxy response receiver
func (dx *DnsProxy) ResponseHandler(fn func(*Response)) { dx.responseHandler = fn }
func (dx *DnsProxy) handleResponse(r *Response) {
    if dx.responseHandler == nil {
        return
    }

    dx.responseHandler(r)
}

func (dx *DnsProxy) proxy(req *Request) {
    // handle request here
    var wg sync.WaitGroup
    wg.Add(1)

    go func(wg *sync.WaitGroup) {
        defer wg.Done()
        dx.handleRequest(req)
    }(&wg)

    upstream := <-dx.Remote
    defer upstream.Close()

    wg.Wait()

    // Upstream / Remote
    // write
    _, err := upstream.Write(req.bytes)
    if err != nil {
        panic(err)
    }

    // receive
    p := <-dx.Packet
    _, err = upstream.Read(p)
    if err != nil {
        panic(err)
    }

    // handle response here
    resp := NewResponse(p)
    dx.handleResponse(resp)

    // Downstream / Local
    // write
    _, err = dx.Local.WriteTo(resp.bytes, req.conn)
    if err != nil {
        panic(err)
    }
}

func (dx *DnsProxy) Accept() {
    for {
        // receiver
        request := <-dx.Packet
        _, addr, err := dx.Local.ReadFrom(request) // blocking
        if err != nil {
            // TODO log error here and move on?
            panic(err)
        }

        // offload to not block the receiver
        go dx.proxy(NewRequest(request, addr))
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
