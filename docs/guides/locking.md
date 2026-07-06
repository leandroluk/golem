# Pessimistic locking

`query.Query[T]` gains four lock methods, each compiling to the dialect's row-locking clause:

| Method | Locks |
| --- | --- |
| `.ForUpdate(wait ...query.LockWait)` | exclusive lock — blocks other locking reads and writes |
| `.ForNoKeyUpdate(wait ...query.LockWait)` | Postgres-only — like `ForUpdate` but doesn't block FK checks from other transactions |
| `.ForShare(wait ...query.LockWait)` | shared lock — blocks writes, allows other shared locks |
| `.ForKeyShare(wait ...query.LockWait)` | Postgres-only — weakest lock, only blocks changes to the key |

Each accepts an optional `query.LockWaitNoWait` or `query.LockWaitSkipLocked` (default: block until the row unlocks).

```go
package main

import (
	"context"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
)

func example(ctx context.Context, dataSource *golem.DataSource) error {
	// read-then-write pattern: lock the row, decide what to do based on it, update it —
	// all inside the same transaction. No other transaction can lock/read this row
	// (with FOR UPDATE too) until this transaction commits or rolls back.
	return dataSource.Transaction(ctx, func(tx golem.Tx) error {
		userRepo := repository.Get(tx, UserEntity)

		user, err := userRepo.FindOne(ctx, func(u *User, q *query.Query[User]) {
			q.Where(op.Eq(&u.ID, 42))
			q.ForUpdate() // or q.ForUpdate(query.LockWaitSkipLocked) to skip already-locked rows
		})
		if err != nil {
			return err
		}

		_, err = userRepo.Update(ctx, func(u *User, upd *query.Update[User]) {
			upd.Where(op.Eq(&u.ID, user.ID))
			upd.Set(&u.Name, "safely updated")
		})
		return err
	})
}
```

## Locking outside a transaction fails, on purpose

A lock held by an isolated statement releases as soon as that statement finishes — before your code can act on it. So any lock variant returns an error if `conn` isn't a `golem.Tx`: `repository.Get(dataSource, ...)` directly doesn't count; it must be `repository.Get(tx, ...)` inside `dataSource.Transaction(ctx, func(tx golem.Tx) error {...})`.

## Doesn't apply to aggregations

`repository.Aggregate` doesn't expose locking — most databases (including Postgres) don't allow `FOR UPDATE` together with aggregate functions.

## Support matrix per adapter

Not every database supports every lock strength or wait mode. Unsupported combinations are rejected outright (a compile-time SQL error from `CompileSelect`), never silently downgraded to something weaker.

| Adapter | `ForUpdate` | `ForNoKeyUpdate` | `ForShare` | `ForKeyShare` | `NoWait` | `SkipLocked` |
| --- | :-: | :-: | :-: | :-: | :-: | :-: |
| Postgres | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| MySQL / MariaDB | ✅ | ❌ | ✅ | ❌ | ✅ | ✅ |
| SQL Server | ✅ | ❌ | ✅ | ❌ | ✅ | ✅ |
| SQLite | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Oracle | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ |

- **`ForNoKeyUpdate`/`ForKeyShare`** are Postgres-specific concepts (MVCC key-locking granularity) with no equivalent elsewhere.
- **SQLite** has no row-level locking mechanism at all — it's a single-writer, file-locked database. Every lock method errors.
- **Oracle** has no `FOR SHARE` clause of any kind — shared/read locks are a session/table-level concept there (`LOCK TABLE ... IN SHARE MODE`), not a per-query row lock.
- **SQL Server** implements locking via a table hint (`WITH (UPDLOCK, ROWLOCK)`, etc.), not a trailing clause like the others — invisible from the caller's side, but worth knowing if you're reading the generated SQL.

See [Supported databases](adapters.md) for the full dialect comparison beyond locking.
