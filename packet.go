package main

func GetQuestion(q []byte) []byte {
    i := QUESTION_START
    for ; i<len(q); i++ {
        if q[i] == 0 {
            break
        }
    }

    return q[QUESTION_START:i]
}

func QuestionString(q []byte) string {
    s := ""

    l := int(q[0])
    for i:=1; i<len(q); {
        s += string(q[i:i+l])             
        i += l

        if i >= len(q) {
            break
        }

        s += "."
        l = int(q[i])
        i++
    }

    return s
}
