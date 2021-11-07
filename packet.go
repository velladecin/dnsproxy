package main

import (
    "net"
    "fmt"
)

const (
    packetLen = 512

    // request types
    query = 0
    answer = 1

    // headers index
    id = 0
    flags = 2
    qd = 4
    an = 6
    ns = 8
    ar = 10
)

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

// Query + Reply
// simple wrappers around byte slices to handle modifications

// Query
type Query struct {
    bytes []byte
    conn net.Addr
}

func NewQuery(b []byte, addr net.Addr) *Query {
    return &Query{b, addr}
}

func (r *Query) Trim() {
    // TODO
    return
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

func (r *Answer) Trim() {
    // TODO
    return
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
func queryHeadersModError(field string) error { return getHeadersModError(field, query) }
func answerHeadersModError(field string) error { return getHeadersModError(field, answer) }
func getHeadersModError(field string, request int) error {
    r := "query"
    if request == answer {
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
            p := make([]byte, packetLen)
            ch <- p
        }
    }()

    return ch
}
