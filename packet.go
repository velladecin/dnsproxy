package main

import (
    "net"
    "fmt"
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

;; QUESTION SECTION:
;incoming.telemetry.mozilla.org.	IN	A


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

;; ANSWER SECTION:
incoming.telemetry.mozilla.org.	                    51  IN  CNAME	telemetry-incoming.r53-2.services.mozilla.com.
telemetry-incoming.r53-2.services.mozilla.com.      300 IN  CNAME   telemetry-incoming-b.r53-2.services.mozilla.com.
telemetry-incoming-b.r53-2.services.mozilla.com.    300 IN	CNAME   prod.ingestion-edge.prod.dataops.mozgcp.net.
prod.ingestion-edge.prod.dataops.mozgcp.net.        60	IN  A       35.227.207.240
*/

const (
    QUESTION_LABEL = 12
    COMPRESSED_LABEL = 192  // 11000000
    END_LABEL = 41
    QUERY = iota
    ANSWER
)

type packet interface {
    Type() int
    Data() []byte
}

func rightLabel(b []byte, i map[int]string, l label) (int, map[int]string, error) {
    // 2 bytes of total label length
    // work with those only
    len_total := makeUint(b[0:2])
    data := b[0:2+len_total]
    //fmt.Printf("rightLabel:data: bytelen(%d), len_total(%d) => %+v\n", len(data), len_total, data)

    var err error
    var count int
    index := make(map[int]string)
    for count=2; count<len(data); {
        length := int(data[count])
        //fmt.Printf("------------------------ right length: %d\n", length)

        if length == 0 { // root "." label (end of labal)
            count++
            break
        }

        var s string
        var end int
        if length == COMPRESSED_LABEL {
            fmt.Println("----------- compressed -------")
            pointer := int(data[count+1])

            l, ok := i[pointer]
            if !ok {
                err = fmt.Errorf("right: Cannot find compression pointer(%d), count(%d)", pointer, count)
                break
            }

            s = l
            end = count + 2 // 2 bytes = compressed label + pointer
        } else if l.ttype == "A" && l.class == "IN" {
            for i:=count; i<count+len_total; i++ {
                if s == "" {
                    s = fmt.Sprintf("%d", data[i])
                    continue
                }

                s = fmt.Sprintf("%s.%d", s, data[i])
            }

            end = len(data)
            fmt.Printf("============>>>>: %d <> %d\n", count, end)
        } else {
            start := count + 1
            end = start + length

            s = string(data[start:end])
        }

        // index lookup
        for key, val := range index {
            index[key] = fmt.Sprintf("%s.%s", val, s)
        }

        if _, ok := index[count]; !ok {
            index[count] = s
        }

        count = end
    }

    return count, index, err
}

func leftLabel(b []byte, i map[int]string) (int, map[int]string, error) {
    index := make(map[int]string)
    var err error
    var count int
    for count=0; count<len(b); {
        length := int(b[count])

        if length == 0 { // end of left label
            break
        }

        if length == COMPRESSED_LABEL {
            count++
            pointer := int(b[count])

            l, ok := i[pointer]
            if !ok {
                err = fmt.Errorf("Cannot find compression pointer(%d), count(%d)", pointer, count)
                break
            }

            // TODO logic - can this be partial label?
            index[count] = l
            count++

            continue
        }

        start := count + 1
        end := start + length

        s := string(b[start:end])

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

    return count, index, err
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
    fmt.Printf("qdecim: %d\n", q.bytes)

    label_index := make(map[int]string)
    for count:=QUESTION_LABEL; count<len(q.bytes); {
        if makeUint(q.bytes[count:count+3]) == END_LABEL {
            //fmt.Println(">>>>>>>>>>>>>>>>>>>> END<<<<<<<<<<<<")
            break
        }

        fmt.Printf("------------------------ Query => LEFT (only) ------------------------ count: %d\n", count)
        pos, i, err := leftLabel(q.bytes[count:], label_index)
        if err != nil {
            panic(err)
        }

        // "i" is index relative to current position (count)
        // lowest index is the full label
        lowest_idx := -1
        for index, label := range i {
            idx := index + count
            label_index[idx] = label

            if lowest_idx == -1 {
                lowest_idx = idx
            } else {
                if idx < lowest_idx {
                    lowest_idx = idx
                }
            }
        }

        // question label is single (left) label
        // move to next byte after the end of it
        count += (pos + 1)

        ttype, class := getTypeClassByPosition(q, count)
        count += 4

        fmt.Printf("query.Label():QUESTION: %s %s %s\n", label_index[lowest_idx], class, ttype)
    }

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

func getTypeClassByPosition(p packet, pos int) (string, string) {
    bytes := p.Data()
    return getType(makeUint(bytes[pos:pos+2])), getClass(makeUint(bytes[pos+2:pos+2+2]))
}

func getTtlByPosition(p packet, pos int) int {
    bytes := p.Data()
    return makeUint(bytes[pos:pos+4])
}

type label struct {
    left, right string
    ttype, class string
    ttl int
}

func (a *Answer) Label() []string {
    fmt.Printf("adecim: %d\n", a.bytes)

    labelz := make([]label, 0)

    left_label := true
    label_index := make(map[int]string)
    for count:=QUESTION_LABEL; count<len(a.bytes); {
        fmt.Printf("EEEEEENDDDDDDD: %d\n", a.bytes[count:count+3])
        if makeUint(a.bytes[count:count+3]) == END_LABEL {
            fmt.Println(">>>>>>>>>>>>>>>>>>>> END<<<<<<<<<<<<")
            break
        }

        //fmt.Println(count)

        var pos int
        var i map[int]string
        var err error

        if left_label {
            fmt.Printf("------------------------ LEFT ------------------------ count: %d\n", count)
            pos, i, err = leftLabel(a.bytes[count:], label_index)
        } else {
            fmt.Printf("------------------------ RIGHT ------------------------ count: %d\n", count)
            pos, i, err = rightLabel(a.bytes[count:], label_index, labelz[len(labelz)-1])
        }

        if err != nil {
            panic(err)
        }

        // "i" is index relative to current position (count)
        // lowest index is the full label
        lowest_idx := -1
        for index, label := range i {
            idx := index + count
            label_index[idx] = label

            if lowest_idx == -1 {
                lowest_idx = idx
            } else {
                if idx < lowest_idx {
                    lowest_idx = idx
                }
            }
        }

        if left_label {
            var ttype, class string
            var ttl int
            if count == QUESTION_LABEL {
                // question label is single (left) label
                // move to next byte after the end of it
                count += (pos + 1)

                ttype, class = getTypeClassByPosition(a, count)
                count += 4

                // always first label and without right side
                fmt.Printf("QUESTION: %s %s %s\n", label_index[lowest_idx], class, ttype)
            } else {
                // answer label is left + right label set
                // and possible multiples of those
                count += pos

                ttype, class = getTypeClassByPosition(a, count)
                count += 4
                ttl = getTtlByPosition(a, count)
                count += 4

                left_label = false // end of left label

                // N+1 label - beginning of
                fmt.Printf("ANSWER left: %s %s %s %d\n", label_index[lowest_idx], class, ttype, ttl)
            }

            labelz = append(labelz, label {
                left: label_index[lowest_idx],
                class: class,
                ttype: ttype,
                ttl: ttl,
            })

            //fmt.Printf("LEFT count: %d <> pos: %d <> lowest: %d\n", count, pos, lowest_idx)
            //fmt.Printf("LEFT i: %+v\n", i)
            //fmt.Printf("LEFT %+v\n", label_index)
            //fmt.Printf("LEFT %d\n", a.bytes[count:])
            //fmt.Printf("LEFT err: %+v\n", err)
        } else {
            count += pos
            left_label = true

            // label already exists only missing right side
            labelz[len(labelz)-1].right = label_index[lowest_idx]

            fmt.Printf("ANSWER right: %s\n", label_index[lowest_idx])
            //fmt.Printf("RGHT:count: %d <> pos: %d <> lowest: %d\n", count, pos, lowest_idx)
            //fmt.Printf("RGHT:i: %+v\n", i)
            //fmt.Printf("RGHT:index: %+v\n", label_index)
            //fmt.Printf("RGHT: %d\n", a.bytes[count:])
            //fmt.Printf("RGHT:err: %+v\n", err)
        }

        if count >= 210 {
            break
        }
    }

    //fmt.Printf("%+v\n", labelz)
    for _, l := range labelz {
        fmt.Printf("%+v\n", l)
    }

    return []string{}
}

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
func makeUint(b []byte) int {
    var i int
    switch len(b) {
        case 2: i = int(b[0])<<8  | int(b[1])
        case 3: i = int(b[0])<<16 | int(b[1])<<8  | int(b[2])
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
