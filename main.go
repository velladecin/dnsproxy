package main

import (
_    "fmt"
)

func main() {
    dx, err := NewDnsProxy()
    if err != nil {
        panic(err)
    }

    dx.QueryHandler(func(q *Query){
        q.Label()
    /*
        fmt.Println(">>>>>>>>>>> Query")
        id := q.Id()
        fmt.Printf(">>>>> id: %b\n", id)

        flags := q.Flags()
        fmt.Printf(">>>>> fl: %b\n", flags)

        qd := q.Qd()
        fmt.Printf(">>>>> qd: %b\n", qd)

        an := q.An()
        fmt.Printf(">>>>> an: %b\n", an)

        ns := q.Ns()
        fmt.Printf(">>>>> ns: %b\n", ns)

        ar := q.Ar()
        fmt.Printf(">>>>> ar: %b\n", ar)

        l := Label(q.bytes[12:])
        fmt.Printf("%+v\n", l)
        */
    })

    dx.AnswerHandler(func(q *Query, a *Answer){
        //q.Label()
        a.Label()

        /*
        if qlabel[0] == "incoming.telemetry.mozilla.org" || qlabel[0] == "google.com" || qlabel[0] == "kdk01dkd.com" {
            //fmt.Println(">>>>>>>>>>> AnswerHandler")
            //qid := q.Id()
            //aid := a.Id()
            //fmt.Printf(">>>>> id: %b <> %b\n", qid, aid)

            //qflags := q.Flags()
            //aflags := a.Flags()
            //fmt.Printf(">>>>> fl: %b <> %b\n", qflags, aflags)

            qqd := q.Qd()
            aqd := a.Qd()
            fmt.Printf(">>>>> qd: %b <> %b\n", qqd, aqd)

            qan := q.An()
            aan := a.An()
            fmt.Printf(">>>>> an: %b <> %b\n", qan, aan)

            qns := q.Ns()
            ans := a.Ns()
            fmt.Printf(">>>>> ns: %b <> %b\n", qns, ans)

            qar := q.Ar()
            aar := a.Ar()
            fmt.Printf(">>>>> ar: %b <> %b\n", qar, aar)

            fmt.Printf("S: %s\n", qlabel[0])
            fmt.Println(">>>> QQQQ")
            fmt.Printf("D: %d\n", q.bytes[12:])
            //fmt.Printf("B: %b\n", q.bytes[12:])
            fmt.Println(">>>> AAAA")
            fmt.Printf("D: %d\n", a.bytes[12:])
            fmt.Printf("B: %b\n", a.bytes[12:])

            a.Label()
            //o := a.Label()
            //fmt.Println(len(o))
            //fmt.Printf("%+v\n", o)
        }
        */
    })

    dx.Accept()
}
