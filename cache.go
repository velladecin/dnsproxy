package main

import (
    _"fmt"
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
    }
}
func (c *cache) UpdateRdata(rd Rdata) {
    c.item[rd.QueryStr()] = r.GetBytes()
}
func (c *cache) Bytes(s string) []byte {
    if b, ok := c.item[s]; ok {
        return b
    }
    return nil
}
