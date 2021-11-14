package main

import (
    "net"
    "fmt"
_    "strings"
)

const (
    // TYPE
    A = iota + 1
    NS
    MD          // obsolete use MX (mail destination)
    MF          // obsolete use MX (mail forwarder)
    CNAME
    SOA
    MB          // experimental (mail box)
    MG          // experimental (mail group member)
    MR          // experimental (mail rename domain name)
    NULL        // experimental
    WKS         // well known service description
    PTR
    HINFO       // host info
    MINFO       // mailbox info
    MX
    TXT
)

func getType(i int) string {
    var t string
    switch ; {
        case i == A:    t = "A"
        case i == NS:   t = "NS"
        case i == CNAME:t = "CNAME"
        case i == SOA:  t = "SOA"
        case i == PTR:  t = "PTR"
        case i == MX:   t = "MX"
        case i == TXT:  t = "TXT"
        default:        t = "OTHER"
    }

    return t
}

const (
    // CLASS
    IN = iota + 1
    CS          // obsolete
    CH          // chaos
    HS          // hesiod
)

func getClass(i int) string {
    var t string
    switch ; {
        case i == IN:   t = "IN"
        case i == CH:   t = "CH"
        default:        t = "OTHER"
    }

    return t
}

// LABELS

/*
    // Question
    // question only
                                    1  1  1  1  1  1
      0  1  2  3  4  5  6  7  8  9  0  1  2  3  4  5
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                                               |
    /                     QNAME                     /
    /                                               /
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                     QTYPE                     |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                     QCLASS                    |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

    // Answer (DNS RR)
    // question (above) followed by N+1 answers (bellow)
                                  1  1  1  1  1  1
    0  1  2  3  4  5  6  7  8  9  0  1  2  3  4  5
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                                               |
    /                                               /
    /                     NAME                      /
    /                                               /
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                     TYPE                      |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                     CLASS                     |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                     TTL                       |
    |                                               |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                   RDLENGTH                    |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--|
    /                    RDATA                      /
    /                                               /
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
*/

const (
    QUESTION_LABEL = 12
    COMPRESSED_LABEL = 192  // 11000000
    QUERY = iota
    ANSWER
)

type packet interface {
    Type() int
    Data() []byte
}

type label struct {
    name, ttype, class string
    ttl int
}

/*
    Query   - single label
    Answer  - pair(s) of labels

;; QUESTION SECTION:
;incoming.telemetry.mozilla.org.	IN	A

;; ANSWER SECTION:
incoming.telemetry.mozilla.org.	51 IN	CNAME	telemetry-incoming.r53-2.services.mozilla.com.
telemetry-incoming.r53-2.services.mozilla.com. 300 IN CNAME telemetry-incoming-b.r53-2.services.mozilla.com.
telemetry-incoming-b.r53-2.services.mozilla.com. 300 IN	CNAME prod.ingestion-edge.prod.dataops.mozgcp.net.
prod.ingestion-edge.prod.dataops.mozgcp.net. 60	IN A 35.227.207.240
*/

type labelz struct {
    name [][]string
    ttype, class string
    ttl int
}

func getQuestionLabel(p packet) (*labelz, map[int]string, int) {
    l := &labelz{}
    index := make(map[int]string)
    label_end_pos := 0

    data := p.Data()
    for count:=12; count<len(data[12:]); {
        llen := int(data[count])

        if llen == 0 { // end of label
            l.name = append(l.name, []string{index[12]})
            l.ttype = getType(makeUint(data[count+1:count+1+2]))
            l.class = getClass(makeUint(data[count+3:count+3+2]))
            label_end_pos = count + 4 // 2 bytes type, 2 bytes class

            break
        }

        // label consists of length byte followed by that number of bytes
        // label: start = +1, end = start+length
        start := count + 1
        end := start + llen

        s := string(data[start:end])

        // index lookup
        for key, val := range index {
            index[key] = fmt.Sprintf("%s.%s", val, s)     
        }

        if _, ok := index[count]; !ok {
            index[count] = s
        }

        // next label length byte
        count = end
    }

    return l, index, label_end_pos
}

func getAnswerLabel(p packet, int pos) (*labelz, map[int]string, int) {
    l := &labelz{}
    index := make(map[int]string)
    label_end_pos := 0

    data := p.Data()
    for count:=pos; count<len(data[pos:]); {
        if data[count] == COMPRESSED_LABEL {
            pointer := int(data[count+1])
            if pointer == QUESTION_LABEL {
                l1, m1, p1 := getQuestionLabel(data)
            }
        } else if data[count] == 0 { // enf of label
        }
    }
}

func getLabel(p packet, pos int) (*label, map[int]string, int) {
    l := &label{}

    // lookup provides all parts of a label with its starting position (index in byte slice/packet). This is useful for dealing with packet compression.
    // The incoming "pos" variable is the start of label and therefore will contain the full label string.
    // m[12] = "maps.google.com"
    // m[17] = "google.com"
    // m[24] = "com"

    lookup := make(map[int]string)
    label_end_pos := 0

    data := p.Data()
    for count:=pos; count<len(data[pos:]); {
        llen := int(data[count]) // label length

        if llen == COMPRESSED_LABEL {
            pointer := int(data[count+1])
            l1, m1, p1 := getLabel(p, pointer)

            if pointer == QUESTION_LABEL {
                // don't have TTL
                l1.ttype = getType(int(makeUint16(data[count+2], data[count+3])))
                l1.class = getClass(int(makeUint16(data[count+4], data[count+5])))
                l1.ttl = int(makeUint32(data[count+5+1:count+5+1+4]))
                fmt.Printf("x: %+v\n", l1)
                fmt.Printf("x: %+v\n", m1)
                fmt.Printf("x: %+v\n", p1)
                break
            }
        } else if llen == 0 { // end of label
            l.name = lookup[pos]
            l.ttype = getType(int(makeUint16(data[count+1], data[count+2])))
            l.class = getClass(int(makeUint16(data[count+3], data[count+4])))

            label_end_pos = count + 4

            if p.Type() == ANSWER && pos != QUESTION_LABEL {
                fmt.Println(100, data[label_end_pos+1])
                fmt.Println(100, data[label_end_pos+1+1])
                fmt.Println(100, data[label_end_pos+1+2])
                fmt.Println(100, data[label_end_pos+1+3])
                fmt.Println(100, data[label_end_pos+1+4])

                l.ttl = int(makeUint32(data[label_end_pos+1:label_end_pos+1+4]))
                label_end_pos += 4
            }

            break
        }

        // 1+N of labels, each consisting of length byte followed by that number of bytes
        // label: start = +1, end = start+length
        start := count + 1
        end := start + llen

        s := string(data[start:end])

        // lookup
        for key, val := range lookup {
            lookup[key] = fmt.Sprintf("%s.%s", val, s)     
        }

        if _, ok := lookup[count]; !ok {
            lookup[count] = s
        }

        // next label length byte
        count = end
    }

    return l, lookup, label_end_pos
}

type labels struct {
    index map[int]string
    question []*label
    answer []*label

}

/*
    The below are the same RFC but latest (Nov 2021) and older versions.
    The older versions, though, do have some nice explanations.
    https://datatracker.ietf.org/doc/html/rfc6895
    https://datatracker.ietf.org/doc/html/rfc6840
    http://www.networksorcery.com/enp/rfc/rfc1035.txt

    Headers:

                                   1  1  1  1  1  1
     0  1  2  3  4  5  6  7  8  9  0  1  2  3  4  5
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                      ID                       |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |QR|   OpCode  |AA|TC|RD|RA| Z|AD|CD|   RCODE   |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                QDCOUNT/ZOCOUNT                |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                ANCOUNT/PRCOUNT                |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                NSCOUNT/UPCOUNT                |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                    ARCOUNT                    |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+

    // Qr - Type of Query
    //   0 query
    //   1 response

    // Opcode - Kind of query
    //   0 query
    //   1 inverse query (obsolete)
    //   2 status
    //   3 unassigned
    //   4 notify
    //   5 update
    //   6-15 future use

    // Aa - Authoritative Answer
    //   only used in response

    // Tc - Truncation

    // Rd - Recursion desired
    //   expected to be copied from query to response

    // Ra - Recursion available
    //   only used in response

    // Z - Future use
    //   should (must?) be 0

    // Ad - Authentic Data
    //   see https://datatracker.ietf.org/doc/html/rfc6840#section-5.7 (if interested)

    // Cd - Checking disabled
    //   expected to be copied from query to response, should be always set
    //   see https://datatracker.ietf.org/doc/html/rfc6840#section-5.9 (if interested)

    // Rcode - Response code
    //   0 no error
    //   1 format error
    //   2 server failure
    //   3 nxdomain
    //   4 not implemented (literally error code)
    //   5 refused
    //   6 name exists when it should not
    //   7 RR set exists when it should not
    //   8 RR does not exist when it should
    //   9 server not auth for zone OR not authorized
    //   10 name not in zone
    //   11-15 future use
    //   there are others, see RFC for details

    // Qdcount - Number of entries in question section

    // Ancount - Number of RRs in answer question

    // Nscount - Number of NS in authority section

    // Arcount - Number of record in additional records section
*/

const (
    id = iota << 1
    flags
    qd
    an
    ns
    ar
)

// Query + Answer
// simple wrappers around byte slices to handle headers modifications

// Query
type Query struct {
    bytes []byte
    conn net.Addr
}

func NewQuery(b []byte, addr net.Addr) *Query {
    return &Query{b, addr}
}

func (q *Query) Type() int { return QUERY }
func (q *Query) Data() []byte { return q.bytes }

func (q *Query) Label() []string {
    label, lookup, pos := getLabel(q, 12)

    fmt.Printf("q: %+v\n", label)
    fmt.Printf("q: %+v\n", lookup)
    fmt.Printf("q: %+v\n", pos)

    return []string{}
}

func (r *Query) Id() []byte { return r.bytes[id:id+2] }
func (r *Query) SetId(b []byte) error {
    if len(b) < 2 {
        return queryHeadersModError("ID")
    }

    r.bytes[id] = b[0]
    r.bytes[id+1] = b[1]
    return nil
}

func (r *Query) Flags() []byte { return r.bytes[flags:flags+2] }
func (r *Query) SetFlags(b []byte) error {
    if len(b) < 2 {
        return queryHeadersModError("FLAGS")
    }

    r.bytes[flags] = b[0]
    r.bytes[flags+1] = b[1]
    return nil
}

func (r *Query) Qd() []byte { return r.bytes[qd:qd+2] }
func (r *Query) SetQd(b []byte) error {
    if len(b) < 2 {
        return queryHeadersModError("QD")
    }

    r.bytes[qd] = b[0]
    r.bytes[qd+1] = b[1]
    return nil
}

func (r *Query) An() []byte { return r.bytes[an:an+2] }
func (r *Query) SetAn(b []byte) error {
    if len(b) < 2 {
        return queryHeadersModError("AN")
    }

    r.bytes[an] = b[0]
    r.bytes[an+1] = b[1]
    return nil
}

func (r *Query) Ns() []byte { return r.bytes[ns:ns+2] }
func (r *Query) SetNs(b []byte) error {
    if len(b) < 2 {
        return queryHeadersModError("NS")
    }

    r.bytes[ns] = b[0]
    r.bytes[ns+1] = b[1]
    return nil
}

func (r *Query) Ar() []byte { return r.bytes[ar:ar+2] }
func (r *Query) SetAr(b []byte) error {
    if len(b) < 2 {
        return queryHeadersModError("AR")
    }

    r.bytes[ar] = b[0]
    r.bytes[ar+1] = b[1]
    return nil
}


// Answer
type Answer struct {
    bytes []byte
}

func NewAnswer(b []byte) *Answer {
    return &Answer{b}
}

func (a *Answer) Type() int { return ANSWER }
func (a *Answer) Data() []byte { return a.bytes }

func (a *Answer) Label() []string {
    //label, lookup, pos := getLabel(a, 12)
    //fmt.Printf("a: %+v\n", label)
    //fmt.Printf("a: %+v\n", lookup)
    //fmt.Printf("a: %+v\n", pos)

    var l *label
    var lookup map[int]string
    pos := 12

    fmt.Printf("%d\n", a.Data())

    for count:=0; count<2; count++ {
        l, lookup, pos = getLabel(a, pos)

        fmt.Printf("a: %+v\n", l)
        fmt.Printf("a: %+v\n", lookup)
        fmt.Printf("a: %+v\n", pos)
        fmt.Println("---------")

        // pos is end of label
        // start of next one is +1
        pos++

        fmt.Println("--------")
    }
        
    return []string{}
}

/*
func (a *Answer) Label() []string {
    l := labels{index: make(map[int]string)}

    // QUESTION label (ql)
    // first byte after headers (12)
    ql := &label{}

    a_start := 0 // start of answer
    for count:=12; count<len(a.bytes[12:]); {
        llen := int(a.bytes[count]) // label length

        if llen == 0 { // end of label
            // get label type + class
            // and position of start of answer label
            ql.ttype = getType(int(makeUint16(a.bytes[count+1], a.bytes[count+2])))
            ql.class = getClass(int(makeUint16(a.bytes[count+3], a.bytes[count+4])))

            l.question = append(l.question, ql)

            a_start = count+5
            break
        }

        // 1+N of labels, each consisting of length byte followed by that number of bytes
        // label: start = +1, end = start+length
        start := count + 1
        end := start + llen

        // label + lookup index
        s := string(a.bytes[start:end])
        l.index[count] = s

        // composite label
        if ql.name == "" {
            ql.name = s
        } else {
            ql.name = fmt.Sprintf("%s.%s", ql.name, s)
        }

        // next label length byte
        count = end
    }

    fmt.Printf("%+v\n", ql)
    fmt.Printf("astart: %d\n", a_start)
    fmt.Printf("%+v\n", l)

    // ANSWER (al)
    //var aa []*label
    al := &label{}

    for count:=a_start; count<len(a.bytes[a_start:]); {
        if a.bytes[count] == 192 { // compressed label, also end of label
            pointer := int(a.bytes[count+1])
            s := l.index[pointer]

            if al.name == "" {
                al.name = s
            } else {
                al.name = fmt.Sprintf("%s.%s", al.name, s)
            }

            al.ttype = getType(int(makeUint16(a.bytes[count+2], a.bytes[count+3])))
            al.class = getClass(int(makeUint16(a.bytes[count+4], a.bytes[count+5])))
            al.ttl = int(makeUint32(a.bytes[count+5:count+5+4]))
        }

        break
    }

    fmt.Printf("%+v\n", al)

    return []string{}
}
*/

func (r *Answer) Id() []byte { return r.bytes[id:id+2] }
func (r *Answer) SetId(b []byte) error {
    if len(b) < 2 {
        return answerHeadersModError("ID")
    }

    r.bytes[id] = b[0]
    r.bytes[id+1] = b[1]
    return nil
}

func (r *Answer) Flags() []byte { return r.bytes[flags:flags+2] }
func (r *Answer) SetFlags(b []byte) error {
    if len(b) < 2 {
        return answerHeadersModError("FLAGS")
    }

    r.bytes[flags] = b[0]
    r.bytes[flags+1] = b[1]
    return nil
}

func (r *Answer) Qd() []byte { return r.bytes[qd:qd+2] }
func (r *Answer) SetQd(b []byte) error {
    if len(b) < 2 {
        return answerHeadersModError("QD")
    }

    r.bytes[qd] = b[0]
    r.bytes[qd+1] = b[1]
    return nil
}

func (r *Answer) An() []byte { return r.bytes[an:an+2] }
func (r *Answer) SetAn(b []byte) error {
    if len(b) < 2 {
        return answerHeadersModError("AN")
    }

    r.bytes[an] = b[0]
    r.bytes[an+1] = b[1]
    return nil
}

func (r *Answer) Ns() []byte { return r.bytes[ns:ns+2] }
func (r *Answer) SetNs(b []byte) error {
    if len(b) < 2 {
        return answerHeadersModError("NS")
    }

    r.bytes[ns] = b[0]
    r.bytes[ns+1] = b[1]
    return nil
}

func (r *Answer) Ar() []byte { return r.bytes[ar:ar+2] }
func (r *Answer) SetAr(b []byte) error {
    if len(b) < 2 {
        return answerHeadersModError("AR")
    }

    r.bytes[ar] = b[0]
    r.bytes[ar+1] = b[1]
    return nil
}


// helpers
func makeUint16(b1, b2 byte) uint16 {
    return uint16(b1)<<8 | uint16(b2)
}

func makeUint32(b []byte) uint32 {
    fmt.Printf("+>>>> %+v\n", b)
    if len(b) != 4 {
        panic("Not enough bytes for makeUint32()")
    }

    return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func makeUint(b []byte) int {
    var i int
    switch len(b) {
        case 2: i = int(b[0])<<8  | int(b[1])
        case 4: i = int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
        default:
            panic(fmt.Sprintf("Unsupported integer size: %d", len(b)))
    }

    return i
}

func queryHeadersModError(field string) error { return getHeadersModError(field, QUERY) }
func answerHeadersModError(field string) error { return getHeadersModError(field, ANSWER) }
func getHeadersModError(field string, request int) error {
    r := "query"
    if request == ANSWER {
        r = "answer"
    }

    return &HeadersModError{
        err: fmt.Sprintf("%s missing input bytes", field),
        request: r,
    }
}

func packetFactory(ch chan []byte) chan []byte {
    go func() {
        for {
            p := make([]byte, 512)
            ch <- p
        }
    }()

    return ch
}
