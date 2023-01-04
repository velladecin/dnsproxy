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
        */

    r1 := RRset{&Rdata{"google.com", "1.1.1.1", A, 100}}
    r1.CheckValid()
    p1 := r1.GetPacket()
    fmt.Printf("%+v\n", p1)

    r2 := RRset{&Rdata{"decin.cz", "100.100.100.100", A, 150},
                &Rdata{"decin.cz", "100.100.100.102", A, 140},
                &Rdata{"decin.cz", "100.100.100.101", A, 130}}
    r2.CheckValid()
    p2 := r2.GetPacket()

    //r3 := RRset{&Nxdomain{l1:"google.com", mname: "ns1.google.com", rname: "vella.vella.org"}}
    r3 := RRset{&Nxdomain{l1:"karel.cz", mname: "ns1.nano.cz", rname: "dns.nano.cz"}}
    r3.CheckValid()
    p3 := r3.GetPacket()

    r4 := RRset{&Rdata{"neco.cz", "second.org", CNAME, 10},
                &Rdata{"second.org", "third.cz", CNAME, 20},
                &Rdata{"third.cz", "1.2.3.4", A, 30}}
    r4.CheckValid()
    p4 := r4.GetPacket()

    dx.Handler(func (query Packet, client net.Addr) *Packet {
        fmt.Printf("client: ===> %+v\n", client)
        fmt.Printf("client: ===> %+v\n", client.Network())
        fmt.Printf("client: ===> %+v\n", client.String())
        fmt.Printf("query: ====> %+v\n", query)

        var answer *Packet
        switch query.Question() {
        //case "google.com":  answer = p1
        case "google.com":  answer = p1
        case "decin.cz":    answer = p2
        case "karel.cz":    answer = p3
        case "neco.cz":     answer = p4
        }
        if answer != nil {
            answer.IngestPacketId(query.bytes[:IDLEN])
        }
        return answer

        /*
        var answer *Packet
        switch query.questionString() {
        case "google.com":  answer = query.getAnswer(r1)
        case "decin.cz":    answer = query.getAuthoritativeAnswer(r2)
        //case "incoming.telemetry.mozilla.org": answer = query.getAuthoritativeAnswer(r3)
        case "bla.com": answer = query.getAuthoritativeAnswer(r3)
        }
        return answer
        */
    })

    fmt.Printf("-- %+v\n", dx)
    dx.Accept()
}
