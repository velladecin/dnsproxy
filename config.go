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
//var ip4     = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
var rrx     = regexp.MustCompile(`\.rr$`)

type warning []string

// Host config

type host struct {
    name string
    port int
    proto string
}

// Accepts ipv4 net definiton
// ip.ad.d.r:port, port is optional
func NewHost4(s string) (host, error) {
    ip4a := regexp.MustCompile(`^(\d+\.\d+\.\d+\.\d+)\:?(\d+)?$`)

    // ensure to have ip/port values
    // leave full ip validation to the listener
    var i string
    var p int
    var m []string

    if m = ip4a.FindStringSubmatch(s); m == nil {
        return host{}, fmt.Errorf("Invalid IPv4 net: %s", s)
    }

    i = m[1]
    if m[2] == "" {
        p = DNS_PORT
    } else {
        // convert
        // don't need to catch err
        p, _ = strconv.Atoi(m[2])
    }

    return NewHost(i, p, IPv4)
}

// Accepts ipv6 net definition
// [ip]:port, port is optional
// ip (without [])
func NewHost6(s string) (host, error) {
    ip6a := regexp.MustCompile(`^\[([a-f0-9\:]{3,39})\]:?(\d+)?$`)
    ip6b := regexp.MustCompile(`^([a-f0-9\:]{3,39})$`)

    // ensure to have ip/port values
    // leave full ip validation to the listener
    var i string
    var p int
    var m []string

    if m = ip6a.FindStringSubmatch(s); m == nil {
        m = ip6b.FindStringSubmatch(s)
    }

    if m == nil {
        return host{}, fmt.Errorf("Invalid IPv6 net: %s", s)
    }

    i = m[1]
    switch len(m) {
    case 2:
        p = DNS_PORT
    case 3:
        if m[2] == "" {
            p = DNS_PORT
        } else {
            // convert
            // don't need to catch err
            p, _ = strconv.Atoi(m[2])
        }
    }

    return NewHost(i, p, IPv6)
}

func NewHost(ip string, port int, proto string) (host, error) {
    if port < PORT_MIN || port > PORT_MAX {
        return host{}, fmt.Errorf("Port out of range: %d", port)
    }

    // leave full ip validation to the listener

    switch proto {
    case IPv4:
        if ok := rIp4.MatchString(ip); !ok {
            return host{}, fmt.Errorf("Invalid %s: %s", proto, ip)
        }
    case IPv6:
        if ok := rIp6.MatchString(ip); !ok {
            return host{}, fmt.Errorf("Invalid %s: %s", proto, ip)
        }
    default:
        return host{}, fmt.Errorf("Invalid proto: %s", proto)
    }

    return host{ip, port, proto}, nil
}

func (h host) netConnString() string {
    var s string

    switch h.proto {
    case IPv4: s = fmt.Sprintf("%s:%d", h.name, h.port)
    case IPv6: s = fmt.Sprintf("[%s]:%d", h.name, h.port)
    }

    return s
}


// Server config

func defaultConfig() ([]host, []host, bool, []host, []host, int, int, string, string, string, string, string, bool) {
    // local connection
    h, _ := NewHost4(LOCAL_HOST4)
    lh4 := []host{h}

    h, _ = NewHost6(LOCAL_HOST6)
    lh6 := []host{h}

    // remote connection
    h1, _ := NewHost4(REMOTE_HOST41)
    h2, _ := NewHost4(REMOTE_HOST42)
    rh4 := []host{h1, h2}

    h1, _ = NewHost4(REMOTE_HOST61)
    h2, _ = NewHost4(REMOTE_HOST62)
    rh6 := []host{h1, h2}

    return lh4, lh6, PROXY, rh4, rh6, WORKER_UDP, WORKER_TCP, RR_DIR, SERVER_RELOAD, DEFAULT_DOMAIN, SERVER_LOG, CACHE_LOG, DEBUG
}

type cfg struct {
    // config file
    config string

    // local listeners
    listener4 []host
    listener6 []host

    // remote dialers
    dialer4 []host
    dialer6 []host

    // local workers (listeners)
    workerUDP int
    workerTCP int

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

    // proxy
    proxy bool
}

func newCfg(path string) (*cfg, []string, error) {
    // default config
    lh4, lh6, proxy, rh4, rh6, wUdp, wTcp, rrDir, cUpd, dDom, sLog, cLog, debug := defaultConfig()
    c := &cfg{path, lh4, lh6, rh4, rh6, wUdp, wTcp, rrDir, cUpd, dDom, sLog, cLog, debug, proxy}

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

func (c *cfg) fromDisk() (warning, error) {
    // defaults
    lh4, lh6, proxy, rh4, rh6, wUdp, wTcp, rrDir, cUpd, dDom, sLog, cLog, debug := defaultConfig()

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
        case "listener.v4", "listener.v6", "proxy.dialer.v4", "proxy.dialer.v6":
            // no want no spaces here
            s := space.ReplaceAllString(cs[1], "")

            // v4, v6 hosts
            var hosts []host
            for _, h := range strings.Split(s, ",") {
                switch cs[0] {
                case "listener.v4", "proxy.dialer.v4":
                    v4, err := NewHost4(h)
                    if err != nil {
                        panic(err)
                    }

                    hosts = append(hosts, v4)
                case "listener.v6", "proxy.dialer.v6":
                    v6, err := NewHost6(h)
                    if err != nil {
                        panic(err)
                    }

                    hosts = append(hosts, v6)
                }
            }

            // update config
            switch cs[0] {
                case "listener.v4":     lh4 = hosts
                case "listener.v6":     lh6 = hosts
                case "proxy.dialer.v4": rh4 = hosts
                case "proxy.dialer.v6": rh6 = hosts
            }

        case "proxy":
            if err := onOff(cs[1]); err != nil {
                return nil, fmt.Errorf("'proxy' %s", err.Error())
            }
            if cs[1] == "off" {
                proxy = false 
            }

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
    c.listener4 = lh4
    c.listener6 = lh6
    c.dialer4 = rh4
    c.dialer6 = rh6
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

// local host strings
// used for binding to local addresses
func (c *cfg) localNetConnString4() []string {
    return c.localNetConnString(IPv4)
}

func (c *cfg) localNetConnString6() []string {
    return c.localNetConnString(IPv6)
}

func (c *cfg) localNetConnString(net string) []string {
    var s []string

    switch net {
    case IPv4:
        for _, h := range c.listener4 {
            s = append(s, h.netConnString())
        }
    case IPv6:
        for _, h := range c.listener6 {
            s = append(s, h.netConnString())
        }
    default:
        // this should not happen
        panic("Baad net: " + net)
    }

    return s
}


/*
// local host strings are the same
func (c *cfg) localNetConnString() string {
    return c.listener.netConnString()
}

func (c *cfg) localHostString() string {
    return c.localNetConnString()
}
*/

// remote host strings
// provides random one host from the pool
// used for connection to upstream (dialer)
func (c *cfg) remoteNetConnString4() []string {
    return c.remoteNetConnString(IPv4)
}

func (c *cfg) remoteNetConnString6() []string {
    return c.remoteNetConnString(IPv6)
}

func (c *cfg) remoteNetConnString(net string) []string {
    var s []string

    switch net {
    case IPv4:
        for _, h := range c.dialer4 {
            s = append(s, h.netConnString())
        }
    case IPv6:
        for _, h := range c.dialer6 {
            s = append(s, h.netConnString())
        }
    default:
        // this should not happen
        panic("Baad net: " + net)
    }

    return s
}

// single net addr
// used for dialer connection
func (c *cfg) remoteNetConnDialer4() string {
    return c.remoteNetConnDialer(IPv4)
}

func (c *cfg) remoteNetConnDialer6() string {
    return c.remoteNetConnDialer(IPv6)
}

func (c *cfg) remoteNetConnDialer(net string) string {
    rand.Seed(time.Now().UnixMicro())
    var s string

    switch net {
    case IPv4: s = c.dialer4[rand.Intn(len(c.dialer4))].netConnString()
    case IPv6: s = c.dialer6[rand.Intn(len(c.dialer6))].netConnString()
    default:
        // this should not happen
        panic("Baad net: " + net)
    }

    return s
}



/*
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
*/

/*
// provides all upstream details
// used for logging
func (c *cfg) remoteHostString4() string {
    return c.remoteHostString(IPv4)
}

func (c *cfg) remoteHostString6() string {
    return c.remoteHostString(IPv6)
}

func (c *cfg) remoteHostString(net string) string {
    var s []string

    switch net {
    case IPv4:
        for _, h := range c.dialer4 {
            s = append(s, h.netConnString()) 
        }
    case IPv6:
        for _, h := range c.dialer6 {
            s = append(s, h.netConnString()) 
        }
    default:
        // this should not happen
        panic("Baad net: " + net)
    }

    return strings.Join(s, ", ")
}
*/


/*
func (c *cfg) remoteHostString() string {
    var s []string
    for _, h := range c.dialer {
        s = append(s, h.netConnString()) 
    }

    return strings.Join(s, ", ")
}
*/

func (c *cfg) isIpv4() bool {
    return c.isNet(IPv4)
}

func (c *cfg) isIpv6() bool {
    return c.isNet(IPv6)
}

func (c *cfg) isNet(net string) bool {
    var b bool

    switch net {
    case IPv4: b = len(c.listener4) > 0
    case IPv6: b = len(c.listener6) > 0
    default:
        panic("Baad net: " + net)
    }

    return b
}

func (c *cfg) validNet4() bool {
    return c.validNet(IPv4)
}

func (c *cfg) validNet6() bool {
    return c.validNet(IPv6)
}

func (c *cfg) validNet(net string) bool {
    var b1, b2 bool

    switch net {
    case IPv4:
        b1 = isIpv4()
        b2 = c.isIpv4()
    case IPv6:
        b1 = isIpv6()
        b2 = c.isIpv6()
    default:
        panic("Baad net: " + net)
    }

    if b1 && b2 {
        return true
    }

    return false
}
