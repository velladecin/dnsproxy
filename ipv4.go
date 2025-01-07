package main

import (
    "net"
    "regexp"
    "strings"
)

var rIp4 = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)

// called on sever start
// and so panic()
func localIface() []string {
    addr, err := net.InterfaceAddrs()
    if err != nil {
        panic(err)
    }

    var addrs []string

    // strip /subnet
    for _, a := range addr {
        s := strings.Split(a.String(), "/")

        if len(s) != 2 {
            panic("Unsupported interface: " + a.String())
        }

        addrs = append(addrs, s[0])
    }

    return addrs
}

func localIface4() []string {
    var i4 []string

    for _, i := range localIface() {
        if rIp4.MatchString(i) {
            i4 = append(i4, i)
        }
    }

    return i4
}

func isIpv4() bool {
    i4 := localIface4()
    return len(i4) > 0
}
