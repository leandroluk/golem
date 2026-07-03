# Spec — Transactions (Milestone 8)

## Features

- Support transaction blocks via `DataSource.Transaction`.
- The callback receives `tx golem.Tx` which implements `golem.Conn`, allowing repositories to run inside the transaction interchangeably with `*DataSource`.
- Automatic Commit: if the callback returns `nil`, the transaction is committed.
- Automatic Rollback: if the callback returns a non-nil error or panics, the transaction is rolled back.

## API Example
```go
err := dataSource.Transaction(ctx, func(tx golem.Tx) error {
	userRepo := repository.Get(tx, UserEntity)
	u, err := userRepo.Insert(ctx, &User{Name: "Alice"})
	if err != nil {
		return err
	}
	return nil
})
```
