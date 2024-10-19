package logger

import (
    "log"
    "os"
    "io"
)

const (
    STDOUT = "stdout"
    STDERR = "stderr"
)

type Logger struct {
    *log.Logger
}

func NewHandles(f string) (i, w, c, d Logger) {
    var fh *os.File
    var err error

    if f == STDOUT { fh = os.Stdout }
    if f == STDERR { fh = os.Stderr }

    if fh == nil {
        fh, err = os.OpenFile(f, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
        if err != nil {
            panic(err)
        }
    }

    // LstdFlags contain Ldate + Ltime
    flags := log.LstdFlags|log.Lshortfile

    i = Logger{log.New(fh, "INFO: ", flags)}
    w = Logger{log.New(fh, "WARN: ", flags)}
    c = Logger{log.New(fh, "CRITICAL: ", flags)}
    d = Logger{log.New(fh, "DEBUG: ", flags)}
    return
}

func doNotClose(f *os.File) bool {
    if f.Name() == os.Stdout.Name() || f.Name() == os.Stderr.Name() {
        return true
    }

    return false
}

// don't close os level file descriptors

func (l Logger) Close() {
    if doNotClose(l.Writer().(*os.File)) {
        return
    }

    l.Writer().(*os.File).Close()
}

func Close(i io.Writer) {
    if doNotClose(i.(*os.File)) {
        return
    }

    i.(*os.File).Close()
}
