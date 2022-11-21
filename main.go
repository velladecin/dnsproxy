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

    r1 := rrset{
        &rr{"google.com", "1.1.1.1", 1},
        &rr{"google.com", "2.2.2.2", 1}}
    fmt.Printf("%+v\n", r1)

    dx.Handler(func (p Packet) *Packet {
        fmt.Printf("%+v\n", p)
        var answer Packet
        if p.Question() == "google.com" {
            h := p.GetHeaders()
            h.SetAnswer()
            fmt.Printf("headers: %+v\n", h)
        }

        return &answer
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
