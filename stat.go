package main

import (
    iofs "io/fs"
    "syscall"
    "errors"
    "strings"
)

type fstat struct {
    path string
    inode uint64
    ctime int64
    mode uint32
    err error
}

func newFstat(path string) *fstat {
    var st syscall.Stat_t

    err := syscall.Stat(path, &st)
    if err != nil {
        if ! errors.Is(err, iofs.ErrNotExist) {
            panic(path + ": " + err.Error())
        }
    }

    return &fstat{path, st.Ino, st.Ctim.Sec, st.Mode, err}
}

func (fs *fstat) exists() bool {
    if errors.Is(fs.err, iofs.ErrNotExist) {
        return false
    }

    return true
}

func (fs *fstat) copy(f *fstat) {
    fs.inode = f.inode
    fs.ctime = f.ctime
    fs.mode = f.mode
    fs.err = f.err
}

func (fs *fstat) worldReadable() bool {
    p := strings.Split(fs.path, "/")

    s := ""
    for i:=0; i<len(p); i++ {
        if p[i] == "" {
            // absolute path
            continue
        }

        s += ("/" + p[i])

        ff := newFstat(s)

        if (ff.mode & otherR) == 0 {
            return false
        }
    }

    return true
}
