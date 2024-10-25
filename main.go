package main

import (
    "flag"
    _"fmt"
)

var config string
var stdout bool

func init() {
    flag.StringVar(&config, "config", "/etc/dns-proxy/dns-proxy.cfg", "DNS proxy config file")
    flag.BoolVar(&stdout, "stdout", false, "Print to STDOUT")
    flag.Parse()
}

func main() {
    s := NewServer(config, stdout)
    //fmt.Printf("%+v\n", s)
    s.Run()
}
