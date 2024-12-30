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
    // no want no spaces here
    s = space.ReplaceAllString(s, "")

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

func defaultConfig() (host, bool, []host, int, int, string, string, string, string, string, bool) {
    lh, _ := newHosts(LOCAL_HOST)
    rh, _ := newHosts(fmt.Sprintf("%s, %s", REMOTE_HOST1, REMOTE_HOST2))
    return lh[0], PROXY, rh, WORKER_UDP, WORKER_TCP, RR_DIR, SERVER_RELOAD, DEFAULT_DOMAIN, SERVER_LOG, CACHE_LOG, DEBUG
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
    workerUDP int
    workerTCP int

    // Resource Records dir
    rrDir string

    // cache update/reload
    cacheUpdate string

    // default domain
    defaultDomain string

    // resolv.conf search
    //resolvconf []string

    // logs
    serverLog string
    cacheLog string

    // debug
    debug bool

    // proxy
    proxy bool
}

func newCfg(config string) (*cfg, []string, error) {
    // default config
    lHost, proxy, rHost, wUdp, wTcp, rrDir, cUpd, dDom, sLog, cLog, debug := defaultConfig()
    c := &cfg{config, lHost, rHost, wUdp, wTcp, rrDir, cUpd, dDom, sLog, cLog, debug, proxy}

    // disk config
    warn, err := c.fromDisk()
    if err != nil {
        return nil, warn, err
    }

    return c, warn, err
}

func readFile(path string) ([]string, error) {
    var lines []string

    fh, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer fh.Close()

    scanner := bufio.NewScanner(fh)
    for scanner.Scan() {
        line := scanner.Text()

        if ok := comment.MatchString(line); ok {
            continue
        }
        if ok := empty.MatchString(line); ok {
            continue
        }

        lines = append(lines, line)
    }

    if err := scanner.Err(); err != nil {
        return nil, err
    }

    return lines, nil
}

func (c *cfg) fromDisk() ([]string, error) {
    // defaults
    lHost, proxy, rHost, wUdp, wTcp, rrDir, cUpd, dDom, sLog, cLog, debug := defaultConfig()

    lines, err := readFile(c.config)
    if err != nil {
        panic(err)
    }

    warnings := make([]string, 0)
    pd := false
    for _, line := range lines {
        line = space.ReplaceAllString(line, "")

        cs := strings.Split(line, "=")
        if len(cs) != 2 {
            return nil, errors.New("Invalid config: " + line)
        }

        switch cs[0] {
        case "listener":
            h, err := newHosts(cs[1])
            if err != nil {
                return nil, err
            }

            if len(h) != 1 {
                return nil, errors.New("'listener' must be single definition")
            }

            lHost = h[0]

        case "proxy":
            if err := onOff(cs[1]); err != nil {
                return nil, fmt.Errorf("'proxy' %s", err.Error())
            }
            if cs[1] == "off" {
                proxy = false 
            }

        case "proxy.dialer":
            pd = true
            h, err := newHosts(cs[1])
            if err != nil {
                return nil, err
            }

            rHost = h

        case "worker.udp":
            i, err := strconv.Atoi(cs[1])
            if err != nil {
                return nil, err
            }

            if i > WORKER_MAX {
                return nil, fmt.Errorf("'worker.udp' over limit: %d (max: %d)", i, WORKER_MAX)
            }

            wUdp = i

        case "worker.tcp":
            i, err := strconv.Atoi(cs[1])
            if err != nil {
                return nil, err
            }

            if i > WORKER_MAX {
                return nil, fmt.Errorf("'worker.tcp' over limit: %d (max: %d)", i, WORKER_MAX)
            }

            wTcp = i

        case "rr.dir":
            rrDir = cs[1]

        case "cache.update":
            switch cs[1] {
            case SERVER_RELOAD:
            case FILE_CHANGE:
            default:
                return nil, fmt.Errorf("cache.update unknown value: " + cs[1])
            }
            cUpd = cs[1]

        case "default.domain":
            // default domain limited
            // to 256 chars
            if len(cs[1]) > 256 {
                return nil, errors.New("default.domain definition too long")
            }
            dDom = cs[1]

        // location check is bit further down
        case "server.log":
            sLog = cs[1]

        case "cache.log":
            cLog = cs[1]

        case "debug":
            if err := onOff(cs[1]); err != nil {
                return nil, fmt.Errorf("'debug' %s", err.Error()) 
            }

            if cs[1] == "on" {
                debug = true
            }

        default:
            return nil, errors.New("Unknown config option: " + line)
        }
    }

    // make sure rr.dir exists and world readable
    rrstat := newFstat(rrDir)
    if !rrstat.exists() {
        panic(rrstat.err)
    }
    if !rrstat.worldReadable() {
        panic("Permission denied, needs o+r: " + rrstat.path)
    }

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

    if !proxy && pd {
        warnings = append(warnings, "'proxy' disabled and 'proxy.dialer' defined (will be ignored)")
    }

    // update config
    c.listener = lHost
    c.dialer = rHost
    c.workerUDP = wUdp
    c.workerTCP = wTcp
    c.rrDir = rrDir
    c.cacheUpdate = cUpd
    c.defaultDomain = dDom
    c.serverLog = sLog
    c.cacheLog = cLog
    c.debug = debug
    c.proxy = proxy

    if len(warnings) > 0 {
        return warnings, nil
    }

    return nil, nil
}

func onOff(s string) error {
    var err error
    switch s {
    case "on", "off":
    default:
        err = errors.New("not valid (accepts: on/off)")
    }

    return err
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
