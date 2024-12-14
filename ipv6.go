package main

import (
    "fmt"
    "regexp"
    "strings"
    "encoding/hex"
)

func ipv6StoB(ip string) []byte {
    ip = strings.ToLower(ip)

    // no :, must be full length
    if ok, _ := regexp.MatchString(`\:`, ip); !ok {
        // must be 32 length
        if ok, _ := regexp.MatchString(`^[a-f0-9]{32}$`, ip); !ok {
            panic("bad IPv6: " + ip)
        }

        b, err := hex.DecodeString(ip)
        if err != nil {
            panic(err)
        }

        return b
    }

    // ipv6 = 8 nibbles, separated by :[:] (up to 7 of them)
    // 8x4 = 32 + 7 = 39
    if ok, _ := regexp.MatchString(`^[a-f0-9\:]{3,39}$`, ip); !ok {
        panic("bad IPv6: " + ip)
    }

    nibble := strings.Split(ip, ":")

    if nibble[0] == "" {
        // ::1
        // this gives extra empty element at the front of the slice
        // which does not happen with first nibble populated
        nibble = nibble[1:]
    }

    if nibble[len(nibble)-1] == "" {
        // 1::
        // this gives extra empty element at the end of the slice
        // which does not happen with last nibble populated
        nibble = nibble[:len(nibble)-1]
    }

    for i, n := range nibble {
        // a::b
        if n == "" {
            // (7-i)-len(nibble[i+1:]) is the top index starting from zero (0..index)
            // 0-2 is length 3, hence +1 at the end
            missing_nibbles := make([]string, (7-i)-len(nibble[i+1:])+1)
            for j:=0; j<len(missing_nibbles); j++ {
                missing_nibbles[j] = "0"
            }

            nibble = append(nibble[:i], append(missing_nibbles, nibble[i+1:]...)...)
            break
        }
    }

    var ip6 string
    for _, v := range nibble {
        if len(v) != 4 {
            v = fmt.Sprintf("%04s", v)
        }

        ip6 += v
    }

    b, err := hex.DecodeString(ip6)
    if err != nil {
        panic(err)
    }

    return b
}

func ipv6BtoS(ip []byte, full bool) string {
    // 1 nibble = 16 bits = 2 bytes

    s := make([]string, 0)
    for i:=1; i<len(ip); i+=2 {
        if full {
            s = append(s, fmt.Sprintf("%02x%02x", ip[i-1], ip[i]))
            continue
        }

        if ip[i-1] == 0 && ip[i] == 0 {
            // nibble is 0

            // if first do 0 and move on
            if len(s) == 0 {
                s = append(s, "0")
                continue
            }

            // if not first check previous
            // to determine if :: is needed
            switch s[len(s)-1] {
            case "0":
                // previous is 0, replace with "" to create ::
                // see Join() below
                s[len(s)-1] = ""
            case "":
                // previous is "", :: continues (nothing to do)
            default:
                // append 0
                s = append(s, "0")
            }
        } else {
            // remove leading 0
            switch ip[i-1] {
            case 0:
                s = append(s, fmt.Sprintf("%x", ip[i]))
            default:
                s = append(s, fmt.Sprintf("%x%02x", ip[i-1], ip[i]))
            }
        }        
    }

    // single nibble populated
    // add another "" to create :: in Join below
    if len(s) == 2 {
        switch s[0] {
        case "":
            // ::1
            s = append([]string{""}, s...)
        default:
            // 1::
            s = append(s, "")
        }
    }

    return strings.Join(s, ":")
}
