package main

import (
    "fmt"
)

type HeadersModError struct {
    err, request string
}

func (e *HeadersModError) Error() string {
    return fmt.Sprintf("%s in %s", e.err, e.request)
}
