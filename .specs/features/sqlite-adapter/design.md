# SQLite Adapter (M17) — Design

**Spec**: `.specs/features/sqlite-adapter/spec.md`
**Status**: Draft

## Package

`driver/sqlite` (`github.com/leandroluk/golem/driver/sqlite`), same file layout as `driver/mysql`:
`sqlite.go` (`Options`/`New`), `connector.go`, `dsn.go`, `dialect.go`, plus `_test.go`/
`_integration_test.go` pairs for each.

## Driver library

`modernc.org/sqlite` via `database/sql` — pure Go (no cgo/C compiler), per user decision recorded
in spec.md's Out of Scope. Registers the `"sqlite"` `database/sql` driver name. Confirmed via
Context7 + web search: current `modernc.org/sqlite` bundles SQLite 3.53.2 — comfortably past 3.35.0
(2021), the version that introduced the `RETURNING` clause (sqlite.org/lang_returning.html).

**This changes the Insert/Update shape versus M16.** Unlike MySQL, SQLite supports
`INSERT ... RETURNING *` / `UPDATE ... RETURNING *` natively — no `LAST_INSERT_ID()` + follow-up
`SELECT` dance needed. `driver/sqlite`'s `Insert`/`Update` are architecturally closer to
`driver/postgres`'s single-round-trip shape than to `driver/mysql`'s multi-round-trip one, even
though the connection model (embedded, no server) is totally different. `stmt.Insert.PrimaryKey`/
`stmt.Update.PrimaryKey` (added in M16, AD-038) are simply unused here — same as
`driver/postgres` already ignores them (zero value, no behavior change, no core code touched).

`sqlIface` (same abstraction pattern as `driver/mysql`'s) wraps `*sql.DB`'s surface so
`DATA-DOG/go-sqlmock` can substitute for it in unit tests — `go.mod` already carries this
dependency (added for M16), now used by a second adapter.

## Connector / DSN

No network address space to resolve (no host/port/user/password) — `Options` is intentionally
much smaller than Postgres's/MySQL's:

```go
type Options struct {
	Path    string // file path, or ":memory:" (default when empty)
	Logging bool
	Logger  golem.Logger
}
```

`resolveDSN(o *Options) string` — unlike Postgres's/MySQL's version, this never fails (no DSN
string to parse, no malformed-input case exists at this layer), so it returns a plain `string`,
not `(string, error)`:

- `Path == ""` defaults to `:memory:`.
- Builds a `modernc.org/sqlite` URI: `"file:" + path + "?" + pragmas`.
- When `path == ":memory:"`, prepends `cache=shared` to the query string — the standard SQLite URI
  idiom (sqlite.org's "In-Memory Databases" page) for making a named in-memory database visible to
  more than one connection within the same process, instead of each new connection getting its own
  private, empty `:memory:` database.
- Pragmas are always forced, not user-configurable (internal implementation requirements, same
  category as M16's `parseTime=true`):
  - `_pragma=foreign_keys(1)` — SQLite defaults foreign key enforcement OFF for backward
    compatibility; without this, `Cascade`/`ConflictDetection`'s FK-violation subtests would
    silently pass with no constraint enforced at all.
  - `_pragma=busy_timeout(5000)` — cheap insurance against spurious `SQLITE_BUSY` under the
    connection-pool contention described below, never user-facing.

`connector.Connect()` mirrors `driver/mysql`'s shape (`sql.Open` → `Ping` → wrap in `dialect{db}`),
with one addition: **`db.SetMaxOpenConns(1)`**, always, non-configurable. Two independent reasons
converge on the same fix:

1. SQLite itself is single-writer — concurrent writers from a naive pool just serialize behind
   `SQLITE_BUSY` retries anyway.
2. A `:memory:` database (even with `cache=shared`) still depends on at least one connection
   staying open; bounding the pool to exactly one connection removes any risk of the pool silently
   opening a second, divergent connection under load.

This is the mechanism behind spec.md's Edge Case about concurrent in-memory writers — solved once,
at the connector level, not per-caller.

## Identifier quoting, placeholders, lock syntax

- **Identifiers**: double-quoted (`"col"`, `"table"."col"`) — SQLite follows the SQL standard here,
  so `quoteIdent` is a straight port of `driver/postgres`'s version, not `driver/mysql`'s backtick
  one.
- **Placeholders**: unnumbered `?`, same as `driver/mysql` — `compilePredicate` drops the
  `argOffset *int` parameter entirely (SQLite's driver also supports `?NNN`/`:name`, but plain `?`
  matches the simpler MySQL-style compiler already written).
- **Pagination**: `LIMIT ? OFFSET ?` — identical shape to both existing adapters (per INSIGHT.md).
- **Upsert**: `INSERT ... ON CONFLICT (key) DO UPDATE SET ...` — same syntax family as Postgres
  (per INSIGHT.md), so `SaveOne`/`SaveMany`'s compiled SQL shape is closer to `driver/postgres`'s
  than to `driver/mysql`'s `ON DUPLICATE KEY UPDATE`.
- **Locking**: SQLite has **no row-level locking clause at all** — no `SELECT ... FOR ...` syntax
  of any kind (it locks at the database-file level instead). `lockClauseSQL` therefore has **no
  success case** — every `Strength` value (`update`/`no_key_update`/`share`/`key_share`) falls into
  the same `default: return error` branch, unconditionally. This is the first adapter where that
  function is all default-case, and it's a deliberate, spec'd outcome (spec.md P3 AC7), not a
  placeholder to fill in later. `Capabilities.Locking` for `driver/sqlite` is every field `false`
  (`Update`, `NoKeyUpdate`, `Share`, `KeyShare`, `NoWait`, `SkipLocked`) — `internal/dialecttest`'s
  `Locking` subtest group skips entirely for this adapter, same "loud error, never silent no-op"
  contract M14/M16 already established, just with a wider unsupported set. `lockClauseSQL`'s
  always-errors behavior still needs its own direct unit test (not exercised by the conformance
  suite at all, since every `Capabilities.Locking` field is `false`), to keep the 100%-coverage bar.

## Value conversion (`Bind`/`Scan`)

SQLite has 5 storage classes (NULL/INTEGER/REAL/TEXT/BLOB) and *type affinity*, not a fixed column
type system — every `golem.ColumnType` kind maps onto one of these 5, per INSIGHT.md:

| `golem.ColumnType` kind | SQLite storage class | Bind produces | Scan expects |
| --- | --- | --- | --- |
| `boolean` | INTEGER (0/1) | `bool` (driver.Value permits `bool` directly — SQLite's own boolean handling, no MySQL-style string workaround needed) | `bool`, `int64` fallback |
| `smallint`/`integer`/`bigint` | INTEGER | `int64` | `int64` |
| `decimal`/`float` | REAL | `float64` | `float64` |
| `char`/`varchar`/`text`/`json` | TEXT | `string` | `string`, `[]byte` fallback |
| `uuid` | TEXT | `string` (same `[16]byte`→string formatting as `driver/mysql`'s `CHAR(36)` case) | `string`, `[]byte` fallback |
| `date` | TEXT | `string`, formatted `"2006-01-02"` | parsed back to `time.Time` (date-only, UTC midnight) |
| `datetime` | TEXT | `string`, formatted `"2006-01-02 15:04:05.999999999"` | parsed back to `time.Time` |
| `time` | TEXT | `string`, formatted `"15:04:05.999999999"` (same fixed-reference-date pattern as `driver/mysql`'s `parseMySQLTime`, since Go has no time-of-day-only type) | parsed back to `time.Time` on a fixed reference date (`2000-01-01`, matching M16's convention) |
| `blob` | BLOB | `[]byte` | `[]byte` |

**Decision: do the date/time formatting ourselves in `Bind`/`Scan`, rather than relying on
`modernc.org/sqlite`'s `_pragma`/`_texttotime`/`_inttotime` DSN options.** Those options primarily
affect `ColumnType.ScanType()` metadata (used by reflection-based scanning helpers like `sqlx`),
not necessarily the raw value returned by a generic `Scan(&any)` — which is what `collectRows`
(ported from `driver/mysql`, see below) uses. Formatting/parsing explicitly in `Bind`/`Scan` avoids
depending on unconfirmed driver-internal behavior and matches how `driver/mysql` already handles
its own ambiguous case (`TIME` via `parseMySQLTime`) — golem's `Dialect` boundary owns this
conversion, not the underlying driver.

**`DECIMAL(p,s)` precision note** (spec.md P2 AC2): SQLite's REAL storage class is an IEEE 754
double, not an arbitrary-precision decimal. This is a real, documented limitation of this adapter,
not a bug — no attempt is made to emulate exact fixed-point semantics.

## Row scanning (`collectRows`)

Ported near-verbatim from `driver/mysql/dialect.go`'s `collectRows` (`database/sql`-generic
`Rows.Scan([]any)` into freshly-allocated `any` destinations, keyed by column name). Unlike MySQL,
no `normalizeCell`-style post-processing by reported SQL column type is needed: `driver.Value`'s
contract restricts every driver (including `modernc.org/sqlite`) to exactly `int64`, `float64`,
`bool`, `[]byte`, `string`, `time.Time`, or `nil` — and per the Bind/Scan table above, `Scan`
already handles every one of those raw Go types per `golem.ColumnType` kind directly, with no
column-type-name lookup required (SQLite's dynamic typing turns out to need *less*
disambiguation machinery here than MySQL's fixed-but-ambiguously-surfaced column types did,
because this adapter never asks the driver to guess — every value it stores was formatted by
`Bind` in the first place).

## Insert / Update (single round-trip, `RETURNING *`)

Mirrors `driver/postgres`'s shape exactly, swapping `pgx.CollectOneRow`/`pgx.CollectRows` for the
ported `collectRows` helper (database/sql-based, like `driver/mysql`):

```go
func (d *dialect) Insert(ctx context.Context, conn golem.Conn, s *stmt.Insert) (map[string]any, error) {
	// INSERT INTO "table" ("col1","col2") VALUES (?,?) RETURNING *
	rows, err := d.getExecutor(conn).QueryContext(ctx, sql, args...)
	...
	results, err := collectRows(rows)
	// error if len(results) == 0 — mirrors driver/mysql's "no row found after insert" case
	return results[0], nil
}
```

`Update` follows the same pattern as `driver/postgres`'s: build `UPDATE ... SET ... WHERE ...`,
append `RETURNING *`, `QueryContext`, `collectRows`, return the full slice. No PK-capture-before-
write dance (that's purely a MySQL/no-`RETURNING` workaround) — `RETURNING *` already returns the
post-update rows in the same statement, so there's no window where `Sets` could invalidate the
original `Where` before it's evaluated.

## Error mapping (`IsConflict`, `mapError`)

`modernc.org/sqlite` returns `*sqlite.Error` with a `Code() int` method giving the **extended**
SQLite result code (confirmed via Context7: `SQLITE_CONSTRAINT_UNIQUE = 2067`,
`SQLITE_CONSTRAINT_PRIMARYKEY = 1555`, `SQLITE_CONSTRAINT_FOREIGNKEY = 787`,
`SQLITE_CONSTRAINT = 19` as the generic primary code). SQLite's own convention: an extended code's
low byte (`code & 0xFF`) always equals its primary result code — so `IsConflict` checks
`code&0xFF == 19` (`SQLITE_CONSTRAINT`) as a Postgres-style "whole class" catch-all, while
`mapError` maps the two specific extended codes golem has sentinels for:

- `SQLITE_CONSTRAINT_UNIQUE` (2067) or `SQLITE_CONSTRAINT_PRIMARYKEY` (1555) → `golem.ErrDuplicateKey`
- `SQLITE_CONSTRAINT_FOREIGNKEY` (787) → `golem.ErrForeignKeyViolation`

Any other `SQLITE_CONSTRAINT*` variant (`CHECK`, `NOTNULL`, etc.) reports `IsConflict() == true`
but wraps with no specific sentinel (passes through unchanged) — same "unmapped errors pass through"
stance as both existing adapters. **Flagged as needing implementation-time confirmation**: whether
`modernc.org/sqlite` enables extended result codes by default (vs. requiring an explicit
`_pragma`/API call) wasn't independently confirmed via Context7/web search — verify empirically
against a real constraint violation while implementing `mapError`'s unit test; if it turns out
`Code()` returns only the primary code (19) by default, add whatever `_pragma`/config the driver
exposes to enable extended codes, since the two specific sentinels genuinely need the extended
variant to distinguish UNIQUE from FOREIGNKEY.

## Transactions (`Begin`/`getExecutor`)

Identical shape to `driver/mysql`'s: `*sql.DB.BeginTx(ctx, nil)` → `*sql.Tx`, wrapped the same way;
`getExecutor` does the same `golem.Tx.Underlying().(*sqliteTx)` type-assertion dance.

## Testing

- **Unit** (`_test.go`, no build tag): `DATA-DOG/go-sqlmock` substituting for `sqlIface`, covering
  Bind/Scan per kind (including date/time formatting round-trips and the fixed-reference-date TIME
  case), CompileSelect/Delete, Insert/Update (`RETURNING *` shape), `lockClauseSQL`'s
  always-errors behavior (every strength, since conformance never exercises it), IsConflict/
  mapError (all `SQLITE_CONSTRAINT*` branches), Begin/getExecutor, DSN resolution
  (`:memory:` vs. file path, pragma string construction). Target: 100% statement coverage, same
  bar as every other package.
- **Conformance/lifecycle** (`connector_lifecycle_test.go` + `conformance_test.go`, **no
  `//go:build integration` tag**, revised from an earlier draft that did tag them): per
  TESTING.md's own rule of thumb ("anything that opens a real network connection is integration;
  anything else is unit"), an in-memory SQLite database opens no network connection at all — so
  gating these behind the same build tag `driver/postgres`/`driver/mysql` genuinely need (real
  Docker containers) was an unwarranted carry-over from their file-naming pattern, not a real
  requirement. Untagging them means they run under the default `go test ./... -short` / `task
  coverage`, which is also what gives `internal/dialecttest` non-zero coverage for the first time —
  every prior adapter's conformance suite needs Docker, so `internal/dialecttest` was previously
  only ever exercised via `-tags=integration`, invisible to `task coverage`'s `-short` run. Both
  still open `sqlite.New(func(o *Options) { o.Path = ":memory:" })` directly — **no
  `GOLEM_SQLITE_TEST_DSN` env var, no `docker-compose` service, no `Taskfile.yml` change** needed at
  all, and `task test-integration`'s `-tags=integration ./...` still picks these files up too
  (untagged files compile into every build), so they simply run twice — once via `-short`, once
  via `-tags=integration` — which is harmless given how fast an in-memory database is.
- **Conformance schema DDL**: uses plain `CREATE TABLE` (not `CREATE TEMPORARY TABLE`) — SQLite
  temporary tables are connection-scoped, and this adapter's connection pool is already bounded to
  1 connection per `Run` call (its own `:memory:` database, discarded when the test's `DataSource`
  closes), so an ordinary `CREATE TABLE` is already exactly as ephemeral as `driver/postgres`'s
  temporary tables are, without needing the `TEMPORARY` keyword at all. `PRIMARY KEY AUTOINCREMENT`
  on an `INTEGER PRIMARY KEY` column, `FOREIGN KEY ... ON DELETE {CASCADE|SET NULL|RESTRICT}` (only
  enforced because `resolveDSN` forces `_pragma=foreign_keys(1)`).

## Tech Decisions (only non-obvious ones)

| Decision | Choice | Rationale |
| --- | --- | --- |
| Driver library | `modernc.org/sqlite` (pure Go) over `mattn/go-sqlite3` (cgo) | User decision — no C toolchain dependency across 3 CI OSes; matches M17's "no external dependency" goal |
| Insert/Update round-trip | Single round-trip via `RETURNING *`, `driver/postgres`-shaped | SQLite (3.53.2, bundled) supports `RETURNING` since 3.35.0 — no `LAST_INSERT_ID()` workaround needed, unlike M16 |
| Foreign key enforcement | Always force `PRAGMA foreign_keys = ON` via DSN, non-optional | SQLite defaults this OFF; without it, Cascade/ConflictDetection FK tests would silently pass unenforced |
| Connection pool size | Always `SetMaxOpenConns(1)`, non-optional | SQLite is single-writer; also required so a shared `:memory:` DSN doesn't fragment across multiple pooled connections |
| Date/time representation | `Bind`/`Scan` format/parse text explicitly, not driver DSN options | Avoids depending on `_texttotime`/`_inttotime`'s unconfirmed effect on generic `Scan(&any)`; golem's `Dialect` boundary owns the conversion, consistent with M16's own `TIME` handling |
| Locking | Every strength unconditionally unsupported | SQLite has no row-level lock clause of any kind — first adapter where `Capabilities.Locking` is entirely `false` |
