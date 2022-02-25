package main

import (
    "fmt"
    "strings"
    "regexp"
    "strconv"
)

type packet []byte

// packet skeleton
type Pskel struct {
    header, question, footer []byte
    rr [][]byte
}

func NewPacketSkeleton(p packet) (*Pskel, error) {
    skel := &Pskel{header: p[:QUESTION_LABEL_START]}

    // question
    for i:=QUESTION_LABEL_START; i<len(p); i++ {
        if p[i] == 0 {
            // end of question followed by 2+2 bytes of type, class
            // +1 (extra) to account for slice range definition
            skel.question = p[QUESTION_LABEL_START:i+5]
            break
        }
    }

    cur_pos := len(skel.header) + len(skel.question)

    // QUERY/question label ends here
    if skel.Type() == QUERY {
        skel.footer = p[cur_pos:]
        return skel, nil
    }

    // answer (RRs)
    // L1                  TTL CLASS     TYPE   L2
    // -------------------------------------------------------
    // c1.domain.com.		10	  IN	CNAME	c2.domain.com.
    // c2.domain.com.       10    IN        A   1.1.1.1

    for i:=cur_pos; i<len(p); i++ {
        // 0 is end of L1 (first byte of TYPE)
        // 2 bytes of TYPE, 2 bytes of CLASS, 4 bytes of TTL
        // 2 bytes of total length of L2
        if p[i] == 0 {
            // verify end, 2 zero bytes
            if (p[i] | p[i+1]) == 0 {
                skel.footer = p[i:]
                break
            }

            i += 8                      // type, class, ttl
            i += makeUint(p[i:i+2]) + 1 // L2 length + 1 to accomodate range syntax

            // don't increment i any further,
            // it'll increment itself on top of loop
            skel.rr = append(skel.rr, p[cur_pos:i+1])
            cur_pos = i + 1
        }
    }

    return skel, nil
}

func (p *Pskel) Question() string {
    var question string
    for i:=0; i<len(p.question); {
        l := int(p.question[i])
        if l == 0 {
            // ignore last 4 bytes of CLASS, TYPE
            break
        }

        i++
        if question != "" {
            question += "."
        }

        question += string(p.question[i:i+l])
        i += l
    }

    return question
}

func (p *Pskel) Type() int {
    i, _ := getBit(p.header[Flags2], QR)
    return i
}

func (p *Pskel) TypeString() string {
    if p.Type() == QUERY {
        return "QUERY"
    }

    return "ANSWER"
}

func (p *Pskel) Bytes() []byte {
    var b []byte
    b = append(b, p.header...)
    b = append(b, p.question...)

    for _, r := range p.rr {
        b = append(b, r...)
    }

    b = append(b, p.footer...)
    return b
}

// ##################
// ## Headers Mods ##
// ##################

// ##
// ## Flags - byte 1
// QR
func (p *Pskel) SetQuery() { p.header[Flags1], _ = unsetBit(p.header[Flags1], QR) }
func (p *Pskel) SetAnswer()  { p.header[Flags1] |= (1<<QR) }
// OPCODE
func (p *Pskel) SetOpcode(i int) error {
    if i < QUERY || i > UPDATE {
        return fmt.Errorf("OPCODE value not supported: %d", i)
    }

    p.header[Flags1], _ = unsetBit(p.header[Flags1], 6, 5, 4, 3)
    p.header[Flags1] |= uint8(i)
    return nil
}
func (p *Pskel) SetOpcodeQuery() error { return p.SetOpcode(QUERY) } // 99% of requests will be for DNS resolution (OPCODE query)
// AA
func (p *Pskel) SetAaTrue() {p.header[Flags1] |= (1<<AA)}
func (p *Pskel) SetAaFalse() {p.header[Flags1], _ = unsetBit(p.header[Flags1], AA)}
// TC
func (p *Pskel) SetTcTrue() {p.header[Flags1] |= (1<<TC)}
func (p *Pskel) SetTcFalse() {p.header[Flags1], _ = unsetBit(p.header[Flags1], TC)}
// RD
func (p *Pskel) SetRdTrue() {p.header[Flags1] |= (1<<RD)}
func (p *Pskel) SetRdFalse() {p.header[Flags1], _ = unsetBit(p.header[Flags1], RD)}

// ##
// ## Flags - byte 2
// RA
func (p *Pskel) SetRaTrue() { p.header[Flags2] |= (1<<RA) }
func (p *Pskel) SetRaFalse() { p.header[Flags2], _ = unsetBit(p.header[Flags2], RA)}
// AD
func (p *Pskel) SetAdTrue() { p.header[Flags2] |= (1<<AD) }
func (p *Pskel) SetAdFalse() { p.header[Flags2], _ = unsetBit(p.header[Flags2], AD)}
// RCODE
func (p *Pskel) SetRcode(i int) error {
    if i < NOERROR || i > NAME_NOT_IN_ZONE {
        return fmt.Errorf("RCODE value not supported: %d", i)
    }

    p.header[Flags2], _ = unsetBit(p.header[Flags2], 3, 2, 1, 0)
    p.header[Flags2] |= uint8(i)
    return nil
}
func (p *Pskel) SetRcodeNoErr() error { return p.SetRcode(NOERROR) }
func (p *Pskel) SetRcodeFmtErr() error { return p.SetRcode(FORMATERROR) }
func (p *Pskel) SetRcodeServFail() error { return p.SetRcode(SERVFAIL) }
func (p *Pskel) SetRcodeNxdomain() error { return p.SetRcode(NXDOMAIN) }
func (p *Pskel) SetNxDomain() error { return p.SetRcodeNxdomain() } // likely used often
func (p *Pskel) SetRcodeNotImpl() error { return p.SetRcode(NOTIMPLEMENTED) }
func (p *Pskel) SetRcodeRefused() error { return p.SetRcode(REFUSED) }
func (p *Pskel) SetRcodeNoAuth() error { return p.SetRcode(NOAUTH) }
func (p *Pskel) SetRcodeNotInZone() error { return p.SetRcode(NAME_NOT_IN_ZONE) }

// ##
// ## Header counts
func (p *Pskel) SetHeaderCount(pos, i int) error {
    if pos < QDcount1 || pos > ARcount2 {
        return fmt.Errorf("Invalid header count byte position: %d", pos)
    }

    p.header[pos] = byte(i)
    return nil
}
// QDcount
func (p *Pskel) SetQDcount(i int) error {
    if err := validHeaderCount(i); err != nil {
        return err
    }

    p.SetHeaderCount(QDcount1, 0)
    p.SetHeaderCount(QDcount2, i)
    return nil
}
// ANcount
func (p *Pskel) SetANcount(i int) error {
    if err := validHeaderCount(i); err != nil {
        return err
    }

    p.SetHeaderCount(ANcount1, 0)
    p.SetHeaderCount(ANcount2, i)
    return nil
}
// NScount
func (p *Pskel) SetNScount(i int) error {
    if err := validHeaderCount(i); err != nil {
        return err
    }

    p.SetHeaderCount(NScount1, 0)
    p.SetHeaderCount(NScount2, i)
    return nil
}
// ARcount
func (p *Pskel) SetARcount(i int) error {
    if err := validHeaderCount(i); err != nil {
        return err
    }

    p.SetHeaderCount(ARcount1, 0)
    p.SetHeaderCount(ARcount2, i)
    return nil
}

// ##
// ## Resource Records mods
func (p *Pskel) SetRR(rr *Rr) {
    p.rr = append(p.rr, rr.PacketBytes())
    p.SetANcount(len(p.rr))
    // TODO RFC6891 - OPT pseudo-RR
}


// ###################
// ## Resource Recs ##
// ###################

type RrMod func(*Rr)
type Rr struct {
    L1, L2 string
    Ttype, Class, Ttl int
}

// TODO
// start with l1, l2 single string
// later move onto []string of [l1,l2] to do smth like
// incoming.telemetry.mozilla.org.	33 IN	CNAME	telemetry-incoming.r53-2.services.mozilla.com.
// telemetry-incoming.r53-2.services.mozilla.com. 88 IN CNAME prod.ingestion-edge.prod.dataops.mozgcp.net.
// prod.ingestion-edge.prod.dataops.mozgcp.net. 33	IN A 34.120.208.123

var ipx = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
func NewRr(l1, l2 string, mods ...RrMod) *Rr {
    rr := &Rr{l1, l2, A, IN, 218}
    for _, m := range mods {
        m(rr)
    }

    // TODO deal with L1 somehow

    // currently L2 must be IP addr
    if ok := ipx.MatchString(rr.L2); !ok {
        panic("Invalid IP addr in RR")
    }

    return rr
}
func (rr *Rr) PacketBytes() []byte {
    l2 := strings.Split(rr.L2, ".")
    length := 10        // 2B pointer + 2B TYPE + 2B CLASS + 4B TTL
    length += 2         // 2B len(l2)
    length += len(l2)   // xB l2 value

    b := make([]byte, length)
    for i:=0; i<12; i++ {
        switch i {
        case 0:                 b[i] = 192
        case 1:                 b[i] = 12
        case 2, 4, 6, 7, 8, 10: b[i] = 0
        case 3:                 b[i] = byte(rr.Ttype)
        case 5:                 b[i] = byte(rr.Class)
        case 9:                 b[i] = byte(rr.Ttl)
        case 11:                b[i] = byte(len(l2))
        }
    }

    for i, j := 0, 12; i<len(l2); i++ {
        a, _ := strconv.Atoi(l2[i])
        b[j] = byte(a)
        j++
    }

    return b
}

// mods
func RrTtl(ttl int) RrMod {
    if ttl > 254 { // single byte fit
        panic(fmt.Sprintf("Unsupported RR TTL value: %d", ttl))
    }

    return func(r *Rr) {
        r.Ttl = ttl
    }
}
func RrClass(class int) RrMod {
    if class < IN || class > HS {
        panic(fmt.Sprintf("Unsupported RR CLASS value: %d", class))
    }

    return func(r *Rr) {
        r.Class = class
    }
}
func RrType(ttype int) RrMod {
    if ttype < A || ttype > TXT {
        panic(fmt.Sprintf("Unsupported RR TYPE value: %d", ttype))
    }

    return func(r *Rr) {
        r.Ttype = ttype
    }
}



// ######################
// ## Helper functions ##
// ######################

func validHeaderCount(i int) error {
    if i < 0 || i > 100 {
        return fmt.Errorf("Unsupported header count value: %d", i)
    }

    return nil
}

func makeUint(b []byte) int {
    var i int
    switch len(b) {
    case 2: i = int(b[0])<<8  | int(b[1])
    case 3: i = int(b[0])<<16 | int(b[1])<<8  | int(b[2])
    case 4: i = int(b[0])<<32 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
    default:
        panic(fmt.Sprintf("Unsupported integer size: %d", len(b)))
    }

    return i
}

// Max 8 values allowed in []pos (8 bits per byte)
// Each values must be between 0-7 (bit index 0 - 7 within the byte)
func unsetBit(b byte, pos ...int) (byte, error) {
    if len(pos) < 1 || len(pos) > 8 {
        return b, fmt.Errorf("8 bits in byte, got: %d", len(pos))
    }

    original := b
    for _, p := range pos {
        if err := validBitPos(p); err != nil {
            return original, err
        }

        b |= (1<<p) // set
        b ^= (1<<p) // xor
    }

    return b, nil
}

func isBitSet(b byte, pos int) (bool, error) {
    set, err := getBit(b, pos)
    if err != nil {
        return false, err
    }

    if set == 1 {
        return true, nil
    }

    return false, nil
}

func getBit(b byte, pos int) (int, error) {
    if err := validBitPos(pos); err != nil {
        return -1, err
    }

    if (b & (1<<pos)) == 0 {
        return 0, nil
    }

    return 1, nil
}

func validBitPos(pos int) error {
    if pos < 0 || pos > 7 {
        return fmt.Errorf("0-7 bit indices in byte, got: %d", pos)
    }

    return nil
}
