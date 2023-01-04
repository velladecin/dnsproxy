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
// splice hostname by each label (separated by '.')
// and record position index of each of those in the resulting byte slice
func MapLabel(s string) *LabelMap {
    // +1 for length at the beginning of first label
    // google . com = 6 google 3 com
    m := make(map[string]int)
    b := make([]byte, len(s)+1)

    l:=0
    for i:=len(s)-1; i>=0; i-- {
        if s[i] == '.' {
            // s[i+1:] to not include '.' (.com vs com)
            m[s[i+1:]] = QUESTION_LABEL_START+i+1
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
// add formatted IP to byte slice
// nothing to add to indexes
func (lm *LabelMap) extendIp(ip string) {
    // 2 bytes rdlength, 4 bytes ipv4
    b := make([]byte, 6)
    b[1] = 4
    for i, octet := range strings.Split(ip, ".") {
        o, _ := strconv.Atoi(octet)
        b[i+2] = byte(o)
    }
    lm.bytes = append(lm.bytes, b...)
}
func (lm *LabelMap) extendRR(l1, l2 string, typ, class, ttl int) {
    // l1 is always pointer
    lm.bytes = append(lm.bytes, []byte{COMPRESSED_LABEL, byte(lm.index[l1])}...)
    // type, class, ttl
    lm.typeClassTtl(typ, class, ttl)
    // build l2
    switch typ {
    case A:
        lm.extendIp(l2)
    case CNAME:
        parts := strings.Split(l2, ".")
        i:=0
        for ; i<len(parts); i++ {
            str := strings.Join(parts[i:], ".")
            if _, ok := lm.index[str]; ok {
                break
            }
        }

        if i == 0 {
            panic(fmt.Sprintf("L2 has full label match - this is baad: %s", l2))
        }

        known := strings.Join(parts[i:], ".")
        unknown := strings.Join(parts[:i], ".")

        l := MapLabel(unknown)
        // update global indexes
        for part, pos := range l.index {
            var idxkey string
            if i == len(parts) {
                idxkey = part
            } else {
                idxkey = fmt.Sprintf("%s.%s", part, known)
            }

            lm.index[idxkey] = len(lm.bytes)+pos+2 // extra 2 bytes of total length
        }
        // when nothing is known we need to add root
        // otherwise label pointer

        if i == len(parts) {
            // add '.' root
            l.bytes = append(l.bytes, []byte{0}...)
        } else {
            l.bytes = append(l.bytes, []byte{COMPRESSED_LABEL, byte(lm.index[known])}...)
        }
        // now prepend rdlength
        fmt.Printf("*************: %d\n", len(l.bytes))
        l.bytes = append(itobs(16, uint64(len(l.bytes))), l.bytes...)
        // now chuck it to global bytes
        lm.bytes = append(lm.bytes, l.bytes...)
    default:
        panic(fmt.Sprintf("DNS type not supported: %d", typ))
    }
}

// l1 is always a pointer
// l2 can be IP addr (A) or
//    can be another label (CNAME) in which case can also be partial match
//func (lm *LabelMap) extend(s string, l1 bool) {
func (lm *LabelMap) extend(s string) {
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

    switch i {
        // full match - can only be 1st label
        // example.com A 1.1.1.1
        // example.com A 2.2.2.2
        // or
        // example.com CNAME cname.example.com
        // cname.example.com A 1.1.1.1
        case 0:
        /*
            if ! l1 {
                panic(fmt.Sprintf("L2 has full match and that should not happen: %s", known))
            }
            */
            lm.bytes = append(lm.bytes, []byte{192, byte(lm.index[known])}...)

        // no match - can only be 2nd label
        // example.com CNAME otherexample.org
        // otherexample.org A 1.1.1.1
        case len(parts):
        /*
            if l1 {
                panic(fmt.Sprintf("L1 has no match and that should not happen: %s", unknown))
            }
            */
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
        /*
            if l1 {
                panic(fmt.Sprintf("L1 has partial match and that should not happen: %s/%s", known, unknown))
            }
            */
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
    // local byte array to hold the resulting bytes
    // we need this to be able to tell the total length of SOA
    b := make([]byte, 0)

    // decide if and what parts of these are known and/or uknown
    // either of the two (known/unknown) can be an empty string (or not)
    // and the switch further down decides which is to be used and when
    for _, s := range []string{mname, rname} {
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

        // i == 0
        //  is full match with only known part and will need label pointer
        // i == len(parts)
        //  is no match with no known part and will need full byte definition
        // i == X
        //  is partial match with both known+unknown parts and will need byte definition and label pointer

        switch i {
        case 0:
            b = append(b, []byte{COMPRESSED_LABEL, byte(lm.index[known])}...)
        default:
            l := MapLabel(unknown)
            // update global indexes
            for part, pos := range l.index {
                var idxkey string
                if i == len(parts) {
                    idxkey = part
                } else {
                    idxkey = fmt.Sprintf("%s.%s", part, known)
                }

                lm.index[idxkey] = len(lm.bytes)+pos+len(b)+2 // extra 2 bytes of total length
            }

            // save the result
            b = append(b, l.bytes...)
            if i == len(parts) {
                // add '.' root
                b = append(b, []byte{0}...)
            } else {
                // add label pointer
                b = append(b, []byte{COMPRESSED_LABEL, byte(lm.index[known])}...)
            }
        }
        fmt.Println("==============")
    }

    // update lm.bytes with the full SOA byte slice
    // 2 bytes of total length: len(b) + 20 bytes (serial, .. below)
    lm.bytes = append(lm.bytes,
               append(itobs(16, uint64(len(b)+20)), b...)...)

    // append serial, refresh, retry, expire, ttl
    lm.serialRefreshRetryExpireTtl(serial, refresh, retry, expire, ttl)
}
func (lm *LabelMap) serialRefreshRetryExpireTtl(serial, refresh, retry, expire, ttl int) {
    lm.bytes = append(lm.bytes,
               append(itobs(32, uint64(serial)),
               append(itobs(32, uint64(refresh)),
               append(itobs(32, uint64(retry)),
               append(itobs(32, uint64(expire)), itobs(32, uint64(ttl))...)...)...)...)...)
}
func (lm *LabelMap) typeClassTtl(t, c, l int) {
    lm.bytes = append(lm.bytes,
               append(itobs(16, uint64(t)),
               append(itobs(16, uint64(c)), itobs(32, uint64(l))...)...)...)
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
    // add root(.), 2 bytes each of type, class
    lm.bytes = append(lm.bytes, []byte{ROOT, 0, A, 0, IN}...)
}
// integer to byte slice
func itobs(size, i uint64) []byte {
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
    // not sure why but PutUint produces the result in reverse
    for i, j := 0, len(b)-1; i<j; i, j = i+1, j-1 {
        b[i], b[j] = b[j], b[i]
    }
    return b
}
