# Connecting (DataSource)

A `*golem.DataSource` is built with `golem.NewDataSource`, which takes one or more `golem.Option` values — always including exactly one connector (`postgres.New`, `mysql.New`, `mssql.New`, `sqlite.New`, or `oracle.New`) and, optionally, `golem.Entities(...)`.

```go
package main

import (
	"fmt"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/driver/postgres"
)

func main() {
	dataSource, err := golem.NewDataSource(
		// defaults to "default" if omitted — see "Naming a DataSource" below
		golem.DataSourceName("example"),
		postgres.New(func(o *postgres.Options) {
			o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
			o.Logging = true
		}),
	)
	if err != nil {
		panic("could not build data source: " + err.Error())
	}
	defer dataSource.Close()

	if err := dataSource.Connect(); err != nil {
		panic("could not connect to database: " + err.Error())
	}

	fmt.Println("connected to database")
}
```

`NewDataSource` only builds the value and validates options — no network call happens until `Connect()`. `Close()` releases the underlying pool; safe to call even if `Connect()` was never called or failed.

### MustNewDataSource

`golem.MustNewDataSource(opts...)` is `NewDataSource` without the error return — it panics instead. Useful where a failed `DataSource` is unrecoverable anyway (`main()`, test setup, a DI container's provider constructor):

```go
dataSource := golem.MustNewDataSource(
	postgres.New(func(o *postgres.Options) { o.DSN = dsn }),
)
defer dataSource.Close()
```

## Adapter options

Every adapter accepts either a full `DSN` string, discrete connection fields, or both (see "DSN precedence" below). `Logging`/`Logger` are common to all 5 — see [Custom logger](../README.md#custom-logger).

### Postgres

```go
import "github.com/leandroluk/golem/driver/postgres"

postgres.New(func(o *postgres.Options) {
	o.DSN = "postgres://user:pass@localhost:5432/db?sslmode=disable"
	// or discrete fields:
	o.Host = "localhost"
	o.Port = 5432
	o.User = "postgres"
	o.Password = "1234"
	o.Database = "db"
	o.SSLMode = "disable"
})
```

Driver: `jackc/pgx/v5`. Placeholders: `$1, $2, ...`. `Insert`/`Update` use `RETURNING *` — single round-trip.

### MySQL / MariaDB

```go
import "github.com/leandroluk/golem/driver/mysql"

mysql.New(func(o *mysql.Options) {
	o.DSN = "user:pass@tcp(localhost:3306)/db?parseTime=true"
	// or discrete fields:
	o.Host = "localhost"
	o.Port = 3306
	o.User = "root"
	o.Password = "1234"
	o.Database = "db"
	o.TLSConfig = "" // "", "true", "false", "skip-verify", or a name registered via mysql.RegisterTLSConfig
})
```

Driver: `go-sql-driver/mysql`. **`parseTime=true` is required** in the DSN — without it, `DATETIME`/`TIMESTAMP` columns scan as `[]byte`, not `time.Time`. Placeholders: unnumbered `?`. No `RETURNING` — `Insert`/`Update` use `LAST_INSERT_ID()` plus a follow-up `SELECT` (multi-round-trip).

### SQL Server (MSSQL)

```go
import "github.com/leandroluk/golem/driver/mssql"

mssql.New(func(o *mssql.Options) {
	o.DSN = "sqlserver://user:pass@localhost:1433?database=db"
	// or discrete fields:
	o.Host = "localhost"
	o.Port = 1433
	o.User = "sa"
	o.Password = "Your_Strong_Passw0rd!"
	o.Database = "db"
})
```

Driver: `microsoft/go-mssqldb`. Placeholders: `@p1, @p2, ...`. `Insert`/`Update` use `OUTPUT INSERTED.*` — single round-trip. Row locking is a table hint (`WITH (...)`), not a trailing clause.

### SQLite

```go
import "github.com/leandroluk/golem/driver/sqlite"

sqlite.New(func(o *sqlite.Options) {
	o.Path = "app.db" // or ":memory:" for an in-memory database
})
```

Driver: `modernc.org/sqlite` (pure Go, no cgo). No `Host`/`Port`/`User`/`Password` — just a file `Path` (or `:memory:`). Placeholders: unnumbered `?`. `Insert`/`Update` use native `RETURNING *` (SQLite 3.35+) — single round-trip. No row-level locking of any kind (single-writer file model).

### Oracle

```go
import "github.com/leandroluk/golem/driver/oracle"

oracle.New(func(o *oracle.Options) {
	o.DSN = "oracle://user:pass@localhost:1521/FREEPDB1"
	// or discrete fields:
	o.Host = "localhost"
	o.Port = 1521
	o.User = "system"
	o.Password = "1234"
	o.ServiceName = "FREEPDB1" // Oracle's connection unit — a service name / pluggable database, not a "Database"
})
```

Driver: `sijms/go-ora/v2` (pure Go, no cgo, no Oracle Instant Client needed). Placeholders: numbered `:1, :2, ...`. No `RETURNING`-for-arbitrary-columns — `Insert`/`Update` use a plain write plus a follow-up `SELECT` (multi-round-trip), same shape as MySQL. Targets Oracle 12c+ (needs `OFFSET/FETCH` pagination).

See [Supported databases](adapters.md) for the full dialect comparison table.

## DSN precedence

Every adapter's `resolveDSN` follows the same rule:

- If only `DSN` is set, it's used as-is (re-parsed and re-serialized, so a malformed DSN fails fast with a descriptive error).
- If only discrete fields are set, a DSN is built from them.
- If **both** are set, each non-zero discrete field overrides only its corresponding part of the DSN — the rest is preserved untouched. `Port == 0` is treated as "not set," so it never clobbers a port already present in the DSN.
- If **neither** is set, `Connect()` returns a descriptive config error.

This means you can keep a base DSN in an environment variable and override just the password (or database name, for a different environment) via a discrete field, without re-assembling the whole string.

## Naming a DataSource and retrieving it later

`golem.DataSourceName(name string)` names the instance (defaults to `"default"` if omitted). `NewDataSource` registers it under that name, so any other part of your program can fetch it back without threading a reference through manually:

```go
ds, err := golem.GetDataSource("example") // or golem.GetDataSource() for the "default"-named one
if err != nil {
	panic(err) // golem.ErrDataSourceNotFound if that name was never created, or was already Close()'d
}
```

## Registering entities

`golem.Entities(entities ...any)` attaches your `golem.NewEntity`-declared entities to the `DataSource`, so `dataSource.Connect()` can validate them upfront (e.g. that every `ForeignKey`'s target entity is also registered):

```go
dataSource, err := golem.NewDataSource(
	postgres.New(func(o *postgres.Options) { o.DSN = dsn }),
	golem.Entities(UserEntity, PostEntity),
)
```

See [Declaring schemas](schema.md) for how `UserEntity`/`PostEntity` are declared.
