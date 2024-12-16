package main

import (
    "fmt"
    "strings"
)


//
// Question

func QuestionByte(q []byte) []byte {
    if len(q) == 0 {
        return q
    }

    i := QUESTION_START
    for ; i<len(q); i++ {
        if q[i] == 0 {
            break
        }
    }

    return q[QUESTION_START:i]
}

func Question(q []byte) string {
    qb := QuestionByte(q)
    s := ""

    if len(qb) == 0 {
        return s
    }

    l := int(qb[0])
    for i:=1; i<len(qb); {
        s += string(qb[i:i+l])             
        i += l

        if i >= len(qb) {
            break
        }

        s += "."
        l = int(qb[i])
        i++
    }

    return s
}


//
// Request TYPE

func RequestTypeByte(q []byte) []byte {
    // TYPE is 2 bytes after question
    i := QUESTION_START+len(QuestionByte(q))
    i++

    return q[i:i+2]
}

func RequestType(q []byte) int {
    return int(RequestTypeByte(q)[1])
}

func TypeString(i int) string {
    return RequestTypeString(i)
}

// deprecated, move to TypeString(i)
func RequestTypeString(i int) string {
    var s string

    switch i {
    case A:     s = "A"
    case CNAME: s = "CNAME"
    case SOA:   s = "SOA"
    case PTR:   s = "PTR"
    case MX:    s = "MX"
    default:    s = fmt.Sprintf("not-yet-implemented(%d)", i)
    }

    return s
}


//
// Response

func Response(b []byte) string {
    i := QUESTION_START

    // skip question, type, class
    // but keep track of type
    var t int
    for ; i<len(b); i++ {
        if b[i] == 0 {
            t = int(b[i+2])

            // type(2) + class(2) + next pos index(1)
            i += 5
            break
        }
    }

    switch t {
    case A, PTR:
    default:
        return "TODO: " + RequestTypeString(t)
    }

    resp := make([]string, 0)

    // answer
    // host, class, type, ttl, <answer>
    // ...
    for ; i<len(b); i++ {
        if b[i] == 0 {
            break
        }

        if b[i] == 192 {
            // label pointer (full hostname)
            i++
        }

        // track type
        t = int(b[i+2])

        // type(2), class(2), ttl(4)
        i += 8

        // A <answer> = IP addr
        if t == A {
            // length(2) + next pos index(1)
            i += 3

            // first octet
            ip := fmt.Sprintf("%d", int(b[i]))
            // other octets
            for j:=1; j<4; j++ {
                ip += fmt.Sprintf(".%d", int(b[i+j]))
            }
            i += 3

            resp = append(resp, ip)
            continue
        }

        // CNAME <answer> = hostname
        // PTR   <answer> = hostname
        if t == CNAME || t == PTR {
            // 2 bytes total length
            // there should not be 192 here!
            i += 2

            j, host := readLabel(i+1, b)
            i += j

            resp = append(resp, fmt.Sprintf("(%s)%s", RequestTypeString(t), host))
        }
    }

    return strings.Join(resp, ", ")
}

func readLabel(i int, b []byte) (int, string) {
    lbl := ""
    j := 0

    for ; i<len(b); i++ {
        if b[i] == 0 {
            // end
            break
        }

        if b[i] == 192 {
            // end of label (partial label pointer)
            _, p := readLabel(int(b[i+1]), b)

            lbl += fmt.Sprintf(".%s", p)
            // increment processed bytes
            j += 2

            break
        }

        if lbl != "" {
            lbl += "."
        }

        // +1        start of slice
        // +1 + b[i] end of slice
        lbl += string(b[i+1:i+1+int(b[i])])

        // increment i by b[i]
        // do not +1 as the loop will do this when it turns
        i += int(b[i])

        // increment processed bytes
        j = 1+len(lbl)
    }

    return j, lbl
}
