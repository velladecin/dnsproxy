package main

import (
    "flag"
    _"fmt"
)

var config string
//var debugx, stdout bool
var stdout bool

func init() {
    flag.StringVar(&config, "config", "/etc/dns-proxy/dns-proxy.cfg", "DNS proxy config file")
    //flag.BoolVar(&debugx, "debug", false, "Show debug messages")
    flag.BoolVar(&stdout, "stdout", false, "Print to STDOUT")
    flag.Parse()
}

func main() {
    //s := NewServer(config, debugx, stdout)
    s := NewServer(config, stdout)
    //fmt.Printf("%+v\n", s)
    s.Run()
}
