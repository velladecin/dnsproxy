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
