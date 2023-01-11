package main

import (
    "fmt"
    "strings"
    "time"
    "regexp"
    "sync"
)

var iprgx *regexp.Regexp = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
var emptyrgx *regexp.Regexp = regexp.MustCompile(`^\s*$`)

type Rdata interface {
    GetBytes() []byte
    QueryStr() string
}

//
// RRs

type rr struct {
    // l1 hostname, l2 hostname or IP (must match type of record)
    l1, l2 string

    // type of record
    typ int

    // time to live
    ttl int
}
func (r *rr) defaults() {
    // l1, l2 are given and typ is determined from l2
    r.typ = A
    if ! iprgx.MatchString(r.l2) {
        r.typ = CNAME
    }
    if r.ttl == 0 {
        r.ttl = 300
    }
}

// Resource Record Set
// hostname(s) must be first, IP addr(s) must be last
// multiple hostnames will be interpreted as CNAMEs
// at least two records must be supplied
// at least one IP addr must be supplied
//
// example: NewRRset(google.com, moogle.com, 1.1.1.1, 2.2.2.2)
// google.com CNAME moogle.com
// moogle.com A 1.1.1.1
// moogle.com A 2.2.2.2
type rrset struct {
    // given/desired RRs
    // this slice does not change
    recs []*rr

    // RRs from which actual DNS response will be built
    // in case of "checked" rrset this slice will change depending on check results
    recsactive []*rr

    // locking to safely do changes to recsactive
    // while using them to dynamically build DNS records
    sync.Mutex
}
func NewRRset(recs ...string) *rrset {
    var host []string
    var ip []string
    for i, j, x, y := 0, len(recs)-1, 0, 0; i<len(recs); i, j = i+1, j-1 {
        // hostnames
        if iprgx.MatchString(recs[i]) {
            // this string is matching IP addr and marks the end of hostnames
            // stop collecting hosts
            x++
        }
        if x == 0 {
            host = append(host, recs[i])
        }

        // ips
        if ! iprgx.MatchString(recs[j]) {
            // this string is not matching IP addr and marks the end of IPs
            // stop collecting IPs
            y++
        }
        if y == 0 {
            ip = append(ip, recs[j])
        }

        // loop ctl
        if x > 0 && y > 0 {
            // both (hostnames & IPs) collections have stopped
            // there is nothing else for us to do
            break
        }
    }
    // validate RR chain
    if len(host) == 0 || len(ip) == 0 || (len(host) + len(ip)) != len(recs) {
        panic("Usage: NewRRset(hostname, ipaddr, ...)")
    }
    // build RR
    var rc []*rr
    for j, h := range host {
        // A(s)
        if j == len(host)-1 {
            // last hostname (in possible CNAME chain)
            // add IP(s) and exit 
            for _, i := range ip {
                rc = append(rc, &rr{l1: h, l2: i})
                rc[len(rc)-1].defaults()
            }
            break
        }

        // CNAME(s)
        rc = append(rc, &rr{l1: h, l2: host[j+1]})
        rc[len(rc)-1].defaults()
    }
    return &rrset{recs: rc, recsactive: rc}
}
func NewRRsetChecked(c Check, recs ...string) *rrset {
    rs := NewRRset(recs...)
    go c.Run(rs)
    return rs
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
func (rs *rrset) QueryStr() string {
    return rs.recs[0].l1
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
func (nx *nxdomain) QueryStr() string {
    return nx.question
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
