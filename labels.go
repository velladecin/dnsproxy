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
func MapLabelQuestion(s string) *LabelMap {
    // essentially a copy of MapLabel
    // with additional 5 bytes of: 1b root, 2b each of type, class
    lm := MapLabel(s)
    lm.bytes = append(lm.bytes, []byte{ROOT, 0, A, 0, IN}...)
    return lm
}
func (lm *LabelMap) labelize(s string) (string, string) {
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
    return known, unknown
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
// TODO TODO
// extendRR and extendSOA are very similar and would be nice to unify those somehow.
// currently I cannot see an easy way forward as they both work just this slightly differently,
// mainly SOA has got a loop of which we need to keep track..
func (lm *LabelMap) extendRR(l1, l2 string, typ, class, ttl int) {
    // l1 is always label pointer
    // type, class, ttl are given
    lm.bytes = append(lm.bytes, []byte{COMPRESSED_LABEL, byte(lm.index[l1])}...)
    lm.typeClassTtl(typ, class, ttl)

    // l2 needs to be determined
    switch typ {
    case A:
        lm.extendIp(l2)
    case CNAME:
        // unknown == ""
        //  this should not happen!
        // known == ""
        //  no match with only unknown part and will need root '.' added
        // known, unknown != ""
        //  partial match with both known+unknown parts and will need label pointer added (to the known part)
        known, unknown := lm.labelize(l2)
        if unknown == "" {
            // is this needed, validation should catch this?
            panic(fmt.Sprintf("L2 has full label match - this is baad: %s", l2))
        }
        l := MapLabel(unknown)
        // update global indexes
        for part, pos := range l.index {
            idxkey := part
            if known != "" {
                idxkey = idxkey + "." + known
            }
            lm.index[idxkey] = len(lm.bytes)+pos+2 // extra 2 bytes of total length
        }

        if known == "" {
            l.bytes = append(l.bytes, []byte{ROOT}...)
        } else {
            l.bytes = append(l.bytes, []byte{COMPRESSED_LABEL, byte(lm.index[known])}...)
        }
        // prepend rdlength
        l.bytes = append(itobs(16, uint64(len(l.bytes))), l.bytes...)
        // add to global bytes
        lm.bytes = append(lm.bytes, l.bytes...)
    default:
        panic(fmt.Sprintf("DNS type not supported: %d", typ))
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
        // unknown == ""
        //  full match with only known part and will need label pointer
        // known == ""
        //  no match with only unknown part and will need full byte definition
        // known, unknown != ""
        //  partial match with both known+unknown parts and will need byte definition and label pointer
        known, unknown := lm.labelize(s)
        if unknown == "" {
            b = append(b, []byte{COMPRESSED_LABEL, byte(lm.index[known])}...)
        } else {
            l := MapLabel(unknown)
            // update global indexes
            for part, pos := range l.index {
                idxkey := part
                if known != "" {
                    idxkey = idxkey + "." + known
                }
                lm.index[idxkey] = len(lm.bytes)+pos+len(b)+2 // extra 2 bytes of total length
            }

            // save the result
            b = append(b, l.bytes...)
            if known == "" {
                // add '.' root
                b = append(b, []byte{ROOT}...)
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
