package main

import (
    "fmt"
    "net/http"
    "context"
    "time"
)

const (
    CHECK_FREQUENCY = 20 // seconds
)

type result struct {
    rr *rr
    err string
    rcode int
}
type Check interface {
    Name() string
    Net() string
    Port() int
    Run(*rrset)
}
type HttpCheck struct {
    name, net string
    port int
    results chan result
}
func (hc HttpCheck) Name() string { return hc.name }
func (hc HttpCheck) Net() string { return hc.net }
func (hc HttpCheck) Port() int { return hc.port }
func (hc HttpCheck) Run(rs *rrset) {
    for {
        for _, rec := range rs.rrs {
            res := result{rr: rec}
            if rec.typ != 1 {
                go func() {hc.results <- res}()
                continue
            }

            go func(r *rr) {
                ctx, timeout := context.WithTimeout(context.Background(), 3*time.Second)
                defer timeout()

                req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s://%s", hc.name, r.l2), nil)
                if err != nil {
                    res.err = err.Error()
                    res.rcode = 500
                    hc.results <- res
                    return
                }
                resp, err := http.DefaultClient.Do(req)
                if err != nil {
                    res.err = err.Error()
                    res.rcode = 500
                    hc.results <- res
                    return
                }
                res.rcode = resp.StatusCode
                hc.results <- res
            }(rec)
        }

        active := make([]*rr, 0)
        for i:=0; i<len(rs.rrs); i++ {
            res := <-hc.results
            if res.rcode < 299 {
                active = append(active, res.rr)
                continue
            }
            // TODO log errors here?
        }
        rs.Lock()
        rs.rrsactive = active
        rs.Unlock()
        time.Sleep(CHECK_FREQUENCY*time.Second)
    }
}
func NewHttpsCheck() HttpCheck { return newHttpCheck(443) }
func NewHttpCheck() HttpCheck { return newHttpCheck(80) }
func newHttpCheck(port int) HttpCheck {
    name := "http"
    if port == 443 { name += "s" }
    return HttpCheck{name, "tcp", port, make(chan result)}
}
