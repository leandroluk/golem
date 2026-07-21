package core

import (
	"context"
	"testing"
)

// fakeConn is a test-only type used to verify that Conn can be implemented
// from inside package golem (since isConn is unexported, only types in this
// package can satisfy the interface).
type fakeConn struct{}

func (fakeConn) isConn() {}

func (fakeConn) Dialect() Dialect { return nil }

func (fakeConn) Parser() Parser { return nil }

func (fakeConn) Exec(ctx context.Context, sql string, args ...any) (Result, error) {
	return nil, nil
}

var _ Conn = (*fakeConn)(nil)

func TestFakeConnImplementsConn(t *testing.T) {
	_ = fakeConn{}
}
