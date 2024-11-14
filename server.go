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
)

// TODO signals to terminate

// debug
var debug bool

// server, cache log
var sInfo, sWarn, sCrit, sDebg Logger
var cInfo, cWarn, cCrit, cDebg Logger


//
// server

type Server struct {
    // listening processes
    procs []Worker

    // prepares and provides empty packets
    packet chan []byte

    // prepares remote dialer strings (ip:port)
    // to use for upstream connection
    dialer chan string
    
    // local cache
    //cache *Cache

    // server config
    cfg *cfg

    // listening socket config
    netcfg net.ListenConfig
}


func NewServer(config string, stdout bool) Server {
    conf, err := newCfg(config)
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
    err = filepath.Walk(conf.rrDir, func(path string, fi os.FileInfo, err error) error {
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

    sInfo.Printf("== Server Configuration ==")
    sInfo.Printf("Listener: %s", conf.localHostString())
    sInfo.Printf("Dialer: %s", conf.remoteHostString())
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
        make([]Worker, conf.worker),
        make(chan []byte, PACKET_PREP_Q_SIZE),
        make(chan string, PACKET_PREP_Q_SIZE),
        conf,
        net.ListenConfig{
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
    cache.Dump()

    // signals
    sigch := make(chan os.Signal, 1)
    signal.Notify(sigch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGHUP)
    go func(ch chan os.Signal, c *Cache) {
        for {
            sig := <-ch
            sInfo.Printf("Received signal: %s", sig)

            if sig != syscall.SIGHUP {
                // terminate

                for i, w := range srv.procs {
                    sInfo.Printf("Closing listener #%d", i+1) 
                    w.listener.Close()
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
            if srv.cfg.cacheUpdate == SERVER_RELOAD { 
                sInfo.Printf("Reloading cache as per config")
                c.Load(false)
            }
        }
    }(sigch, cache)

    // packets
    go func(c chan []byte) {
        for {
            c <- make([]byte, PACKET_SIZE)
        }
    }(srv.packet)

    // dialer
    go func(c chan string) {
        for {
            c <- srv.cfg.remoteNetConnString()
        }
    }(srv.dialer)

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
                    s, err := stat(f.path)
                    if err != nil {
                        panic(err)
                    }

                    if ! f.equals(fstat{f.path, s.Ino, s.Ctim.Sec, s.Mode}) {
                        // update
                        f.inode = s.Ino
                        f.ctime = s.Ctim.Sec

                        reload = true
                    }
                }

                if reload {
                    c.Load(false)
                }

                time.Sleep(1 * time.Second)
            }
        }(w, cache)
    }

    // workers
    for i:=0; i<srv.cfg.worker; i++ {
        l, err := srv.netcfg.ListenPacket(context.Background(), "udp4", srv.cfg.localNetConnString())
        if err !=  nil {
            panic(err)
        }

        sInfo.Printf("Listener #%d socket bind: %s", i+1, l.LocalAddr().String())
        srv.procs[i] = Worker{l, cache, i+1}
    }

    return srv
}

func (s Server) Run() {
    for i, p := range s.procs {
        go p.Accept(s.dialer, s.packet)
        sInfo.Printf("Listener #%d accepting connections", i+1)
    }

    // drop server process privs down to nobody
    // need to be able to read RR file(s)

    uinfo, err := user.Lookup("nobody")
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

    // worker id
    id int
}

func (w Worker) Accept(d chan string, p chan []byte) {

    // don't panic() in the below connections
    // client will just timeout

    for {
        query := <-p
        answer := <-p

        // receiver
        ql, addr, err := w.listener.ReadFrom(query) // blocking
        if err != nil {
            if ql > 0 {
                sCrit.Print("Could not read request: bytes read %d, err: %s: ", ql, err.Error())
            }

            continue
        }

        qs := Question(query[0:ql])

        sInfo.Printf("#%d: Query id: %d, type: %s, len: %d, question: %s", w.id, bytesToInt(query[:2]), RequestTypeString(RequestType(query[0:ql])), ql, qs)
        if debug {
            sDebg.Printf("#%d: Query id: %d, bytes: %+v", w.id, bytesToInt(query[:2]), query[0:ql])
        }

        // check local cache or
        // contact remote/dialer

        if a := w.cache.Get(RequestType(query[0:ql]), qs); a != nil {
            a.CopyRequestId(query)
            answer = a.serializePacket(answer)

            sInfo.Printf("#%d: Resp id: %d, len: %d, answer: %s", w.id, bytesToInt(answer[:2]), len(answer), a.ResponseString())
        } else {
            dialer, err := net.Dial("udp4", <-d)
            if err != nil {
                if debug {
                    sDebg.Printf("#%d: Failed to dial upstream: %s", w.id, err.Error())
                }

                continue
            }

            if debug {
                sDebg.Printf("#%d: Dialing to upstream: %s", w.id, dialer.RemoteAddr().String())
            }

            // request timeout
            dialer.SetDeadline(time.Now().Add(time.Second * CONNECTION_TIMEOUT))

            al := 0
            al, err = dialer.Write(query[0:ql])
            if err != nil {
                sCrit.Printf("#%d: Failed to write query to upstream, written: %d, error: %s", w.id, al, err.Error())
                continue
            }

            if debug {
                sDebg.Printf("#%d: Bytes written to dialer: %d", w.id, al)
            }

            al = 0
            al, err = dialer.Read(answer)
            if err != nil {
                sCrit.Printf("#%d: Failed to read from upstream, read: %d, error: %s", w.id, al, err.Error())
                continue
            }

            answer = answer[0:al]

            // TODO fish out answer from upstream
            sInfo.Printf("#%d, Resp id: %d, upstream: %s, len: %d", w.id, bytesToInt(answer[:2]), dialer.RemoteAddr().String(), al)
            dialer.Close()
        }
        
        if debug {
            sDebg.Printf("#%d: Resp id: %d, bytes: %+v", w.id, bytesToInt(answer[:2]), answer)
        }

        // write answer back to client

        _, err = w.listener.WriteTo(answer, addr)
        if err != nil {
            sCrit.Printf("#%d: Failed to write answer back to the client: %s", w.id, err.Error())
        }
    }
}
