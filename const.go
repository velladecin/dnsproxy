package main
// see autopsy.txt for more details

/*
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
*/

// Flags byte[2]
const (
    RD = iota
    TC
    AA
    _ // opcode start
    _
    _
    _ // opcode end
    QR
)

// OPCODE
const (
    QUERY = iota
    IQUERY      // obsolete rfc6895
    STATUS
    _
    NOTIFY
    UPDATE
)

// The below relates to the QR bit and has got nothing to do with OPCODE.
// But QR is 0 for query and 1 for answer so we're piggybacking on it here..
func getRequestString(i int) string {
    if i == QUERY {
        return "QUERY"
    }

    return "ANSWER"
}


// Flags byte[3]
const (
    _ = iota // rcode start
    _
    _
    _        // rcode end
    CD
    AD
    Z
    RA
)

// RCODE
const (
    NOERR = iota
    FORMERR
    SERVFAIL
    NXDOMAIN
    NOTIMP
    REFUSED
    _
    _
    _
    NOTAUTH
    NOTZONE
)

//
// Counts
const (
    QDCOUNT = iota
    ANCOUNT
    NSCOUNT
    ARCOUNT
)
func getHeadersCount(i int) (string, error) {
    switch i {
        case QDCOUNT: return "QDCOUNT", nil
        case ANCOUNT: return "ANCOUNT", nil
        case NSCOUNT: return "NSCOUNT", nil
        case ARCOUNT: return "ARCOUNT", nil
    }

    return "", &HeadersUnknownFieldError{
        err: "Unknown headers count requested",
        val: i,
    }
}


/*
    Labels:

    Question
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

    Answer (RR)
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
    QUESTION_LABEL_START = 12
    COMPRESSED_LABEL = 192  // 11000000
    LABEL_END = 41
)

// QTYPE, RR Type
const (
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
    switch i {
        case A:     return "A"
        case NS:    return "NS"
        case CNAME: return "CNAME"
        case SOA:   return "SOA"
        case PTR:   return "PTR"
        case MX:    return "MX"
        case TXT:   return "TXT"
    }
    /*
    switch ; {
        case i == A:    return "A"
        case i == NS:   return "NS"
        case i == CNAME:return "CNAME"
        case i == SOA:  return "SOA"
        case i == PTR:  return "PTR"
        case i == MX:   return "MX"
        case i == TXT:  return "TXT"
    }
    */

    return "OTHER_T"
}

// QCLASS, RR Class
const (
    IN = iota + 1
    CS          // obsolete
    CH          // chaos
    HS          // hesiod
)

func getClass(i int) string {
    switch ; {
        case i == IN: return "IN"
        case i == CH: return "CH"
    }

    return "OTHER_C"
}
