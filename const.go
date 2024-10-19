package main

// file config
const (
    // config options
    localHostCfg    = "local.host"
    localPortCfg    = "local.port"
    remoteHostCfg   = "remote.host"
    remotePortCfg   = "remote.port"
    localRRCfg      = "local.rr"
    defaultDomainCfg  = "default.domain"
    serverLogCfg    = "server.log"
    cacheLogCfg     = "cache.log"

    // config default values
    // NOTE: ports are strings!
    localHost       = "127.0.0.1"
    localPort       = "5353"
    remoteHost      = "10.176.226.15"
    remotePort      = "53"
    localRR         = "/etc/dns-proxy/resource-records.txt"
    defaultDomain   = "t3.internal"
    serverLog       = "/scripts/net/var/log/dns-proxy/server.log"
    cacheLog        = "/scripts/net/var/log/dns-proxy/cache.log"
)

// service config
const (
    // seconds
    // applies to dialing to upstream
    CONNECTION_TIMEOUT = 1

    // prep this many empty packets
    // to handle incoming requests
    PACKET_PREP_Q_SIZE = 10
)


// packet
const (
    // default size of DNS UDP packet
    // no EDNS (yet) :-)
    PACKET_SIZE = 1024

    // index 0-1
    QUERY_ID_LEN = 2

    // index 0-11
    HEADER_LEN = QUERY_ID_LEN + 10

    // index 12
    QUESTION_START = HEADER_LEN

    // according to docs this number can be (any?) above 190
    // but I've not seen it other than 192
    LABEL_POINTER = 192

    // type
    A       = 1
    PTR     = 12
    CNAME   = 5
    SOA     = 6

    // RCODE
    FMTERROR = 1
    SERVFAIL = 2
    NXDOMAIN = 3

    // class
    IN      = 1

    // arbitrary numbers which should not matter as client would not be localy caching answers
    // if the client does cache then 10s TTL would be good time to be still responsive to changes
    TTL     = 10
    // SOA timers should not really matter
    // (SOA SERIAL is current timestamp when cache loads)
    REFRESH = 7200
    RETRY   = 900
    EXPIRE  = 86400
    MINIMUM = 43200

    // int
    INT16    = 1<<4
    INT32    = 1<<5
)

// headers
const (
    // byte[2]
    // make it response, auth answer, recursion desired
    RESP    = 1<<7
    AA      = 1<<2
    RD      = 1

    // byte[3]
    // recursion available
    RA      = 1<<7

    // byte[5]
    // number of entries in questions
    QDCOUNT = 1

    // byte[7]
    // number of entries in answer
    ANCOUNT = 1

    // byte[9]
    // number of entries in authority
    NSCOUNT = 1

    // byte[11]
    // number of entries in additional section, we're adding "default"
    ARCOUNT = 1
)

// SOA
const (
    // .com is default
    COM = "a.gtld-servers.net. nstld.verisign-grs.com."

    // add below and update switch in answer.go
    ORG = "a0.org.afilias-nst.info. hostmaster.donuts.email."
    CZ  = "a.ns.nic.cz. hostmaster.nic.cz."
    AU  = "q.au. hostmaster.donuts.email."
)
