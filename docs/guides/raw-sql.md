# Raw SQL (escape hatch)

No builder covers 100% of the cases, so there's an escape hatch to raw SQL at two levels.

## `golem.Conn.Exec`

`Exec` is part of `golem.Conn`, so it works the same on `*golem.DataSource` and on `golem.Tx` inside a transaction. It runs any statement and returns a `golem.Result`, since different queries return different things — a `SELECT` has rows to iterate, an `UPDATE` without `RETURNING` only has an affected count, an `UPDATE ... RETURNING` has both:

```go
type Result interface {
	// advances to the next row (like sql.Rows.Next); false once rows run out or there are none
	Next() bool
	// current row as column→value; only valid after Next() == true
	Scan() (map[string]any, error)
	// rows affected by the statement; doesn't depend on Next/Scan, works even without RETURNING
	RowsAffected() (int64, error)
}
```

```go
package main

import (
	"context"
	"fmt"

	"github.com/leandroluk/golem"
)

func example(ctx context.Context, dataSource *golem.DataSource) error {
	// RowsAffected works even without RETURNING; Next/Scan only yield rows if the
	// statement returns any (here, via RETURNING)
	result, err := dataSource.Exec(ctx, "UPDATE users SET age = age + 1 WHERE id = $1 RETURNING *", 1)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	fmt.Println("rows affected:", affected)

	for result.Next() {
		row, err := result.Scan()
		if err != nil {
			return err
		}
		fmt.Println(row)
	}
	return nil
}
```

## `Repository[T].Exec`

Runs a raw read query but scans the result into type `T` directly (returns `[]T`, no `Result` needed), reusing the column→field mapping that already exists (the same names declared via `Col(...).Name(...)`) — no tags or manual scanning needed:

```go
users, err := golem.NewRepository(dataSource, UserEntity).Exec(ctx, "SELECT * FROM users WHERE age > $1", 18)
```

## Quick reference

| Method | Where | Returns | Description |
| --- | --- | --- | --- |
| `Exec(ctx, sql string, args ...any) (golem.Result, error)` | `golem.Conn` (`DataSource`/`Tx`) | `Result` (`Next`/`Scan`/`RowsAffected`) | raw statement, no bound type |
| `Exec(ctx, sql string, args ...any) ([]T, error)` | `Repository[T]` | N rows | raw read, scanned into type `T` |

> Placeholder syntax (`$1`, `?`, `@p1`, `:1`) is dialect-specific — see [Supported databases](adapters.md) for the exact syntax each adapter expects.
