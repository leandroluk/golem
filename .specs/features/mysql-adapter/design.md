# MySQL/MariaDB Adapter (M16) — Design

## Package

`driver/mysql` (`github.com/leandroluk/golem/driver/mysql`), mirroring `driver/postgres`'s file
layout exactly: `postgres.go` → `mysql.go` (`Options`/`New`), `connector.go`, `dsn.go`,
`dialect.go`, plus `_test.go`/`_integration_test.go` pairs for each.

## Driver library

`github.com/go-sql-driver/mysql` via `database/sql` — the de facto standard, per user decision.
Unlike `pgx` (which `driver/postgres` uses natively, bypassing `database/sql` entirely),
`go-sql-driver/mysql` only registers a `database/sql/driver.Driver`; `*sql.DB` itself is the
connection pool (there's no separate pool type to wrap, unlike `pgxpool.Pool`).

`sqlIface` (mirroring `postgres.go`'s `pgxPoolIface`) abstracts `*sql.DB`'s surface so
`DATA-DOG/go-sqlmock` (MySQL/`database/sql`'s equivalent of `pgxmock`) can substitute for it in
unit tests, exactly like `driver/postgres`'s `pgxmock`-based tests do.

## Core-schema gap found while designing Insert (no `RETURNING` in MySQL)

`stmt.Insert{Table, Columns, Values}` carries no primary-key information — `driver/postgres`
never needed it (`INSERT ... RETURNING *` gets the whole row back in one round trip, no PK lookup
needed). MySQL has no `RETURNING`; the standard 2-round-trip pattern is `INSERT` +
`LAST_INSERT_ID()` + a follow-up `SELECT ... WHERE <pk> = ?`, but building that `WHERE` needs to
know which column(s) are the primary key — information `Repository[T].Insert` has
(`r.meta.PrimaryKey`) but never passed down.

**Decision:** Add `PrimaryKey []string` to `stmt.Insert`, populated by
`repository.Insert`/`InsertMany` from `r.meta.PrimaryKey` (same value `SaveOne`/`Delete`/etc.
already read for their own `WHERE pk = ...` construction). `driver/postgres`'s `Insert` ignores
the new field entirely (zero value, backward compatible, no behavior change, no test change
needed there). `driver/mysql`'s `Insert` uses it to build the follow-up `SELECT`:

1. Run `INSERT INTO table (cols...) VALUES (?...)`.
2. For each `PrimaryKey` column: if it's already in `s.Columns` (composite/explicit PK — the
   caller provided a non-zero value, so it wasn't omitted), use that same bound value directly.
   If it's NOT in `s.Columns` (the common case: a single auto-increment PK, omitted because it
   was zero-valued — see `repository.Insert`'s zero-value omission), use `LastInsertId()`.
   (MySQL `AUTO_INCREMENT` only ever applies to one column, so at most one `PrimaryKey` entry can
   legitimately be missing from `s.Columns` — if more than one is missing, that's a caller
   configuration error, not a state this adapter tries to paper over.)
3. Run `SELECT * FROM table WHERE pk1 = ? AND pk2 = ? ...` with the resolved PK values, return the
   single row as `map[string]any`.

`Update` needs no such change — its follow-up `SELECT ... WHERE ...` reuses `stmt.Update.Where`
verbatim (already fully known; `RETURNING`'s replacement is just "run the same predicate again").

This is exactly the kind of core-assumption gap M15/M16 were built to surface (per M15's spec.md
"Success Criteria": a required core change is a signal, not a failure) — recorded as its own AD
once implemented, not silently patched.

## Identifier quoting, placeholders, lock syntax

- Identifiers: backtick-quoted (`` `col` ``, `` `table`.`col` ``) instead of Postgres's
  double-quotes — a straight port of `quoteIdent`.
- Placeholders: MySQL/`go-sql-driver` uses unnumbered `?`, not `$N` — `compilePredicate` drops the
  `argOffset *int` parameter entirely (nothing in the emitted SQL text needs the position number,
  unlike Postgres's `$1`, `$2`, ...); args are still appended in call order, same as Postgres.
- Lock clause: `FOR UPDATE` / `FOR SHARE` (MySQL 8.0+), optionally `NOWAIT`/`SKIP LOCKED`
  (MySQL 8.0.1+). `no_key_update`/`key_share` strengths hit the same `default: return error`
  branch Postgres's own `lockClauseSQL` already has for anything it doesn't recognize — MySQL's
  list of "doesn't recognize" is just bigger. This is what actually enforces
  `Capabilities.Locking`'s `NoKeyUpdate`/`KeyShare` being `false` for real: `query.Query[T]` has
  no dialect awareness, so nothing stops a caller from writing `.ForNoKeyUpdate()` against a MySQL
  `DataSource` — it compiles fine up to `lockClauseSQL`, which then errors, same "loud failure
  over silent no-op" pattern as the outside-a-transaction guard (M14/AD-030).
- Pagination: `LIMIT ? OFFSET ?` — identical shape to Postgres, just `?` instead of `$N`.

## Value conversion (`Bind`/`Scan`)

Per INSIGHT.md's type table, only 2 kinds actually differ from Postgres's Go-type expectations:

| Kind | Postgres native type | MySQL native type | Bind/Scan adjustment |
| --- | --- | --- | --- |
| `boolean` | `BOOLEAN` | `TINYINT(1)` | None needed — `go-sql-driver` already surfaces `TINYINT(1)` as Go `bool` when the column is declared `BOOLEAN`/`BOOL` (MySQL aliases); raw `int64` fallback (same as Postgres's Scan) covers a plain `TINYINT` some schemas might use instead |
| `uuid` | `UUID` | `CHAR(36)` | None needed at the Go-value level — MySQL has no native UUID type, so a `CHAR(36)` column already round-trips as a Go `string`, which is exactly what `Bind`/`Scan`'s `"uuid"` case already expects |

Every other kind (`smallint`/`integer`/`bigint`, `decimal`/`float`, `char`/`varchar`/`text`,
`date`/`datetime`/`time`, `blob`, `json`) maps onto Go types `go-sql-driver/mysql` already
produces natively via `database/sql.Rows.Scan` into `any` — **provided the DSN sets
`parseTime=true`** (otherwise `DATE`/`DATETIME`/`TIME` columns come back as `[]byte`/`string`, not
`time.Time`). `driver/mysql/dsn.go`'s `resolveDSN` forces `cfg.ParseTime = true` unconditionally —
this is an internal implementation requirement of matching golem's `Dialect` contract, not a
user-facing option (no `Options` field for it; nothing about "should date columns become
`time.Time`" is optional given `golem.ColumnType`'s design).

Confirmed empirically (matching M15/AD-037's earlier finding for Postgres's `pgtype.Numeric`):
`database/sql`'s generic `Scan(&dest any)` behavior for `DECIMAL` columns depends on the
driver — `go-sql-driver/mysql` returns `[]byte` for `DECIMAL`/`NUMERIC` by default (not
`float64`), so `Scan`'s `"decimal"` case must also accept `[]byte`/`string` and `strconv.ParseFloat`
it, alongside the `float64`/`float32` cases already shared with Postgres's version.

## Connector / DSN

`resolveDSN` uses `mysql.ParseDSN`/`mysql.Config`/`cfg.FormatDSN()` (the driver library's own DSN
parser) instead of reimplementing Postgres's `net/url`-based approach — MySQL DSNs
(`user:pass@tcp(host:port)/db?param=value`) aren't `net/url`-parseable (no scheme, bracket
syntax), but the driver already ships a purpose-built parser/builder. Same precedence rules as
Postgres's `resolveDSN` (discrete fields override only their part of an existing DSN; `Port == 0`
never clobbers a port already present), reimplemented against `mysql.Config`'s fields
(`User`/`Passwd`/`Addr`/`DBName`/`TLSConfig`) instead of a `net/url.URL`.

`Options` mirrors `postgres.Options`'s shape: `DSN`, `Host`, `Port`, `User`, `Password`,
`Database`, `Logging`, `Logger`. `SSLMode string` (Postgres's field name) becomes
`TLSConfig string` (MySQL's own field name — `""`/`"true"`/`"false"`/`"skip-verify"`/`"preferred"`,
or a name registered via `mysql.RegisterTLSConfig`) rather than forcing a semantically-mismatched
shared name across dialects.

`connector.Connect()` mirrors Postgres's shape: resolve DSN → open (`sql.Open` — lazy, doesn't
dial) → `Ping()` (forces the real round trip, surfaces bad host/credentials immediately, same
reasoning as Postgres's connector doc comment) → wrap in `dialect{db: ...}`.

## Error mapping (`IsConflict`, `mapError`)

`go-sql-driver/mysql` returns `*mysql.MySQLError{Number uint16, Message string}`. Mapped:

- `1062` (`ER_DUP_ENTRY`) → `golem.ErrDuplicateKey`
- `1451` (`ER_ROW_IS_REFERENCED_2`, blocked delete/update on parent) → `golem.ErrForeignKeyViolation`
- `1452` (`ER_NO_REFERENCED_ROW_2`, bad FK on insert/update child) → `golem.ErrForeignKeyViolation`

`IsConflict` returns `true` for any of these 3 codes (mirroring Postgres's whole-class-23 check,
but MySQL has no single "class" prefix covering exactly these — enumerate the 3 explicitly).

## Transactions (`Begin`/`getExecutor`)

`*sql.DB.BeginTx(ctx, nil)` returns `*sql.Tx`, wrapped the same way `driver/postgres`'s `pgTx`
wraps `pgx.Tx`. `getExecutor` mirrors Postgres's type-assertion-on-`Underlying()` pattern exactly
(`golem.Tx.Underlying().(*mysqlTx)` → use the `*sql.Tx`; otherwise use the pool-level `*sql.DB`).

## Testing

- **Unit** (`_test.go`, no build tag): `DATA-DOG/go-sqlmock` substituting for `sqlIface`, mirroring
  every `pgxmock`-based test `driver/postgres` has — Bind/Scan per kind, CompileSelect/Delete,
  Insert (including the new 2-round-trip + PK-resolution logic), Update, Query/Exec/ExecRaw,
  IsConflict, Begin/getExecutor, DSN resolution. Target: 100% statement coverage, same bar as
  every other package (with the same class of accepted-unreachable-defensive-branch exceptions
  already established, if any turn up).
- **Integration** (`_integration_test.go`, `//go:build integration`): `connector_integration_test.go`
  (connection lifecycle, mirroring Postgres's) + `conformance_integration_test.go` (calls
  `internal/dialecttest.Run` with MySQL-flavored `Schema` DDL and `Capabilities` — every `Locking`
  field `true` except `NoKeyUpdate`/`KeyShare`).
- **Docker**: `.docker/docker-compose.test.yml` gains a `mysql` service (image `mysql:8`, its own
  port so it can run alongside the existing `postgres` service). `Taskfile.yml`'s
  `test-integration` target gains a `GOLEM_MYSQL_TEST_DSN`-equivalent env var and runs
  `driver/mysql`'s integration tests the same way it already runs `driver/postgres`'s.
