package main

import (
    "net"
    "time"
    "syscall"
    // had to link in go-<ver>/src
    // golang.org -> cmd/vendor/golang.org
    "golang.org/x/sys/unix"
    "context"
    "strings"
    "path/filepath"
    "os"
    "os/signal"
    "os/user"
    "strconv"
    "sync"
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
    workers []*Worker

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

    cInfo, cWarn, cCrit, cDebg = NewHandles(conf.serverLog)
    sInfo, sWarn, sCrit, sDebg = NewHandles(conf.cacheLog)

    // RR files
    // must be world readable otherwise 'nobody' will not
    // be able to stat() the files for changes

    rf := make([]string, 0)
    // don't capture (returned) err from filepath.Walk
    // as we panic() on all errors
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
    sInfo.Printf("Workers: %d", conf.worker)
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
        workers:  make([]*Worker, conf.worker),
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
    for i:=0; i<srv.cfg.worker; i++ {
        w, err := NewWorker(srv.netcfg, srv.cfg.localNetConnString(), srv.cfg.proxy, cache, packeter, dialer, i+1)
        if err != nil {
            panic(err)
        }

        srv.workers[i] = w
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
                for i:=0; i<len(srv.workers); i++ {
                    // exit notify
                    close(srv.workers[i].exit)

                    // close listening socket
                    srv.workers[i].listener.Close()

                    // exit confirmation
                    <-srv.workers[i].exited
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
    for i, w := range s.workers {
        go w.Accept()
        sInfo.Printf("Listener #%d accepting connections", i+1)
    }

    // keep server running
    for {
        time.Sleep(1*time.Second)
    }
}



//
// worker

type Worker struct {
    // local listener
    listener net.PacketConn

    // cache
    cache *Cache

    // predeclared empty packets
    packeter chan []byte

    // upstream dialer
    dialer chan string

    // sync
    wg sync.WaitGroup

    // proxy
    proxy bool

    // shutdown request
    exit chan bool

    // shutdown confirmation
    exited chan bool

    // worker id
    id int
}

func NewWorker(lcfg net.ListenConfig, iface string, x bool, c *Cache, p chan []byte, d chan string, id int) (*Worker, error) {
    l, err := lcfg.ListenPacket(context.Background(), "udp4", iface)
    if err != nil {
        return nil, err
    }

    sInfo.Printf("Listener #%d socket bind: %s", id, l.LocalAddr().String())

    w := &Worker{
        listener:   l,
        cache:      c,
        packeter:   p,
        dialer:     d,
        //wg:         sync.WaitGroup,
        proxy:	    x,
        exit:       make(chan bool),
        exited:     make(chan bool),
        id:         id,
    }

    return w, nil
}

func (w *Worker) processRequest(query, answer []byte, addr net.Addr) {
    qs := Question(query)
    rt := RequestType(query)

    sInfo.Printf("#%d: Query id: %d, type: %s, len: %d, question: %s", w.id, bytesToInt(query[:2]), RequestTypeString(rt), len(query), qs)
    if debug {
        sDebg.Printf("#%d: Query id: %d, bytes: %+v", w.id, bytesToInt(query[:2]), query)
    }

    // answer length
    al := 0

    // check local cache
    // and answer with what is found
    if a := w.cache.Get(rt, qs); a != nil {
        a.CopyRequestId(query)
        al = a.serializePacket(answer)

        sInfo.Printf("#%d: Resp id: %d, len: %d, answer: %s", w.id, bytesToInt(answer[:2]), al, a.ResponseString())
    } else {
        // proxy mode
        switch w.proxy {
        case true:
            // proxy on, dial upstream
            dialer, err := net.Dial("udp4", <-w.dialer)
            if err != nil {
                if debug {
                    sDebg.Printf("#%d: Failed to dial upstream: %s", w.id, err.Error())
                }

                return
            }
            defer dialer.Close()

            if debug {
                sDebg.Printf("#%d: Dialing to upstream: %s", w.id, dialer.RemoteAddr().String())
            }

            // upstream connection timeout
            dialer.SetDeadline(time.Now().Add(time.Second * CONNECTION_TIMEOUT))

            al, err = dialer.Write(query)
            if err != nil {
                sCrit.Printf("#%d: Failed to write query to upstream, written: %d, error: %s", w.id, al, err.Error())
                return
            }

            if debug {
                sDebg.Printf("#%d: Bytes written to dialer: %d", w.id, al)
            }

            al = 0
            al, err = dialer.Read(answer)
            if err != nil {
                sCrit.Printf("#%d: Failed to read from upstream, read: %d, error: %s", w.id, al, err.Error())
                return
            }

            sInfo.Printf("#%d, X-ON, Resp id: %d, upstream: %s, len: %d, answer: %s", w.id, bytesToInt(answer[:2]), dialer.RemoteAddr().String(), al, Response(answer))
        case false:
            // proxy off, refuse
            a := NewNxdomain(qs)
            a.CopyRequestId(query)
            al = a.serializePacket(answer)

            sInfo.Printf("#%d: X-OFF, Resp id: %d, len: %d, answer: %s", w.id, bytesToInt(answer[:2]), al, a.ResponseString())
        }
    }
    
    if debug {
        sDebg.Printf("#%d: Resp id: %d, bytes: %+v", w.id, bytesToInt(answer[:2]), answer[:al])
    }

    // write answer back to client

    _, err := w.listener.WriteTo(answer[:al], addr)
    if err != nil {
        sCrit.Printf("#%d: Failed to write answer back to the client: %s", w.id, err.Error())
    }
}

func (w *Worker) Accept() {
    for {
        query := <-w.packeter

        // receiver (blocking)
        ql, addr, err := w.listener.ReadFrom(query)
        if err != nil {
            select {
            case <-w.exit:
                sInfo.Printf("Listener #%d shutting down", w.id)
                w.wg.Wait()
                close(w.exited)

                // jump out
                return

            default:
                sCrit.Printf("Could not read request: bytes read %d, err: %s: ", ql, err.Error())
            }

            continue
        }

        // offload processing
        // to free up the listener
        w.wg.Add(1)
        go func() {
            go w.processRequest(query[0:ql], <-w.packeter, addr)
            w.wg.Done()
        }()
    }
}
