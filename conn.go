package golem

import (
	"context"
	"fmt"
)

// Conn is a sealed marker interface implemented only by types in this
// package (DataSource today, Tx later).
type Conn interface {
	isConn()
	// Dialect returns the active Dialect for this connection. This is the
	// mechanism repository (a separate package) uses to reach the Dialect
	// given only a Conn value.
	Dialect() Dialect
	// Exec executes a raw SQL statement with optional arguments.
	Exec(ctx context.Context, sql string, args ...any) (Result, error)
}

// Tx represents an active database transaction.
type Tx interface {
	Conn
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	Underlying() any
}

// TxConn is implemented by database-specific transaction implementations.
type TxConn interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type txImpl struct {
	dialect Dialect
	txConn  TxConn
}

func (*txImpl) isConn() { return }

func (t *txImpl) Dialect() Dialect {
	return t.dialect
}

func (t *txImpl) Commit(ctx context.Context) error {
	return t.txConn.Commit(ctx)
}

func (t *txImpl) Rollback(ctx context.Context) error {
	return t.txConn.Rollback(ctx)
}

func (t *txImpl) Underlying() any {
	return t.txConn
}

// NewTx wraps a dialect-specific TxConn into a golem.Tx.
func NewTx(dialect Dialect, txConn TxConn) Tx {
	return &txImpl{dialect: dialect, txConn: txConn}
}

func (t *txImpl) Exec(ctx context.Context, sql string, args ...any) (Result, error) {
	rows, affected, err := t.dialect.ExecRaw(ctx, t, sql, args)
	if err != nil {
		return nil, err
	}
	return &rawResult{rows: rows, rowsAffected: affected, currentIndex: -1}, nil
}

// Result represents the outcome of a raw SQL execution.
type Result interface {
	Next() bool
	Scan() (map[string]any, error)
	RowsAffected() (int64, error)
}

type rawResult struct {
	rows         []map[string]any
	rowsAffected int64
	currentIndex int
}

func (r *rawResult) Next() bool {
	if r.currentIndex+1 < len(r.rows) {
		r.currentIndex++
		return true
	}
	return false
}

func (r *rawResult) Scan() (map[string]any, error) {
	if r.currentIndex < 0 || r.currentIndex >= len(r.rows) {
		return nil, fmt.Errorf("golem: Scan called out of bounds or before Next")
	}
	return r.rows[r.currentIndex], nil
}

func (r *rawResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}
