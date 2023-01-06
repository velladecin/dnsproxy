package main

import (
    "fmt"
    "strings"
    "time"
    "regexp"
)

var iprgx *regexp.Regexp = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
var emptyrgx *regexp.Regexp = regexp.MustCompile(`^\s*$`)

type Rdata interface {
    GetBytes() []byte
}

//
// RRs

type rr struct {
    // l1 hostname, l2 hostname or IP (dependency with type of record)
    l1, l2 string

    // type of record
    typ int

    // time to live
    ttl int
}
func (r *rr) defaults() {
    // l1, l2 are given and typ is determined from l2
    if iprgx.MatchString(r.l2) {
        r.typ = A
    } else {
        r.typ = CNAME
    }
    if r.ttl == 0 {
        r.ttl = 300
    }
}

type rrset struct {
    recs []*rr
}
func NewRRset(recs ...[]string) *rrset {
    if len(recs) < 1 || len(recs) > 10 {
        panic("Usage: NewRRset([l1, l2], ...), (limit: 1-10)")
    }

    rc := make([]*rr, len(recs))
    for i, rec := range recs {
        if len(rec) != 2 {
            panic("Usage: NewRRset([l1, l2]) - length must be 2")
        }

        ok1 := emptyrgx.MatchString(rec[0])
        ok2 := emptyrgx.MatchString(rec[1])
        if ok1 || ok2 {
            panic("Usage: NewRRset([l1, l2]) - both must have non-empty value")
        }

        rc[i] = &rr{l1: rec[0], l2: rec[1]}
        rc[i].defaults()

        // check RR chains, ignoring the first record
        // which by now must be in correct format
        if i > 0 {
            switch rc[i].typ {
            case CNAME:
                if rc[i-1].typ != CNAME {
                    panic("NewRRset: broken types of CNAME chain")
                }
                if rc[i-1].l2 != rc[i].l1 {
                    panic("NewRRset: broken CNAME chain")
                }
            case A:
                switch rc[i-1].typ {
                case CNAME:
                    if rc[i-1].l2 != rc[i].l1 {
                        panic("Broken CNAME/A chain")
                    }
                case A:
                    if rc[i-1].l1 != rc[i].l1 {
                        panic("Broken A record set")
                    }
                }
            }
        }
    }

    return &rrset{recs: rc}
}
func (rs *rrset) GetBytes() []byte {
    var lm *LabelMap
    for i, rec := range rs.recs {
        if i == 0 {
            lm = MapLabelQuestion(rec.l1)
        }

        lm.extendRR(rec.l1, rec.l2, rec.typ, IN, rec.ttl)
    }

    h := NewAnswerHeaders()
    h.SetANcount(len(rs.recs))
    return append(h.Bytes, lm.bytes...)
}



//
// NXDOMAIN

type nxdomain struct {
    // DNS question to which we return nxdomain
    // can be a.com or *.a.com (TODO)
    question string

    // authority giving the answer
    // defaults to the last question label and will be ignored even if explicitly defined
    authority string

    // mname - source server
    // rname - responsible mailbox
    mname, rname string

    // as expected
    serial, refresh, retry, expire, ttl int
}
// up to 3 string arguments (question, mname, rname) where question must be provided
// other two will be determined if not explicitly given
func NewNxdomain(str ...string) *nxdomain {
    if len(str) < 1 || len(str) > 3 {
        panic("Usage: NewNxdomain(question [, mname, rname])")
    }

    n := &nxdomain{}
    for i, s := range str {
        ok := emptyrgx.MatchString(s)
        switch i {
        case 0:
            if ok {
                panic("Nxdomain: empty question string")
            }
            n.question = s
        case 1: 
            if ! ok {
                n.mname = s
            }
        case 2:
            if ! ok {
                n.rname = s
            }
        }
    }

    n.defaults()
    return n
}
func (nx *nxdomain) defaults() {
    // question is given and authority is determined from question
    // update what else needs it
    parts := strings.Split(nx.question, ".")
    nx.authority = parts[len(parts)-1]

    if nx.mname == "" {
        nx.mname = "ns1.versig." + nx.authority
    }
    if nx.rname == "" {
        nx.rname = "dns-admin.versig." + nx.authority
    }
    if nx.serial == 0 {
        nx.serial = int(time.Now().Unix()) // very long time before it overflows
    }
    if nx.refresh == 0 {
        nx.refresh = 900
    }
    if nx.retry == 0 {
        nx.retry = 300
    }
    if nx.expire == 0 {
        nx.expire = 604800
    }
    if nx.ttl == 0 {
        nx.ttl = 900
    }
}
func (nx *nxdomain) GetBytes() []byte {
    // question
    lm := MapLabelQuestion(nx.question)
    // auth
    lm.bytes = append(lm.bytes, []byte{COMPRESSED_LABEL, byte(lm.index[nx.authority])}...)
    // type, class, ttl
    lm.typeClassTtl(SOA, IN, nx.ttl)
    // SOA
    lm.extendSOA(nx.mname, nx.rname, nx.serial, nx.refresh, nx.retry, nx.expire, nx.ttl)
    //lm.bytes = append(lm.bytes, byte(0))

    h := NewNxdomainHeaders()
    h.SetNScount(1)

    fmt.Printf("3: NFlm: %+v\n", lm)
    return append(h.Bytes, lm.bytes...)
}

// return DNS question as a string
// would be (mostly?) used on the incoming DNS request to find what was requested
func QueryStr(q []byte) string {
    var s string
    l := int(q[QUESTION_LABEL_START])
    for i:=QUESTION_LABEL_START+1;; {
        s += string(q[i:i+l])

        i += l
        l = int(q[i])
        if l == 0 {
            break
        }
        i++
        s += "."
    }
    return s
}


//
// Headers

type Headers struct {
    Bytes []byte
}
func NewHeaders() *Headers {
    return &Headers{make([]byte, 12)}
}
func NewAnswerHeaders() *Headers {
    h := NewHeaders()
    h.SetAnswer()
    h.SetRD()
    h.SetRA()
    h.SetQDcount(1)
    return h
}
func NewNxdomainHeaders() *Headers {
    h := NewAnswerHeaders()
    h.SetNxdomain()
    return h
}

// query type - question/answer
func (h *Headers) SetAnswer() {
    h.Bytes[Flags1] |= (1<<QR)
}
// counts
func (h *Headers) SetQDcount(i int) {
    h.Bytes[QDcount1] = 0
    h.Bytes[QDcount2] = 1
}
func (h *Headers) SetANcount(i int) {
    h.Bytes[ANcount1] = 0
    h.Bytes[ANcount2] = byte(i)
}
func (h *Headers) SetNScount(i int) {
    h.Bytes[NScount1] = 0
    h.Bytes[NScount2] = byte(i)
}
// authoritative answer
func (h *Headers) UnsetAA() { h.aa(false) }
func (h *Headers) SetAA()   { h.aa(true) }
func (h *Headers) aa(b bool) {
    h.Bytes[Flags1] |= (1<<AA)      // set
    if ! b {
        h.Bytes[Flags1] ^= (1<<AA)  // unset
    }
}
// recursion
func (h *Headers) UnsetRD() { h.rd(false) }
func (h *Headers) SetRD()   { h.rd(true) }
func (h *Headers) rd(b bool) {
    h.Bytes[Flags1] |= (1<<RD)
    if ! b {
        h.Bytes[Flags1] ^= (1<<RD)
    }
}
func (h *Headers) UnsetRA() { h.ra(false) }
func (h *Headers) SetRA()   { h.ra(true) }
func (h *Headers) ra(b bool) {
    h.Bytes[Flags2] |= (1<<RA)
    if ! b {
        h.Bytes[Flags2] ^= (1<<RA)
    }
}
// authentic data
func (h *Headers) UnsetAD() { h.ad(false) }
func (h *Headers) SetAD()   { h.ad(true) }
func (h *Headers) ad(b bool) {
    h.Bytes[Flags2] |= (1<<AD)
    if ! b {
        h.Bytes[Flags2] ^= (1<<AD)
    }
}
// RCODE
func (h *Headers) SetNxdomain() { h.SetRcode(NXDOMAIN) }
func (h *Headers) SetRcode(i uint8) {
    if i > NAME_NOT_IN_ZONE {
        panic(fmt.Sprintf("RCODE not supported: %d", i))
    }

    //h.UnsetBitInByte(Flags2, RCODE...)
    // this should be as clean as whistle
    // therefore only set
    h.Bytes[Flags2] |= i
}


func (h *Headers) UnsetBitInByte(byt uint8, bit ...uint8) {
    if len(bit) == 0 {
        return
    }

    for _, b := range bit {
        if b > 7 {
            panic(fmt.Sprintf("0-7 bit indexes in byte, got: %d", b))
        }

        // h.Bytes[byt] will bomb out
        // if byt is not valid index
        h.Bytes[byt] |= (1<<b)
        h.Bytes[byt] ^= (1<<b)
    }
}
