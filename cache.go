package main

import (
    "fmt"
    "os"
    "bufio"
    "regexp"
    "strings"
    "sync"
    "errors"
)

type Cache struct {
    // cache
    pool map[string]*Answer

    // cache reload lock
    mux *sync.RWMutex

    // file with local RR
    file  string

    // default domain
    domain string
}

var rDot = regexp.MustCompile(`\.`)
var rIp4 = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
var rHost = regexp.MustCompile(`^[a-zA-Z0-9\-\.]+$`)

func NewCache(rr, domain string) *Cache {
    c := &Cache{
        make(map[string]*Answer),
        &sync.RWMutex{},
        rr,
        domain,
    }

    c.Load(true)
    return c
}

// cache.Load() panics on errors on server start up
// otherwise errors and won't update

func (c *Cache) Load(init bool) {
    if init {
        cInfo.Print("Initializing cache")
    } else {
        cInfo.Print("Reloading cache")
    }

    start := regexp.MustCompile(`^\s+`)
    mid   := regexp.MustCompile(`\s+`)
    end   := regexp.MustCompile(`\s+$`)
    rTTL  := regexp.MustCompile(`^ttl\:\d+$`)

    answers := make(map[string]*Answer)

    fh, err := os.Open(c.file)
    if err != nil {
        // it is not strictly necessary to have local RRs defined, even though there's no real reason to dns-proxy then :)
        // and so if the RR file does not exist, notify the log about it but continue on.
        // Perhaps, the file will show up later and cache will be loaded then..

        if errors.Is(err, os.ErrNotExist) {
            cCrit.Print("Could not load cache, file does not exist: " + c.file)
            return
        }

        if init {
            panic(err)
        }

        cCrit.Print("Could not load cache: " + err.Error())
        return

    }
    defer fh.Close()

    // composites records
    // CNAME, A
    cn := make(map[string]string)
    an := make(map[string][]string)

    fail := false
    scanner := bufio.NewScanner(fh)
    for scanner.Scan() {
        line := scanner.Text()

        if ok := comment.MatchString(line); ok {
            continue
        }
        if ok := empty.MatchString(line); ok {
            continue
        }

        line = start.ReplaceAllString(line, "")
        line = mid.ReplaceAllString(line, " ")
        line = end.ReplaceAllString(line, "")

        sl := strings.Split(line, " ")

        // require at least 2 columns
        if len(sl) < 2 {
            cCrit.Print("Invalid resource record line: " + line)
            fail = true
            break
        }

        // add default domain if needed
        if ok := rDot.MatchString(sl[0]); !ok {
            sl[0] += "."
            sl[0] += c.domain
        }

        // check hostname
        if ok := rHost.MatchString(sl[0]); !ok {
            cWarn.Print("Invalid hostname: " + sl[0])
            fail = true
            break
        }

        if rIp4.MatchString(sl[0]) {
            cCrit.Print("Invalid host (looks to be IP?): " + line)
            fail = true
            break
        }

        a := true
        //auth := false
        ptr := false 
        cname := false

        if sl[1] == "nxdomain" {
            if len(sl) > 2 {
                cCrit.Print("Flags do not make sense with NXDOMAIN: " +  line)
                fail = true
                break
            }

            answers[sl[0]] = NewNxdomain(sl[0])
            continue
        }

        for _, f := range sl[2:] {
            switch f {
            //case "auth":    auth = true
            case "ptr":     ptr = true
            case "cname":
                            a = false
                            cname = true
            default:
                // TODO
                // flags with values
                fmt.Println(f)
                if ok := rTTL.MatchString(f); ok {
                    t := strings.Split(f, ":")
                    fmt.Println(">>>>>>>>>>>>>>>>> " + t[1])
                    continue
                }

                fail = true
                break
            }
        }

        if fail {
            cCrit.Print("Unknown flag(s): " + line)
            break
        }

        if ptr && cname {
            cCrit.Print("Invalid definition: PTR+CNAME: " + line)
            fail = true
            break
        }

        // we don't support standalone PTR
        // therefore if PTR then also A
        if a {
            if ok := rIp4.MatchString(sl[1]); !ok {
                cCrit.Print("Invalid IP addr: " + line)
                fail = true
                break
            }

            // use these later for
            // for CNAME definition
            an[sl[0]] = append(an[sl[0]], sl[1])
        }

        if ptr {
            // must be valid since 'if a {}' above passed
            iaa := InAddrArpa(sl[1])
            answers[iaa] = NewPtr(iaa, sl[0])
            continue
        }

        if cname {
            cn[sl[0]] = sl[1]
            continue
        }
    }

    if err := scanner.Err(); err != nil {
        if init {
            panic(err)
        }

        cCrit.Print("Could not read RR file: " + c.file + ": " + err.Error())
        return
    }

    if fail {
        err := "!!!!! ERROR: Won't load cache"

        if init {
            panic(err)
        }

        cCrit.Print(err)
        return
    }

    // process A records
    // order counts and A must be done before CNAME
    for h, ips := range an {
        a, err := NewA(h, ips)
        if err != nil {
            if init {
                panic(err)
            }

            cCrit.Print("Could not process A record: " + h + ", " + err.Error())
            return
        }

        answers[a.QuestionString()] = a
    }

    // process CNAMEs
    for h1, h2 := range cn {
        // chain on 2nd hostname
        n, err := cnameChain(h2, cn, answers)
        if err != nil {
            if init {
                panic(err)
            }

            cCrit.Print(err.Error())
            return
        }

        n = append(n, "")
        copy(n[1:], n[0:])
        n[0] = h2

        a, err := NewCname(h1, n, answers)
        if err != nil {
            if init {
                panic(err)
            }

            cCrit.Print(err.Error())
            return
        }

        answers[a.QuestionString()] = a
    }

    cInfo.Printf("Cache entries loaded: %d", len(answers))

    // safe reload
    if debug {
        cDebg.Print("Locking and reloading cache")
    }

    c.mux.RLock()
    c.pool = answers
    c.mux.RUnlock()
}

func (c *Cache) Get(s string) *Answer {
    // don't think this is needed
    // safe read
    //c.mux.RLock()
    //defer c.mux.RUnlock()

    if a, ok := c.pool[s]; ok {
        if debug {
            cDebg.Print("Found in cache: " + s)
        }

        return a
    }

    if debug {
        cDebg.Print("Not found in cache: " + s)
    }

    return nil
}

func InAddrArpa(ip string) string {
    o := strings.Split(ip, ".")
    return fmt.Sprintf("%s.%s.%s.%s.in-addr.arpa", o[3], o[2], o[1], o[0])
}

func cnameChain(s string, cn map[string]string, answers map[string]*Answer) ([]string, error) {
    r := make([]string, 0)

    if _, ok := cn[s]; ok {
        r = append(r, cn[s])

        next, err := cnameChain(cn[s], cn, answers) 
        if err != nil {
            return nil, err
        }

        r = append(r, next...)
    } else {
        if _, ok := answers[s]; !ok {
            fmt.Println("ERROR ERROR: " + s)
            return nil, fmt.Errorf("Cannot find A record: " + s)
        }
    }

    return r, nil
}
