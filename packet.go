package main

import (
    "fmt"
    "reflect"
)

type RR interface {
    defaults()    // populate missing attributes with default values
}
type RRset []RR
type Rdata struct {
    l1, l2 string
    typ, ttl int
}
func (rd *Rdata) defaults() {
    // don't do anything with l1 or l2 which must be given by user
    if rd.ttl == 0 {
        rd.ttl = 100
    }
    // TODO check typ based on l2
}

type Nxdomain struct {
    l1, mname, rname string
    serial, refresh, retry, expire, ttl int
}
// TODO fix this int values
func (nx *Nxdomain) defaults() {
    if nx.mname == "" {
        nx.mname = "ns1.google.com"
    }
    if nx.rname == "" {
        nx.rname = "dns-admin.google.com"
    }
    // don't do anything with l1 which must be given by user
    if nx.serial == 0 {
        nx.serial = 100
    }
    if nx.refresh == 0 {
        nx.refresh = 90
    }
    if nx.retry == 0 {
        nx.retry = 80
    }
    if nx.expire == 0 {
        nx.expire = 70
    }
    if nx.ttl == 0 {
        nx.ttl = 60
    }
}

//type Packet []byte
type Packet struct {
    bytes []byte
}
// TODO trim when necessary (eg: upstream answer)
func NewAnswerPacket(b []byte) *Packet {
    p := NewQueryPacket(b)
    return &p
}
func NewQueryPacket(b []byte) Packet {
    return Packet{b}
}
func (p *Packet) Question() string {
    var q string
    l := int(p.bytes[QUESTION_LABEL_START])
    for i:=QUESTION_LABEL_START+1;; {
        q += string(p.bytes[i:i+l])

        i += l
        l = int(p.bytes[i])
        if l == 0 {
            break
        }

        i++
        q += "."
    }
    return q
}

const (
    RDATA = iota + 1
    NOTFOUND
)
func (rs RRset) GetPacket() *Packet { // this is run()
    var p *Packet
    switch rs.Type() {
        case RDATA:
            p = rs.rdata()
            p.SetAnswer()
            p.SetANcount(len(rs))
            p.SetQDcount(1)
            p.SetRD()
            p.SetRA()
        case NOTFOUND:
            p = rs.notfound()
            p.SetAnswer()
            p.SetRD()
            p.SetRA()
            p.SetQDcount(1)
            p.SetNxdomain()
            p.SetNScount(1)
            fmt.Println(">>>>>> NXDOMAIN")
            /*
; <<>> DiG 9.16.33 <<>> @localhost kdk.google.com
;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 60855
;; flags: qr rd ra; QUERY: 1, ANSWER: 0, AUTHORITY: 1, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 512
;; QUESTION SECTION:
;kdk.google.com.            IN  A

;; AUTHORITY SECTION:
google.com.     60  IN  SOA ns1.google.com. dns-admin.google.com. 491868622 900 900 1800 60
            */
    }
    return p
}
func (rs RRset) Type() int {
    // empty RRset will fail here
    var s string = reflect.TypeOf(rs[0]).String()
    var t int
    switch s {
        case "*main.Rdata": t = RDATA
        case "*main.Nxdomain": t = NOTFOUND
        default:
            panic(fmt.Sprintf("Unsupported type: %s", s))
    }
    return t
}
func (rs RRset) rdata() *Packet {
    var lm *LabelMap
    for i:=0; i<len(rs); i++ {
        ri := reflect.Indirect(reflect.ValueOf(rs[i]))
        l1 := ri.FieldByName("l1").String()
        l2 := ri.FieldByName("l2").String()
        typ := int(ri.FieldByName("typ").Int())
        ttl := int(ri.FieldByName("ttl").Int())

        // build question only once
        if i == 0 {
            lm = MapLabel(l1)
            lm.finalizeQuestion()
        }

        // 1st label
        lm.extend(l1, true)
        // type, class, ttl
        //lm.bytes = append(lm.bytes, []byte{0, byte(typ), 0, IN, 0, 0, 0, byte(ttl)}...)
        lm.typeClassTtl(typ, IN, ttl)
        // 2nd label
        switch typ {
        case A: lm.extendIp(l2)
        case CNAME: lm.extend(l2, false)
        }
    }

    // add headers
    p := &Packet{append(make([]byte, HEADERSLEN), lm.bytes...)}
    p.bytes = append(p.bytes, ROOT)

    fmt.Printf("packet: %+v\n", p)
    return p
}
func (rs RRset) notfound() *Packet {
    ri := reflect.Indirect(reflect.ValueOf(rs[0])) // nxdomain has only one member
    l1 := ri.FieldByName("l1").String()
    mname := ri.FieldByName("mname").String()
    rname := ri.FieldByName("rname").String()
    serial := int(ri.FieldByName("serial").Int())
    refresh := int(ri.FieldByName("refresh").Int())
    retry := int(ri.FieldByName("retry").Int())
    expire := int(ri.FieldByName("expire").Int())
    ttl := int(ri.FieldByName("ttl").Int())
    fmt.Printf("NotFound: %s <> %s <> %s <> %d <> %d <> %d <> %d <> %d\n",
                    l1, mname, rname, serial, refresh, retry, expire, ttl)

    // label map question
    lm := MapLabel(l1)
    lm.finalizeQuestion()
    //fmt.Printf("1: NFlm: %+v\n", lm)
    // TODO get the l1.suffix version of the question
    lm.extend("google.com", true)
    lm.typeClassTtl(SOA, IN, ttl)
    //fmt.Printf("2: NFlm: %+v\n", lm)
    lm.extendSOA(mname, rname, serial, refresh, retry, expire, ttl)
    lm.bytes = append(lm.bytes, byte(0))
    fmt.Printf("3: NFlm: %+v\n", lm)

    p := &Packet{append(make([]byte, HEADERSLEN), lm.bytes...)}
    return p

    /*

62 32 129 131 0 1 0 0 0 1 0 1
// question
34 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 120 3 120 120 120 0 0 1 0 1
// auth class/type/ttl
192 47 0 6 0 1 0 0 3 132
// 2b length
// SOA (mname, rname, ...)
0 49 1 97 3 110 105 99 192 47 5 97 100 109 105 110 5 116 108 100 110 115 7 103 111 100 97 100 100 121 0 99 170 69 211 0 0 7 8 0 0 1 44 0 9 58 128 0 0 7 8 0 0 41 2 0 0 0 0 0 


vella@vella ~/git/github/dnsproxy $ dig xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.xxx

; <<>> DiG 9.16.33 <<>> xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.xxx
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 40341
;; flags: qr rd ra; QUERY: 1, ANSWER: 0, AUTHORITY: 1, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 512
;; QUESTION SECTION:
;xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.xxx.	IN A

;; AUTHORITY SECTION:
xxx.			900	IN	SOA	a.nic.xxx. admin.tldns.godaddy. 1672103379 1800 300 604800 1800


=====================================================================================================
[237 183 129 131 0 1 0 0 0 1 0 1
    // question
  3 107 100 107 6 103 111 111 103 108 101 3 99 111 109 0 0 1 0 1
    // auth class/type/ttl
  192 16 0 6 0 1 0 0 0 60
    // 2byte total length
    // mname
    // rname
  0 38 3 110 115 49 192 16 9 100 110 115 45 97 100 109 105 110 192 16 29 81 81 206 0 0 3 132 0 0 3 132 0 0 7 8 0 0 0 60 0 0 41 2 0 0 0 0 0 0 ]

; <<>> DiG 9.16.33 <<>> @localhost kdk.google.com
;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 60855
;; flags: qr rd ra; QUERY: 1, ANSWER: 0, AUTHORITY: 1, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 512
;; QUESTION SECTION:
;kdk.google.com.            IN  A

;; AUTHORITY SECTION:
google.com.     60  IN  SOA ns1.google.com. dns-admin.google.com. 491868622 900 900 1800 60
  */
}
func (rs RRset) CheckValid() {
    for i, r := range rs {
        r.defaults()

        v := reflect.Indirect(reflect.ValueOf(r))
        switch reflect.TypeOf(r).String() {
        case "*main.Rdata":
            // TODO
            // eval fields as per regex, etc..
            l1 := v.FieldByName("l1").String()
            l2 := v.FieldByName("l2").String()
            typ := int(v.FieldByName("typ").Int())
            ttl := int(v.FieldByName("ttl").Int())

            for k, v := range map[string]string{"l1":l1, "l2":l2} {
                if v == "" {
                    panic(fmt.Sprintf("Invalid %s: ''(empty)", k))
                }
            }
            if l1 == l2 {
                panic(fmt.Sprintf("Broken: %s + %s\n", l1, l2))
            }
            if ttl < 1 {
                panic(fmt.Sprintf("Broken TTL: %d\n", ttl))
            }

            if i > 0 {
                // this would have been eval-ed above
                // no need to do this again
                pv := reflect.Indirect(reflect.ValueOf(rs[i-1]))
                pl1 := pv.FieldByName("l1").String()
                pl2 := pv.FieldByName("l2").String()
                ptyp := int(pv.FieldByName("typ").Int())

                switch typ {
                case CNAME:
                    if pl2 != l1 {
                        panic(fmt.Sprintf("Broken CNAME chain: %s + %s\n", pl2, l1))
                    }
                case A:
                    switch ptyp {
                    case CNAME:
                        if pl2 != l1 {
                            panic(fmt.Sprintf("Broken CNAME/A chain: %s + %s\n", pl2, l1))
                        }
                    case A:
                        if pl1 != l1 {
                            panic(fmt.Sprintf("Broken A record: %s + %s\n", pl1, l1))
                        }
                    }
                }
            }

        case "*main.Nxdomain":
            l1 := v.FieldByName("l1").String()
            mname := v.FieldByName("mname").String()
            rname := v.FieldByName("rname").String()
            serial := int(v.FieldByName("serial").Int())
            refresh := int(v.FieldByName("refresh").Int())
            retry := int(v.FieldByName("retry").Int())
            expire := int(v.FieldByName("expire").Int())
            ttl := int(v.FieldByName("ttl").Int())

            for k, v := range map[string]string{"l1":l1, "mname":mname, "rname":rname} {
                if v == "" {
                    panic(fmt.Sprintf("Invalid %s: ''(empty)", k))
                }
            }
            for k, v := range map[string]int{"serial":serial, "refresh":refresh, "retry":retry, "expire":expire, "ttl":ttl} {
                if v < 1 {
                    panic(fmt.Sprintf("Invalid %s: %d", k, v))
                }
            }

        default:
            panic(fmt.Sprintf("Unsupported type: %+v\n", reflect.TypeOf(r).String()))
        }
    }
}


//
// Headers

func (p *Packet) IngestPacketId(id []byte) {
    if len(id) != IDLEN {
        panic(fmt.Sprintf("Invalid packet ID length: %d", len(id)))
    }
    fmt.Printf("%+v\n", p)
    p.bytes[Id1] = id[Id1]
    p.bytes[Id2] = id[Id2]

    // TODO return error instead of crashing to allow proxy to keep working(?)
    if p.bytes[Id1] == 0 && p.bytes[Id2] == 0 {
        panic("Invalid packet ID: 0")
    }
}
// query type - question/answer
func (p *Packet) SetAnswer() { p.bytes[Flags1] |= (1<<QR) }
// counts
func (p *Packet) SetQDcount(i int) {
    p.bytes[QDcount1] = 0
    p.bytes[QDcount2] = 1
}
func (p *Packet) SetANcount(i int) {
    p.bytes[ANcount1] = 0
    p.bytes[ANcount2] = byte(i)
}
func (p *Packet) SetNScount(i int) {
    p.bytes[NScount1] = 0
    p.bytes[NScount2] = byte(i)
}
// authoritative answer
func (p *Packet) UnsetAA() { p.aa(false) }
func (p *Packet) SetAA()   { p.aa(true) }
func (p *Packet) aa(b bool) {
    p.bytes[Flags1] |= (1<<AA)      // set
    if ! b {
        p.bytes[Flags1] ^= (1<<AA)  // unset
    }
}
// recursion
func (p *Packet) UnsetRD() { p.rd(false) }
func (p *Packet) SetRD()   { p.rd(true) }
func (p *Packet) rd(b bool) {
    p.bytes[Flags1] |= (1<<RD)
    if ! b {
        p.bytes[Flags1] ^= (1<<RD)
    }
}
func (p *Packet) UnsetRA() { p.ra(false) }
func (p *Packet) SetRA()   { p.ra(true) }
func (p *Packet) ra(b bool) {
    p.bytes[Flags2] |= (1<<RA)
    if ! b {
        p.bytes[Flags2] ^= (1<<RA)
    }
}
// authentic data
func (p *Packet) UnsetAD() { p.ad(false) }
func (p *Packet) SetAD()   { p.ad(true) }
func (p *Packet) ad(b bool) {
    p.bytes[Flags2] |= (1<<AD)
    if ! b {
        p.bytes[Flags2] ^= (1<<AD)
    }
}
// RCODE
func (p *Packet) SetNxdomain() { p.SetRcode(NXDOMAIN) }
func (p *Packet) SetRcode(i uint8) {
    if i > NAME_NOT_IN_ZONE {
        panic(fmt.Sprintf("RCODE not supported: %d", i))
    }

    //p.unsetBitInByte(Flags2, RCODE...)
    // this should be as clean as whistle
    // therefore only set
    p.bytes[Flags2] |= i
}


func (p *Packet) unsetBitInByte(byt uint8, bit ...uint8) {
    if len(bit) == 0 {
        return
    }

    for _, b := range bit {
        if b > 7 {
            panic(fmt.Sprintf("0-7 bit indexes in byte, got: %d", b))
        }

        // p[byt] will bomb out
        // if byt is not valid index
        p.bytes[byt] |= (1<<b)
        p.bytes[byt] ^= (1<<b)
    }
}
