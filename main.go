package main

import (
    "flag"
    "fmt"
)

var config string
var stdout bool

func init() {
    flag.StringVar(&config, "config", "/etc/dpx/dpx.cfg", "DNS proxy config file")
    flag.BoolVar(&stdout, "stdout", false, "Print to STDOUT")
    flag.Parse()
}

func main() {
    fmt.Println(config)
    fmt.Println(stdout)
    s := NewServer(config, stdout)
    //fmt.Printf("%+v\n", s)
    s.Run()
}
