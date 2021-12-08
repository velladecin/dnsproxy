package main

// RR Type
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
    switch ; {
        case i == A:    return "A"
        case i == NS:   return "NS"
        case i == CNAME:return "CNAME"
        case i == SOA:  return "SOA"
        case i == PTR:  return "PTR"
        case i == MX:   return "MX"
        case i == TXT:  return "TXT"
    }

    return "OTHER_T"
}

// RR Class
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

//
// Flags

// byte[2]
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


// byte[3]

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
