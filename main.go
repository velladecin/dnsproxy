package main

import (
    "fmt"
    "net"
)

func main() {
    dx, err := NewDnsProxy()
    if err != nil {
        panic(err)
    }

    /*
    r1 := rrset{
        &rr{"google.com", "1.1.1.1", 1, 100},
        &rr{"google.com", "2.2.2.2", 1, 200},
        &rr{"google.com", "3.3.3.3", 1, 300}}
        */

    r1 := rrset{
        &rr{"google.com", "velladec.org", CNAME, 10},
        &rr{"velladec.org", "velladec.in", CNAME, 20},
        &rr{"velladec.in", "1.1.1.1", A, 20},
        &rr{"velladec.in", "2.2.2.2", A, 30}}
    r2 := rrset{
        &rr{l1: "decin.cz", l2: "100.100.100.100", typ: A, ttl: 100},
        &rr{l1: "decin.cz", l2: "200.200.200.200", typ: A, ttl: 200}}
    r3 := rrset{
        //&rr{"incoming.telemetry.mozilla.org", "telemetry-incoming.r53-2.services.mozilla.com", 5, 10},
        &rr{"bla.com", "telemetry-incoming.r53-2.services.mozilla.com", 5, 70},
        &rr{"telemetry-incoming.r53-2.services.mozilla.com", "prod.ingestion-edge.prod.dataops.mozgcp.net", 5, 80},
        &rr{"prod.ingestion-edge.prod.dataops.mozgcp.net", "34.120.208.123", 1, 90}}
    //r4 := rrset{
    //    &rr{"velladec.in", "in", "nxdomain"}}
    //fmt.Printf("r4: %+v\n", r4)

    dx.Handler(func (query Packet, client net.Addr) *Packet {
        fmt.Printf("client: ===> %+v\n", client)
        fmt.Printf("client: ===> %+v\n", client.Network())
        fmt.Printf("client: ===> %+v\n", client.String())
        var answer *Packet
        switch query.questionString() {
        case "google.com":  answer = query.getAnswer(r1)
        case "decin.cz":    answer = query.getAuthoritativeAnswer(r2)
        //case "incoming.telemetry.mozilla.org": answer = query.getAuthoritativeAnswer(r3)
        case "bla.com": answer = query.getAuthoritativeAnswer(r3)
        }
        return answer
    })

/*
    dx.Handler(func(q *Pskel) *Pskel {
        fmt.Printf("Q: %+v\n", q)
        fmt.Printf("q: %s\n", q.Question())
        fmt.Printf("h1: %+v\n", q.Headers())
        q.SetAnswer()
        fmt.Printf("h2: %+v\n", q.Headers())
        return q
    })
    */

    fmt.Printf("-- %+v\n", dx)
    dx.Accept()
}
