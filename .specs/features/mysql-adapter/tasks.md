# MySQL/MariaDB Adapter (M16) Tasks

**Design**: `.specs/features/mysql-adapter/design.md`
**Status**: DONE. `task test-integration` verified green against real MySQL 8. Found and fixed 2
core gaps + 1 MySQL-specific requirement along the way (AD-038/AD-039 in STATE.md):
`stmt.Insert`/`stmt.Update` needed `PrimaryKey`, `assignFieldValue` needed numeric→bool, and
`driver/mysql` needed column-type-aware row normalization (`rows.ColumnTypes()`), unlike
Postgres's self-describing wrapper types.

Sequential — each task depends on the previous one's types/files existing. Executed directly, no
sub-agent dispatch (same approach as M15).

---

## Task Breakdown

### T0: Core-schema change — `stmt.Insert.PrimaryKey`

**What**: Add `PrimaryKey []string` to `internal/stmt.Insert`; `repository.Insert`/`InsertMany`
populate it from `r.meta.PrimaryKey`. `driver/postgres`'s `Insert` untouched (ignores the new
field) — confirm its existing tests still pass unmodified.
**Requirement**: MYSQL-04
**Gate**: `go build ./... && go test ./... -short`

---

### T1: `go.mod` dependency + package skeleton

**What**: Add `github.com/go-sql-driver/mysql` to `go.mod`. Create `driver/mysql/mysql.go`
(`Options`, `New`, `sqlIface`), mirroring `postgres.go`'s shape exactly (DSN/discrete
fields/Logging/Logger, `TLSConfig` instead of `SSLMode`).
**Depends on**: none
**Gate**: `go build ./...`

---

### T2: `dsn.go` — `resolveDSN`/`buildDSN` via `mysql.Config`

**What**: `driver/mysql/dsn.go` — DSN precedence logic per design.md, using
`mysql.ParseDSN`/`mysql.NewConfig`/`cfg.FormatDSN()`. Forces `cfg.ParseTime = true` always.
**Depends on**: T1
**Tests**: unit, mirroring `driver/postgres/dsn_test.go`'s exact case list (DSN-only,
fields-only, both-set partial overrides, neither-set error, malformed-DSN error)

---

### T3: `connector.go`

**What**: `driver/mysql/connector.go` — `connector{opts, db}`, `Connect()`/`Close()`/`log()`,
mirroring `driver/postgres/connector.go`'s shape (swappable `newSQLDB` var for test injection,
`Ping()` after open, lifecycle logging).
**Depends on**: T2
**Tests**: unit (`go-sqlmock`), mirroring `connector_test.go`

---

### T4: `dialect.go` — `Bind`/`Scan`

**What**: `driver/mysql/dialect.go`'s `Bind`/`Scan` methods, per design.md's value-conversion
table (boolean/uuid need no adjustment from Postgres's version; decimal/float's `Scan` also
accepts `[]byte`/`string` since `go-sql-driver` returns those for `DECIMAL`, not `float64`).
**Depends on**: T1
**Tests**: unit, mirroring `driver/postgres/dialect_test.go`'s Bind/Scan test list per `ColumnType` kind

---

### T5: `dialect.go` — `CompileSelect`/`CompileDelete`/`compilePredicate`/`lockClauseSQL`

**What**: Backtick `quoteIdent`, `?`-placeholder `compilePredicate` (no `argOffset` needed),
`aggregateSQLFunc`/`projectionSQL` (same `CAST(... AS DOUBLE)`-equivalent reasoning —
MySQL's cast target is `DECIMAL`/`DOUBLE`, confirm exact syntax), `lockClauseSQL` (`FOR
UPDATE`/`FOR SHARE`, `NOWAIT`/`SKIP LOCKED`, error for `no_key_update`/`key_share`).
**Depends on**: T4
**Tests**: unit, mirroring `driver/postgres/dialect_test.go`'s CompileSelect/CompileDelete/
compilePredicate/lock-clause test list

---

### T6: `dialect.go` — `Insert` (2-round-trip + PK resolution)

**What**: `Insert` per design.md's algorithm (resolve PK values from `s.Columns`/`LastInsertId()`,
follow-up `SELECT`).
**Depends on**: T0, T5
**Tests**: unit (`go-sqlmock`) — single auto-increment PK (common case), composite/explicit PK
(no `LastInsertId()` needed), `LastInsertId()` error, follow-up-`SELECT` error/zero-rows

---

### T7: `dialect.go` — `Update`/`Query`/`Exec`/`ExecRaw`

**What**: `Update` (`UPDATE` + follow-up `SELECT` reusing `s.Where`), `Query`/`Exec`/`ExecRaw`
straight ports (no `RETURNING`-shaped difference — these never used it).
**Depends on**: T5
**Tests**: unit, mirroring `driver/postgres/dialect_test.go`'s Update/Query/Exec/ExecRaw list

---

### T8: `dialect.go` — `IsConflict`/`mapError`/`Begin`/`getExecutor`

**What**: MySQL error-code mapping (1062/1451/1452) per design.md; `Begin`/`getExecutor`
mirroring Postgres's `pgTx`/type-assertion pattern.
**Depends on**: T7
**Tests**: unit, mirroring `driver/postgres/dialect_test.go`'s IsConflict/Begin/getExecutor list

---

### T9: Docker + Taskfile wiring

**What**: `.docker/docker-compose.test.yml` gains a `mysql` service (own port). `Taskfile.yml`'s
`test-integration` target runs `driver/mysql`'s integration tests against it.
**Depends on**: none (can happen anytime before T10)
**Gate**: `docker compose -f .docker/docker-compose.test.yml up -d --wait` brings up both services healthy

---

### T10: `connector_integration_test.go` + `conformance_integration_test.go`

**What**: `driver/mysql/connector_integration_test.go` (connection lifecycle, mirroring Postgres's),
`driver/mysql/conformance_integration_test.go` (MySQL `Schema` DDL + `Capabilities`, calls
`dialecttest.Run`).
**Depends on**: T3, T6, T8, T9
**Gate**: `task test-integration` green (real MySQL 8+, Docker)

---

### T11: Coverage + docs

**What**: Close any unit-test coverage gaps to 100% (or documented accepted-exceptions, same
class as M15/AD-037's). Update `.specs/project/ROADMAP.md` (M16 → DONE), `.specs/project/STATE.md`
(new AD for the `stmt.Insert.PrimaryKey` core change + MySQL-specific decisions),
`README.md` (Implementation Status checkbox).
**Depends on**: T10
**Gate**: `task coverage` shows 100% (or documented exceptions); `task test-integration` still green
