package main

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

func RequestTypeByte(q []byte) []byte {
    // TYPE is 2 bytes after question
    i := QUESTION_START+len(QuestionByte(q))
    i++

    return q[i:i+2]
}

func RequestType(q []byte) int {
    // first byte is 0
    return int(RequestTypeByte(q)[1])
}
