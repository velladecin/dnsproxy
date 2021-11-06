package main

import (
    "fmt"
)

func main() {
    dx, err := NewDnsProxy()
    if err != nil {
        panic(err)
    }

    dx.RequestHandler(func(r *Request){
        id := r.Id()
        fmt.Printf(">>>>> %b\n", id)
    })

    dx.Accept()
}
