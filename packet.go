package main

import (
    "net"
    "fmt"
)

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
    //QUERY = iota
    //QUESTION = iota
    //ANSWER
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

func (q *Query) Data() []byte { return q.bytes }
func (q *Query) Type() int {
    i, _ := getBit(q.bytes[2], QR)
    return i
}

// Answer
type Answer struct {
    bytes []byte
}

func NewAnswer(b []byte) *Answer {
    return &Answer{b}
}

func (a *Answer) Data() []byte { return a.bytes }
func (a *Answer) Type() int {
    i, _ := getBit(a.bytes[2], QR)
    return i
}

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

//
// Flags

// qr
func (p *pskel) SetQuery() { p.header[2] |= 128 }
func (p *pskel) SetAnswer() { p.header[2], _ = unsetBit(p.header[2], 7) }
/*
func queryHeadersModError(field string) error { return getHeadersModError(field, QUESTION) }
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
*/

// opcode
func (p *pskel) UnsetOpcode() { p.header[2], _ = unsetBit(p.header[2], 6, 5, 4, 3) }
func (p *pskel) SetOpcode(i int) error {
    if i < QUERY || i > UPDATE {
        j, _ := getBit(p.header[2], QR)

        return &HeadersModError{
            err: fmt.Sprintf("OPCODE set to invalid value(%d)", i),
            request: j,
        }
    }

    p.UnsetOpcode()
    p.header[2] |= uint8(i)
    return nil
}

// rcode
func (p *pskel) UnsetRcode() { p.header[3], _ = unsetBit(p.header[3], 3, 2, 1, 0) }
func (p *pskel) SetRcode(i int) error {
    if i < NOERR || i > NOTZONE {
        j, _ := getBit(p.header[2], QR)

        return &HeadersModError{
            err: fmt.Sprintf("RCODE set to invalid value(%d)", i),
            request: j,
        }
    }

    p.UnsetRcode()
    p.header[3] |= uint8(i)
    return nil
}
func (p *pskel) SetNoErr() { p.UnsetRcode() } // noerr is 0
func (p *pskel) SetFormErr() { p.SetRcode(FORMERR) }
func (p *pskel) SetServFail() { p.SetRcode(SERVFAIL) }
func (p *pskel) SetNxDomain() { p.SetRcode(NXDOMAIN) }
func (p *pskel) SetNotImp() { p.SetRcode(NOTIMP) }
func (p *pskel) SetRefused() { p.SetRcode(REFUSED) }

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
            // verify end
            if (d[count] | d[count+1] | d[count+2]) == LABEL_END {
                skel.footer = d[count:]
                break
            }

            count += 7 // type, class, ttl
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

// helpers
func unsetBit(b byte, pos ...int) (byte, error) {
    if len(pos) < 1 || len(pos) > 8 {
        return b, fmt.Errorf("1-8 bits in byte, got: %d", len(pos))
    }

    bb := b
    for _, p := range pos {
        pi := int(p)
        if pi < 0 || pi > 7 {
            return bb, fmt.Errorf("0-7 bit indexes in byte, got: %d", pi)
        }

        b |= (1<<pi)
        b ^= (1<<pi)
    }

    return b, nil
}

func getBit(b byte, pos int) (int, error) {
    if pos < 0 || pos > 7 {
        return -1, fmt.Errorf("0-7 bit indexes in byte, got: %d", pos)
    }

    if (b & (1<<pos)) == 0 {
        return 0, nil
    }

    return 1, nil
}

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

/*
func queryHeadersModError(field string) error { return getHeadersModError(field, QUESTION) }
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
*/

func packetFactory(ch chan []byte) chan []byte {
    go func() {
        for {
            p := make([]byte, 512)
            ch <- p
        }
    }()

    return ch
}
