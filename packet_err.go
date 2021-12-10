package main

import (
    "fmt"
)

type HeadersModError struct {
    err string
    request int
}

func (e *HeadersModError) Error() string {
    return fmt.Sprintf("%s in %s packet", e.err, getRequestString(e.request))
}

type HeadersUnknownFieldError struct {
    err string
    val int
}

func (e *HeadersUnknownFieldError) Error() string {
    return fmt.Sprintf("%s, val(%d)", e.err, e.val)
}

type LabelModError struct {
    err string
    request int
}

func (e *LabelModError) Error() string {
    return fmt.Sprintf("%s in %s(%d)", e.err, getRequestString(e.request))
}
