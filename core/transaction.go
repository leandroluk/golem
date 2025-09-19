// Package core provides the fundamental building blocks of the golem ORM.
// This file defines transaction management utilities, including helpers
// for injecting transactions into context and executing ergonomic callbacks.
package core

import "context"

// transactionKey is an unexported type used as the key for storing
// a Transaction in a context.Context. Using a private type prevents
// collisions with other context values.
type transactionKey struct{}

// WithTransaction injects a Transaction into the given context.
//
// This allows database operations to detect and reuse an ongoing
// transaction automatically.
//
// Example:
//
//	tx, _ := driver.Transaction(ctx)
//	txCtx := core.WithTransaction(ctx, tx)
//	userModel.Create(txCtx, &user)
func WithTransaction(ctx context.Context, tx Transaction) context.Context {
	return context.WithValue(ctx, transactionKey{}, tx)
}

// TransactionFrom extracts a Transaction from the given context, if any.
//
// Returns nil if the context does not contain a transaction.
func TransactionFrom(ctx context.Context) Transaction {
	if v, ok := ctx.Value(transactionKey{}).(Transaction); ok {
		return v
	}
	return nil
}

// TransactionFunc is the callback signature used for ergonomic transactions.
//
// If the function returns an error, the transaction is rolled back.
// If it returns nil, the transaction is committed.
type TransactionFunc func(txCtx context.Context) error

// RunTransaction executes a function inside a transaction, handling commit
// and rollback automatically.
//
// If fn returns an error, the transaction is rolled back and the error
// is returned. If fn succeeds, the transaction is committed.
//
// Example:
//
//	err := core.RunTransaction(ctx, driver, func(txCtx context.Context) error {
//	    if err := userModel.Create(txCtx, &user); err != nil {
//	        return err
//	    }
//	    if err := orderModel.Create(txCtx, &order); err != nil {
//	        return err
//	    }
//	    return nil
//	})
func RunTransaction(ctx context.Context, driver Driver, fn TransactionFunc) error {
	tx, err := driver.Transaction(ctx)
	if err != nil {
		return err
	}
	txCtx := WithTransaction(ctx, tx)

	if err := fn(txCtx); err != nil {
		_ = tx.Rollback(ctx) // rollback on error
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
