# SQLite Adapter (M17) Tasks

**Design**: `.specs/features/sqlite-adapter/design.md`
**Status**: DONE. `go test -tags=integration ./driver/sqlite/...` green (`TestSQLite_Conformance`,
`TestConnector_*`) against a real in-memory SQLite database — no Docker involved. T8's flagged
uncertainty (`sqlite.Error.Code()` extended vs. primary) confirmed empirically: extended (2067/
1555/787). One unplanned correction found mid-T4/T6 (not in the original task list): `Bind` is
never actually called by `repository` (same dead-code fact AD-037 already established for Postgres)
— so `Bind`'s original date/time TEXT-formatting design silently never took effect; fixed by
simplifying `Bind`/`Scan` to a plain `time.Time` pass-through (matching `driver/postgres`) and
forcing `_time_format=sqlite` in the DSN instead, with `collectRows`'s `normalizeCell` handling the
one remaining gap (TIME-declared columns) the driver doesn't already auto-convert. See AD-040/AD-041
in STATE.md.

Sequential — each task depends on the previous one's types/files existing, same approach as
M15/M16 (executed directly, no sub-agent dispatch). Unlike M16, there is no T0 core-schema change:
`stmt.Insert.PrimaryKey`/`stmt.Update.PrimaryKey` already exist (added for M16) and this adapter
simply doesn't need them (`RETURNING *` replaces their reason for existing). There is also no
Docker/Taskfile wiring task — this adapter needs neither, which is the point of M17 existing at
all.

---

## Task Breakdown

### T1: `go.mod` dependency + package skeleton

**What**: Add `modernc.org/sqlite` to `go.mod`. Create `driver/sqlite/sqlite.go` (`Options` —
`Path`/`Logging`/`Logger` only, `New`, `sqlIface` — same shape as `driver/mysql/mysql.go`'s but
without the network-related fields).
**Depends on**: none
**Requirement**: SQLITE-01
**Gate**: `go build ./...`

---

### T2: `dsn.go` — `resolveDSN`

**What**: `driver/sqlite/dsn.go` — `resolveDSN(o *Options) string` (no error return, per design.md:
nothing to fail at this layer). `Path == ""` → `:memory:`. Builds
`"file:" + path + "?" + pragmas`, prepending `cache=shared` only when `path == ":memory:"`.
Pragmas always forced: `_pragma=foreign_keys(1)`, `_pragma=busy_timeout(5000)`.
**Depends on**: T1
**Requirement**: SQLITE-01, SQLITE-03
**Tests**: unit — `:memory:` default, explicit file path (no `cache=shared`), pragma string
construction

---

### T3: `connector.go`

**What**: `driver/sqlite/connector.go` — `connector{opts, db}`, `Connect()`/`Close()`/`log()`,
mirroring `driver/mysql/connector.go`'s shape (swappable `newSQLDB` var for test injection, `Ping()`
after open, lifecycle logging). `Connect()` additionally calls `db.SetMaxOpenConns(1)`,
unconditionally, right after `sql.Open` succeeds and before `Ping()`.
**Depends on**: T2
**Requirement**: SQLITE-01
**Tests**: unit (`go-sqlmock`), mirroring `driver/mysql/connector_test.go`

---

### T4: `dialect.go` — `Bind`/`Scan`

**What**: `driver/sqlite/dialect.go`'s `Bind`/`Scan` methods per design.md's value-conversion
table: `boolean` binds/scans as Go `bool` directly (no MySQL-style string workaround);
`smallint`/`integer`/`bigint` as `int64`; `decimal`/`float` as `float64`; `char`/`varchar`/`text`/
`json`/`uuid` as `string`; `date`/`datetime`/`time` formatted to/parsed from `TEXT` using the three
layouts in design.md (`time` uses the same fixed-reference-date convention as
`driver/mysql`'s `parseMySQLTime`); `blob` as `[]byte`.
**Depends on**: T1
**Requirement**: SQLITE-05, SQLITE-06, SQLITE-07, SQLITE-08
**Tests**: unit, one round-trip case per `ColumnType` kind (mirroring
`driver/mysql/dialect_test.go`'s Bind/Scan list), plus explicit date/datetime/time format-string
assertions

---

### T5: `dialect.go` — `CompileSelect`/`CompileDelete`/`compilePredicate`/`lockClauseSQL`

**What**: Double-quote `quoteIdent` (ported from `driver/postgres`, not `driver/mysql`),
`?`-placeholder `compilePredicate` (no `argOffset`, ported from `driver/mysql`'s version),
`aggregateSQLFunc`/`projectionSQL`, pagination (`LIMIT ? OFFSET ?`). `lockClauseSQL` has **no
success branch at all** — every `Strength` value returns an error (SQLite has no row-lock clause).
**Depends on**: T4
**Requirement**: SQLITE-09 (partial — locking half), SQLITE-13, SQLITE-14
**Tests**: unit, mirroring `driver/postgres/dialect_test.go`'s CompileSelect/CompileDelete/
compilePredicate list, plus a direct test asserting `lockClauseSQL` errors for every one of the 4
strengths (this path is never reached via the conformance suite, since `Capabilities.Locking` is
all-`false` — needs its own test to hit the coverage bar)

---

### T6: `dialect.go` — `Insert`/`Update` (`RETURNING *`) + `collectRows`

**What**: `collectRows` ported from `driver/mysql/dialect.go` (generic `database/sql` row→map
scanning; no `normalizeCell`-equivalent needed here, per design.md — `Bind`/`Scan` already own
every conversion). `Insert`: `INSERT INTO ... VALUES (...) RETURNING *`, single round-trip,
`QueryContext` + `collectRows`, error if zero rows come back. `Update`: `UPDATE ... SET ... WHERE
... RETURNING *`, same single-round-trip shape — no PK-capture-before-write logic (that's MySQL-
only). `stmt.Insert.PrimaryKey`/`stmt.Update.PrimaryKey` fields are read from but otherwise
ignored (same as `driver/postgres`).
**Depends on**: T5
**Requirement**: SQLITE-09, SQLITE-10
**Tests**: unit (`go-sqlmock`) — insert happy path, insert-zero-rows-back error, update happy path
(0 rows, 1 row, multiple rows), mirroring `driver/postgres/dialect_test.go`'s Insert/Update list
(not `driver/mysql`'s, since the SQL shape now matches Postgres's)

---

### T7: `dialect.go` — `Query`/`Exec`/`ExecRaw`

**What**: Straight ports of `driver/mysql`'s versions (no `RETURNING`-shape difference — these
never used it either).
**Depends on**: T6
**Requirement**: SQLITE-01
**Tests**: unit, mirroring `driver/mysql/dialect_test.go`'s Query/Exec/ExecRaw list

---

### T8: `dialect.go` — `IsConflict`/`mapError`/`Begin`/`getExecutor`

**What**: SQLite error-code mapping per design.md: `errors.As` into `*sqlite.Error`, `IsConflict`
true when `Code()&0xFF == 19` (`SQLITE_CONSTRAINT`), `mapError` maps `2067`/`1555`
(`UNIQUE`/`PRIMARYKEY`) → `golem.ErrDuplicateKey`, `787` (`FOREIGNKEY`) → `golem.
ErrForeignKeyViolation`. **First**: write one throwaway/manual test against a real
`:memory:` DB provoking a UNIQUE and a FOREIGN KEY violation to confirm `Error.Code()` actually
returns the extended code (2067/787) by default, not the primary one (19) — design.md flagged this
as unconfirmed. If it returns only the primary code, find and apply whatever config
(`_pragma`/driver option) enables extended codes before writing the real unit tests. `Begin`/
`getExecutor` mirror `driver/mysql`'s `*sql.Tx`/type-assertion pattern exactly.
**Depends on**: T7
**Requirement**: SQLITE-11, SQLITE-12
**Tests**: unit, mirroring `driver/mysql/dialect_test.go`'s IsConflict/Begin/getExecutor list, all
`SQLITE_CONSTRAINT*` branches (`UNIQUE`, `PRIMARYKEY`, `FOREIGNKEY`, plus one unmapped variant e.g.
`CHECK`/`NOTNULL` to prove `IsConflict` true / sentinel-less passthrough)

---

### T9: `connector_lifecycle_test.go` + `conformance_test.go`

**What**: `driver/sqlite/connector_lifecycle_test.go` (connection lifecycle against a real
`:memory:` DB). `driver/sqlite/conformance_test.go` — SQLite `Schema` (plain
`CREATE TABLE`, `INTEGER PRIMARY KEY` for autoincrement-eligible PKs, `FOREIGN KEY ... ON DELETE
{CASCADE|SET NULL|RESTRICT}`) + `sqliteCapabilities` (every `Locking` field `false`), calling
`dialecttest.Run(t, sqliteConformanceSchema, sqliteCapabilities, New(func(o *Options) { o.Path =
":memory:" }))` — no DSN env var, no Docker service. **No `//go:build integration` tag** (revised
mid-implementation, see design.md's Testing section): neither file opens a real network connection,
so per TESTING.md's own classification rule these are unit tests, not integration ones — running
them under the default `go test ./... -short` / `task coverage` is also what finally gives
`internal/dialecttest` non-zero coverage (previously 0%, since every prior adapter's conformance
suite genuinely needs Docker and was invisible to `-short`).
**Depends on**: T3, T6, T8
**Requirement**: SQLITE-01, SQLITE-02, SQLITE-03, SQLITE-04
**Gate**: `go test ./driver/sqlite/... -short` green with zero Docker/Compose involvement (also
still runs under `-tags=integration ./...` and `task test-integration`, harmlessly twice)

---

### T10: Coverage + docs

**What**: Close any unit-test coverage gaps to 100% (or documented accepted exceptions, same class
as M15/M16's). Update `.specs/project/ROADMAP.md` (M17 → DONE), `.specs/project/STATE.md` (new AD
for the `RETURNING`-based Insert/Update decision, the all-`false` `Capabilities.Locking` precedent,
and the `Error.Code()` extended-vs-primary confirmation from T8), `README.md` (Implementation
Status checkbox).
**Depends on**: T9
**Gate**: `task coverage` shows 100% (or documented exceptions); `task test-integration` still green
