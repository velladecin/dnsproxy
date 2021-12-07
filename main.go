package main

import (
    "fmt"
)

func main() {
    dx, err := NewDnsProxy()
    if err != nil {
        panic(err)
    }

    dx.QueryHandler(func(q *Query){
        //q.Label()
        skel, err := PacketAutopsy(q)
        if err != nil {
            panic(err.Error())
        }
        fmt.Printf("Q: %+v\n\n", skel)
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
        //q.Label() // <<-- this works
        fmt.Println()
        fmt.Println()
        skel, err := PacketAutopsy(a)
        if err != nil {
            panic(err.Error())
        }
        fmt.Printf("A: %+v\n", skel)

        skel.SetNoErr()
        fmt.Printf("%b\n", skel.header)

        skel.SetFormErr()
        fmt.Printf("%b\n", skel.header)

        skel.SetServFail()
        fmt.Printf("%b\n", skel.header)

        skel.SetNotImp()
        fmt.Printf("%b\n", skel.header)

        skel.SetNxDomain()
        fmt.Printf("%b\n", skel.header)

        /*
        if qlabel[0] == "incoming.telemetry.mozilla.org" || qlabel[0] == "google.com" || qlabel[0] == "kdk01dkd.com" {
            //fmt.Println(">>>>>>>>>>> AnswerHandler")
            a.Label()
            //o := a.Label()
            //fmt.Println(len(o))
            //fmt.Printf("%+v\n", o)
        }
        */
    })

    dx.Accept()
}
