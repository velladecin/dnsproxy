package main

import (
    "fmt"
    "strings"
    "strconv"
    "encoding/binary"
)

type LabelMap struct {
    bytes []byte
    index map[string]int
}

func MapLabel(s string) *LabelMap {
    m := make(map[string]int)
    b := make([]byte, len(s)+1) // byte index is +1 for extra byte (length) at beginning of first label

    l:=0
    for i:=len(s)-1; i>=0; i-- {
        if s[i] == '.' {
            // s[i+1:] to not include '.' (.com vs com)
            m[s[i+1:]] = i+1+QUESTION_LABEL_START
            b[i+1] = byte(l)
            l=0
            continue
        }

        b[i+1] = s[i]
        l++
    }

    m[s] = 0+ QUESTION_LABEL_START
    b[0] = byte(l)

    return &LabelMap{b, m}
}
func (lm *LabelMap) extendIp(ip string) {
    // 2 bytes rdlength
    // 4 bytes IPv4
    b := make([]byte, 6)
    b[1] = 4
    for i, octet := range strings.Split(ip, ".") {
        o, _ := strconv.Atoi(octet)
        b[i+2] = byte(o)
    }
    // nothing to add to index
    lm.bytes = append(lm.bytes, b...)
}

// l1 is always a pointer
// l2 can be IP addr (A) or
//    can be another label (CNAME) in which case can also be partial match
func (lm *LabelMap) extend(s string, l1 bool) {
    parts := strings.Split(s, ".")
    i:=0
    for ; i<len(parts); i++ {
        str := strings.Join(parts[i:], ".")
        if _, ok := lm.index[str]; ok {
            break
        }
    }

    known := strings.Join(parts[i:], ".")
    unknown := strings.Join(parts[:i], ".")

    //fmt.Printf("i: %d <> k: %s <> u: %s\n", i, known, unknown)

    switch i {
        // full match - can only be 1st label
        // example.com A 1.1.1.1
        // example.com A 2.2.2.2
        // or
        // example.com CNAME cname.example.com
        // cname.example.com A 1.1.1.1
        case 0:
            if ! l1 {
                panic(fmt.Sprintf("L2 has full match and that should not happen: %s", known))
            }
            lm.bytes = append(lm.bytes, []byte{192, byte(lm.index[known])}...)

        // no match - can only be 2nd label
        // example.com CNAME otherexample.org
        // otherexample.org A 1.1.1.1
        case len(parts):
            if l1 {
                panic(fmt.Sprintf("L1 has no match and that should not happen: %s", unknown))
            }
            l := MapLabel(unknown)
            l.rdata(len(lm.bytes))
            lm.bytes = append(lm.bytes, l.bytes...)
            for k, v := range l.index {
                lm.index[k] = v
            }

        // partial match - can only be 2nd label (is this true?)
        // www.example.com CNAME example.com
        // example.com A 1.1.1.1
        default:
            if l1 {
                panic(fmt.Sprintf("L1 has partial match and that should not happen: %s/%s", known, unknown))
            }
            l := MapLabel(unknown)
            // create pointer to what we known
            l.bytes = append(l.bytes, []byte{192, byte(lm.index[known])}...)
            // overwrite index to include the known part
            m := make(map[string]int)
            for k, v := range l.index {
                m[fmt.Sprintf("%s.%s", k, known)] = v
            }
            l.index = m
            // call rdata()
            l.rdata(len(lm.bytes))
            lm.bytes = append(lm.bytes, l.bytes...)
            for k, v := range l.index {
                lm.index[k] = v
            }
    }
}
func (lm *LabelMap) extendSOA(mname, rname string, serial, refresh, retry, expire, ttl int) {
    // class/type/ttl
    lm.typeClassTtl(SOA, IN, ttl)
    lm.mnameRname(mname, rname)
    lm.serialRefreshRetryExpireTtl(serial, refresh, retry, expire, ttl)
}
func (lm *LabelMap) mnameRname(mname, rname string) {
    //l := MapLabel(mname)
    //fmt.Printf("==>>>> %+v\n", l)

    parts := strings.Split(mname, ".")
    i:=0
    for ; i<len(parts); i++ {
        str := strings.Join(parts[i:], ".")
        if _, ok := lm.index[str]; ok {
            break
        }
    }

    known := strings.Join(parts[i:], ".")
    unknown := strings.Join(parts[:i], ".")
    fmt.Printf("k: %s\n", known)
    fmt.Printf("u: %s\n", unknown)
    fmt.Printf("m: %+v\n", lm)

    l := MapLabel(unknown)
    fmt.Printf("l1: %+v\n", l)
    l.bytes = append(l.bytes, []byte{192, byte(lm.index[known])}...)
    fmt.Printf("l2: %+v\n", l)


    panic("end")
}
func (lm *LabelMap) serialRefreshRetryExpireTtl(serial, refresh, retry, expire, ttl int) {
    lm.bytes = append(lm.bytes,
               append(itob(32, uint64(serial)),
               append(itob(16, uint64(refresh)),
               append(itob(16, uint64(retry)),
               append(itob(16, uint64(expire)), itob(16, uint64(ttl))...)...)...)...)...)
}
func (lm *LabelMap) typeClassTtl(t, c, l int) {
    lm.bytes = append(lm.bytes,
               append(itob(16, uint64(t)),
               append(itob(16, uint64(c)), itob(32, uint64(l))...)...)...)
}


//
// helpers

func (lm *LabelMap) rdata(masterlen int) {
    // add rdlength
    b := []byte{0, byte(len(lm.bytes))}
    lm.bytes = append(b, lm.bytes...)
    // move indexes by two (bytes) to account for rdlength
    // move indexes by masterlength (bytes)
    for k, v := range lm.index {
        lm.index[k] = v+2+masterlen
    }
}
func (lm *LabelMap) finalizeQuestion() {
    // add root(.), 2 bytes type, 2 bytes class
    lm.bytes = append(lm.bytes, []byte{ROOT, 0, A, 0, IN}...)
    /*
    lm.bytes = append(lm.bytes,
               append([]byte{ROOT},
               append(itob(16, uint64(SOA)), itob(16, uint64(IN))...)...)...)
    */
}

func itob(size, i uint64) []byte {
    if i > uint64(1<<size-1) {
        panic(fmt.Sprintf("Int%d overflow: %d", size, i))
    }
    s := size/8
    b := make([]byte, s)
    switch s {
    case 1: b[0] = uint8(i)
    case 2: binary.LittleEndian.PutUint16(b, uint16(i)) 
    case 4: binary.LittleEndian.PutUint32(b, uint32(i)) 
    case 8: binary.LittleEndian.PutUint64(b, uint64(i)) 
    default:
        panic(fmt.Sprintf("Unsupported int size: %d", size))
    }
    // not sure why but PuUint produces the result in reverse
    for i, j := 0, len(b)-1; i<j; i, j = i+1, j-1 {
        b[i], b[j] = b[j], b[i]
    }
    return b
}