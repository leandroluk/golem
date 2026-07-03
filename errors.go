package golem

import "errors"

// ErrNotFound indicates a query (FindOne, FindMany, SaveOne, etc.) found no matching row.
var ErrNotFound = errors.New("golem: not found")

