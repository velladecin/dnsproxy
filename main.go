package main

import (
    "fmt"
)

func main() {
    dx, err := NewDnsProxy()
    if err != nil {
        panic(err)
    }

    dx.QueryHandler(func(r *Query){
        fmt.Println(">>>>>>>>>>> Query")
        id := r.Id()
        fmt.Printf(">>>>> id: %b\n", id)

        flags := r.Flags()
        fmt.Printf(">>>>> fl: %b\n", flags)

        qd := r.Qd()
        fmt.Printf(">>>>> qd: %b\n", qd)

        an := r.An()
        fmt.Printf(">>>>> an: %b\n", an)

        ns := r.Ns()
        fmt.Printf(">>>>> ns: %b\n", ns)

        ar := r.Ar()
        fmt.Printf(">>>>> ar: %b\n", ar)
    })

    dx.AnswerHandler(func(q *Query, a *Answer){
        fmt.Println(">>>>>>>>>>> AnswerHandler")
        qid := q.Id()
        aid := a.Id()
        fmt.Printf(">>>>> id: %b <> %b\n", qid, aid)

        qflags := q.Flags()
        aflags := a.Flags()
        fmt.Printf(">>>>> fl: %b <> %b\n", qflags, aflags)

        qd := a.Qd()
        fmt.Printf(">>>>> qd: %b\n", qd)

        an := a.An()
        fmt.Printf(">>>>> an: %b\n", an)

        ns := a.Ns()
        fmt.Printf(">>>>> ns: %b\n", ns)

        ar := a.Ar()
        fmt.Printf(">>>>> ar: %b\n", ar)
    })

    dx.Accept()
}
