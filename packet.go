package main

import (
    "net"
    "fmt"
)

/*
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
    switch ; {
        case i == A:    return "A"
        case i == NS:   return "NS"
        case i == CNAME:return "CNAME"
        case i == SOA:  return "SOA"
        case i == PTR:  return "PTR"
        case i == MX:   return "MX"
        case i == TXT:  return "TXT"
    }

    return "OTHER"
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
*/

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
    LABEL_END = 41
    QUERY = iota
    ANSWER
)

type packet interface {
    Type() int
    Data() []byte
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

type pskel struct {
    header, question, footer []byte
    rr [][]byte
}

func (p *pskel) Id()      []byte { return p.header[:2] }
func (p *pskel) Flags()   []byte { return p.header[2:4] }
func (p *pskel) Qdcount() []byte { return p.header[4:6] }
func (p *pskel) Ancount() []byte { return p.header[6:8] }
func (p *pskel) Nscount() []byte { return p.header[8:10] }
func (p *pskel) Arcount() []byte { return p.header[10:12] }
func (p *pskel) Question()[]byte { return p.question }
func (p *pskel) Rr()    [][]byte { return p.rr }

func (p *pskel) SetNxDomain() { p.header[3] |= NXDOMAIN } // opcode

/*
func (p *pskel) Qr() byte { return p.header[13] }
func (p *pskel) QrQuery() { }
func (p *pskel) QrAnswer() { }
*/

func PacketAutopsy(p packet) (*pskel, error) {
    d := p.Data()
    fmt.Printf("ALL: %d\n\n", d)

    // header
    skel := &pskel{header: d[:QUESTION_LABEL]}

    // question
    for count:=QUESTION_LABEL; count<len(d); count++ {
        if d[count] == 0 {
            // end of question followed by 2+2 bytes of type, class
            skel.question = d[QUESTION_LABEL:count+5]
            break
        }
    }

    cur_pos := len(skel.header) + len(skel.question)

    // question label ends here
    // type QUERY has got nothing else to do
    if p.Type() == QUERY {
        // verify end, is this really needed?
        if (d[cur_pos] | d[cur_pos+1] | d[cur_pos+2]) != LABEL_END {
            return skel, fmt.Errorf("QUERY packet corrupted")
        }

        skel.footer = d[cur_pos:]
        return skel, nil
    }

    // answer
    // L1                  TTL CLASS     TYPE   L2
    // c1.domain.com.		10	  IN	CNAME	c2.domain.com.
    // c2.domain.com.       10    IN        A   1.1.1.1

    for count:=cur_pos; count<len(d); count++ {
        // 0 is end of L1 (first byte of TYPE)
        // 2 bytes of TYPE, 2 bytes of CLASS, 4 bytes of TTL
        // 2 bytes of total length of L2
        if d[count] == 0 {
            fmt.Printf("===================: %d\n", count)
            // verify end
            if (d[count] | d[count+1] | d[count+2]) == LABEL_END {
                skel.footer = d[count:]
                break
            }

            count += 7 // type, class, ttl
            fmt.Printf("===================: %d\n", count)
            L2_len := makeUint(d[count+1:count+1+2]) // L2 length, 2 bytes + 1 to accommodate range syntax
            count += 2
            count += L2_len // end

            // don't increment count any further,
            // it'll increment itself on top of loop
            skel.rr = append(skel.rr, d[cur_pos:count+1])
            cur_pos = count + 1
        }
    }

    return skel, nil
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
