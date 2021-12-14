package main

import (
    "net"
    "fmt"
    "strings"
)

type packet interface {
    Type() int
    Data() []byte
}

// Query + Answer
// small diff between these two.. should they be merged?

// Query
type Query struct {
    bytes []byte
    conn net.Addr
}

func NewQuery(b []byte, addr net.Addr) *Query { return &Query{b, addr} }
func (q *Query) Data() []byte { return q.bytes }
func (q *Query) Type() int {
    i, _ := getBit(q.bytes[2], QR)
    return i
}

// Answer
type Answer struct {
    bytes []byte
}

func NewAnswer(b []byte) *Answer { return &Answer{b} }
func (a *Answer) Data() []byte { return a.bytes }
func (a *Answer) Type() int {
    i, _ := getBit(a.bytes[2], QR)
    return i
}

// packet skeleton
type pskel struct {
    header, question, footer []byte
    rr [][]byte
}

func (p *pskel) Id()      []byte { return p.header[:2] }
func (p *pskel) Flags()   []byte { return p.header[2:4] }
func (p *pskel) Qdcount() []byte { return p.header[4:6] }
func (p *pskel) Ancount() []byte { return p.header[6:8] }
func (p *pskel) Nscount() []byte { return p.header[8:10] }
func (p *pskel) Arcount() []byte { return p.header[10:12] }
func (p *pskel) Question()[]byte { return p.question }
func (p *pskel) Rr()    [][]byte { return p.rr }
func (p *pskel) getRequestType() int {
    i, _ := getBit(p.header[2], QR)
    return i
}

//
// Labels

// question
func (p *pskel) GetQuestion() string {
    var l string
    for i:=0; i<len(p.question); {
        llen := int(p.question[i]) // length of label
        if llen == 0 {
            break
        }

        // move to beginning of label and collect it
        i++
        l1 := string(p.question[i:i+llen])
        // initate new or append
        if l == "" {
            l = l1
        } else {
            l = fmt.Sprintf("%s.%s", l, l1)
        }
        // move on to next one
        i += llen
    }

    return l
}

func (p *pskel) SetQuestion(q string) error {
    // get TYPE, CLASS from existing
    tc := p.question[len(p.question)-4:]
    return p.SetQuestionFull(q, makeUint(tc[:2]), makeUint(tc[2:4]))
}

func (p *pskel) SetQuestionFull(q string, ttype, class int) error {
    if len(q) == 0 {
        return &LabelModError{
            err: "Label cannot be zero length",
            request: p.getRequestType(),
        }
    }

    if ttype < 1 || ttype > TXT {
        return &LabelModError{
            err: fmt.Sprintf("Unknown TYPE(%d)", ttype),
            request: p.getRequestType(),
        }
    }

    if class < 1 || class > HS {
        return &LabelModError{
            err: fmt.Sprintf("Unknown CLASS(%d)", class),
            request: p.getRequestType(),
        }
    }

    xtra := 1   // byte for initial length
    xtra++      // byte for label end(0)
    xtra += 2   // 2 bytes TYPE
    xtra += 2   // 2 bytes CLASS
    b := make([]byte, len(q)+xtra)

    count := 0
    for _, part := range strings.Split(q, ".") {
        bytes := []byte(part)

        b[count] = byte(len(bytes))
        count++

        for i:=0; i<len(bytes); i++ {
            b[count] = bytes[i]
            count++
        }
    }

    fmt.Printf("SetQuestion(): %+v\n", p.question)
    fmt.Printf("SetQuestion(): %+v\n", b)

    b[count] = 0 // end of label
    count++
    b[count] = 0 // type
    count++
    b[count] = byte(ttype)
    count++
    b[count] = 0
    count++
    b[count] = byte(class)

    // append TYPE, CLASS (2 bytes each)
    fmt.Printf("SetQuestion(): %+v\n", b)
    return nil
}

//
// Flags

// qr
func (p *pskel) SetQuery() { p.header[2] |= (1<<QR) }
func (p *pskel) SetAnswer() { p.header[2], _ = unsetBit(p.header[2], QR) }

// opcode
func (p *pskel) UnsetOpcode() { p.header[2], _ = unsetBit(p.header[2], 6, 5, 4, 3) }
func (p *pskel) SetOpcode(i int) error {
    if i < QUERY || i > UPDATE {
        return &HeadersModError{
            err: fmt.Sprintf("OPCODE set to invalid value(%d)", i),
            request: p.getRequestType(),
        }
    }

    p.UnsetOpcode()
    p.header[2] |= uint8(i)
    return nil
}

// aa
func (p *pskel) UnsetAa() { p.header[2], _ = unsetBit(p.header[2], AA) }
func (p *pskel) SetAa() {
    // answer only (auth answer)
    if p.getRequestType() == QUERY {
        return
    }

    p.header[2] |= (1<<AA)
}

// rd
func (p *pskel) UnsetRd() { p.header[2], _ = unsetBit(p.header[2], RD) }
func (p *pskel) SetRd() { p.header[2] |= (1<<RD) }

// ra
func (p *pskel) UnsetRa() { p.header[3], _ = unsetBit(p.header[3], RA) }
func (p *pskel) SetRa() { p.header[3] |= (1<<RA) }

// ad (DNSSEC related and which is TODO)
func (p *pskel) UnsetAd() { p.header[3], _ = unsetBit(p.header[3], AD) }
func (p *pskel) SetAd() { p.header[3] |= (1<<AD) }

// cd (DNSSEC related and which is TODO)
func (p *pskel) UnsetCd() { p.header[3], _ = unsetBit(p.header[3], CD) }
func (p *pskel) SetCd() { p.header[3] |= (1<<CD) }

// rcode
func (p *pskel) UnsetRcode() { p.header[3], _ = unsetBit(p.header[3], 3, 2, 1, 0) }
func (p *pskel) SetRcode(i int) error {
    if i < NOERR || i > NOTZONE {
        return &HeadersModError{
            err: fmt.Sprintf("RCODE set to invalid value(%d)", i),
            request: p.getRequestType(),
        }
    }

    p.UnsetRcode()
    p.header[3] |= uint8(i)
    return nil
}
func (p *pskel) SetNoErr() { p.UnsetRcode() } // noerr is 0
func (p *pskel) SetFormErr() { p.SetRcode(FORMERR) }
func (p *pskel) SetServFail() { p.SetRcode(SERVFAIL) }
func (p *pskel) SetNxDomain() { p.SetRcode(NXDOMAIN) }
func (p *pskel) SetNotImp() { p.SetRcode(NOTIMP) }
func (p *pskel) SetRefused() { p.SetRcode(REFUSED) }

// QD count - num of entries in question section
func (p *pskel) GetQdcount() int { return makeUint(p.header[4:6]) }
func (p *pskel) SetQdcount(i int) error { return p.setHeadersCounts(i, QDCOUNT) }
// AN count - num or RRs in answer section
func (p *pskel) GetAncount() int { return makeUint(p.header[6:8]) }
func (p *pskel) SetAncount(i int) error { return p.setHeadersCounts(i, ANCOUNT) }
// NS count - num of name server RRs in authority section
func (p *pskel) GetNscount() int { return makeUint(p.header[8:10]) }
func (p *pskel) SetNscount(i int) error { return p.setHeadersCounts(i, NSCOUNT) }
// AR count - num of RRs in additional section
func (p *pskel) GetArcount() int { return makeUint(p.header[10:12]) }
func (p *pskel) SetArcount(i int) error { return p.setHeadersCounts(i, ARCOUNT) }

func (p *pskel) setHeadersCounts(i, t int) error {
    var from, to int
    var cnt string
    switch t {
        case QDCOUNT:
        from, to = 4, 6
        cnt = "QDCOUNT"
        case ANCOUNT:
        from, to = 6, 8
        cnt = "ANCOUNT"
        case NSCOUNT:
        from, to = 8, 10
        cnt = "NSCOUNT"
        case ARCOUNT:
        from, to = 10, 12
        cnt = "ARCOUNT"
        default:
        return &HeadersModError{
            err: fmt.Sprintf("Unknown headers count type: %d", t),
            request: p.getRequestType(),
        }
    }

    if i < 1 || i > 100 {
        return &HeadersModError{
            err: fmt.Sprintf("%s set to invalid value(%d), supported max(100)", cnt, i),
            request: p.getRequestType(),
        }
    }

    // unset
    for count:=from; count<to; count++ {
        p.header[count] = uint8(0)
    }
    // set assigns to the upper byte
    p.header[from+1] |= uint8(i)
    return nil
}

func PacketAutopsy(p packet) (*pskel, error) {
    d := p.Data()
    fmt.Printf("ALL: %d\n\n", d)

    // header
    skel := &pskel{header: d[:QUESTION_LABEL_START]}

    // question
    for count:=QUESTION_LABEL_START; count<len(d); count++ {
        if d[count] == 0 {
            // end of question followed by 2+2 bytes of type, class
            skel.question = d[QUESTION_LABEL_START:count+5]
            break
        }
    }

    cur_pos := len(skel.header) + len(skel.question)

    // question label ends here
    // type QUERY has got nothing else to do
    if p.Type() == QUERY {
        // verify end, is this really needed?
        if (d[cur_pos] | d[cur_pos+1] | d[cur_pos+2]) != LABEL_END {
            return skel, fmt.Errorf("QUERY packet corrupted")
        }

        skel.footer = d[cur_pos:]
        return skel, nil
    }

    // answer
    // L1                  TTL CLASS     TYPE   L2
    // c1.domain.com.		10	  IN	CNAME	c2.domain.com.
    // c2.domain.com.       10    IN        A   1.1.1.1

    for count:=cur_pos; count<len(d); count++ {
        // 0 is end of L1 (first byte of TYPE)
        // 2 bytes of TYPE, 2 bytes of CLASS, 4 bytes of TTL
        // 2 bytes of total length of L2
        if d[count] == 0 {
            // verify end
            if (d[count] | d[count+1] | d[count+2]) == LABEL_END {
                skel.footer = d[count:]
                break
            }

            count += 7 // type, class, ttl
            L2_len := makeUint(d[count+1:count+1+2]) // L2 length, 2 bytes + 1 to accommodate range syntax
            count += 2
            count += L2_len // end

            // don't increment count any further,
            // it'll increment itself on top of loop
            skel.rr = append(skel.rr, d[cur_pos:count+1])
            cur_pos = count + 1
        }
    }

    return skel, nil
}

// helpers
func unsetBit(b byte, pos ...int) (byte, error) {
    if len(pos) < 1 || len(pos) > 8 {
        return b, fmt.Errorf("1-8 bits in byte, got: %d", len(pos))
    }

    bb := b
    for _, p := range pos {
        pi := int(p)
        if pi < 0 || pi > 7 {
            return bb, fmt.Errorf("0-7 bit indexes in byte, got: %d", pi)
        }

        b |= (1<<pi)
        b ^= (1<<pi)
    }

    return b, nil
}

func getBit(b byte, pos int) (int, error) {
    if pos < 0 || pos > 7 {
        return -1, fmt.Errorf("0-7 bit indexes in byte, got: %d", pos)
    }

    if (b & (1<<pos)) == 0 {
        return 0, nil
    }

    return 1, nil
}

func makeUint(b []byte) int {
    var i int
    switch len(b) {
        case 2: i = int(b[0])<<8  | int(b[1])
        case 3: i = int(b[0])<<16 | int(b[1])<<8  | int(b[2])
        case 4: i = int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
        default:
            panic(fmt.Sprintf("Unsupported integer size: %d", len(b)))
    }

    return i
}

func packetFactory(ch chan []byte) chan []byte {
    go func() {
        for {
            p := make([]byte, 512)
            ch <- p
        }
    }()

    return ch
}
