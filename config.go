package main

import (
    "fmt"
    "bufio"
    "strings"
    "regexp"
    "os"
    "errors"
    "strconv"
    "path/filepath"
    "math/rand"
    "time"
)

var comment = regexp.MustCompile(`^\s*#`)
var space   = regexp.MustCompile(`\s+`)
var empty   = regexp.MustCompile(`^\s*$`)
var comma   = regexp.MustCompile(`,`)
var ip4     = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)

type host struct {
    name string
    port int
}

func newHosts(s string) ([]host, error) {
    var hosts []host

    for _, h := range strings.Split(s, ",") {
        hs := strings.Split(h, ":")

        if hs[0] != "" {
            if ok := ip4.MatchString(hs[0]); !ok {
                return nil, errors.New("Invalid IP: " + hs[0])
            }
        }

        port := DNS_PORT
        if hs[1] != "" {
            i, err := strconv.Atoi(hs[1])
            if err != nil {
                return nil, err
            }

            if i < PORT_MIN || i > PORT_MAX {
                return nil, errors.New("Port out of range: " + hs[1])
            }

            port = i
        }

        hosts = append(hosts, host{hs[0], port})
    }

    return hosts, nil
}

func (h host) netConnString() string {
    return fmt.Sprintf("%s:%d", h.name, h.port)
}


type cfg struct {
    // local host:port ready
    // to be used in net.Conn
    localHost host

    // local workers (listeners)
    localWorker int

    // remote hosts:port ready
    // to be used in net.Conn
    remoteHost []host

    // file of local RR
    // definitions
    localRR string

    // default domain
    defaultDomain string

    // logs
    serverLog string
    cacheLog string
}

func newCfg(config string) (*cfg, error) {
    // config defaults
    lh, _ := newHosts(LOCAL_HOST)
    c := &cfg{
        lh[0],
        WORKER,
        make([]host, 0),
        LOCAL_RR,
        DEFAULT_DOMAIN,
        SERVER_LOG,
        CACHE_LOG,
    }

    // evaluate disk config options

    fh, err := os.Open(config)
    if err != nil {
        return nil, err
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

        cs := strings.Split(line, "=")
        if len(cs) != 2 {
            return nil, errors.New("Invalid config: " + line)
        }

        switch cs[0] {
        case "local.host":
            h, err := newHosts(cs[1])
            if err != nil {
                return nil, err
            }

            if len(h) != 1 {
                return nil, errors.New("local.host must be single definition")
            }

            c.localHost = h[0]

        case "local.worker":
            i, err := strconv.Atoi(cs[1])
            if err != nil {
                return nil, err
            }

            if i > WORKER_MAX {
                return nil, fmt.Errorf("Too many workers: %d, limit %d", i, WORKER_MAX)
            }

            c.localWorker = i

        case "remote.host":
            h, err := newHosts(cs[1])
            if err != nil {
                return nil, err
            }

            c.remoteHost = h

        case "local.rr":
            // this file may or may not exist
            // therefore do not check
            c.localRR = cs[1]

        case "default.domain":
            // default domain limited
            // to 256 chars
            if len(cs[1]) > 256 {
                return nil, errors.New("default.domain definition too long")
            }
            c.defaultDomain = cs[1]

        // location check is bit further down
        case "server.log":
            c.serverLog = cs[1]

        case "cache.log":
            c.cacheLog = cs[1]

        default:
            return nil, errors.New("Unknown config option: " + line)
        }
    }

    if err := scanner.Err(); err != nil {
        return nil, err
    }

    defer fh.Close()

    // make sure we can log
    for _, d := range []string{filepath.Dir(c.serverLog), filepath.Dir(c.cacheLog)} {
        _, err := os.Stat(d)
        if err != nil {
            if os.IsNotExist(err) {
                // does not exist
                // is fine
                if e := os.MkdirAll(d, os.ModePerm); e != nil {
                    // could not create destination dir
                    // is not fine
                    return nil, e
                }

                // success creating
                // log destination dir
                continue
            }

            // other err than not-exist
            // is not fine
            return nil, err
        }
    }

    return c, nil
}

// local host strings are the same
func (c *cfg) localNetConnString() string {
    return c.localHost.netConnString()
}

func (c *cfg) localHostString() string {
    return c.localNetConnString()
}

// remote host strings are different
func (c *cfg) remoteNetConnString() string {
    // select one upstream
    // for network connection
    l := len(c.remoteHost)

    if l == 1 {
        return c.remoteHost[0].netConnString()
    }

    rand.Seed(time.Now().Unix())
    return c.remoteHost[rand.Intn(l)].netConnString()
}

func (c *cfg) remoteHostString() string {
    // provide all upstream details
    var s []string
    for _, h := range c.remoteHost {
        s = append(s, h.netConnString()) 
    }

    return strings.Join(s, ", ")
}
