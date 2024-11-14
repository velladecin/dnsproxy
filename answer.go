package main

import (
_    "fmt"
    "strings"
    "regexp"
    "bytes"
    "encoding/binary"
    "time"
    "strconv"
)

type Answer struct {
    // 1+N answer section
    rr [][]string

    // 1+N additional section
    addi [][]string

    // packet header
    header []byte 

    // packet body
    body []byte

    // map of known labels with their index
    // to use for label pointers
    lblMap map[string]int

    // byte index
    i int

    // type of answer
    t int
}

// cache is used for A record lookup
func NewMx(q, mxhost string, cache map[int]map[string]*Answer) (*Answer, error) {
    a := &Answer{[][]string{[]string{q, mxhost}},
                 make([][]string, 0),
                 make([]byte, HEADER_LEN),
                 make([]byte, PACKET_SIZE-HEADER_LEN),
                 make(map[string]int),
                 0,
                 MX}

    // add A record
    a.addi = append(a.addi, cache[A][mxhost].rr...)

    if debug {
        cDebg.Print("New MX: " + a.QandR())
    }

    // headers
    a.setRespHeaders()
    //fmt.Printf("======> %+v\n", a.header)

    // question
    a.Question()

    // answer
    //fmt.Printf("======> %+v\n", a.rr)
    a.labelize(a.QuestionString())

    a.body[a.i+1] = MX
    a.body[a.i+3] = IN
    a.body[a.i+7] = TTL
    // index a.i+8,9 is for total length
    // of MX priority + host
    a.i += 10

    // -1 as we moved over to next byte
    tl := a.i-1

    // priority
    a.body[a.i+1] = MXPRIO
    a.i += 2
    // mx host
    a.labelize(mxhost)

    // TODO why -1 here???
    // total length
    a.body[tl] = byte(a.i-tl-1)

    // additional / A record
    for _, r := range a.addi {
        // response
        a.labelize(r[0])

        a.body[a.i+1] = A
        a.body[a.i+3] = IN
        a.body[a.i+7] = TTL
        a.i += 8

        _, err := a.labelizeIp(r[1])
        if err != nil {
            return nil, err
        }
    }

    a.additional()

    //fmt.Printf("+++>> %+v\n", a.body)

    return a, nil
}

// cache is used for A record lookup
func NewCname(q string, r []string, cache map[int]map[string]*Answer) (*Answer, error) {

    // TODO - make sure there's no CNAME loop!

    a := &Answer{make([][]string, 0),
                 make([][]string, 0),
                 make([]byte, HEADER_LEN),
                 make([]byte, PACKET_SIZE-HEADER_LEN),
                 make(map[string]int),
                 0,
                 CNAME}
    i := 0
    next := ""
    for i, next = range r {
        var s string
        if i == 0 {
            // first part of CNAME chain 
            s = q
        } else {
            // other parts of CNAME chain
            s = r[i-1]
        }

        a.rr = append(a.rr, []string{s, next})
    }

    // A record is for last hostname
    // in CNAME chain
    last := a.rr[i][1]
    a.rr = append(a.rr, cache[A][last].rr...)

    // headers
    a.setRespHeaders()

    // question
    a.Question()

    // response
    for _, r := range a.rr {
        // response

        // two types mixed together here
        // c1   CNAME arec
        // arec A     Ip.ad.d.r 
        isIp := rIp4.MatchString(r[1])

        a.labelize(r[0])

        if isIp {
            a.body[a.i+1] = A
        } else {
            a.body[a.i+1] = CNAME
        }
        a.body[a.i+3] = IN
        a.body[a.i+7] = TTL
        a.i += 8

        // A rec
        if isIp {
            _, err := a.labelizeIp(r[1])
            if err != nil {
                return nil, err
            }

            continue
        }

        // CNAME
        // needs 2 bytes for total length
        tl := a.i
        a.i += 2

        l, _ := a.labelize(r[1])

        // input length at 2nd byte
        a.body[tl+1] = byte(l)
    }

    // additional
    a.additional()

    return a, nil
}

func NewPtr(q, soa string) *Answer {

    // TODO: looks like that PTR can also have many answers.. :/

    a := &Answer{[][]string{[]string{q, soa}},
                 make([][]string, 0),
                 make([]byte, HEADER_LEN),
                 make([]byte, PACKET_SIZE-HEADER_LEN),
                 make(map[string]int),
                 0,
                 PTR}

    if debug {
        cDebg.Print("New PTR: " + a.QandR())
    }

    // headers
    a.setRespHeaders()

    // question
    a.Question()

    // response
    a.labelize(a.QuestionString())

    a.body[a.i+1] = PTR
    a.body[a.i+3] = IN
    a.body[a.i+7] = TTL
    // index a.i+8,9 is for total length
    // of SOA labels below
    a.i += 10

    x, p := a.labelize(a.ResponseString())
    if !p {
        // needs root(0)
        a.i++
        x++
    }

    // total length
    a.body[a.i-1-x] = byte(x)

    // additional
    a.additional()

    return a
}

func NewA(h string, ip []string) (*Answer, error) {

    // TODO: Make sure you don't have the same IPs for multi records

    rr := make([][]string, len(ip))
    for i, p := range ip {
        rr[i] = []string{h, p}
    }

    a := &Answer{rr,
                 make([][]string, 0),
                 make([]byte, HEADER_LEN),
                 make([]byte, PACKET_SIZE-HEADER_LEN),
                 make(map[string]int),
                 0,
                 A}

    if debug {
        cDebg.Print("New A: " + a.QandR())
    }

    // headers
    a.setRespHeaders()

    // question
    a.Question()

    for _, r := range a.rr {
        // response
        a.labelize(r[0])

        a.body[a.i+1] = A
        a.body[a.i+3] = IN
        a.body[a.i+7] = TTL
        a.i += 8

        _, err := a.labelizeIp(r[1])
        if err != nil {
            return nil, err
        }
    }

    // additional
    a.additional()

    return a, nil
}

func NewNxdomain(q string) *Answer {
    a := &Answer{make([][]string, 1),
                 make([][]string, 0),
                 make([]byte, HEADER_LEN),
                 make([]byte, PACKET_SIZE-HEADER_LEN),
                 make(map[string]int),
                 0,
                 NXDOMAIN}

    a.setRespHeaders()

    lbl := strings.Split(q, ".")

    // work out soa
    // from last label of hostname
    soalbl := lbl[len(lbl)-1]

    switch soalbl {
    case "org": a.rr[0] = []string{q, ORG}
    case "cz":  a.rr[0] = []string{q, CZ}
    case "au":  a.rr[0] = []string{q, AU}
    default:    a.rr[0] = []string{q, COM}
    }

    if debug {
        cDebg.Print("New NXDOMAIN: " + a.QandR())
    }

    // question
    a.Question()

    // response
    a.labelize(soalbl)

    a.body[a.i+1] = SOA
    a.body[a.i+3] = IN
    a.body[a.i+7] = TTL
    // index a.i+8,9 is for total length
    // of SOA labels below
    a.i += 10

    // soa has two parts and is labelized as such
    // re-use x for total length calculation
    x := 0
    for _, s := range strings.Split(a.ResponseString(), " ") {
        i, p := a.labelize(s)

        // add root to each
        // but only if no label pointer
        x += i
        if !p {
            a.i++
            x++
        }
    }

    // SOA timers
    // the conversion of uint64 to int in time.Now().Unix()
    // will fail at some point, long time from now :)

    for _, timer := range []int{int(time.Now().Unix()), REFRESH, RETRY, EXPIRE, MINIMUM} {
        j := 0
        for _, b := range intToBytes(timer, INT32) {
            a.body[a.i+j] = b
            j++
        }

        if debug {
            cDebg.Printf("SOA timer: %d, %+v", timer, a.body[a.i:a.i+j])
        }

        a.i += j

        // must also increment x
        // for total length
        x += j
    }

    // update total length which are 2 bytes in front of the soa labels
    // first byte is 0 (ignore), second is the actual total length
    // and for that reason -1 on a.i
    a.body[a.i-1-x] = byte(x)
    
    // additional section
    a.additional()

    return a
}

func (a *Answer) Question() {
    a.labelize(a.rr[0][0])

    t := byte(a.t)

    // NXDOMAIN only supports
    // A records
    if a.t == NXDOMAIN {
        t = A
    }

    // CNAME is also A
    if a.t == CNAME {
        t = A
    }

    // add type, class
    a.body[a.i+2] = byte(t)
    a.body[a.i+4] = IN
    // and move on
    a.i += 5
}

func (a *Answer) setRespHeaders() {
    // common headers
    a.header[2] |= RESP|RD
    a.header[3] |= RA

    // QD count is always 1
    a.header[5] = QDCOUNT

    // AN count
    // no answer for NXDOMAIN
    if a.t != NXDOMAIN {
        a.header[7] = byte(len(a.rr))
    }

    // AR count
    a.header[11] = ARCOUNT + byte(len(a.addi))

    if a.t == NXDOMAIN {
        // NXDOMAIN headers

        // TODO authoritative
        // this breaks query status, not sure why exactly..
        // status: NXDOMAIN
        // vs
        // status: YXRRSET
        //a.header[3] |= AA

        // RCODE Name Error
        a.header[3] |= NXDOMAIN

        // auth server count
        a.header[9] |= NSCOUNT
        return
    }

}

func (a *Answer) additional() {
    // this gives size of packet (length) if it is over the standard 512 bytes,
    // that's my understanding anyway.. (EDNS?)

    // 11 additional bytes at the end

    // bytes
    a.body[a.i+2] = 41
    a.i += 3

    // 2 bytes
    for i, b := range intToBytes(PACKET_SIZE, INT16) {
        a.body[a.i+i] = b 
    }
    a.i += 2

    // +empty bytes at the end
    a.i += 6
}

// copy request ID into local answer
func (a *Answer) CopyRequestId(q []byte) {
    if debug {
        // TODO add the actual ID here
        cDebg.Print("Updating packet header with request ID: " + a.QandR())
    }

    a.header[0] = q[0]
    a.header[1] = q[1]
}

// updates an empty packet with full bytes as per (pre-cached) answer
// using a previously declared p to hopefully save some time
// yet, returning a sub slice which may be declaring a new slice anyways..?
func (a *Answer) serializePacket(p []byte) []byte {
    i := 0
    for ; i<a.i; i++ {
        if i < HEADER_LEN {
            p[i] = a.header[i]
        }

        p[HEADER_LEN+i] = a.body[i]
    }

    return p[:HEADER_LEN+i]
}

// strictly speaking IPs do not have labels. But for the sake of
// consistency inputting IP into the packet is called labelize too
func (a *Answer) labelizeIp(ip string) (int, error) {
    // 6 bytes of length(2), ip addr octets(4)

    // skip first byte
    a.body[a.i+1] = byte(4)
    a.i += 2

    for m, o := range strings.Split(ip, ".") {
        v, err := strconv.Atoi(o)
        if err != nil {
            return 2, err
        }

        a.body[a.i+m] = byte(v)
    }
    a.i += 4

    return 6, nil
}

// This creates label structure for given string, inputs that into a.body and
// returns count of how many bytes were processed and whether label pointer was used.
// Any root(0) and/or total length details are done upstream.
var enddot *regexp.Regexp = regexp.MustCompile(`\.$`)
func (a *Answer) labelize(s string) (int, bool) {
    lbl := strings.Split(enddot.ReplaceAllString(s, ""), ".")
    ai := a.i
    pointer := false

    // position in lbl
    j := 0
    for {
        l := strings.Join(lbl[j:], ".")

        if x, ok := a.lblMap[l]; ok {
            // pointer is 2 bytes only
            // no need to do anything else

            a.body[a.i] = LABEL_POINTER
            a.body[a.i+1] = byte(HEADER_LEN+x)
            a.i += 2

            pointer = true
            break
        }

        // label

        // update a.labelMap with current label and position index
        a.lblMap[l] = a.i

        // label length
        a.body[a.i] = byte(len(lbl[j]))
        a.i++

        // update a.body with bytes from current label
        k := 0
        for ; k<len(lbl[j]); k++ {
            a.body[a.i+k] = lbl[j][k]
        }

        // move a.i forward by length of processed label
        a.i += k

        // move j forward to next label
        j++

        // stop if we have no more labels
        if j >= len(lbl) {
            break
        }
    }

    return a.i - ai, pointer
}

func (a *Answer) QuestionString() string {
    return a.rr[0][0]
}

func (a *Answer) ResponseString() string {
    var s string
    switch a.t {
    // simple question/answer
    case PTR, NXDOMAIN:
        s = a.rr[0][1]

    // composite question/answer(s)
    case CNAME:
        // collect chain of CNAMEs
        for i, ss := range a.rr {
            if i == 0 {
                s = ss[1]
                continue
            }

            s += ", " + ss[1] 
        }

    case A:
        // collect IPs
        for _, ss := range a.rr {
            if len(s) == 0 {
                s = ss[1]
                continue
            }

            s += ", " + ss[1]
        }
    }

    return s
}

func (a *Answer) QandR() string {
    return a.QuestionString() + ", " + a.ResponseString()
}

// The two below currently work only for uint16, uint32
// Technically bytesToInt() only needs uint16 but for
// sake of consistency it also supports uint32

// errors below should not happen as input either comes from an existing query
// or is defined in const.go (famous last words..)
func bytesToInt(b []byte) int {
    if len(b) == 2 {
        var i uint16
        err := binary.Read(bytes.NewReader(b), binary.BigEndian, &i)
        if err != nil {
            panic(err)
        }

        return int(i)
    }

    var j uint32
    err := binary.Read(bytes.NewReader(b), binary.BigEndian, &j)
    if err != nil {
        panic(err)
    }

    return int(j)
}

func intToBytes(i, t int) []byte {
    var err error
    buf := new(bytes.Buffer)

    switch t {
    case INT16: err = binary.Write(buf, binary.BigEndian, int16(i))
    case INT32: err = binary.Write(buf, binary.BigEndian, int32(i))
    }
    
    if err != nil {
        // this should not happen
        // as 'i' is correctly pre-defined (see const.go)
        panic(err)
    }

    return buf.Bytes()
}
