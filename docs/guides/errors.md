# Typed errors

Golem exposes sentinel errors in the `golem` package for the most common cases. Always check with `errors.Is`, never by comparing the string message — each dialect/driver speaks differently. The adapter translates the driver's native error (e.g. Postgres's SQLSTATE `23505`) into the matching sentinel, preserving the original error underneath via `%w`, so `errors.Unwrap`/`errors.As` can still reach the driver's native error when the sentinel isn't granular enough.

## Sentinels

| Sentinel | Meaning |
| --- | --- |
| `golem.ErrNotFound` | no matching row found (`FindOne`/`SaveOne` with no match) — `Update` never triggers this; matching 0 rows there is not an error |
| `golem.ErrDuplicateKey` | `Unique` constraint violation (single or composite) |
| `golem.ErrForeignKeyViolation` | `ForeignKey` violation — a write points at something that doesn't exist, or a delete removes something still referenced |
| `golem.ErrDataSourceNotFound` | `golem.GetDataSource(name)` called with a name no `NewDataSource` call registered (or one already `Close()`'d) |

If the driver returns an error that doesn't map to any of the above yet, the original error surfaces unchanged — golem never forces an unrecognized error into a generic "unknown" sentinel.

```go
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
)

func handle(ctx context.Context, userRepo *repository.Repository[User]) {
	user, err := userRepo.FindOne(ctx, func(t *User, q *query.Query[User]) {
		q.Where(op.Eq(&t.ID, 999))
	})
	if errors.Is(err, golem.ErrNotFound) {
		fmt.Println("user does not exist")
		return
	}
	if err != nil {
		panic(err) // a real infra error (connection dropped, etc.)
	}

	_, err = userRepo.Insert(ctx, &User{Email: user.Email})
	if errors.Is(err, golem.ErrDuplicateKey) {
		fmt.Println("email already registered")
		return
	}
	if err != nil {
		panic(err)
	}
}
```

## What each adapter maps

| Adapter | `ErrDuplicateKey` source | `ErrForeignKeyViolation` source |
| --- | --- | --- |
| Postgres | SQLSTATE `23505` | SQLSTATE `23503` |
| MySQL / MariaDB | error `1062` (`ER_DUP_ENTRY`) | errors `1451`/`1452` (`ER_ROW_IS_REFERENCED_2`/`ER_NO_REFERENCED_ROW_2`) |
| SQL Server | errors `2627`/`2601` | error `547` |
| SQLite | `SQLITE_CONSTRAINT_UNIQUE`/`SQLITE_CONSTRAINT_PRIMARYKEY` | `SQLITE_CONSTRAINT_FOREIGNKEY` |
| Oracle | `ORA-00001` | `ORA-02291` (insert/update side) / `ORA-02292` (delete side) |

Every mapping is applied via `errors.As` against the driver's own native error type, so the original error is never lost — it's always reachable via `errors.Unwrap`/`errors.As` even after being wrapped in a `golem.Err*` sentinel.
