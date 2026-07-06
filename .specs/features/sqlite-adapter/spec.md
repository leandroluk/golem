# SQLite Adapter (M17) Specification

## Status: DONE — see design.md/tasks.md for implementation notes and AD-040/AD-041 in STATE.md

## Problem Statement

M15 built a reusable conformance harness (`internal/dialecttest`) and M16 proved it against MySQL,
a database with meaningfully different SQL from Postgres. M17 is the third adapter, `driver/sqlite`,
and the first one genuinely different in kind: no server, no network round-trip, no Docker service
— an embedded, single-file (or in-memory) database with dynamic typing (5 storage classes, not a
fixed column-type system) and no real row-level locking at all. It's the sharpest test yet of
whether AD-015/AD-016's dialect-agnostic core actually holds when a dialect's storage model doesn't
resemble Postgres/MySQL's at all.

## Goals

- [ ] `driver/sqlite` implements `golem.Dialect`/`golem.Connector`, passing `internal/dialecttest.Run`
      (M15) against a real (in-memory or temp-file) SQLite 3 database
- [ ] Every M1-M14 capability golem exposes (CRUD, joins, soft delete, cascade, hooks, transactions,
      raw SQL, typed errors, relations, preload, aggregates) works identically from the caller's
      point of view — same `Repository[T]`/`query.*` API, only the `driver/sqlite` import changes
- [ ] Confirm or correct AD-015/AD-016's dialect-agnostic design assumptions against a database with
      dynamic typing and no row-level locking, the two furthest-from-Postgres traits of any adapter
      so far
- [ ] `driver/sqlite`'s own test suite (unit + conformance) never needs Docker — matches this
      adapter's whole reason for existing on the roadmap (see M17 in ROADMAP.md)

## Out of Scope

| Item | Reason |
| --- | --- |
| `mattn/go-sqlite3` (cgo driver) | User decision: `modernc.org/sqlite` (pure Go, no cgo/C compiler) keeps `driver/sqlite`'s tests (and any consumer's build) simple across all 3 CI OSes (linux/macos/windows) without a C toolchain dependency — consistent with this milestone's own "no external dependency" goal |
| Real row-level locking | SQLite has no such thing (whole-file/whole-database locking only) — every lock strength either maps to a database-level lock or is declared unsupported via `Capabilities.Locking`, never faked |
| `entity.Table`-level `CHECK` constraints, `ENUM`/`SET`-like types, or any DDL generation | Same reasoning as M16: golem never generates DDL (AD-012), and no `golem.ColumnType`/`entity.Table` API models these yet |
| Concurrent-writer stress testing (`SQLITE_BUSY` under real multi-process contention) | Out of scope for a conformance pass — `internal/dialecttest` runs single-process; busy/lock-timeout behavior beyond what `golem.Tx` already contracts is a separate investigation if a real need shows up |
| WAL vs. rollback-journal mode as a user-facing `Options` toggle | Not requested; adapter picks one sane default (see design.md) rather than exposing a new cross-cutting option this milestone |

---

## User Stories

### P1: `driver/sqlite` passes the M15 conformance suite ⭐ MVP

**User Story**: As someone using golem, I want a `driver/sqlite` adapter that works exactly like
`driver/postgres`/`driver/mysql` from the caller's side, so that switching to an embedded database
for tests/local dev/small deployments means changing one import, not rewriting application code.

**Why P1**: Entire point of M17 — everything else is in service of making this true against a
storage model that diverges from every prior adapter.

**Acceptance Criteria**:

1. WHEN `driver/sqlite/conformance_integration_test.go` calls `dialecttest.Run` with a real SQLite
   (in-memory) `golem.Connector` THEN every non-locking-gated subtest SHALL pass.
2. WHEN `Capabilities.Locking` is evaluated for SQLite THEN every lock-strength field SHALL be
   declared per design.md's mapping (database-level lock or unsupported — never silently a no-op),
   and the harness SHALL skip (not fail) whichever strengths are declared unsupported.
3. WHEN the same `entity.New`-declared schema and `Repository[T]` calls that work against
   `driver/postgres`/`driver/mysql` are pointed at a `driver/sqlite`-backed `DataSource` THEN they
   SHALL produce the same results (modulo Out of Scope's DDL-feature gaps, which no test exercises).
4. WHEN `driver/sqlite`'s own tests run (unit or conformance) THEN no Docker service or external
   process SHALL be required — an in-memory or temp-file database is sufficient.

**Independent Test**: `task test-integration` (extended to also run SQLite) runs `driver/sqlite`'s
conformance suite green, with zero `docker-compose` services added for it.

---

### P2: Dynamic typing doesn't leak into the caller

**User Story**: As a `driver/sqlite` maintainer, I want every `golem.ColumnType` kind to round-trip
correctly despite SQLite's 5 storage classes (NULL/INTEGER/REAL/TEXT/BLOB), so callers never see a
value with the wrong Go type just because SQLite stored it loosely.

**Why P2**: The single largest behavioral gap between SQLite and every prior adapter — Postgres and
MySQL both have a fixed column type system; SQLite's type affinity is a much weaker guarantee, so
`Bind`/`Scan` carry more correctness burden here than anywhere else in the codebase.

**Acceptance Criteria**:

1. WHEN a `golem.ColumnType` kind has no exact SQLite storage class (`BOOLEAN` → `INTEGER` 0/1,
   `UUID`/`DATE`/`DATETIME`/`TIME`/`CHAR`/`VARCHAR`/`TEXT`/`JSON` → `TEXT`, `DECIMAL`/`FLOAT` →
   `REAL`, `BLOB` → `BLOB`, per INSIGHT.md's type table) THEN `Bind`/`Scan` SHALL convert
   transparently in both directions.
2. WHEN a `DECIMAL(p,s)` column round-trips through SQLite's `REAL` (a float, not an arbitrary
   -precision decimal) THEN the adapter SHALL document this as a real precision limitation (SQLite
   has no fixed-point decimal storage class) rather than attempt to emulate exact decimal semantics.
3. WHEN `golem.UUID()`-typed data is written as `TEXT` THEN it SHALL round-trip as the same string
   value on read (adapter never generates DDL — AD-012 — actual column affinity is the operator's
   responsibility, same stance as M16's `CHAR(36)` UUID handling).
4. WHEN a `BOOLEAN` column stores `INTEGER` 0/1 THEN `Scan` SHALL convert it to Go `bool`, mirroring
   M16's `assignFieldValue` numeric→bool fix (AD-038) rather than needing a second core-level fix.

**Independent Test**: `internal/dialecttest`'s `BindScanRoundTrip` subtest group passes unmodified
against `driver/sqlite` for every `golem.ColumnType` kind.

---

### P3: Insert/Update, conflict mapping, pagination, and locking-as-declared-unsupported

**User Story**: As a `driver/sqlite` maintainer, I want insert/update round-trips, `SQLITE_CONSTRAINT`
error mapping, and pagination syntax correctly wired, and locking strengths honestly declared
unsupported, so error handling and query behavior match what golem callers already expect or fail
loudly instead of silently misbehaving.

**Why P3**: Necessary for full conformance (P1) but mechanical once P2's type-conversion strategy
exists; locking is the one area where "correctly wired" means "correctly refused," not translated.

**Acceptance Criteria**:

1. WHEN `Insert` runs against an `INTEGER PRIMARY KEY` (SQLite's `AUTOINCREMENT`-eligible rowid
   alias) THEN the adapter SHALL retrieve the generated ID via `LastInsertId()` and follow up with a
   `SELECT` by that PK to return the full row — matching `golem.Dialect.Insert`'s
   `(map[string]any, error)` contract, same round-trip shape as M16's `driver/mysql` (AD-038's
   `stmt.Insert.PrimaryKey` field is reused here, not re-invented).
2. WHEN `Update` runs THEN the adapter SHALL issue the `UPDATE`, then a follow-up `SELECT` for every
   row matching the original `WHERE` clause (captured via `stmt.Update.PrimaryKey`, same reasoning
   as M16 — `Sets` may modify a column the original `Where` filters on).
3. WHEN a write violates a `Unique` constraint THEN `IsConflict` SHALL return `true` for SQLite's
   `SQLITE_CONSTRAINT_UNIQUE`/`SQLITE_CONSTRAINT_PRIMARYKEY` result codes, wrapped as
   `golem.ErrDuplicateKey`.
4. WHEN a write violates a `ForeignKey` constraint THEN the adapter SHALL map
   `SQLITE_CONSTRAINT_FOREIGNKEY` to `golem.ErrForeignKeyViolation` — noting FK enforcement itself
   requires `PRAGMA foreign_keys = ON` per connection, which the adapter SHALL set (see design.md).
5. WHEN `q.Limit(x).Offset(y)` are set THEN the compiled SQL SHALL end with `LIMIT x OFFSET y` (same
   shape as Postgres/MySQL — no translation needed).
6. WHEN `SaveOne`/`SaveMany` issue their upsert-shaped write THEN the compiled SQL SHALL use
   `INSERT ... ON CONFLICT (key) DO UPDATE SET` (same syntax family as Postgres, per INSIGHT.md).
7. WHEN `.ForUpdate()`/`.ForNoKeyUpdate()`/`.ForShare()`/`.ForKeyShare()` are requested inside a
   `golem.Tx` THEN the adapter SHALL declare every strength unsupported in `Capabilities.Locking`
   (SQLite has no `SELECT ... FOR ...` clause of any kind) — `internal/dialecttest`'s harness skips
   these subtests entirely rather than the adapter silently compiling a no-op.

**Independent Test**: `internal/dialecttest`'s `CRUD`/`ConflictDetection`/`Locking` subtest groups
pass (or correctly `SKIP`, for `Locking`) against `driver/sqlite`.

---

## Edge Cases

- WHEN two goroutines write to the same in-memory SQLite `DataSource` concurrently THEN the
  underlying `database/sql` connection pool SHALL be constrained (`SetMaxOpenConns(1)` or
  equivalent, see design.md) so SQLite's single-writer model doesn't surface as flaky
  `SQLITE_BUSY` errors in the conformance suite itself — this is a suite-stability concern, not a
  new golem-level API.
- WHEN `internal/dialecttest.Run` opens an in-memory database (`:memory:` or `file::memory:`) THEN
  it must resolve to one shared connection/pool per test — SQLite's `:memory:` databases are
  private to the connection that created them, so a naive multi-connection pool would see a fresh,
  empty database per connection instead of one shared schema.
- WHEN the `Widget`/`Parent`/`CascadeChild`/etc. conformance schema's `FOREIGN KEY` constraints are
  declared THEN the adapter's connector SHALL ensure `PRAGMA foreign_keys = ON` is set on every new
  connection (SQLite defaults this OFF for backward compatibility) — otherwise `Cascade`/
  `ConflictDetection`'s FK-violation subtests would silently pass with no constraint enforced.
- WHEN a `DECIMAL`/`FLOAT` value requires more precision than an IEEE 754 double provides THEN
  precision loss is expected and documented (Out of Scope / P2 AC2) — not a bug to fix at the
  adapter level.
- WHEN `NOWAIT`/`SKIP LOCKED` are requested (any lock strength) THEN the request SHALL be rejected
  the same way M14/M16 reject unsupported strengths — consistent "error, never silently no-op"
  contract across all three adapters so far.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| SQLITE-01 | P1: Passes M15 conformance | Verified | Done (T1,T3,T7,T9) |
| SQLITE-02 | P1: Passes M15 conformance | Verified | Done (T9) |
| SQLITE-03 | P1: Passes M15 conformance | Verified | Done (T2,T9) |
| SQLITE-04 | P1: Passes M15 conformance | Verified | Done (T9) |
| SQLITE-05 | P2: Dynamic typing doesn't leak | Verified | Done (T4) |
| SQLITE-06 | P2: Dynamic typing doesn't leak | Verified | Done (T4) |
| SQLITE-07 | P2: Dynamic typing doesn't leak | Verified | Done (T4) |
| SQLITE-08 | P2: Dynamic typing doesn't leak | Verified | Done (T4) |
| SQLITE-09 | P3: Insert/Update/conflict/pagination/locking | Verified | Done (T5,T6) |
| SQLITE-10 | P3: Insert/Update/conflict/pagination/locking | Verified | Done (T6) |
| SQLITE-11 | P3: Insert/Update/conflict/pagination/locking | Verified | Done (T8) |
| SQLITE-12 | P3: Insert/Update/conflict/pagination/locking | Verified | Done (T8) |
| SQLITE-13 | P3: Insert/Update/conflict/pagination/locking | Verified | Done (T5) |
| SQLITE-14 | P3: Insert/Update/conflict/pagination/locking | Verified | Done (T5) |
| SQLITE-15 | P3: Insert/Update/conflict/pagination/locking | Verified | Done (T5) |

**Coverage:** 15 total, 15 mapped to tasks (T1-T10), 0 unmapped

---

## Success Criteria

- [ ] `driver/sqlite`'s unit tests (mocked, via `DATA-DOG/go-sqlmock` — same library as `driver/mysql`,
      since `modernc.org/sqlite` is also a `database/sql` driver) hit 100% statement coverage,
      matching every other package's bar in this repo
- [ ] `driver/sqlite/conformance_integration_test.go` passes `task test-integration` (or a plain
      `go test -tags=integration`, since no Docker service is needed) with every locking-strength
      subtest reporting `SKIP`, not `PASS`/`FAIL`
- [ ] No changes needed to `golem`, `entity`, `repository`, `query`, `op`, `join`, `relation`,
      `internal/stmt`, or `internal/dialecttest` — if M17 needs one, that's a signal M15/M16's
      "dialect-agnostic core" premise (AD-015/AD-016) had a gap, and it gets its own AD entry
      explaining what and why
