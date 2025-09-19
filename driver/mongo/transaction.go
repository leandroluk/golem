// Package driver provides database driver implementations for the golem ORM.
// This file defines the mongoTransaction type, which adapts MongoDB sessions
// to the core.Transaction interface used by the ORM.
package driver

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// mongoTransaction wraps a MongoDB session and implements the core.Transaction interface.
//
// It ensures that Commit or Rollback ends the session after the operation is complete.
type mongoTransaction struct {
	session mongo.Session
}

// Commit finalizes the transaction and ends the session.
//
// If Commit succeeds, all changes performed during the session are made permanent.
func (transaction *mongoTransaction) Commit(ctx context.Context) error {
	defer transaction.session.EndSession(ctx)
	return transaction.session.CommitTransaction(ctx)
}

// Rollback aborts the transaction and ends the session.
//
// Any changes performed during the session are discarded.
func (transaction *mongoTransaction) Rollback(ctx context.Context) error {
	defer transaction.session.EndSession(ctx)
	return transaction.session.AbortTransaction(ctx)
}
