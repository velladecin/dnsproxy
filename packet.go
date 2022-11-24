package main

import (
    "fmt"
    "strings"
    "strconv"
    "regexp"
)

type Packet []byte

func (p Packet) getHeaders() []byte { return p[0:QUESTION_LABEL_START] }
func (p Packet) questionString() string {
    var q string
    l := int(p[QUESTION_LABEL_START])
    for i:=QUESTION_LABEL_START+1;; {
        q += string(p[i:i+l])

        i += l
        l = int(p[i])
        if l == 0 {
            break
        }

        i++
        q += "."
    }
    return q
}
func (p Packet) getAuthoritativeAnswer(rs rrset) *Packet {
    a := p.getAnswer(rs)
    a.setAA()
    a.unsetRA()
    return a
}
func (p Packet) getAnswer(rs rrset) *Packet {
    h := Packet(p.getHeaders())
    h.setAnswer()
    h.setANcount(len(rs))
    h.setRA()
    h.unsetAD()

    body := run(rs)
    b := make([]byte, len(h)+len(body))
    for x:=0; x<(len(h)+len(body)); x++ {
        if x < len(h) {
            b[x] = h[x]
            continue
        }
        b[x] = body[x-len(h)]
    }
    return (*Packet)(&b)
}


//
// Headers

// query (type) answer
func (p Packet) setAnswer() { p[Flags1] |= (1<<QR) }
// number or RRs in answer, assuming this fits in single byte
func (p Packet) setANcount(i int) {
    p[ANcount1] = byte(0)
    p[ANcount2] = byte(i)
}
// authoritative answer
func (p Packet) unsetAA() { p.aa(false) }
func (p Packet) setAA()   { p.aa(true) }
func (p Packet) aa(b bool) {
    p[Flags1] |= (1<<AA)        // set
    if ! b {
        p[Flags1] ^= (1<<AA)    // unset
    }
}
// recursion
func (p Packet) unsetRD() { p.rd(false) }
func (p Packet) setRD()   { p.rd(true) }
func (p Packet) rd(b bool) {
    p[Flags1] |= (1<<RD)
    if ! b {
        p[Flags1] ^= (1<<RD)
    }
}
func (p Packet) unsetRA() { p.ra(false) }
func (p Packet) setRA()   { p.ra(true) }
func (p Packet) ra(b bool) {
    p[Flags2] |= (1<<RA)
    if ! b {
        p[Flags2] ^= (1<<RA)
    }
}
// authentic data
func (p Packet) unsetAD() { p.ad(false) }
func (p Packet) setAD()   { p.ad(true) }
func (p Packet) ad(b bool) {
    p[Flags2] |= (1<<AD)
    if ! b {
        p[Flags2] ^= (1<<AD)
    }
}



//
// RR

type RR interface {
    validate() bool
    bytes() []byte
}
type Nxdomain struct {
    l1, l2 string
    typ, ttl int
}
type Rdata struct {
    l1, l2 string // l1: name, l2: resource data
    typ, ttl int
}
type RRset []*RR




type rr struct {
    l1, l2 string // l1: name, l2: rdata
    typ, ttl int
}
type rrset []*rr
type labelMap struct {
    bytes []byte
    lmap map[string]int
}
func mapLabel(s string, question bool) labelMap {
    // offset our substring to *not* include '.' (+1)
    // offset position to account for length at the beginning of label (+1)
    // QUESTION has a single byte of length
    // RR has two bytes of RDLENGTH
    offset := RDLENGTH
    if question {
        offset--
    }
    m := make(map[string]int)
    b := make([]byte, len(s)+offset+1) // +1 for '.' (root) at the end

    l:=0
    for i:=len(s)-1; i>=0; i-- {
        if s[i] == '.' {
            pos := i+offset
            m[s[pos:]] = pos // mapping
            b[pos] = byte(l) // bytes
            l = 0
            continue
        }

        if i == 0 {
            //m[s] = 0
            m[s] = offset-1
        }

        b[i+offset] = s[i]
        l++
    }
    b[offset-1] = byte(l)
    return labelMap{b, m}
}
func run(rs rrset) []byte {
    rs.validate()
    // packet
    // 1st dimension is the full packet - N+1 bytes
    // 2nd dimension is each RRs        - N+1 RRs
    // 3rd dimension is the actual RR   - [label1], [type, class, ttl], [label2]
    // [    # packet
    //  [   # RR
    //   ["label1.com"], [type, class, ttl], ["label2.com" or IP]
    //  ],
    //  ... # N+1 RRs
    // ]
    packet := make([][][]byte, 1+len(rs)) // QUESTION + RRs
    lmap := make(map[string]int)

    // QUESTION:
    // same as first label and fully serialized (no pointer)
    lm := mapLabel(rs[0].l1, true)
    lmap = lm.lmap
    packet[0] = make([][]byte, Q_PARTSLEN)
    packet[0] = [][]byte{lm.bytes, []byte{0, 1, 0, 1}}

    // RRs
    for i, r := range rs {
        j := i+1
        packet[j] = make([][]byte, RR_PARTSLEN)

        // l1 - always known
        packet[j][0] = []byte{COMPRESSED_LABEL, byte(lmap[r.l1]+HEADERSLEN)}

        // TYPE(2), CLASS(2), TTL(4)
        packet[j][1] = []byte{0, byte(r.typ), 0, 1, 0, 0, 0, byte(r.ttl)}

        // l2 - always unknown
        switch r.typ {
            case A:
            needsroot := false
            if i == len(rs)-1 {
                needsroot = true
            }
            fmt.Printf("i: %d <> ndr: %+v <> lbl: %s\n", i, needsroot, r.l2)
            packet[j][2] = serializeIp(r.l2, needsroot)

            case CNAME:
            // currpos is real position (index) of start of this label in the packet and has 3 parts
            // 1. headers
            currpos := 0
            // 2. N+1 previous RRs
            for n:=0; n<=i; n++ {
                for m:=0; m<len(packet[n]); m++ {
                    currpos += len(packet[n][m])
                }
            }
            // 3. its own l1 + class/type/ttl
            //    +1 as we're currently at last index before the label start
            currpos += len(packet[j][0])+len(packet[j][1])
            // TODO catch if this already exists as it may indicate a CNAME loop,
            // validateHostname() should catch this though..?
            lmap[r.l2] = currpos+RDLENGTH
            lm = mapLabel(r.l2, false)

            // have populated lmap with new values
            // now I need to split the label to find what needs to be serialized and what is to come from lmap
            for n:=0; n<len(r.l2); n++ {
                if r.l2[n] == '.' {
                    // found a match
                    if pos, isknown := lmap[r.l2[n+1:]]; isknown {
                        b1 := serializeString(r.l2[:n], false)  // serialize unknown part
                        b2 := []byte{192, byte(pos)}            // make pointer to the known part
                        packet[j][2] = make([]byte, RDLENGTH+len(b1)+len(b2))
                        packet[j][2][0] = byte(0)
                        packet[j][2][1] = byte(len(b1)+len(b2))
                        // populate with b1 + b2
                        x := 0
                        for ; x<len(b1); x++ {
                            packet[j][2][RDLENGTH+x] = b1[x]
                        }
                        for y:=0; y<len(b2); y++ {
                            packet[j][2][RDLENGTH+x+y] = b2[y]
                        }

                        break
                    }

                    // no known match, record it here
                    lmap[r.l2[n+1:]] = currpos+n
                }
            }

            // no match, serialize the full label
            if len(packet[j][2]) == 0 {
                b2 := serializeString(r.l2, true)
                b2len := len(b2)

                packet[j][2] = make([]byte, RDLENGTH+b2len)
                packet[j][2][0] = byte(0)
                packet[j][2][1] = byte(b2len)
                // populate with b2
                for x:=0; x<len(b2); x++ {
                    packet[j][2][RDLENGTH+x] = b2[x]
                }
            }
        }
    }
    //fmt.Printf("2. lmap: %+v\n", lmap)
    //fmt.Printf("2. PACKET: %+v\n", packet)

    r := make([]byte, 0)
    for i:=0; i<len(packet); i++ {
        for j:=0; j<len(packet[i]); j++ {
            for k:=0; k<len(packet[i][j]); k++ {
                r = append(r, packet[i][j][k])
            }
        }
    }
    return r
}
func serializeIp(ip string, addroot bool) []byte {
    // +2 to add 2 bytes of length (for ipv4), also rpos below
    // +1 to add root (0) if desired
    offset := 2
    if addroot {
        offset++
    }
    ret := make([]byte, 4+offset)
    ret[0] = 0
    ret[1] = 4

    for i, octet := range strings.Split(ip, ".") {
        o, err := strconv.Atoi(string(octet))
        if err != nil {
            panic(err)
        }
        rpos := i+2
        ret[rpos] = byte(o)
    }
    return ret
}
func serializeString(s string, addroot bool) []byte {
    // replace all '.' with length of that label
    // add (+1) to add length of first label
    // add (+1) to add root (0) if desired
    offset := 1
    if addroot {
        offset++
    }
    ret := make([]byte, len(s)+offset)

    l:=0
    for i:=len(s)-1; i>=0; i-- {
        rpos := i+1 // +1 for length of first label
        if s[i] == '.' {
            ret[rpos] = byte(l)
            l = 0
            continue
        }

        ret[rpos] = s[i]
        l++
    }
    ret[0] = byte(l) // first label length
    return ret
}
func (rs rrset) validate() bool {
    // 1. set of A records
    // 2. chain of CNAME records
    for _, r := range rs {
        switch r.typ {
            case A: validateA(r.l1, r.l2)
            case CNAME: validateCNAME(r.l1, r.l2)
            default:
                panic("Invalid DNS record")
        }
    }

    return true
}
var hostx *regexp.Regexp = regexp.MustCompile(`[a-z0-9\-\.]+`)
var ipx *regexp.Regexp = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
func validateA(host, ip string) bool { return validateHostname(host) == validateIp(ip) }
func validateCNAME(host1, host2 string) bool { return validateHostname(host1) == validateHostname(host2) }
func validateHostname(hostname string) bool {
    if ! hostx.MatchString(hostname) {
        panic("Hostname can only contain: a-z 0-9 - .")
    }
    h := strings.Split(hostname, ".")
    if len(h) < 2 {
        panic("Hostname must have at least 2 parts/domains")
    }

    return true
}
func validateIp(ip string) bool {
    if ! ipx.MatchString(ip) {
        panic("IP must be in format: num.num.num.num")
    }

    for i, octet := range strings.Split(ip, ".") {
        o, err := strconv.Atoi(octet) 
        if err != nil {
            panic(err)
        }

        if o > 255 {
            panic("IP octet invalid: > 255")
        }
        maxlow := 0
        if i == 0 { // first octet must be 1+
            maxlow = 1
        }
        if o < maxlow {
            panic(fmt.Sprintf("IP octet invalid: < %d", maxlow))
        }
    }

    return true
}
