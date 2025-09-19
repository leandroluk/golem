// Package driver provides database driver implementations for the golem ORM.
// This file defines the postgresTransaction type, which adapts pgx.Tx
// to the core.Transaction interface used by the ORM.
package driver

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// postgresTransaction wraps a pgx.Tx and implements the core.Transaction interface.
//
// It allows the ORM to manage transactions in a driver-agnostic way.
type postgresTransaction struct {
	transaction pgx.Tx
}

// Commit finalizes the transaction, making all changes permanent.
//
// If Commit fails, the transaction is not applied and the error is returned.
func (transaction *postgresTransaction) Commit(ctx context.Context) error {
	return transaction.transaction.Commit(ctx)
}

// Rollback aborts the transaction, discarding all changes made during it.
//
// If Rollback fails, the error is returned, but the transaction is still considered closed.
func (transaction *postgresTransaction) Rollback(ctx context.Context) error {
	return transaction.transaction.Rollback(ctx)
}
