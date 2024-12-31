package main

import (
    "net"
    "time"
    "syscall"
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
    sInfo.Printf("Listener: %s", conf.localHostString())
    sInfo.Printf("Proxy: %v", conf.proxy)
    sInfo.Printf("Proxy dialer: %s", conf.remoteHostString())
    sInfo.Printf("UDP Workers: %d", conf.workerUDP)
    sInfo.Printf("TCP Workers: %d", conf.workerTCP)
    //sInfo.Printf("Resource records (rr) dir: %s", conf.rrDir)
    sInfo.Printf("Resource records (rr) files: %s", strings.Join(rf, ", "))
    sInfo.Printf("Cache update: %s", conf.cacheUpdate)
    sInfo.Printf("Default domain: %s", conf.defaultDomain)
    sInfo.Printf("Server log: %s", conf.serverLog)
    sInfo.Printf("Cache log: %s", conf.cacheLog)
    sInfo.Printf("Debug: %v", conf.debug)

    //
    // build up server

    srv := Server{
        worker: make([]Worker, conf.workerUDP + conf.workerTCP),
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

    // dialer
    dialer := make(chan string, PACKET_PREP_Q_SIZE)
    go func(d chan string) {
        for {
            d <- srv.cfg.remoteNetConnString()
        }
    }(dialer)

    // packeter
    packeter := make(chan []byte, PACKET_PREP_Q_SIZE)
    go func(c chan []byte) {
        for {
            c <- make([]byte, PACKET_SIZE)
        }
    }(packeter)

    // workers
    for i:=0; i<len(srv.worker); i++ {
        var w Worker
        var err error

        if i < conf.workerUDP {
            w = NewWorkerUDP()
        } else {
            w = NewWorkerTCP()
        }

        err = w.Start(srv.netcfg, srv.cfg.localNetConnString(), srv.cfg.proxy, cache, packeter, dialer, i+1)
        if err != nil {
            panic(err)
        }

        srv.worker[i] = w
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
        sInfo.Printf("Listener #%d accepting %s connections", i+1, w.Type())
    }

    // keep server running
    for {
        time.Sleep(1*time.Second)
    }
}
