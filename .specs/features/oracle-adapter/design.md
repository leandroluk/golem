# Oracle Adapter (M19) — Design

## Research note

Every design decision below was verified against a real Oracle Database 23ai Free container
(`gvenzl/oracle-free:23-slim`, Docker Hub, no Oracle Container Registry auth wall — reachable and
pulled successfully from this environment, unlike M18's `mcr.microsoft.com` friction), via a
throwaway Go probe using `github.com/sijms/go-ora/v2` directly against `database/sql`. Several
initial assumptions carried over from M18/INSIGHT.md turned out to be wrong for Oracle specifically
— see the "Confirmed via empirical probe" section for the exact commands/output this design is
based on.

## Package

`driver/oracle`, mirroring `driver/mssql`'s file layout: `oracle.go` (`Options`/`New`),
`dsn.go` (`resolveDSN`), `connector.go`, `dialect.go`, plus `*_test.go` unit tests and
`connector_integration_test.go`/`conformance_integration_test.go` (both `//go:build integration` —
Oracle, like SQL Server, genuinely needs a running server; no in-memory mode like SQLite).

## Driver library

`github.com/sijms/go-ora/v2` (user decision — pure Go, no cgo, no Oracle Instant Client needed,
keeping every adapter this project builds itself free of native dependencies, same property
`driver/sqlite` established in M17). DSN scheme: `oracle://user:password@host:port/service_name`
(go-ora's own URL format, confirmed via probe — no separate `?database=` query param like MSSQL's
DSN shape; the service name is a URL path segment, closer to Postgres's shape).

## Minimum version: Oracle 12c+

User decision, locked before design started. `OFFSET/FETCH` pagination requires 12c+; earlier
versions need a `ROWNUM`-subquery `CompileSelect` path, explicitly out of scope (ROADMAP.md/spec.md).

## Identifier quoting, placeholders, pagination

- **Quoting**: double-quote (`"col"`, `"table"."col"`) — Oracle's ANSI-standard identifier quoting,
  same family as Postgres/SQLite, not MySQL's backticks or MSSQL's brackets.
- **Placeholders**: `:1`, `:2`, ... (numbered, colon-prefixed) — confirmed via probe against
  `go-ora`'s driver; NOT `?` (MySQL/SQLite) or `$N`/`@pN` (Postgres/MSSQL).
- **Pagination**: `OFFSET y ROWS FETCH NEXT x ROWS ONLY` (12c+, identical clause text to M18/MSSQL).
  **Critical divergence from M18, confirmed empirically**: unlike SQL Server, **Oracle does NOT
  require an `ORDER BY` for `OFFSET/FETCH` to be valid syntax** — `SELECT * FROM t OFFSET 0 ROWS
  FETCH NEXT 1 ROWS ONLY` with no `ORDER BY` at all executes without error. This means
  `stmt.Select.PrimaryKey` (added in M18, core schema) is read but **never needs to force-inject a
  default `ORDER BY`** for this adapter — `CompileSelect` only emits `ORDER BY` when the caller
  explicitly set `s.OrderBy`, exactly like Postgres/MySQL/SQLite's `LIMIT/OFFSET` shape, just with
  different pagination clause text. `PrimaryKey` stays unused here (same "read but unused" stance as
  every non-MSSQL adapter's `stmt.Insert/Update.PrimaryKey`).
- **`repository.Exists()`'s `Count: true` + pagination query — a second divergence from M18,
  confirmed empirically and worth flagging explicitly**: Oracle rejects `ORDER BY <real column>`
  on a bare `COUNT(*)` (no `GROUP BY`) with `ORA-00937: not a single-group group function` — same
  restriction M18 hit. But unlike MSSQL, **Oracle also rejects the `ORDER BY (SELECT NULL)` idiom**
  that solved it there (`ORA-00937` again — Oracle applies the same-group-function check to any
  `ORDER BY` expression, not just direct column references). This doesn't matter for Oracle, though,
  because of the point above: since `OFFSET/FETCH` never requires an `ORDER BY` here, the fix is
  simply to never emit one for a `Count` query — `CompileSelect` skips `ORDER BY` entirely whenever
  `s.Count` is `true` and `s.OrderBy` is empty (not "inject a workaround," just "don't emit the
  clause Oracle doesn't need and would reject anyway").

## Value conversion (`Bind`/`Scan`)

**The one part of M19 with no direct precedent** — Oracle represents every numeric
`golem.ColumnType` kind (`boolean`→`NUMBER(1)`, `smallint`→`NUMBER(5)`, `integer`→`NUMBER(10)`,
`bigint`→`NUMBER(19)`, `decimal`→`NUMBER(p,s)`, `float`→`FLOAT`, itself a `NUMBER` variant) as
`NUMBER`, and `rows.ColumnTypes()[i].DatabaseTypeName()` reports the literal string `"NUMBER"` for
every one of them — no per-kind distinction the way MySQL/MSSQL's `DatabaseTypeName()` gives
(`"DECIMAL"` vs `"BIGINT"` etc.). Confirmed via probe: a generic `Scan(&any)` against every `NUMBER`
column, regardless of declared precision/scale, returns a Go **`string`** (e.g. `"123"`, `"19.99"`)
— not `int64`/`float64` natively (unlike Postgres/MSSQL/SQLite) and not `[]byte` (unlike MySQL's
ambiguous DECIMAL/TIME case).

**Final rule (see "Corrections" below for how an initial `sql.ColumnType.DecimalSize()`-based rule
was found wrong and replaced):** `normalizeCell` tries `strconv.ParseInt` first on any
`"NUMBER"`-typed column's string value; only falls back to `strconv.ParseFloat` when the string
isn't a plain integer (i.e., it has a fractional part). `boolean`'s resulting `int64` flows through
`repository.assignFieldValue`'s existing numeric→bool conversion path (M16/AD-039 — already handles
this, no new core change needed); a genuine float value that happens to be a whole number parses as
`int64` too, harmlessly, since the same `assignFieldValue` fallback converts `int64` into a
`float64` struct field without issue.

Other kinds, confirmed via probe:

- `VARCHAR2`/`CHAR`/`CLOB` (`text`/`char`/`varchar` kinds): scan natively as Go `string` — no
  normalization needed. (Note: `go-ora`'s `DatabaseTypeName()` reports `"NCHAR"` for plain
  `VARCHAR2` columns too — a driver quirk/simplification, confirmed empirically, but harmless since
  the raw value is already a correctly-decoded `string` either way.)
- `DATE`/`TIMESTAMP` (`date`/`datetime`/`time` kinds — see "Corrections" below for why `time` also
  uses `TIMESTAMP`, not `INTERVAL DAY TO SECOND`): scan natively as `time.Time` — no normalization
  needed, same as every other adapter.
- `BLOB` (`blob` kind): scans natively as `[]byte` — no normalization needed.
- `RAW`/`UUID`: **design decision — use `VARCHAR2(36)` for the `uuid` kind, not `RAW(16)`.**
  `RAW(16)` scans as raw `[]byte` requiring a byte-format decision (hex string? which endianness?)
  with no natural mapping to golem's plain hyphenated-UUID-string contract, whereas `VARCHAR2(36)`
  scans as a plain Go `string` already in the exact format every other adapter's `uuid` kind expects
  — zero-conversion, same simplicity Postgres's native `UUID` type and MySQL's `CHAR(36)` already
  have. No `normalizeCell` case needed for UUID at all as a result (unlike MSSQL's UNIQUEIDENTIFIER
  byte-reversal case).

`Bind` mirrors every other adapter's pass-through shape per `ColumnType.Kind()` (`boolean`→`bool`,
`smallint`/`integer`/`bigint`→`int64`, `decimal`/`float`→`float64`, etc.) — unit-tested directly but
dead in the real path, same documented stance as every adapter since AD-037.

## Row scanning (`collectRows`)

Same shape as `driver/mysql`/`driver/mssql`'s: `rows.Columns()` + `rows.ColumnTypes()` (for
`DatabaseTypeName()`), scan into freshly-allocated `any` destinations, normalize per cell via
`normalizeCell(dbType string, raw any) any` — same 2-parameter shape as every other adapter (an
earlier draft added a third `scale` parameter, since dropped — see "Corrections" below). Extracted
via `internal/must` (`must.Value`/`must.Exec` + `defer must.Recover(&err)`), same established
pattern as every prior adapter's `collectRows`.

## Insert / Update — multi-round-trip, mirroring `driver/mysql`, NOT `driver/postgres`/`sqlite`/`mssql`

**A real design correction, confirmed empirically, not assumed from Oracle's `RETURNING INTO`
syntax existing:** Oracle's `RETURNING col INTO :bindvar` requires **one output bind variable per
returned column, each pre-allocated with a known Go type** (via `go_ora.Out{Dest: &typedVar}`) — it
cannot express "return every column" the way Postgres/SQLite's `RETURNING *` or SQL Server's `OUTPUT
INSERTED.*` do, since there's no way to declare N out-binds for an arbitrary, statement-shape-
dependent column list without already knowing each column's Go type ahead of compile time. Building
that generically (one `Out{}` per `s.Columns`, each needing a correctly-typed destination inferred
from... nothing, since `stmt.Insert` carries no type information) isn't a viable single-round-trip
design. So `driver/oracle`'s `Insert`/`Update` are shaped like `driver/mysql`'s: plain
`INSERT`/`UPDATE` (no `RETURNING`), then a follow-up `SELECT * WHERE <primary key>` to read the row
back — reusing `stmt.Insert/Update.PrimaryKey` (M16/AD-038) for exactly the purpose it was built for.

**Where this needs one Oracle-specific piece, confirmed via probe**: Oracle has no
`sql.Result.LastInsertId()` equivalent — confirmed empirically that calling `.LastInsertId()` on the
`sql.Result` from a plain `INSERT` into a `GENERATED ALWAYS AS IDENTITY` column returns `(0, nil)`
(silently unsupported, not an error) — MySQL's `LastInsertId()` mechanism simply doesn't exist here.
The one narrow, single-column use of `RETURNING ... INTO` this adapter does need: when an
auto-generated primary key column is NOT present in `s.Columns`/`s.Values` (i.e., the entity relies
on `GENERATED ALWAYS AS IDENTITY` rather than a client-assigned key like a `UUID` string), `Insert`
appends `RETURNING <pk-col> INTO :N` to the `INSERT` statement with one extra `go_ora.Out{Dest:
&generatedID}` bind arg (`generatedID` declared as `int64` — confirmed via probe this round-trips
correctly for a `NUMBER`-backed `IDENTITY` column), executed via `db.ExecContext` (this whole
`INSERT ... RETURNING ... INTO` form is DML with an out-bind, not a resultset — `ExecContext`, never
`QueryContext`, confirmed via probe: attempting it through `Query` produces `ORA-01008: value for
bind variable placeholder was not provided`, since `Query`'s resultset-oriented path doesn't feed
out-binds the way `Exec` does). `Update` never needs this at all — it always has a `WHERE` predicate
identifying existing rows, so it captures matching primary keys *before* running the `UPDATE`
(identical reasoning to `driver/mysql`'s `Update`: re-running the original `WHERE` afterward can't
work when `Sets` modifies a column `Where` itself filters on) and reads them back the same way.

**No "lazy error" concern, unlike M18**: since Oracle's `Insert`/`Update` never use `QueryContext`
for the write itself (only `ExecContext`, with the follow-up read-back using `QueryContext`), a
constraint violation on the write statement surfaces immediately from `ExecContext`'s own returned
error — confirmed via probe (a duplicate-key `Exec` call returns the `*network.OracleError`
directly, no deferred/lazy surfacing the way `go-mssqldb`'s `QueryContext` had for M18).

## Locking

Confirmed via probe: `SELECT ... FOR UPDATE`, `FOR UPDATE NOWAIT`, and `FOR UPDATE SKIP LOCKED` all
execute without error — same trailing-clause shape as Postgres (not a table hint like MSSQL).
**Oracle has no `FOR SHARE` clause of any kind** (a real, permanent SQL-dialect gap, not a version
gate — Oracle's only row-locking read clause has ever been `FOR UPDATE`; shared/read locks are a
session/table-level concept there, `LOCK TABLE ... IN SHARE MODE`, not a per-query row lock).
`NO KEY UPDATE`/`KEY SHARE` remain Postgres-only concepts with no Oracle equivalent, same stance as
every non-Postgres adapter so far.

```go
var oracleCapabilities = dialecttest.Capabilities{
    Locking: dialecttest.LockCapabilities{
        Update:      true,
        NoKeyUpdate: false,
        Share:       false, // Oracle has no FOR SHARE clause at all
        KeyShare:    false,
        NoWait:      true,
        SkipLocked:  true,
    },
}
```

## Error mapping (`IsConflict`, `mapError`)

Confirmed via probe: errors are `*network.OracleError{ErrCode int, ErrMsg string}` (package
`github.com/sijms/go-ora/v2/network`, exported, matches via `errors.As(err, &oraErr)` against a
`**network.OracleError` target — pointer-based, same class as MySQL/SQLite's error types, not
MSSQL's value-type `mssql.Error`). Confirmed exact codes:

- `ErrCode == 1` (`ORA-00001: unique constraint violated`) → `golem.ErrDuplicateKey`
- `ErrCode == 2291` (`ORA-02291: integrity constraint violated - parent key not found`, the
  insert/update-side FK violation) → `golem.ErrForeignKeyViolation`
- `ErrCode == 2292` (`ORA-02292: integrity constraint violated - child record found`, the
  delete-side FK violation — Oracle uses a **different** code for this direction than for
  insert/update, unlike MSSQL's single 547 covering both) → `golem.ErrForeignKeyViolation`

## Transactions (`Begin`/`getExecutor`)

Mirrors `driver/mysql`'s `*sql.Tx`/type-assertion pattern exactly — `go-ora` is a standard
`database/sql` driver, nothing Oracle-specific needed here.

## Connector / DSN

`resolveDSN` builds `oracle://user:password@host:port/service_name` (go-ora's own scheme, confirmed
via probe) from `Options{DSN, Host, Port, User, Password, ServiceName, Logging, Logger}` — mirrors
every other adapter's `Options`/`resolveDSN` shape, with `ServiceName` taking the role
`Database` plays elsewhere (Oracle's connection unit is a service name / PDB, not a "database" in
the Postgres/MySQL sense — `FREEPDB1` in the probe container, matching a real multitenant Oracle's
default pluggable database naming).

## Corrections found during T10's real conformance run (2026-07-06)

The design above was written from a schema/type-mapping probe; running the full M15 conformance
suite against the real container (T10) surfaced 4 more issues, none visible from a smaller probe:

1. **Unquoted DDL identifiers fold to UPPERCASE in Oracle, not lowercase.** Postgres/MySQL/SQLite/
   MSSQL either fold unquoted identifiers to lowercase or preserve case, matching this adapter's own
   `quoteIdent` (which always double-quotes and preserves case) — Oracle folds the OTHER way. An
   unquoted `CREATE TABLE conf_widget` actually creates `CONF_WIDGET`, a different (case-sensitive)
   object from the lowercase `"conf_widget"` every later query asks for via `quoteIdent`, producing
   `ORA-00942: table or view does not exist`. Fix: every identifier in
   `conformance_integration_test.go`'s raw DDL strings is now double-quoted, forcing Oracle to store
   the exact lowercase name the rest of the adapter expects (this is a test-file fix only — a real
   `entity.Table`/migration tool would need the same discipline, but golem has neither, AD-012).
   Separately, the bare column name `uid` also needed quoting because Oracle reserves it as a
   pseudo-column (current session's numeric user ID) — confirmed via `ORA-03050`.
2. **`golem.TIME()`'s Oracle mapping changed from `INTERVAL DAY TO SECOND` to `TIMESTAMP`.**
   `go-ora`'s parameter encoder only knows how to bind a Go `time.Time` as Oracle's `TIMESTAMP WITH
   TIME ZONE` — Oracle refuses the implicit conversion into an `INTERVAL DAY TO SECOND` column
   (`ORA-00932: expression is of data type TIMESTAMP WITH TIME ZONE, which is incompatible with
   expected data type INTERVAL DAY TO SECOND`). Since `repository.Insert`/`Update` never call
   `Dialect.Bind` in the real path (AD-037), there's no column-type-aware hook available to reformat
   just this one column into an Oracle interval literal before binding. `TIMESTAMP` is the pragmatic
   fix — it accepts `time.Time` natively (same as `date`/`datetime`), at the cost of losing
   `INTERVAL`'s multi-day-duration range, a real but already-accepted-elsewhere trade-off class
   (mirrors SQLite's `DECIMAL`-as-`REAL` precision loss, MySQL's `TIME` range limit). This also
   removed `normalizeCell`'s `"IntervalDS_DTY"` case and the `parseOracleInterval` helper entirely —
   no longer needed since `TIMESTAMP` scans natively as `time.Time`, same as every other adapter's
   `date`/`datetime` kinds.
3. **`sql.ColumnType.DecimalSize()`'s scale can't reliably distinguish integers from floats for
   Oracle's `NUMBER` family.** The original design (see "Value conversion" above) used `scale == 0`
   vs `scale != 0` to pick `int64` vs `float64`. This broke `SoftDelete`'s `Count` call
   (`repository: count: failed to parse result from row: map[COUNT(*):1]`): a bare `COUNT(*)`
   expression reports the exact same "unconstrained NUMBER" sentinel scale (`precision=38,
   scale=255`, confirmed via probe) as a genuine `FLOAT` column, yet `COUNT(*)`'s value is always
   integral and `repository.Count()`'s row-parsing only accepts `int64`/`int32`/`int`, not `float64`
   — the scale-based rule mis-parsed it into a `float64` and broke the count. **Fix:**
   `normalizeCell` (now a 2-parameter function again, `scale` dropped entirely) tries
   `strconv.ParseInt` first regardless of scale, falling back to `strconv.ParseFloat` only when the
   string isn't a plain integer. This works because a genuinely-scaled `NUMBER(p,s)` column's string
   form always shows its fractional part (confirmed empirically: `NUMBER(10,2)` renders as `"19.99"`,
   never a bare `"19"` or `"19.99000"` for a value with 2 non-zero decimal digits) — the only
   remaining ambiguity (a `FLOAT`/`SUM`/`AVG` value that happens to net to a whole number, e.g.
   `"3"`) parses as `int64` here too, harmlessly, since `repository.assignFieldValue`'s
   reflect-based `ConvertibleTo` fallback already converts `int64` into a `float64` struct field
   without issue (the same mechanism M16/AD-039 relies on for numeric-to-bool).
4. **`newSQLDB` needs `db.SetMaxOpenConns(1)`, confirmed via a real, intermittently-reproducing bug**
   — not optional, not a style choice. With the default unbounded pool, `repository.go`'s
   client-side `OnDeleteRestrict` check (`applyDeleteCascades`, which opens a transaction via
   `beginCascadeTx` and rolls it back when the check finds a blocking child row) intermittently left
   a LATER, unrelated query on a freshly-pooled connection unable to see an already-committed row
   from a different connection — a stale-read race, not a logic bug (confirmed via direct SQL probe
   that the row was never actually touched; a follow-up `Exists()` call on that exact row
   spuriously returned `false`). The failure was genuinely flaky: it didn't reproduce on every run,
   reproduced only when the FULL conformance suite ran (never for isolated 1-2-subtest-group
   slices), and reproduced inconsistently even then — all consistent with a real connection-pool
   race rather than a deterministic sequencing bug. Serializing every operation onto a single
   connection (`db.SetMaxOpenConns(1)`) eliminated it across many repeated real-container runs (4
   consecutive clean passes after the fix, vs. intermittent failure before). This mirrors
   `driver/sqlite`'s own `SetMaxOpenConns(1)` precedent, though for an entirely different reason
   (SQLite's is about its single-writer file-locking model; this is about `go-ora`/Oracle session
   consistency under concurrent pooled connections — root cause not fully isolated beyond "confirmed
   necessary and confirmed sufficient," same empirical standard applied to every other finding in
   this document).

## Testing

- Unit tests: `DATA-DOG/go-sqlmock` (same library as every other `database/sql`-based adapter).
- Integration: `connector_integration_test.go` (lifecycle, mirrors `driver/mssql`'s) +
  `conformance_integration_test.go` (Oracle DDL per INSIGHT.md's type table, all identifiers
  double-quoted per Correction #1 above: `NUMBER(1)` for BOOLEAN, `VARCHAR2(36)` for UUID (design
  decision above, not `RAW(16)`), `CLOB` for TEXT/JSON, `TIMESTAMP` for TIME (Correction #2 above,
  not `INTERVAL DAY TO SECOND`), `GENERATED ALWAYS AS IDENTITY` for auto-increment PKs — +
  `Capabilities` per the Locking section above), calling `dialecttest.Run`.
- Docker: `.docker/docker-compose.test.yml` gains an `oracle` service using
  `gvenzl/oracle-free:23-slim` (own port, e.g. 51521; `ORACLE_PASSWORD` env var; confirmed this
  image needs no Oracle Container Registry login, unlike Oracle's own official images — a real
  reachability win over M18's `mcr.microsoft.com` friction). `Taskfile.yml` gains
  `GOLEM_ORACLE_TEST_DSN`.

---

## Confirmed via empirical probe (2026-07-06, Oracle 23ai Free container via `gvenzl/oracle-free:23-slim`)

A throwaway Go program (`database/sql` + `github.com/sijms/go-ora/v2`) against a real, freshly
started container confirmed every design decision above, in particular:

- `docker pull gvenzl/oracle-free:23-slim` succeeded directly from this environment — no CDN/auth
  friction like M18's `mcr.microsoft.com`.
- Every `NUMBER(p,s)` variant (`NUMBER(1)`, `NUMBER(5)`, `NUMBER(10)`, `NUMBER(19)`,
  `NUMBER(10,2)`, `FLOAT`) reports `DatabaseTypeName() == "NUMBER"` and scans as a Go `string` via
  generic `Scan(&any)` — `DecimalSize()`'s `scale` is the only signal distinguishing integer-shaped
  (`scale == 0`) from decimal/float-shaped (`scale != 0`, including `FLOAT`'s `255` sentinel).
- `VARCHAR2`/`CHAR`/`CLOB` scan as `string`, `DATE`/`TIMESTAMP` as `time.Time`, `BLOB` as `[]byte` —
  no normalization needed for any of these.
- `INTERVAL DAY TO SECOND` scans as `string` in the form `"+01 02:03:04.000000"`.
- `RAW(16)` scans as raw `[]byte` (16 bytes, no natural string mapping without a byte-format
  decision) — `VARCHAR2(36)` chosen instead for the `uuid` kind, confirmed to scan as a plain
  hyphenated-UUID string already matching golem's contract.
- `OFFSET 0 ROWS FETCH NEXT 1 ROWS ONLY` with **no `ORDER BY`** executed without error — confirmed
  Oracle, unlike SQL Server, does not require one.
- `SELECT COUNT(*) FROM t ORDER BY id OFFSET 0 ROWS FETCH NEXT 1 ROWS ONLY` (real column, no
  `GROUP BY`) and the `ORDER BY (SELECT NULL FROM DUAL)` variant (mirroring MSSQL's idiom) **both**
  failed with `ORA-00937: not a single-group group function` — confirmed the MSSQL-style workaround
  doesn't transfer; the actual fix is to never emit `ORDER BY` for a `Count` query here at all
  (unneeded per the point above).
- `INSERT INTO t (id, name) VALUES (2, 'a')` against a table with `UNIQUE (name)` already containing
  `name='a'` failed immediately with `ORA-00001`, matched via `errors.As` into
  `*network.OracleError{ErrCode: 1}`.
- `INSERT INTO child (id, parent_id) VALUES (1, 999)` (no matching parent) failed with
  `ORA-02291`, matched as `ErrCode: 2291`.
- `DELETE FROM parent WHERE id = 1` (with an existing referencing child row) failed with
  `ORA-02292`, matched as `ErrCode: 2292` — confirmed as a genuinely different code from the
  insert/update-side violation, unlike MSSQL's single 547 for both directions.
- `res.LastInsertId()` after inserting into a `GENERATED ALWAYS AS IDENTITY` column returned
  `(0, nil)` — silently unsupported, confirming `RETURNING ... INTO` (via `go_ora.Out{Dest:
  &typedVar}`, executed through `db.ExecContext`, never `db.QueryContext`) is the only way to
  retrieve an Oracle-generated primary key value in one round-trip.
- `SELECT * FROM t WHERE id = 1 FOR UPDATE NOWAIT` and `... FOR UPDATE SKIP LOCKED` both executed
  without error inside a transaction.
