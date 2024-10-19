package main

import (
    "fmt"
    "net"
    "strings"
    "strconv"
    "regexp"
    "os"
    "bufio"
    "time"
    "path/filepath"
    "vella/logger"
)

// TODO signals to terminate

type Proxy struct {
    // local listener
    listener net.PacketConn

    // TODO make this more than one host
    // remote host:port
    // (dialer)
    rhost string
    rport int

    // prepares and provides empty packets
    packet <-chan []byte

    // local cache
    c *Cache
}

// debug
var debug bool

// server, cache log
var sInfo, sWarn, sCrit, sDebg logger.Logger
var cInfo, cWarn, cCrit, cDebg logger.Logger

// commong regex
var comment = regexp.MustCompile(`^\s*#`)
var space   = regexp.MustCompile(`\s+`)
var empty   = regexp.MustCompile(`^\s*$`)

func NewServer(config string, dbg, stdout bool) *Proxy {
    debug = dbg

    // config defaults
    cfg := make(map[string]string)
    cfg[localHostCfg] = localHost
    cfg[localPortCfg] = localPort
    cfg[remoteHostCfg] = remoteHost
    cfg[remotePortCfg] = remotePort
    cfg[localRRCfg] = localRR
    cfg[defaultDomainCfg] = defaultDomain
    cfg[serverLogCfg] = serverLog
    cfg[cacheLogCfg] = cacheLog

    fh, err := os.Open(config)
    if err != nil {
        panic(err)
    }

    scanner := bufio.NewScanner(fh)
    for scanner.Scan() {
        line := scanner.Text()
        if ok := comment.MatchString(line); ok {
            continue
        }

        if ok := empty.MatchString(line); ok {
            continue
        }

        line = space.ReplaceAllString(line, "")

        c := strings.Split(line, "=")
        if len(c) != 2 {
            panic("Invalid config: " + line)
        }

        if _, ok := cfg[c[0]]; !ok {
            panic("Unknown config option: " + line)
        }

        cfg[c[0]] = c[1]
    }

    if err := scanner.Err(); err != nil {
        panic(err)
    }

    fh.Close()

    // config explicit
    // string
    lhost := cfg[localHostCfg]
    rhost := cfg[remoteHostCfg]
    lrr   := cfg[localRRCfg]
    dom   := cfg[defaultDomainCfg]
    slog  := cfg[serverLogCfg]
    clog  := cfg[cacheLogCfg]

    // make sure we can log
    for _, d := range []string{filepath.Dir(slog), filepath.Dir(clog)} {
        _, err := os.Stat(d)
        if err != nil {
            if os.IsNotExist(err) {
                if e := os.MkdirAll(d, os.ModePerm); e != nil {
                    panic(e)
                }

                continue
            }

            panic(err)
        }
    }

    // int
    var lport, rport int
    for i, v := range []string{localPortCfg, remotePortCfg} {
        j, err := strconv.Atoi(cfg[v])
        if err != nil {
            panic(err)
        }

        if i == 0 {
            lport = j
            continue
        }

        rport = j
    }

    // CLI overwrites config file
    if stdout {
        clog = logger.STDOUT
        slog = logger.STDOUT
    }

    cInfo, cWarn, cCrit, cDebg = logger.NewHandles(clog)
    sInfo, sWarn, sCrit, sDebg = logger.NewHandles(slog)

    sInfo.Printf("== Server Configuration ==")
    sInfo.Printf("Local host: %s", lhost)
    sInfo.Printf("Local port: %d", lport)
    sInfo.Printf("Remote host: %s", rhost)
    sInfo.Printf("Remote port: %d", rport)
    sInfo.Printf("Resource records: %s", lrr)
    sInfo.Printf("Default domain: %s", dom)
    sInfo.Printf("Server log: %s", slog)
    sInfo.Printf("Cache log: %s", clog)

    listener, err := net.ListenPacket("udp4", fmt.Sprintf("%s:%d", lhost, lport))
    if err != nil {
        panic(err)
    }

    if debug {
        sDebg.Print("Local listener up")
    }

    ch := make(chan []byte, PACKET_PREP_Q_SIZE)
    go func(c chan []byte) {
        for {
            c <- make([]byte, PACKET_SIZE)
        }
    }(ch)

    return &Proxy{listener, rhost, rport, ch, NewCache(lrr)}
}

func (p *Proxy) Accept() {

    // don't panic() in the below connections
    // client will hopefully just timeout

    for {
        query := <-p.packet
        answer := <-p.packet

        // receiver
        ql, addr, err := p.listener.ReadFrom(query) // blocking
        if err != nil {
            if ql > 0 {
                sCrit.Print("Could not read request: bytes read %d, err: %s: ", ql, err.Error())
            }

            continue
        }

        // let the request timeout if length is below 15 bytes
        if ql < 15 {
            sCrit.Printf("Query read too short, length < 15: %d, %+v", ql, query)
            continue
        }

        q := GetQuestion(query[0:ql])

        // capture (avoid) empty request
        if len(q) < 3 {
            sCrit.Printf("Invalid or empty question in query: '%+v'", q)
            continue
        }

        sInfo.Printf("Query id: %d, len: %d, question: %s", bytesToInt(query[:2]), ql, QuestionString(q))
        if debug {
            sDebg.Printf("Query id: %d, bytes: %+v", bytesToInt(query[:2]), query[0:ql])
        }

        // check local cache or
        // contact remote/dialer

        if a := p.c.Get(q); a != nil {
            a.CopyRequestId(query)
            answer = a.serializePacket(answer)

            sInfo.Printf("Resp id: %d, len: %d, answer: %s", bytesToInt(answer[:2]), len(answer), a.ResponseString())
        } else {
            if debug {
                sDebg.Print("Dialing to upstream")
            }

            // TODO
            // offload this to goroutine?

            dialer, err := net.Dial("udp4", fmt.Sprintf("%s:%d", p.rhost, p.rport))
            if err != nil {
                if debug {
                    sDebg.Print("Failed to dial upstream: " + err.Error())
                }

                continue
            }

            // request timeout
            dialer.SetDeadline(time.Now().Add(time.Second * CONNECTION_TIMEOUT))

            al := 0
            al, err = dialer.Write(query[0:ql])
            if err != nil {
                sCrit.Printf("Failed to write query to upstream, written: %d, error: %s", al, err.Error())
                continue
            }

            if debug {
                sDebg.Printf("Bytes written to dialer: %d", al)
            }

            al = 0
            al, err = dialer.Read(answer)
            if err != nil {
                sCrit.Print("Failed to read from upstream, read: %d, error: %s", al, err.Error())
                continue
            }

            dialer.Close()

            // TODO fish out answer from upstream
            sInfo.Printf("Resp id: %d, len: %d", bytesToInt(answer[:2]), al)
            answer = answer[0:al]
        }
        
        if debug {
            sDebg.Printf("Resp id: %d, bytes: %+v", bytesToInt(answer[:2]), answer)
            //sDebg.Printf("Resp id: %d, bytes: %+v", bytesToInt(answer[:2]), al, QuestionString(GetQuestion(answer[0:al])), answer[0:al])
        }

        // write answer back to client
        sInfo.Printf("Query len: %d, reply: %s", 10, "reply")

        _, err = p.listener.WriteTo(answer, addr)
        if err != nil {
            sCrit.Print("Failed to write answer back to the client: " + err.Error())
        }
    }
}
