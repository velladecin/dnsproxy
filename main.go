package main

import (
    "fmt"
   _ "net"
    _"strings"
_    "regexp"
)

func main() {
    dx, err := NewDnsProxy()
    if err != nil {
        panic(err)
    }

    dx.cache = NewCache(
        NewRRsetChecked(NewHttpCheck(), "google.com", "1.1.1.1", "142.250.70.206", "162.243.27.213"),
        NewRRset("neco.cz", "second.org", "third.cz", "1.2.3.4"),
        NewNxdomain("decin.cz"),
        NewNxdomain("karel.cz", "ns1.nano.cz", "dns.nano.cz"))
        //NewRRsetChecked(NewHttpCheck(), "google.com", "1.1.1.1", "142.250.70.206", "162.243.27.213"))

    //google := NewRRset("google.com", "1.1.1.1", "2.2.2.2", "3.3.3.3").GetBytes()
    //neco := NewRRset("neco.cz", "second.org", "third.cz", "1.2.3.4").GetBytes()

    /*
    google := NewRRset([]string{"google.com", "velladec.org"},
                   []string{"velladec.org", "velladec.in"},
                   []string{"velladec.in", "1.1.1.1"},
                   []string{"velladec.in", "2.2.2.2"}).GetBytes()

    neco := NewRRset([]string{"neco.cz", "second.org"},
                     []string{"second.org", "third.cz"},
                     []string{"third.cz", "1.2.3.4"}).GetBytes()
    */

    //decin := NewNxdomain("decin.cz").GetBytes()
    //karel := NewNxdomain("karel.cz", "ns1.nano.cz", "dns.nano.cz").GetBytes()

    //rgx := regexp.MustCompile(`(^|\.)test\.cz$`)
    //yahoo := NewRRset("yahoo.com", "100.100.100.100").GetBytes()
    //dx.Handler(func (query []byte, client net.Addr)(answer []byte) {
        /*
        fmt.Printf("CLIENT: %+v\n", client)
        netdets := strings.Split(client.String(), ":")
        switch netdets[0] {
        case "127.0.0.1":
            fmt.Println("==== FROM LOCALHOST")
            if QueryStr(query) == "yahoo.com" {
                answer = yahoo
            }
        case "192.168.1.104":
            fmt.Println(">>>>>> FROM PUBLIC IFACE")
        }
        */

        /*
        question := QueryStr(query)

        switch question {
        case "google.com":  answer = google
        case "neco.cz":     answer = neco
        case "decin.cz":    answer = decin
        case "karel.cz":    answer = karel
        }

        if len(answer) > 0 {
            return answer
        }

        if rgx.MatchString(question) {
            return NewNxdomain(question).GetBytes()
        }

        fmt.Printf("*************** %+v\n", answer)
        */
    //    return answer
    //})

    /*
    r3 := rrset{
        //&rr{"incoming.telemetry.mozilla.org", "telemetry-incoming.r53-2.services.mozilla.com", 5, 10},
        &rr{"bla.com", "telemetry-incoming.r53-2.services.mozilla.com", 5, 70},
        &rr{"telemetry-incoming.r53-2.services.mozilla.com", "prod.ingestion-edge.prod.dataops.mozgcp.net", 5, 80},
        &rr{"prod.ingestion-edge.prod.dataops.mozgcp.net", "34.120.208.123", 1, 90}}
        */

    fmt.Printf("-- %+v\n", dx)
    dx.Accept()
}
