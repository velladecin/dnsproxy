package main

import (
    "fmt"
_    "regexp"
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
    /*
    r1 := rrset{
        &rr{"google.com", "telemetry-incoming.r53-2.services.mozilla.com", 5, 10},
        &rr{"telemetry-incoming.r53-2.services.mozilla.com", "prod.ingestion-edge.prod.dataops.mozgcp.net", 5, 20},
        &rr{"prod.ingestion-edge.prod.dataops.mozgcp.net", "34.120.208.123", 1, 30}}
        */
    r1 := rrset{
        &rr{"google.com", "velladec.org", 5, 10},
        &rr{"velladec.org", "velladec.in", 5, 20},
        &rr{"velladec.in", "1.1.1.1", 1, 20},
        &rr{"velladec.in", "2.2.2.2", 1, 30}}
    fmt.Printf("r1: %+v\n", r1)

    dx.Handler(func (question Packet) *Packet {
        fmt.Printf("question: %+v\n", question)

        var answer *Packet
        if question.Question() == "google.com" {
            h := question.GetHeaders()
            h.SetAnswer()
            h.setANcount(len(r1))
            h.setAA()
            h.setRA(true)

            body := run(r1)
            b := make([]byte, len(h) + len(body))

            /*
            i := 0
            for ; i<len(h); i++ {
                b[i] = h[i]
            }
            for j:=0; j<len(body); j++ {
                b[i+j] = body[j]
            }
            */

            for x:=0; x<(len(h)+len(body)); x++ {
                if x >= len(h) {
                    b[x] = body[x-len(h)]
                } else {
                    b[x] = h[x]
                }
            }

            answer = (*Packet)(&b)
        }

        fmt.Printf("atype: %T\n", answer)
        fmt.Printf("ans: %+v\n", answer)
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

/*
    dx.QuestionHandler(func(q *Pskel) *Pskel {
        fmt.Printf("Q: %+v\n", q)

        fmt.Println(q.Question())

        if ok, _ := regexp.MatchString(`hxa-xxxxxx`, q.Question()); ok {
            q.SetAnswer()
            q.SetRaTrue()
            q.SetRcodeNoErr()
            q.SetAdFalse()
            q.SetRR(NewRr("bla.com", "192.168.1.104"))
            return q
        }


        if q.Question() == "cnnxx.com" {
            fmt.Println("Setting to answer")
            q.SetAnswer()
            q.SetRaTrue()
            //q.SetRcodeNxdomain()
            //q.SetRcodeNotImpl()
            q.SetRcodeNoErr()
            q.SetAdFalse()

            //rr := NewRr("cnn.com", "1.1.1.1", RrTtl(100), RrClass(CH), RrType(CNAME))
            rr := NewRr("cnn.com", "1.1.1.1") // , RrTtl(100), RrClass(CH), RrType(CNAME))
            q.SetRR(rr)

            fmt.Printf("%+v\n", q)
            //fmt.Printf("%b\n", q.header[3])
            return q
        }

        if q.Question() == "decin.cz" {
            rs := NewRrSet(
                NewRr("first", "second", RrType(CNAME), RrTtl(10)),
                NewRr("second", "third", RrType(CNAME), RrTtl(20)),
                NewRr("third", "3.3.3.3", RrTtl(30)))

            fmt.Printf("%+v\n", rs)

            q.SetRR(rs)
            fmt.Printf("%+v\n", q)

            return q
        }

        return nil
    })

    dx.AnswerHandler(func(a *Pskel) {
        fmt.Printf("A: %+v\n", a)
    })
    */

    fmt.Printf("-- %+v\n", dx)
    dx.Accept()
}
