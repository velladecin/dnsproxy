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
    if rd.ttl == 0 {
        rd.ttl = 100
    }
    // TODO check typ based on l2
}

type Nxdomain struct {
    l1, mname, rname string
    serial, refresh, retry, expire, ttl int
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
/*
func (p *Packet) IngestHeaders(h []byte) {
    if len(h) != HEADERSLEN {
        panic(fmt.Sprintf("Invalid headers length: %d", len(h)))
    }
    p.bytes = append(h, p.bytes...)
}
*/
func (p *Packet) IngestPacketId(id []byte) {
    if len(id) != 2 {
        panic(fmt.Sprintf("Invalid packet ID length: %d", len(id)))
    }
    fmt.Printf("%+v\n", p)
    p.bytes[0] = id[0]
    p.bytes[1] = id[1]

    // TODO getInt()
    if p.bytes[0] == 0 && p.bytes[1] == 0 {
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
        case NOTFOUND: p = rs.notfound()
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
        lm.bytes = append(lm.bytes, []byte{0, byte(typ), 0, IN, 0, 0, 0, byte(ttl)}...)
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
    var p *Packet
    ri := reflect.Indirect(reflect.ValueOf(rs[0])) // nxdomain has only one member
    l1 := ri.FieldByName("l1")
    mname := ri.FieldByName("mname")
    rname := ri.FieldByName("rname")
    serial := ri.FieldByName("serial")
    refresh := ri.FieldByName("refresh")
    retry := ri.FieldByName("retry")
    expire := ri.FieldByName("expire")
    ttl := ri.FieldByName("ttl")
    fmt.Printf("%s <> %s <> %s <> %d <> %d <> %d <> %d <> %d\n",
                    l1, mname, rname, serial, refresh, retry, expire, ttl)
    return p
}
func (rs RRset) checkValid() {
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
        default:
            panic(fmt.Sprintf("Unsupported type: %+v\n", reflect.TypeOf(r).String()))
        }
    }
}
