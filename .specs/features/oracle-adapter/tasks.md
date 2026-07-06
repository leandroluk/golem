# Oracle Adapter (M19) — Tasks

No T0 core-schema task this milestone — `stmt.Select.PrimaryKey` (added in M18/AD-045) already
covers everything `driver/oracle` needs from core; it's read but never forces a default `ORDER BY`
here (design.md's pagination section). If implementation surfaces a real core gap, insert a T0 here
and give it its own AD entry, per every prior milestone's pattern.

---

### T1: `go.mod` dependency + package skeleton ✅ DONE

**What**: Add `github.com/sijms/go-ora/v2` to `go.mod`. `driver/oracle/oracle.go`:
`Options{DSN, Host, Port, User, Password, ServiceName, Logging, Logger}`, `sqlIface`, `New`
(mirrors `driver/mssql/mssql.go`'s shape exactly, `ServiceName` replacing `Database`).
**Depends on**: none
**Requirement**: ORACLE-01
**Tests**: unit, mirroring `driver/mssql/mssql_test.go`

---

### T2: `dsn.go` — `resolveDSN` ✅ DONE

**What**: Builds `oracle://user:password@host:port/service_name` (go-ora's own URL scheme,
confirmed via design.md's probe — service name is a URL path segment, not a query param like
MSSQL's `?database=`).
**Depends on**: T1
**Requirement**: ORACLE-01
**Tests**: unit, mirroring `driver/mssql/dsn_test.go`

---

### T3: `connector.go` ✅ DONE

**What**: Mirrors `driver/mssql/connector.go`'s shape exactly (standard `database/sql` connect/
ping/close lifecycle, no Oracle-specific behavior needed) — except `newSQLDB` also calls
`db.SetMaxOpenConns(1)`. **[Added during T10's real conformance run, not optional]**: with the
default unbounded pool, a real, intermittently-reproducing stale-read race surfaced (see design.md's
"Corrections" section #4) — capping the pool to 1 connection eliminated it across many repeated
real-container runs.
**Depends on**: T2
**Requirement**: ORACLE-01
**Tests**: unit, mirroring `driver/mssql/connector_test.go`

---

### T4: `dialect.go` — `Bind`/`Scan` ✅ DONE

**What**: Simple pass-through shape per `ColumnType.Kind()` (mirrors every other adapter's — dead
in the real path per AD-037, but implemented/unit-tested to match the documented contract).
`uuid` kind: plain `string` (design decision — `VARCHAR2(36)`, not `RAW(16)`).
**Depends on**: T1
**Requirement**: ORACLE-04
**Tests**: unit, mirroring `driver/mssql/dialect_test.go`'s Bind/Scan test list per `ColumnType` kind

---

### T5: `dialect.go` — `CompileSelect`/`CompileDelete`/`compilePredicate`/`lockClauseSQL` ✅ DONE

**What**: Double-quote `quoteIdent`, `:N`-numbered `compilePredicate` (`argOffset *int` counter,
same shape as MSSQL's `@pN` but `:` prefix, confirmed via design.md's probe). `OFFSET y ROWS FETCH
NEXT x ROWS ONLY` pagination. **Unlike M18**: no forced default `ORDER BY` when pagination is
requested without one (Oracle doesn't require it — confirmed via probe) — `s.PrimaryKey` is read
but only used if a future need arises; not exercised by this adapter's own logic today. When
`s.Count` is `true`, `ORDER BY` is never emitted at all, even if `s.OrderBy` would otherwise apply —
confirmed via probe that Oracle rejects `ORDER BY <real column>` AND `ORDER BY (SELECT NULL)` alike
on a bare `COUNT(*)` with `ORA-00937`, and since `OFFSET/FETCH` doesn't need `ORDER BY` here, simply
omitting it is correct, not a workaround. `lockClauseSQL` builds a **trailing** `FOR UPDATE
[NOWAIT | SKIP LOCKED]` clause (same shape as Postgres, not a table hint like MSSQL) — `"share"`/
`"no_key_update"`/`"key_share"` strengths error (Oracle has no `FOR SHARE` clause of any kind, a
permanent gap not a version gate).
**Depends on**: T4
**Requirement**: ORACLE-07, ORACLE-08
**Tests**: unit, mirroring `driver/mssql/dialect_test.go`'s CompileSelect/CompileDelete/
compilePredicate list, minus the ORDER-BY-injection cases (don't apply here) plus new cases:
pagination with no `OrderBy` (asserts no `ORDER BY` emitted, no error), `Count` query with
pagination (asserts no `ORDER BY` emitted even if `OrderBy` unset), `FOR UPDATE`/`NOWAIT`/
`SKIP LOCKED` combinations, `"share"`/`"no_key_update"`/`"key_share"` all erroring (unsupported)

---

### T6: `dialect.go` — `Insert`/`Update` (multi-round-trip) + `collectRows`/`normalizeCell` ✅ DONE

**What**: `collectRows` ported from `driver/mssql`'s (generic `database/sql` row→map scanning) +
new `normalizeCell(dbType string, raw any) any` (same 2-parameter shape as every other adapter):
for `"NUMBER"`-typed columns (the one string Oracle reports for every numeric kind), tries
`strconv.ParseInt` first, falling back to `strconv.ParseFloat` only when the string isn't a plain
integer. **[Corrected during T10's real conformance run]** An initial 3-parameter version keyed on
`sql.ColumnType.DecimalSize()`'s `scale` (0 vs non-zero) broke `SoftDelete`'s `Count` call, because
a bare `COUNT(*)` reports the same "unconstrained NUMBER" sentinel scale as a genuine `FLOAT`
column — the `scale` parameter was dropped entirely once the try-`ParseInt`-first rule replaced it
(see design.md's "Corrections" section for the full analysis). `golem.TIME()` maps to `TIMESTAMP`,
not `INTERVAL DAY TO SECOND` (also corrected during T10 — see design.md), so no interval-parsing
helper is needed at all; `TIMESTAMP` scans natively as `time.Time`.

`Insert`: plain `INSERT INTO ... VALUES (...)` via `ExecContext` — no `RETURNING *`/`OUTPUT
INSERTED.*` equivalent exists (design.md's correction: Oracle's `RETURNING ... INTO` needs one
pre-typed out-bind per column, not viable generically). If the primary key column(s) are present in
`s.Columns`/`s.Values` (client-assigned, e.g. UUID), use those values directly for the read-back
`WHERE`. If NOT present (auto-generated `IDENTITY` column), append `RETURNING <pk> INTO :N` with one
`go_ora.Out{Dest: &generatedID}` (`generatedID int64`) bind arg to the same `INSERT` statement
(confirmed via probe: `sql.Result.LastInsertId()` always returns `(0, nil)` for Oracle, silently
unsupported — this is the only way to retrieve the generated value in one round-trip), then issue
the follow-up `SELECT * WHERE <pk>` via `QueryContext` + `collectRows` (same shape as
`driver/mysql`'s `Insert`). `Update`: captures matching primary keys *before* running the `UPDATE`
(same reasoning as `driver/mysql`'s `Update` — re-running the original `WHERE` afterward can't work
when `Sets` modifies a column `Where` itself filters on), then reads them back the same way.
**Depends on**: T5
**Requirement**: ORACLE-04, ORACLE-05, ORACLE-06
**Tests**: unit (`go-sqlmock`) — Insert with client-assigned PK, Insert with auto-generated PK
(`go_ora.Out` bind), Update, zero-rows-back error, `normalizeCell` unit tests (integer string,
whole-number string still parses as int64 not float64, decimal string, unparseable/non-string
left untouched)

---

### T7: `dialect.go` — `Query`/`Exec`/`ExecRaw` ✅ DONE

**What**: Straight ports of `driver/mssql`'s versions — no lazy-error-surfacing concern here
(confirmed via design.md's probe: Oracle's constraint violations surface immediately from
`ExecContext`, unlike `go-mssqldb`'s lazy `QueryContext`), so `mapError` only needs to wrap the
immediate error at each call site, not a `collectRows`-returned one too.
**Depends on**: T6
**Requirement**: ORACLE-01
**Tests**: unit, mirroring `driver/mssql/dialect_test.go`'s Query/Exec/ExecRaw list

---

### T8: `dialect.go` — `IsConflict`/`mapError`/`Begin`/`getExecutor` ✅ DONE

**What**: Oracle error-code mapping per design.md (confirmed `1` → `ErrDuplicateKey`; `2291`
(insert/update-side FK) and `2292` (delete-side FK, a genuinely different code from MSSQL's single
547) → `ErrForeignKeyViolation`), via `errors.As(err, &oraErr)` into `*network.OracleError`
(package `github.com/sijms/go-ora/v2/network`, pointer-based like MySQL/SQLite's error types, not
MSSQL's value-type). `Begin`/`getExecutor` mirror `driver/mysql`'s `*sql.Tx`/type-assertion pattern
exactly.
**Depends on**: T7
**Requirement**: ORACLE-09, ORACLE-10
**Tests**: unit — `network.OracleError{ErrCode: N}` is directly constructible (exported fields,
confirmed via design.md), same easy path as MySQL/SQLite's error types, no heavier mechanism needed

---

### T9: Docker + Taskfile wiring ✅ DONE

**What**: `.docker/docker-compose.test.yml` gains an `oracle` service using
`gvenzl/oracle-free:23-slim` (own port, `ORACLE_PASSWORD` env var — confirmed via design.md's probe
this image needs no Oracle Container Registry login, unlike Oracle's own official images; a real
reachability win over M18's `mcr.microsoft.com` friction, worth a README note only if a similar
problem actually surfaces during real `task test-integration` runs — not assumed upfront this time).
`Taskfile.yml`'s `test-integration` gains a `GOLEM_ORACLE_TEST_DSN`-equivalent env var.
**Depends on**: none (can happen anytime before T10)
**Gate**: `docker compose -f .docker/docker-compose.test.yml up -d --wait` brings up all services
healthy, including `oracle`

---

### T10: `connector_integration_test.go` + `conformance_integration_test.go` ✅ DONE

**What**: `driver/oracle/connector_integration_test.go` (connection lifecycle, mirroring
`driver/mssql`'s). `driver/oracle/conformance_integration_test.go` (Oracle `Schema` DDL —
`NUMBER(1)`/`NUMBER(5)`/`NUMBER(10)`/`NUMBER(19)`/`NUMBER(p,s)`/`FLOAT` per INSIGHT.md's type
table, `VARCHAR2(36)` for UUID (design decision, not `RAW(16)`), `CLOB` for TEXT/JSON,
`INTERVAL DAY TO SECOND` for TIME, `GENERATED ALWAYS AS IDENTITY` for auto-increment PKs — +
`Capabilities` per design.md: `Update`/`NoWait`/`SkipLocked` `true`, `Share`/`NoKeyUpdate`/
`KeyShare` `false`), calling `dialecttest.Run`.
**Depends on**: T3, T6, T8, T9
**Requirement**: ORACLE-01, ORACLE-02, ORACLE-03
**Gate**: `task test-integration` green (real Oracle via Docker)

---

### T11: Coverage + docs ✅ DONE

**What**: Close any unit-test coverage gaps to 100% (or documented accepted exceptions, applying
`internal/must`/`internal/testutil` where genuinely applicable — same bar M15-M18 established).
Update `.specs/project/ROADMAP.md` (M19 → DONE), `.specs/project/STATE.md` (new AD covering the
`NUMBER`-family scan-as-string + scale-based `normalizeCell` finding, the multi-round-trip Insert/
Update + `RETURNING INTO` finding, the "Oracle doesn't need ORDER BY for OFFSET/FETCH, and rejects
MSSQL's `(SELECT NULL)` idiom for Count queries" pagination correction, and the two-different-FK-
error-codes finding), `README.md` (Implementation Status checkbox).
**Depends on**: T10
**Gate**: `task coverage` shows 100% (or documented exceptions); `task test-integration` still green
