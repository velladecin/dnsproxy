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

    // handlers
    // Question handler accepts *Pskel (question) as argument and may return *Pskel (its own answer).
    // If return is nil question will be passed up to the default upstream to be answered.
    // Setting answerHandler while returning answer from questionHandler will have no effect on the answer.
    questionHandler func(*Pskel) *Pskel // question
    answerHandler func(*Pskel)          // answer
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

func (dx *DnsProxy) QuestionHandler(h func(*Pskel) *Pskel) { dx.questionHandler = h }
func (dx *DnsProxy) AnswerHandler(h func(*Pskel)) { dx.answerHandler = h }

func (dx *DnsProxy) proxy_new(query []byte, client net.Addr) {
    qskel, err := NewPacketSkeleton(query)
    if err != nil {
        panic(err)
    }

    var askel *Pskel
    if dx.questionHandler != nil {
        askel = dx.questionHandler(qskel)
    }

    if askel == nil {
        upstream := <-dx.upstreamConn
        defer upstream.Close()

        // Upstream write question
        _, err = upstream.Write(qskel.Bytes())
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

        if dx.answerHandler != nil {
            dx.answerHandler(askel)
        }
    }

    // Downstream write (back) answer
    _, err = dx.Listener.WriteTo(askel.Bytes(), client)
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
