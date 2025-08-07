package main

// file config
const (
    LOCAL_HOST4     = "127.0.0.1:53"
    LOCAL_HOST6     = "[::1]:53"
    PROXY           = true
    REMOTE_HOST41   = "8.8.8.8:53" // dns.
    REMOTE_HOST42   = "8.8.4.4:53" // google (v4)
    REMOTE_HOST61   = "[2001:4860:4860::8844]:53" // dns.
    REMOTE_HOST62   = "[2001:4860:4860::8888]:53" // google (v6)
    WORKER_UDP      = 3
    WORKER_TCP      = 1
    RR_DIR          = "/etc/dpx/rr.d"
    DEFAULT_DOMAIN  = "localnet"
    SERVER_LOG      = "/var/log/dpx/server.log"
    CACHE_LOG       = "/var/log/dpx/cache.log"
    DEBUG           = false

    SERVER_RELOAD   = "on-server-reload"
    FILE_CHANGE     = "on-rr-file-change"

    // limit workers
    WORKER_MAX      = 20
)

// service config
const (
    // max/min port
    PORT_MIN = 0
    PORT_MAX = 1<<16-1

    // default port
    DNS_PORT = 53

    // seconds
    // applies to dialing to upstream
    CONNECTION_TIMEOUT = 1

    // prep this many empty packets to handle incoming requests
    // at max workers this is 5 per worker
    PACKET_PREP_Q_SIZE = 100

    // prep this many connection strings for upsream dialing
    DIALER_PREP_Q_SIZE = 50

    // drop user when running
    SERVICE_OWNER = "nobody"

    // net
    IPv4 = "ipv4"
    IPv6 = "ipv6"
)


// packet
const (
    // default size of DNS UDP packet
    PACKET_SIZE = 2048

    // index 0-1
    QUERY_ID_LEN = 2

    // index 0-11
    HEADER_LEN = QUERY_ID_LEN + 10

    // index 12
    QUESTION_START = HEADER_LEN

    // according to docs this number can be (any?) above 190
    // but I've not seen it other than 192
    LABEL_POINTER = 192

    // length of (label) length which is 2 bytes
    LEN_LEN = 2

    // type
    A       = 1
    CNAME   = 5
    SOA     = 6
    PTR     = 12
    MX      = 15
    TXT     = 16
    AAAA    = 28

    // RCODE
    FMTERROR = 1
    SERVFAIL = 2
    NXDOMAIN = 3
    REFUSED  = 5

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
    // MX priority
    MXPRIO  = 25

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

// file perms
// https://github.com/phayes/permbits/blob/master/permbits.go
// https://stackoverflow.com/questions/28969455/how-to-properly-instantiate-os-filemode
// TODO: fix the casing on the below
const (
    ownerR  uint32 = 1<<8
    ownerW  uint32 = 1<<7
    ownerX  uint32 = 1<<6

    groupR  uint32 = 1<<5
    groupW  uint32 = 1<<4
    groupX  uint32 = 1<<3

    otherR  uint32 = 1<<2
    otherW  uint32 = 1<<1
    otherX  uint32 = 1<<0
)
