<div align="center">
  <img src="assets/banner.png" alt="Golem" />
</div>
<br>

<div align="center">
  <a href="https://github.com/leandroluk/golem/actions">
    <img src="https://github.com/leandroluk/golem/actions/workflows/ci.yml/badge.svg?branch=master" alt="Build Status" />
  </a>
  <a href="https://codecov.io/gh/leandroluk/golem">
    <img src="https://img.shields.io/codecov/c/github/leandroluk/golem/master.svg" alt="Coverage Status" />
  </a>
  <a href="https://pkg.go.dev/github.com/leandroluk/golem">
    <img src="https://pkg.go.dev/badge/github.com/leandroluk/golem.svg" alt="Go Doc" />
  </a>
  <a href="https://github.com/leandroluk/golem/releases">
    <img src="https://img.shields.io/github/release/leandroluk/golem.svg?style=flat-square" alt="Release" />
  </a>
</div>

<br>

A type-safe, [TypeORM](https://typeorm.io/)-inspired ORM for Go, built with generics and field-pointer references instead of struct tags or code generation.

## Why this exists

Most Go ORMs lean on struct tags (`db:"..."`, `gorm:"..."`) or reflection-heavy magic to map fields to columns, and criteria are built from loosely-typed strings (`"age > ?"`) that only fail at runtime. Golem takes a different path: entities are declared with plain structs, and every mapping — columns, keys, relations, hooks, query criteria — is expressed via **pointers to struct fields**, checked by the Go compiler. Rename a field and every place that referenced it fails to compile instead of silently breaking at runtime.

Golem mirrors TypeORM's `DataSource` / `Repository` pattern (already familiar if you've used it in the Node.js world), but stays idiomatic Go: no decorators, no annotations, no generated code — just generics and closures.

## Features

- Field-pointer schema declaration (`entity.New[T]`, `b.Col(&t.Field, golem.VARCHAR(50))`, …) — no struct tags, no codegen
- Typed `Repository[T]` for all CRUD operations (`Insert`, `InsertMany`, `SaveOne`, `SaveMany`, `Update`, `Delete`, `Restore`, `FindOne`, `FindMany`, `Count`, `Exists`)
- Declarative query criteria via closures (`op.Eq`, `op.Gt`, `op.In`, `op.Like`, …) — no string-built WHERE clauses
- Automatic `CreateDate` / `UpdateDate` / soft-delete (`DeleteDate`) timestamp handling
- Foreign keys with real `ON DELETE` behavior (`Cascade`, `SetNull`, `Restrict`) applied by `Repository[T].Delete`, not just accepted and ignored
- Joins (`join.Inner`/`Left`/`Right`/`Full`) with column-to-column `On` conditions
- `Preload` for eager-loading related rows — returns a plain `map[any][]J`, no hidden navigational fields on your structs
- `Aggregate` — `GroupBy`/`Sum`/`Avg`/`Count`/`CountAll`/`Having` into any destination struct, resolved by field pointer
- Pessimistic locking (`.ForUpdate()`, `.ForNoKeyUpdate()`, `.ForShare()`, `.ForKeyShare()`, with `NoWait`/`SkipLocked`) — declared per-dialect, unsupported strengths are rejected rather than silently ignored
- Transactions via `dataSource.Transaction(ctx, func(tx golem.Tx) error {...})`
- Raw SQL escape hatch (`Exec` on both `golem.Conn` and `Repository[T]`) for anything the builder doesn't cover
- Typed error sentinels (`golem.ErrNotFound`, `golem.ErrDuplicateKey`, `golem.ErrForeignKeyViolation`, …) — always `errors.Is`-checkable, never string-matched
- Hooks (`Before/After/OnConflict` × `Create/Update/Delete`) running inside the same transaction as the triggering operation
- 5 production-grade adapters: **Postgres, MySQL/MariaDB, SQL Server, SQLite, Oracle** — same entity declaration, same `Repository[T]` API, only the imported `driver/*` package changes
- 100% statement coverage across the entire codebase

## Requirements

- Go ≥ 1.21 (generics)
- One of the supported databases — see [Supported databases](guides/adapters.md)

## Get started

```bash
go get github.com/leandroluk/golem
go get github.com/leandroluk/golem/driver/postgres
```

```go
package main

import (
	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/driver/postgres"
)

func main() {
	dataSource, err := golem.NewDataSource(
		postgres.New(func(o *postgres.Options) {
			o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
		}),
	)
	if err != nil {
		panic(err)
	}
	defer dataSource.Close()

	if err := dataSource.Connect(); err != nil {
		panic(err)
	}
}
```

From here:

- [Connecting](guides/connecting.md) — every adapter's `Options`, DSN precedence rules, `GetDataSource`
- [Declaring schemas](guides/schema.md) — `entity.New`, columns, keys, indexes, foreign keys, hooks
- [Repository (CRUD)](guides/repository.md) — `Insert`, `SaveOne`, `Update`, `Delete`, `FindOne`/`FindMany`, `Count`/`Exists`
- [Query builder](guides/query-builder.md) — `Where`, `op.*` operators, `OrderBy`, pagination, soft delete
- [Joins](guides/joins.md), [Preload](guides/preload.md), [Aggregations](guides/aggregations.md), [Locking](guides/locking.md)
- [Relations & cascades](guides/relations.md) — foreign keys, `OnDelete` behavior, many-to-many junction entities
- [Raw SQL](guides/raw-sql.md) and [typed errors](guides/errors.md)
- [Supported databases](guides/adapters.md) — dialect differences (placeholders, pagination, locking, error codes) across all 5 adapters
- [Full blog example](examples/blog.md) — every feature above, wired together, runnable against any of the 5 adapters

## Migrations

Out of scope, on purpose. Entities describe runtime mapping/behavior, not a source of truth for DDL — schema management (creating/altering tables, versioning, rollback) is left to an external tool (Liquibase, Flyway, goose, `golang-migrate`, etc.). There's no automatic "synchronize" mode like some ORMs offer in development.

## Custom logger

`golem.Logger` is a 4-method interface (`Debug`/`Info`/`Warn`/`Error`, each `(msg string, args map[string]any)`). Every adapter's `Options` accepts one via `Logger`, plus a `Logging bool` switch — when `false` (the default), nothing is logged regardless of what `Logger` is set to.

```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/driver/postgres"
)

type Logger struct{}

var _ golem.Logger = (*Logger)(nil)

func (l *Logger) log(level, msg string, args map[string]any) {
	raw, _ := json.Marshal(args)
	fmt.Printf("[%s] %s %s\n", level, msg, raw)
}
func (l *Logger) Debug(msg string, args map[string]any) { l.log("DEBUG", msg, args) }
func (l *Logger) Info(msg string, args map[string]any)  { l.log("INFO", msg, args) }
func (l *Logger) Warn(msg string, args map[string]any)  { l.log("WARN", msg, args) }
func (l *Logger) Error(msg string, args map[string]any) { l.log("ERROR", msg, args) }

func main() {
	dataSource, _ := golem.NewDataSource(
		postgres.New(func(o *postgres.Options) {
			o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
			o.Logging = true
			o.Logger = &Logger{}
		}),
	)
	_ = dataSource
}
```

If `Logger` is left `nil` while `Logging` is `true`, `golem.DefaultLogger()` is used instead (prints to stdout via `fmt.Println`).

## Tips

**There's no dedicated `FindByID`.** Look up by primary key with `FindOne` + `op.Eq(&t.ID, id)` — see [Repository](guides/repository.md).

**`Repository[T].Update` has no separate `UpdateOne`/`UpdateMany`.** Criteria-based updates run directly against the database (no runtime instance needed); matching 0 rows is not an error either way.

**Soft delete is automatic once `DeleteDate` is declared.** Every `Where`-capable query filters deleted rows out by default; call `.WithDeleted()` to include them. `Delete`/`Restore` set/clear the timestamp instead of removing the row. Entities without `DeleteDate` are unaffected — `Delete` removes the row, and `.WithDeleted()` is a no-op.

**`Preload` never attaches results back onto your structs.** There's no `user.Posts []Post` field — entities stay plain structs. `Preload` returns a `map[any][]J` grouped by the relation key; you decide what to do with it. See [Preload](guides/preload.md).

**Locking a read outside a transaction fails, on purpose.** A lock held by an isolated statement releases before your code can act on it, so `.ForUpdate()` (or any lock variant) requires `conn` to be a `golem.Tx` — see [Pessimistic locking](guides/locking.md).

**Every dialect's own quirks (placeholders, pagination syntax, locking support, error codes) are documented once, not scattered across examples.** See [Supported databases](guides/adapters.md).

## About the project

Multi-dialect from day one: entities and column types (`golem.ColumnType`) are dialect-agnostic, so writing an entity once and later pointing it at a different `driver/*` package requires no changes to the entity declaration itself. See `.specs/project/PROJECT.md` in the repository for the full vision/scope, and `.specs/project/STATE.md` for the history of every design decision.

## Contributors

Thanks to all the people who contribute! [[Contribute](https://github.com/leandroluk/golem/blob/master/CONTRIBUTING.md)]

## License

MIT License — see [LICENSE](https://github.com/leandroluk/golem/blob/master/LICENSE) for details.
