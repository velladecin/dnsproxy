package main

import (
    "net"
)

const (
    packetLen = 512

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
    |QR|   Opcode  |AA|TC|RD|RA|   Z    |   RCODE   |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                    QDCOUNT                    |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                    ANCOUNT                    |
    +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
    |                    NSCOUNT                    |
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

// Request + Reply
// simple wrappers around byte slices to handle modifications

// Request
type Request struct {
    bytes []byte
    conn net.Addr
}

func NewRequest(b []byte, addr net.Addr) *Request {
    return &Request{b, addr}
}

func (r *Request) Trim() {
    // TODO
    return
}

func (r *Request) Id() []byte { return r.bytes[id:id+2] }
func (r *Request) SetId(b []byte) {
    r.bytes[id] = b[0]
    r.bytes[id+1] = b[1]
}

func (r *Request) Flags() []byte { return r.bytes[flags:flags+2] }
func (r *Request) SetFlags(b []byte) {
    r.bytes[flags] = b[0]
    r.bytes[flags+1] = b[1]
}

func (r *Request) Qd() []byte { return r.bytes[qd:qd+2] }
func (r *Request) SetQd(b []byte) {
    r.bytes[qd] = b[0]
    r.bytes[qd+1] = b[1]
}

func (r *Request) An() []byte { return r.bytes[an:an+2] }
func (r *Request) SetAn(b []byte) {
    r.bytes[an] = b[0]
    r.bytes[an+1] = b[1]
}

func (r *Request) Ns() []byte { return r.bytes[ns:ns+2] }
func (r *Request) SetNs(b []byte) {
    r.bytes[ns] = b[0]
    r.bytes[ns+1] = b[1]
}

func (r *Request) Ar() []byte { return r.bytes[ar:ar+2] }
func (r *Request) SetAr(b []byte) {
    r.bytes[ar] = b[0]
    r.bytes[ar+1] = b[1]
}

// Response
type Response struct {
    bytes []byte
}

func NewResponse(b []byte) *Response {
    return &Response{b}
}

func (r *Response) Trim() {
    // TODO
    return
}

func (r *Response) Id() []byte { return r.bytes[id:id+2] }
func (r *Response) Flags() []byte { return r.bytes[flags:flags+2] }
func (r *Response) Qd() []byte { return r.bytes[qd:qd+2] }
func (r *Response) An() []byte { return r.bytes[an:an+2] }
func (r *Response) Ns() []byte { return r.bytes[ns:ns+2] }
func (r *Response) Ar() []byte { return r.bytes[ar:ar+2] }

// helpers
func packetFactory(ch chan []byte) chan []byte {
    go func() {
        for {
            p := make([]byte, packetLen)
            ch <- p
        }
    }()

    return ch
}
