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

// Flags byte[2,3]
const (
    Flags1 = iota + 2
    Flags2
)
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
    IQUERY              // obsolete rfc6895, 2.2. OpCode Assignment
    STATUS
    UNASSIGNED_OPCODE
    NOTIFY
    UPDATE
)

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
    NOERROR = iota
    FORMATERROR
    SERVFAIL
    NXDOMAIN
    NOTIMPLEMENTED  // query type not implemented/supported
    REFUSED
    NAME_EXIST_BUT_SHOULDNOT
    RRSET_EXIST_BUT_SHOULDNOT
    RR_NOEXIST_BUT_SHOULD
    NOAUTH
    NAME_NOT_IN_ZONE
)

const (
    QDcount1 = iota + 4
    QDcount2
    ANcount1
    ANcount2
    NScount1
    NScount2
    ARcount1
    ARcount2
)

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
