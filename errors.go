package golem

import "errors"

// ErrNotFound indicates FindByID (or similar) found no matching row.
var ErrNotFound = errors.New("golem: not found")
