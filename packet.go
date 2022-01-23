package main

import (
    "fmt"
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
    // c1.domain.com.		10	  IN	CNAME	c2.domain.com.
    // c2.domain.com.       10    IN        A   1.1.1.1

    for i:=cur_pos; i<len(p); i++ {
        // 0 is end of L1 (first byte of TYPE)
        // 2 bytes of TYPE, 2 bytes of CLASS, 4 bytes of TTL
        // 2 bytes of total length of L2
        if p[i] == 0 {
            // verify end
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

// Flags - byte 1

func (p *Pskel) SetQuery()  { p.header[Flags1] |= (1<<QR) }
func (p *Pskel) SetAnswer() { p.header[Flags1], _ = unsetBit(p.header[Flags1], QR) }

func (p *Pskel) SetOpcode(i int) error {
    if i < QUERY || i > UPDATE {
        return fmt.Errorf("Attempt to set OPCODE to invalid value: %d", i)
    }

    p.header[Flags1], _ = unsetBit(p.header[Flags1], 6, 5, 4, 3)
    p.header[Flags1] |= uint8(i)
    return nil
}

// Flags - byte 2

// Helper functions

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
