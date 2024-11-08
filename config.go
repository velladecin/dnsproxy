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
var rrx     = regexp.MustCompile(`\.rr$`)

// Host config

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


// Server config

func defaultConfig() (host, []host, int, string, string, string, string, string, bool) {
    lh, _ := newHosts(LOCAL_HOST)
    return lh[0], make([]host, 0), WORKER, RR_DIR, SERVER_RELOAD, DEFAULT_DOMAIN, SERVER_LOG, CACHE_LOG, false
}

type cfg struct {
    // config file
    config string

    // local host:port ready
    // to be used in net.Conn
    listener host

    // remote hosts:port ready
    // to be used in net.Conn
    dialer []host

    // local workers (listeners)
    worker int

    // Resource Records dir
    rrDir string

    // cache update/reload
    cacheUpdate string

    // default domain
    defaultDomain string

    // logs
    serverLog string
    cacheLog string

    // debug
    debug bool
}

func newCfg(config string) (*cfg, error) {
    // default config
    lHost, rHost, worker, rrDir, cUpd, dDom, sLog, cLog, debug := defaultConfig()
    c := &cfg{config, lHost, rHost, worker, rrDir, cUpd, dDom, sLog, cLog, debug}

    // disk config
    err := c.fromDisk()
    if err != nil {
        return nil, err
    }

    return c, nil
}

func (c *cfg) fromDisk() error {
    // defaults
    lHost, rHost, worker, rrDir, cUpd, dDom, sLog, cLog, debug := defaultConfig()

    // disk config
    fh, err := os.Open(c.config)
    if err != nil {
        return err
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
            return errors.New("Invalid config: " + line)
        }

        switch cs[0] {
        case "listener":
            h, err := newHosts(cs[1])
            if err != nil {
                return err
            }

            if len(h) != 1 {
                return errors.New("listener must be single definition")
            }

            lHost = h[0]

        case "dialer":
            h, err := newHosts(cs[1])
            if err != nil {
                return err
            }

            rHost = h

        case "workers":
            i, err := strconv.Atoi(cs[1])
            if err != nil {
                return err
            }

            if i > WORKER_MAX {
                return fmt.Errorf("workers over limit: %d, limit %d", i, WORKER_MAX)
            }

            worker = i

        case "rr.dir":
            // this file may or may not exist
            // therefore do not check
            rrDir = cs[1]

        case "cache.update":
            switch cs[1] {
            case SERVER_RELOAD:
            case FILE_CHANGE:
            default:
                return fmt.Errorf("cache.update unknown value: " + cs[1])
            }
            cUpd = cs[1]

        case "default.domain":
            // default domain limited
            // to 256 chars
            if len(cs[1]) > 256 {
                return errors.New("default.domain definition too long")
            }
            dDom = cs[1]

        // location check is bit further down
        case "server.log":
            sLog = cs[1]

        case "cache.log":
            cLog = cs[1]

        case "debug":
            if cs[1] != "on" && cs[1] != "off" {
                return errors.New("debug unknown value: " + cs[1])
            }

            if cs[1] == "on" {
                debug = true
            }

        default:
            return errors.New("Unknown config option: " + line)
        }
    }

    if err := scanner.Err(); err != nil {
        return err
    }

    defer fh.Close()

    // make sure we can log
    for _, d := range []string{filepath.Dir(sLog), filepath.Dir(cLog)} {
        _, err := os.Stat(d)
        if err != nil {
            if os.IsNotExist(err) {
                // does not exist
                // is fine
                if e := os.MkdirAll(d, os.ModePerm); e != nil {
                    // could not create destination dir
                    // is not fine
                    return e
                }

                // success creating
                // log destination dir
                continue
            }

            // other err than not-exist
            // is not fine
            return err
        }
    }

    // update config
    c.listener = lHost
    c.dialer = rHost
    c.worker = worker
    c.rrDir = rrDir
    c.cacheUpdate = cUpd
    c.defaultDomain = dDom
    c.serverLog = sLog
    c.cacheLog = cLog
    c.debug = debug

    return nil
}

// local host strings are the same
func (c *cfg) localNetConnString() string {
    return c.listener.netConnString()
}

func (c *cfg) localHostString() string {
    return c.localNetConnString()
}

// remote host strings are different
func (c *cfg) remoteNetConnString() string {
    // select one upstream
    // for network connection
    l := len(c.dialer)

    if l == 1 {
        return c.dialer[0].netConnString()
    }

    rand.Seed(time.Now().UnixMicro())
    return c.dialer[rand.Intn(l)].netConnString()
}

func (c *cfg) remoteHostString() string {
    // provide all upstream details
    var s []string
    for _, h := range c.dialer {
        s = append(s, h.netConnString()) 
    }

    return strings.Join(s, ", ")
}
