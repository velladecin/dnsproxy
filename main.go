package main

import (
    "fmt"
)

func main() {
    dx, err := NewDnsProxy()
    if err != nil {
        panic(err)
    }

    dx.QuestionHandler(func(q *Pskel) *Pskel {
        fmt.Printf("Q: %+v\n", q)

        if q.Question() == "cnn.com" {
            fmt.Println("Setting to answer")
            q.SetAnswer()
            q.SetRaTrue()
            q.SetRcodeNxdomain()
            return q
        }

        return nil
    })

    dx.AnswerHandler(func(a *Pskel) {
        fmt.Printf("A: %+v\n", a)
    })

    fmt.Printf("%+v\n", dx)
    dx.Accept()
}
