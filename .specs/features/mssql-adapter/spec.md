# SQL Server (MSSQL) Adapter (M18) Specification

## Status: In Tasks — see design.md (empirically confirmed) and tasks.md (T1-T12)

## Problem Statement

M15-M17 proved golem's dialect-agnostic core against 3 databases with increasingly different SQL
dialects (Postgres, MySQL, SQLite). M18 is the fourth adapter, `driver/mssql`, targeting SQL Server
2019+ (real product, Docker-testable via `mcr.microsoft.com/mssql/server` on Linux). It's the first
adapter with genuinely different pagination syntax (`OFFSET ... FETCH NEXT` instead of
`LIMIT/OFFSET`, and — unlike every prior adapter — a hard SQL Server error without an accompanying
`ORDER BY`) and row-locking expressed as table hints (`WITH (UPDLOCK, ROWLOCK)`) rather than a
trailing clause — a materially different shape from every prior adapter's `Dialect` implementation,
even though the overall architecture (bind/scan, compile, execute) stays the same. (An initial
draft of this spec also assumed SQL Server's lack of `ON CONFLICT` mattered for `SaveOne`/
`SaveMany` — corrected during design.md: golem has never implemented upsert semantics on any
adapter, so this was never actually a gap to close.)

## Goals

- [ ] `driver/mssql` implements `golem.Dialect`/`golem.Connector`, passing `internal/dialecttest.Run`
      (M15) against a real SQL Server 2019+ instance (Docker)
- [ ] Every M1-M14 capability golem exposes (CRUD, joins, soft delete, cascade, hooks,
      transactions, raw SQL, typed errors, relations, preload, aggregates) works identically from
      the caller's point of view — same `Repository[T]`/`query.*` API, only the `driver/mssql`
      import changes
- [ ] Confirm or correct AD-015/AD-016's dialect-agnostic design assumptions against a database
      whose pagination and locking syntax both diverge from every prior adapter simultaneously
      (Postgres/MySQL/SQLite each diverged in at most one of these)

## Out of Scope

| Item | Reason |
| --- | --- |
| SQL Server versions before 2012 | `OFFSET/FETCH` pagination requires 2012+; no reason to support older versions this milestone (matches M19/Oracle's own "decide minimum version up front" note in ROADMAP.md) |
| Azure SQL Database-specific behavior/quirks | Out of scope — targets on-prem/container SQL Server semantics; Azure SQL is largely compatible but not verified here |
| `entity.Table`-level `CHECK` constraints, computed columns, or any DDL generation | Same reasoning as M16/M17: golem never generates DDL (AD-012), no `golem.ColumnType`/`entity.Table` API models these yet |
| `NOWAIT`/`SKIP LOCKED`-equivalent granular per-row wait control beyond what table hints natively support | SQL Server's `NOWAIT` is a *session-level* `SET LOCK_TIMEOUT 0` or a hint combination, not a clean per-query drop-in for `query.LockWaitNoWait`/`SkipLocked` — resolve the exact mapping in design.md, but if no clean equivalent exists for one of the two, declare it unsupported via `Capabilities.Locking` rather than approximate it |

---

## User Stories

### P1: `driver/mssql` passes the M15 conformance suite ⭐ MVP

**User Story**: As someone using golem, I want a `driver/mssql` adapter that works exactly like
`driver/postgres`/`driver/mysql`/`driver/sqlite` from the caller's side, so that targeting SQL
Server means changing one import, not rewriting application code.

**Why P1**: Entire point of M18 — everything else is in service of making this true against a
dialect whose pagination/locking syntax both differ from every prior adapter at once.

**Acceptance Criteria**:

1. WHEN `driver/mssql/conformance_test.go` (or `conformance_integration_test.go`, per design.md's
   decision on whether this adapter can avoid the Docker-only tag — unlike SQLite, SQL Server
   genuinely needs a running server, so this one almost certainly stays `//go:build integration`)
   calls `dialecttest.Run` with a real SQL Server `golem.Connector` THEN every non-locking-gated
   subtest SHALL pass.
2. WHEN a `Capabilities.Locking` field has no clean SQL Server equivalent THEN it SHALL be declared
   `false`, and the harness SHALL skip (not fail) those subtests.
3. WHEN the same `entity.New`-declared schema and `Repository[T]` calls that work against the 3
   existing adapters are pointed at a `driver/mssql`-backed `DataSource` THEN they SHALL produce
   the same results (modulo Out of Scope's DDL-feature gaps, which no test exercises).

**Independent Test**: `task test-integration` (extended to also stand up SQL Server) runs
`driver/mssql`'s conformance suite green.

---

### P2: Insert/Update work via `OUTPUT INSERTED.*` (single round-trip, like Postgres/SQLite)

**User Story**: As a `driver/mssql` maintainer, I want `Insert`/`Update` to return the
inserted/updated row the same way `driver/postgres`/`driver/sqlite` do, so `Repository[T]` doesn't
need to know which dialect it's talking to.

**Why P2**: SQL Server's `OUTPUT INSERTED.*` clause is functionally equivalent to Postgres's
`RETURNING *`/SQLite's `RETURNING *` — a single-round-trip capability none of MySQL's
`LAST_INSERT_ID()` multi-round-trip machinery is needed for. `stmt.Insert/Update.PrimaryKey`
(M16/AD-038) should be readable but unused here, same as Postgres/SQLite.

**Acceptance Criteria**:

1. WHEN `Insert` runs THEN the adapter SHALL issue `INSERT INTO table (...) OUTPUT INSERTED.* VALUES
   (...)` and return the single row directly from that statement's result set — no follow-up
   `SELECT`.
2. WHEN `Update` runs THEN the adapter SHALL issue `UPDATE table SET ... OUTPUT INSERTED.* WHERE
   ...` and return every updated row directly from that statement's result set.
3. **[Corrected during design — see design.md]** `SaveOne`/`SaveMany` are NOT upsert: they compile
   a plain `stmt.Update` (same as `Repository[T].Update`) and return `golem.ErrNotFound` if the
   entity doesn't exist. Checked against the actual `driver/postgres`/`driver/mysql`/`driver/sqlite`
   source and the whole repo: no adapter has ever implemented `ON CONFLICT`/`ON DUPLICATE KEY
   UPDATE`/`MERGE`-based upsert — INSIGHT.md's per-dialect "UPSERT" table entry describes a feature
   golem has never built. No `MERGE INTO` implementation is needed for M18; AC1/AC2 above (`OUTPUT
   INSERTED.*` for `Insert`/`Update`) are the whole of what `SaveOne`/`SaveMany` need, since they
   route through the same `Dialect.Update`.

**Independent Test**: `internal/dialecttest`'s `CRUD` subtest group (`Insert`/`SaveOne`/`Update`
assertions) passes unmodified against `driver/mssql`.

---

### P3: Pagination, locking hints, and conflict/error mapping correctly wired

**User Story**: As a `driver/mssql` maintainer, I want `OFFSET/FETCH` pagination, table-hint-based
locking, and SQL Server error mapping correctly wired, so query behavior and error handling match
what golem callers already expect.

**Why P3**: Necessary for full conformance (P1) but each piece is independently small and
mechanical once P2's round-trip strategy exists.

**Acceptance Criteria**:

1. WHEN `q.Limit(x).Offset(y)` are set THEN the compiled SQL SHALL end with `OFFSET y ROWS FETCH
   NEXT x ROWS ONLY` — noting SQL Server requires an `ORDER BY` clause for `OFFSET/FETCH` to be
   valid syntax at all; confirm during design what the adapter does when a caller requests
   pagination with no `OrderBy` (a real gap none of Postgres/MySQL/SQLite have, since their
   `LIMIT/OFFSET` don't require it).
2. WHEN `.ForUpdate()`/`.ForShare()` (or whichever strengths map cleanly) run inside a `golem.Tx`
   THEN the compiled SQL SHALL include the correct table hint (`WITH (UPDLOCK, ROWLOCK)` or
   equivalent) attached to the table reference, not a trailing clause like every other adapter so
   far — confirm the exact hint syntax and placement during design.
3. WHEN a write violates a `Unique` constraint THEN `IsConflict` SHALL return `true` for SQL
   Server's error 2627/2601, wrapped as `golem.ErrDuplicateKey`.
4. WHEN a write violates a `ForeignKey` constraint THEN the adapter SHALL map SQL Server's error 547
   to `golem.ErrForeignKeyViolation`.

**Independent Test**: `internal/dialecttest`'s `CRUD`/`Locking`/`ConflictDetection` subtest groups
pass (or correctly `SKIP`, for any unsupported `Locking` strength) against `driver/mssql`.

---

## Edge Cases

- WHEN a `golem.ColumnType` kind has no exact SQL Server type (`BOOLEAN` → `BIT`, `UUID` →
  `UNIQUEIDENTIFIER`, `CHAR`/`VARCHAR`/`TEXT`/`JSON` → `NCHAR`/`NVARCHAR`/`NVARCHAR(MAX)`, per
  INSIGHT.md's type table) THEN `Bind`/`Scan` SHALL convert transparently — matching every prior
  adapter's stance that these remain implemented/unit-tested even though dead in the real
  Insert/Update/Query path (AD-037).
- WHEN pagination (`OFFSET/FETCH`) is requested without an explicit `OrderBy` THEN the adapter's
  behavior SHALL be decided explicitly during design (options: inject a stable default order, e.g.
  by primary key, or return a clear compile-time error) — silently emitting invalid SQL Server
  syntax is not acceptable.
- WHEN `NOWAIT`/`SKIP LOCKED`-equivalent behavior has no clean SQL Server mapping THEN it SHALL be
  declared unsupported via `Capabilities.Locking`, not approximated with a semantically different
  hint.
- WHEN `golem.UUID()`-typed data is written as `UNIQUEIDENTIFIER` THEN it SHALL round-trip as the
  same string value on read (adapter never generates DDL — AD-012 — actual column type in a real
  database is the operator's responsibility, same stance as every prior adapter's UUID handling).

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| MSSQL-01 | P1: Passes M15 conformance | Design | Pending |
| MSSQL-02 | P1: Passes M15 conformance | Design | Pending |
| MSSQL-03 | P1: Passes M15 conformance | Design | Pending |
| MSSQL-04 | P2: Insert/Update via OUTPUT INSERTED.* | Design | Pending |
| MSSQL-05 | P2: Insert/Update via OUTPUT INSERTED.* | Design | Pending |
| MSSQL-06 | P2: Insert/Update via OUTPUT INSERTED.* | Design | Pending |
| MSSQL-07 | P3: Pagination/locking/conflict mapping | Design | Pending |
| MSSQL-08 | P3: Pagination/locking/conflict mapping | Design | Pending |
| MSSQL-09 | P3: Pagination/locking/conflict mapping | Design | Pending |
| MSSQL-10 | P3: Pagination/locking/conflict mapping | Design | Pending |

**Coverage:** 10 total, 0 mapped to tasks, 10 unmapped ⚠️ (expected — Design/Tasks phases haven't run yet)

---

## Success Criteria

- [ ] `driver/mssql`'s unit tests (mocked, via `DATA-DOG/go-sqlmock` — same library as
      `driver/mysql`/`driver/sqlite`, since the SQL Server driver is also a `database/sql` driver)
      hit 100% statement coverage (or documented exceptions, same class as every other adapter's,
      applying `internal/must`/`internal/testutil` where genuinely applicable per AD-042/AD-043)
- [ ] `driver/mssql/conformance_integration_test.go` passes `task test-integration` against a real
      SQL Server 2019+ container, with any genuinely-unsupported locking strength reporting `SKIP`
- [ ] No changes needed to `golem`, `entity`, `repository`, `query`, `op`, `join`, `relation`,
      `internal/stmt`, or `internal/dialecttest` — if M18 needs one, that's a signal the
      "dialect-agnostic core" premise (AD-015/AD-016) had a gap, and it gets its own AD entry
      explaining what and why
