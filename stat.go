package main

import (
    "io/fs"
    "syscall"
    "errors"
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
}

func newFstat(path string) *fstat {
    f, err := stat(path)
    if err != nil {
        panic(err)
    }

    return &fstat{path, f.Ino, f.Ctim.Sec}
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
