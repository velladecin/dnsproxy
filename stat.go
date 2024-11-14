package main

import (
    "io/fs"
    "syscall"
    "errors"
    "strings"
)

func stat(path string) (*syscall.Stat_t, error) {
    var st syscall.Stat_t
    err := syscall.Stat(path, &st)
    if err != nil {
        if ! errors.Is(err, fs.ErrNotExist) {
            return nil, err
        }
    }

    return &st, nil
}

type fstat struct {
    path string
    inode uint64
    ctime int64
    mode uint32
}

func newFstat(path string) *fstat {
    f, err := stat(path)
    if err != nil {
        panic(err)
    }

    return &fstat{path, f.Ino, f.Ctim.Sec, f.Mode}
}

func (fs *fstat) equals(f fstat) bool {
    if fs.path != f.path {
        return false
    }
    if fs.inode != f.inode {
        return false
    }
    if fs.ctime != f.ctime {
        return false
    }

    return true
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
