package main

import (
    "net"
    "time"
    "syscall"
    // had to link in go-<ver>/src
    // golang.org -> cmd/vendor/golang.org
    "golang.org/x/sys/unix"
    "strings"
    "path/filepath"
    "os"
    "os/signal"
    "os/user"
    "strconv"
)

// debug
var debug bool

// server, cache log
var sInfo, sWarn, sCrit, sDebg Logger
var cInfo, cWarn, cCrit, cDebg Logger


//
// server

type Server struct {
    // listening processes
    worker []Worker

    // disk config
    cfg *cfg

    // socket config
    netcfg net.ListenConfig
}

func NewServer(config string, stdout bool) Server {
    conf, warn, err := newCfg(config)
    if err != nil {
        panic(err)
    }

    // global debug
    debug = conf.debug

    // CLI overwrites config file
    if stdout {
        conf.serverLog = STDOUT
        conf.cacheLog = STDOUT
    }

    cInfo, cWarn, cCrit, cDebg = NewHandles(conf.cacheLog)
    sInfo, sWarn, sCrit, sDebg = NewHandles(conf.serverLog)

    // RR files
    // must be world readable otherwise 'nobody' will not
    // be able to stat() the files for changes

    // don't capture (returned) err from filepath.Walk
    // as we panic() on all errors

    rf := make([]string, 0)
    filepath.Walk(conf.rrDir, func(path string, fi os.FileInfo, err error) error {
        // this can only happen when rr.dir
        // does not exist
        if fi == nil {
            panic("Cannot find: " + path)
        }

        // check first then work with .rr files here
        // for that reason panic() is not expected on newFstat()
        if !fi.IsDir() {
            if ok := rrx.MatchString(path); ok {
                fs := newFstat(path)
                if !fs.worldReadable() {
                    panic("Must be world readable: " + fs.path)
                }

                rf = append(rf, path)
            }
        }

        return nil
    })

    if warn != nil {
        sWarn.Printf("== Server Configuration Warning ==")
        for _, w := range warn {
            sWarn.Print(w)
        }
    }

    sInfo.Printf("== Server Configuration ==")

    // listeners, dialers setup based on
    // availability of IPv4/6 and config of the same
    if conf.validNet4() {
        sInfo.Printf("Listener v4: %s", strings.Join(conf.localNetConnString4(), ", "))
    }

    if conf.validNet6() {
        sInfo.Printf("Listener v6: %s", strings.Join(conf.localNetConnString6(), ", "))
    }

    sInfo.Printf("Proxy: %v", conf.proxy)

    if conf.proxy {
        if conf.validNet4() {
            sInfo.Printf("Proxy dialer v4: %s", strings.Join(conf.remoteNetConnString4(), ", "))
        }
        if conf.validNet6() {
            sInfo.Printf("Proxy dialer v6: %s", strings.Join(conf.remoteNetConnString6(), ", "))
        }
    }
    sInfo.Printf("UDP Workers: %d", conf.workerUDP)
    sInfo.Printf("TCP Workers: %d", conf.workerTCP)
    sInfo.Printf("Resource records (rr) files: %s", strings.Join(rf, ", "))
    sInfo.Printf("Cache update: %s", conf.cacheUpdate)
    sInfo.Printf("Default domain: %s", conf.defaultDomain)
    sInfo.Printf("Server log: %s", conf.serverLog)
    sInfo.Printf("Cache log: %s", conf.cacheLog)
    sInfo.Printf("Debug: %v", conf.debug)

    //
    // build up server

    srv := Server{
        worker: make([]Worker, 0),
        cfg:    conf,
        netcfg: net.ListenConfig{
            Control: func (net, addr string, c syscall.RawConn) error {
                return c.Control(func(fd uintptr) {
                    // SO_REUSEADDR
                    err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
                    if err != nil {
                        panic(err)
                    }

                    // SO_REUSEPORT
                    err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
                    if err != nil {
                        panic(err)
                    }
                })
            },
        },
    }

    cache := NewCache(conf.defaultDomain, rf)
    if debug {
        cache.Dump()
    }

    // dialers
    // do these regardless of proxy
    // as they simply won't be used
    d4 := make(chan string, DIALER_PREP_Q_SIZE)
    d6 := make(chan string, DIALER_PREP_Q_SIZE)
    if conf.proxy {
        go func(d chan string) {
            for {
                d <- srv.cfg.remoteNetConnDialer4()
            }
        }(d4)

        go func(d chan string) {
            for {
                d <- srv.cfg.remoteNetConnDialer6()
            }
        }(d6)
    }

    // packeter
    packeter := make(chan []byte, PACKET_PREP_Q_SIZE)
    go func(c chan []byte) {
        for {
            c <- make([]byte, PACKET_SIZE)
        }
    }(packeter)

    // start worker on each
    // configured net interface

    j := 0
    for i:=0; i<conf.workerUDP; i++ {
        if conf.validNet4() {
            for _, iface := range srv.cfg.localNetConnString4() {
                w := NewWorkerUDP()
                err := w.Start4(srv.netcfg, iface, srv.cfg.proxy, cache, packeter, d4, j)
                if err != nil {
                    panic(err)
                }

                srv.worker = append(srv.worker, w)
                j++
            }
        }

        if conf.validNet6() {
            for _, iface := range srv.cfg.localNetConnString6() {
                w := NewWorkerUDP()
                err := w.Start6(srv.netcfg, iface, srv.cfg.proxy, cache, packeter, d6, j)
                if err != nil {
                    panic(err)
                }

                srv.worker = append(srv.worker, w)
                j++
            }
        }
    }

    for i:=0; i<conf.workerTCP; i++ {
        if conf.validNet4() {
            for _, iface := range srv.cfg.localNetConnString4() {
                w := NewWorkerTCP()
                err := w.Start4(srv.netcfg, iface, srv.cfg.proxy, cache, packeter, d4, j)
                if err != nil {
                    panic(err)
                }

                srv.worker = append(srv.worker, w)
                j++
            }
        }

        if conf.validNet6() {
            for _, iface := range srv.cfg.localNetConnString6() {
                w := NewWorkerTCP()
                err := w.Start6(srv.netcfg, iface, srv.cfg.proxy, cache, packeter, d6, j)
                if err != nil {
                    panic(err)
                }

                srv.worker = append(srv.worker, w)
                j++
            }
        }
    }

    // signals

    sigch := make(chan os.Signal, 1)
    signal.Notify(sigch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGHUP)

    go func(ch chan os.Signal, c *Cache) {
        for {
            sig := <-ch
            sInfo.Printf("Received signal: %s", sig)

            if sig != syscall.SIGHUP {
                // graceful shutdown
                for i:=0; i<len(srv.worker); i++ {
                    srv.worker[i].Close()
                }

                // Logger file handles are shared across Info, Warn, Crit, Debg
                // therefore it's enough to close just one of them.
                // logger.go will correctly deal with STDOUT handles

                // cache logger
                cInfo.Print("Closing cache logger handles")
                cInfo.Close()

                // server logger
                sInfo.Print("Closing server logger handles")
                sInfo.Print("Good Bye!")
                sInfo.Close()
                
                os.Exit(0)
            }

            // reload cache
            // when configured so on SIGHUP
            if srv.cfg.cacheUpdate == SERVER_RELOAD { 
                sInfo.Printf("Reloading cache as per config")
                cache.Reload()
            }
        }
    }(sigch, cache)

    if srv.cfg.cacheUpdate == FILE_CHANGE { 
        w := make([]*fstat, len(rf))
        for i, f := range rf {
            w[i] = newFstat(f)
        }

        // RR files watcher
        go func(wf []*fstat, c *Cache) {
            for {
                reload := false
                for _, f := range wf {
                    s := newFstat(f.path)
                    if !s.exists() {
                        // file disappeared, do nothing and log
                        // TODO: this is very noisy and logs every second
                        sCrit.Printf("RR file disappeared: " + s.path)
                        reload = false
                        break
                    }

                    // content change tracking
                    // purely based on change time
                    if f.ctime != s.ctime {
                        // file changed
                        // update f
                        f.copy(s)
                        reload = true
                    }
                }

                if reload {
                    c.Reload()
                }

                time.Sleep(1 * time.Second)
            }
        }(w, cache)
    }

    return srv
}

func (s Server) Run() {
    // drop server process privs down to nobody
    // NOTE: needs to be able to read RR files

    sInfo.Printf("Dropping dpx service user privs to: %s", SERVICE_OWNER)
    uinfo, err := user.Lookup(SERVICE_OWNER)
    if err != nil {
        panic(err)
    }

    // get uid, gid
    uid, err := strconv.Atoi(uinfo.Uid)
    if err != nil {
        panic(err)
    }
    gid, err := strconv.Atoi(uinfo.Gid)
    if err != nil {
        panic(err)
    }

    // unset supplementary groups
    err = syscall.Setgroups([]int{})
    if err != nil {
        panic(err)
    }

    // set uid/gid
    err = syscall.Setgid(gid)
    if err != nil {
        panic(err)
    }
    err = syscall.Setuid(uid)
    if err != nil {
        panic(err)
    }

    // start listening for connections
    for i, w := range s.worker {
        go w.ServeDNS()
        sInfo.Printf("Listener #%d accepting %s connections on %s", i+1, w.Type(), w.ListenAddr().String())
    }

    // keep server running
    for {
        time.Sleep(1*time.Second)
    }
}
