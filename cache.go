package main

import (
    "fmt"
)

type cache struct {
    item map[string][]byte
}

func NewCache(rd ...Rdata) *cache {
    c := &cache{make(map[string][]byte)}
    c.AddRdata(rd...)
    return c
}
func (c *cache) AddRdata(rd ...Rdata) {
    for _, r := range rd {
        c.UpdateRdata(r)

        if n := r.Notify(); n != nil {
            // listen for changes
            go func(crd Rdata, ch chan bool) {
                for {
                    change := <- ch
                    fmt.Printf(">>>>> zcache - getting change notify: %s\n", crd.QueryStr())
                    if change {
                        fmt.Println(">>>>> zcache - UpdateRdata(r): %s\n", crd.QueryStr())
                        c.UpdateRdata(crd)
                    }
                }
            }(r, n)
        }
    }
    fmt.Printf("%+v\n", c.item)
}
func (c *cache) UpdateRdata(rd Rdata) {
    c.item[rd.QueryStr()] = rd.GetBytes()
}
func (c *cache) Bytes(s string) []byte {
    if b, ok := c.item[s]; ok {
        return b
    }
    return nil
}
