# Spec — Hooks (Milestone 7)

## Features

- Support transaction-aware lifecycle hooks on entity operations.
- The 9 hook slots:
  - `BeforeCreate`, `AfterCreate`, `OnConflictCreate`
  - `BeforeUpdate`, `AfterUpdate`, `OnConflictUpdate`
  - `BeforeDelete`, `AfterDelete`, `OnConflictDelete`
- Double hook slot registrations on the same entity trigger a panic:
  - `panic("entity: hook slot '<SlotName>' already registered")`
- Rollback: returning an error from any hook aborts the active operation and rolls back the transaction.

## API Example
```go
entity.AddHook(UserEntity).
	BeforeCreate(func(ctx context.Context, u *User, conn golem.Conn) error {
		if u.Email == "" {
			return errors.New("email is required")
		}
		return nil
	})
```
