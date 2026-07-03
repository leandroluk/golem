package golem

import "context"

// Conn is a sealed marker interface implemented only by types in this
// package (DataSource today, Tx later).
type Conn interface {
	isConn()
	// Dialect returns the active Dialect for this connection. This is the
	// mechanism repository (a separate package) uses to reach the Dialect
	// given only a Conn value.
	Dialect() Dialect
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

func (*txImpl) isConn() {}

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

