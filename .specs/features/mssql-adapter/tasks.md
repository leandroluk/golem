# SQL Server (MSSQL) Adapter (M18) Tasks

**Design**: `.specs/features/mssql-adapter/design.md`
**Status**: In progress

Sequential — each task depends on the previous one's types/files existing, same approach as
M15-M17 (executed directly, no sub-agent dispatch). Original draft dropped T7 (upsert/`MERGE INTO`)
entirely — design.md's Correction section found `SaveOne`/`SaveMany` were never upsert to begin
with (grepped the whole repo: zero `ON CONFLICT`/`MERGE`/upsert usage anywhere, on any adapter).

---

## Task Breakdown

### T0: Core-schema change — `stmt.Select.PrimaryKey` ✅ DONE

**What**: Added `PrimaryKey []string` to `internal/stmt.Select`. `repository.Repository[T].FindMany`
(`repository/repository.go`, the one caller that supports `Limit`/`Offset` against a real entity
table) populates it from `r.meta.PrimaryKey`. `repository.Aggregate`'s own `stmt.Select`
construction (`repository/aggregate.go`) deliberately left empty — an aggregate query's result
columns don't include the source entity's primary key. `driver/postgres`/`driver/mysql`/
`driver/sqlite` all ignore the new field — confirmed zero behavior change (`go build`/`go test
./... -short` green across the board before writing any `driver/mssql` code).
**Requirement**: MSSQL-07
**Gate**: `go build ./... && go test ./... -short` (done, green)

---

### T1: `go.mod` dependency + package skeleton ✅ DONE

**What**: Added `github.com/microsoft/go-mssqldb` to `go.mod`. Created `driver/mssql/mssql.go`
(`Options` — `DSN`/`Host`/`Port`/`User`/`Password`/`Database`/`Logging`/`Logger`, `New`,
`sqlIface`), mirroring `driver/mysql/mysql.go`'s shape.
**Depends on**: none
**Requirement**: MSSQL-01
**Gate**: `go build ./...`

---

### T2: `dsn.go` — `resolveDSN` ✅ DONE

**What**: `driver/mssql/dsn.go` — `resolveDSN(o *Options) (string, error)`, `net/url`-based (like
`driver/postgres`'s), DSN format `sqlserver://user:pass@host:port?database=dbname` (confirmed via
design.md's probe). Same precedence rules as every prior adapter.
**Depends on**: T1
**Requirement**: MSSQL-01
**Tests**: unit — mirroring `driver/postgres/dsn_test.go`'s case list (DSN-only, fields-only,
both-set partial overrides, neither-set error, malformed-DSN error)

---

### T3: `connector.go` ✅ DONE

**What**: `driver/mssql/connector.go` — `connector{opts, db}`, `Connect()`/`Close()`/`log()`,
mirroring `driver/mysql/connector.go`'s shape.
**Depends on**: T2
**Requirement**: MSSQL-01
**Tests**: unit (`go-sqlmock`), mirroring `driver/mysql/connector_test.go`

---

### T4: `dialect.go` — `Bind`/`Scan` ✅ DONE

**What**: `driver/mssql/dialect.go`'s `Bind`/`Scan` methods, simple pass-through shape (mirroring
`driver/postgres`'s — confirmed via design.md's probe that `BIT`/`DATETIME2` scan natively into
`bool`/`time.Time`, no conversion needed). `UUID` kind: `Bind` accepts plain `string`/`[16]byte`
same as every other adapter; `Scan` accepts plain `string` (matching the *documented* contract even
though the real path's raw value needs `normalizeCell`'s reversal first, same "Bind/Scan stay dead
code" stance as every adapter since AD-037).
**Depends on**: T1
**Requirement**: MSSQL-04
**Tests**: unit, mirroring `driver/postgres/dialect_test.go`'s Bind/Scan test list per `ColumnType` kind

---

### T5: `dialect.go` — `CompileSelect`/`CompileDelete`/`compilePredicate`/`lockClauseSQL` ✅ DONE

**What**: Square-bracket `quoteIdent`, `@p`-numbered `compilePredicate` (`argOffset *int` counter,
ported from `driver/postgres`'s shape with `$` → `@p`), `OFFSET y ROWS FETCH NEXT x ROWS ONLY`
pagination. When `s.Limit`/`s.Offset` is set and `s.OrderBy` is empty: if `s.PrimaryKey` is
non-empty, inject `ORDER BY` on those columns; if `s.PrimaryKey` is ALSO empty (the
`repository.Aggregate` case), return a compile error rather than emit invalid SQL Server syntax
(confirmed via design.md's probe that bare `OFFSET/FETCH` is a hard SQL Server error, not just
unspecified ordering). `lockClauseSQL` builds the *table hint* (`WITH (UPDLOCK, ROWLOCK[, NOWAIT|
READPAST])` / `WITH (HOLDLOCK, ROWLOCK)`), attached after the table reference in `CompileSelect`,
not as a trailing clause — this is the one place `CompileSelect`'s overall structure differs from
every prior adapter's, not just `lockClauseSQL`'s content.
**Depends on**: T4, T0
**Requirement**: MSSQL-07, MSSQL-08
**Tests**: unit, mirroring `driver/postgres/dialect_test.go`'s CompileSelect/CompileDelete/
compilePredicate list, plus new cases: OFFSET/FETCH with explicit OrderBy, OFFSET/FETCH without
OrderBy but with PrimaryKey (asserts the injected default), OFFSET/FETCH without OrderBy AND
without PrimaryKey (asserts the compile error), all 4 confirmed lock-hint combinations,
`NoKeyUpdate`/`KeyShare` still erroring (unsupported)

---

### T6: `dialect.go` — `Insert`/`Update` (`OUTPUT INSERTED.*`) + `collectRows`/`normalizeCell` ✅ DONE

**What**: `collectRows` ported from `driver/mysql`'s (generic `database/sql` row→map scanning) +
new `normalizeCell` handling exactly one case — a column reported as `"UNIQUEIDENTIFIER"` via
`rows.ColumnTypes()`: reverse the first 3 byte groups (4/2/2 bytes) of the raw `[]byte`, leave the
last 2 groups (2/6 bytes) unchanged, format as a hyphenated UUID string (confirmed exact transform
via design.md's probe). `Insert`: `INSERT INTO ... OUTPUT INSERTED.* VALUES (...)`, single
round-trip via `QueryContext` (not `ExecContext`) + `collectRows`. `Update`: same shape,
`UPDATE ... SET ... OUTPUT INSERTED.* WHERE ...`. This is also what `SaveOne`/`SaveMany` need —
they route through this same `Update`, no separate method (see design.md's Correction section).
**Depends on**: T5
**Requirement**: MSSQL-04, MSSQL-05
**Tests**: unit (`go-sqlmock`) — Insert/Update happy path, zero-rows-back error, `normalizeCell`
unit tests (UUID byte-reversal correctness, non-UUID columns left untouched, malformed-length
input left untouched)

---

### T7: `dialect.go` — `Query`/`Exec`/`ExecRaw` ✅ DONE

**What**: Straight ports of `driver/mysql`'s versions (no `OUTPUT`-shape difference — these never
used it either).
**Depends on**: T6
**Requirement**: MSSQL-01
**Tests**: unit, mirroring `driver/mysql/dialect_test.go`'s Query/Exec/ExecRaw list

---

### T8: `dialect.go` — `IsConflict`/`mapError`/`Begin`/`getExecutor` ✅ DONE

**What**: SQL Server error-code mapping per design.md (confirmed `2627`/`2601` → `ErrDuplicateKey`,
`547` → `ErrForeignKeyViolation`, via `errors.As` into `*mssql.Error`, `.Number int32` field).
`Begin`/`getExecutor` mirror `driver/mysql`'s `*sql.Tx`/type-assertion pattern exactly.
**Depends on**: T7
**Requirement**: MSSQL-09, MSSQL-10
**Tests**: unit — check whether `mssql.Error` has an exported constructor/settable fields for direct
construction (`mssql.Error{Number: 2627}`-style) before reaching for a heavier mechanism; only fall
back to a real-constraint-violation-driven test (mirroring `driver/sqlite`'s approach) if the type
genuinely can't be constructed directly in a unit test

---

### T9: Docker + Taskfile wiring ✅ DONE

**What**: `.docker/docker-compose.test.yml` gains an `mssql` service (own port, `ACCEPT_EULA=Y`,
strong `MSSQL_SA_PASSWORD`; pin `2022-latest` — matches ROADMAP.md's "2019+" target floor and is
the current LTS-equivalent GA release; the probe used `2025-latest` only for the quick empirical
check, not as a pinning recommendation). `Taskfile.yml`'s `test-integration` gains a
`GOLEM_MSSQL_TEST_DSN`-equivalent env var. README.md's M18 note (already added) covers the
`mcr.microsoft.com` reachability caveat — no further doc changes expected here.
**Depends on**: none (can happen anytime before T10)
**Gate**: `docker compose -f .docker/docker-compose.test.yml up -d --wait` brings up all services
healthy, including `mssql`

---

### T10: `connector_integration_test.go` + `conformance_integration_test.go` ✅ DONE

**What**: `driver/mssql/connector_integration_test.go` (connection lifecycle, mirroring
`driver/mysql`'s). `driver/mssql/conformance_integration_test.go` (SQL Server `Schema` DDL —
`BIT`/`UNIQUEIDENTIFIER`/`DATETIME2`/`NVARCHAR`/`VARBINARY(MAX)` per INSIGHT.md's type table,
`IDENTITY(1,1)` for autoincrement PKs — + `Capabilities` per design.md: `Update`/`Share`/`NoWait`/
`SkipLocked` `true`, `NoKeyUpdate`/`KeyShare` `false`), calling `dialecttest.Run`.
**Depends on**: T3, T6, T8, T9
**Requirement**: MSSQL-01, MSSQL-02, MSSQL-03
**Gate**: `task test-integration` green (real SQL Server via Docker)

---

### T11: Coverage + docs ✅ DONE

**What**: Close any unit-test coverage gaps to 100% (or documented accepted exceptions, applying
`internal/must`/`internal/testutil` where genuinely applicable — same bar M15-M17 established).
Update `.specs/project/ROADMAP.md` (M18 → DONE), `.specs/project/STATE.md` (new AD covering the
`stmt.Select.PrimaryKey` core-schema addition, the confirmed `UNIQUEIDENTIFIER` byte-reversal
finding, and the "SaveOne/SaveMany were never upsert" correction), `README.md` (Implementation
Status checkbox).
**Depends on**: T10
**Gate**: `task coverage` shows 100% (or documented exceptions); `task test-integration` still green
