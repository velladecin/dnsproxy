package main

import (
    "fmt"
)

type HeadersModError struct {
    err string
    request int
}

func (e *HeadersModError) Error() string {
    req := "ANSWER"
    if e.request == QUERY {
        req = "QUERY"
    }

    return fmt.Sprintf("%s in %s packet", e.err, req)
}
