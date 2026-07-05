# MySQL/MariaDB Adapter (M16) Specification

## Status: Planned

## Problem Statement

M1-M15 proved golem's core against Postgres only, and M15 built a reusable conformance harness
(`internal/dialecttest`) specifically so the second adapter wouldn't mean hand-copying integration
tests. M16 is that second adapter: `driver/mysql`, targeting MySQL 8+ (primary) and MariaDB
(best-effort — no dedicated test target this milestone, see Out of Scope). It's the first real
test of whether AD-015/AD-016's dialect-agnostic core (`golem.ColumnType`, `golem.Dialect`,
`internal/stmt`) actually holds up against a database with meaningfully different SQL: no
`RETURNING`, no native UUID/boolean, different upsert/pagination/locking syntax.

## Goals

- [ ] `driver/mysql` implements `golem.Dialect`/`golem.Connector`, passing `internal/dialecttest.Run`
      (M15) against a real MySQL 8+ instance
- [ ] Every M1-M14 capability golem exposes (CRUD, joins, soft delete, cascade, hooks,
      transactions, raw SQL, typed errors, relations, preload, aggregates) works identically from
      the caller's point of view — same `Repository[T]`/`query.*` API, only the `driver/mysql`
      import changes
- [ ] Confirm or correct AD-015/AD-016's dialect-agnostic design assumptions against a database
      that diverges from Postgres more than any dialect tested so far

## Out of Scope

| Item | Reason |
| --- | --- |
| Dedicated MariaDB CI target / conformance run | User decision: MySQL 8+ is primary and verified this milestone; MariaDB is best-effort (same driver, same code), revisit with its own milestone if a real need shows up |
| `NO KEY UPDATE`/`KEY SHARE` locking strengths | No MySQL equivalent exists at all (Postgres-specific, per M15's `Capabilities.Locking`) — declared unsupported, not worked around |
| MySQL-specific features with no golem equivalent (e.g. `ENUM`/`SET` types, storage engine selection, full-text indexes) | Nothing in `golem.ColumnType`/`entity.Table` models these yet; adding them is a separate, larger design question than "port the existing surface to a new dialect" |
| Migrating `.examples/postgres-minimal-blog` to also run against MySQL | The conformance suite (M15) is what proves cross-dialect correctness now; the example stays Postgres-only narrative documentation, consistent with M15's own scope decision |

---

## User Stories

### P1: `driver/mysql` passes the M15 conformance suite ⭐ MVP

**User Story**: As someone using golem, I want a `driver/mysql` adapter that works exactly like
`driver/postgres` from the caller's side, so that switching databases means changing one import
and one `Options` struct, not rewriting application code.

**Why P1**: This is the entire point of M16 — everything else (specific SQL syntax choices) is in
service of making this true.

**Acceptance Criteria**:

1. WHEN `driver/mysql/conformance_integration_test.go` calls `dialecttest.Run` with a real MySQL
   8+ `golem.Connector` THEN every non-locking-strength-gated subtest SHALL pass.
2. WHEN a `Capabilities.Locking` field has no MySQL equivalent (`NoKeyUpdate`, `KeyShare`) THEN it
   SHALL be declared `false`, and the harness SHALL skip (not fail) those subtests.
3. WHEN the same `entity.New`-declared schema and `Repository[T]` calls that work against
   `driver/postgres` are pointed at a `driver/mysql`-backed `DataSource` THEN they SHALL produce
   the same results (modulo Out of Scope's `ENUM`/`SET`/storage-engine gaps, which no test
   exercises since golem has no API surface for them).

**Independent Test**: `task test-integration` (extended to also stand up MySQL) runs
`driver/mysql`'s conformance suite green.

---

### P2: `Insert`/`Update` work without `RETURNING`

**User Story**: As a `driver/mysql` maintainer, I want `Insert`/`Update` to return the
inserted/updated row the same way `driver/postgres` does, so `Repository[T]` doesn't need to know
which dialect it's talking to.

**Why P2**: Not the top-line goal, but the single largest behavioral gap between MySQL and
Postgres (AD-016 exists specifically to make this an adapter concern, not a core one) — every
other feature (joins, cascade, etc.) is comparatively mechanical translation once this works.

**Acceptance Criteria**:

1. WHEN `Insert` runs against an auto-increment primary key THEN the adapter SHALL retrieve the
   generated ID via `LAST_INSERT_ID()` and follow up with a `SELECT` by that PK to return the full
   row — matching `golem.Dialect.Insert`'s `(map[string]any, error)` contract.
2. WHEN `Update` runs THEN the adapter SHALL issue the `UPDATE`, then a follow-up `SELECT` for
   every row matching the original `WHERE` clause, to return all updated rows — matching
   `golem.Dialect.Update`'s `([]map[string]any, error)` contract.
3. WHEN the follow-up `SELECT` (either case) returns 0 rows unexpectedly (a concurrent delete
   between the write and the read-back, for example) THEN the adapter SHALL return a clear error,
   not a zero-value row silently passed through.

**Independent Test**: `internal/dialecttest`'s `CRUD` subtest group (`Insert`/`SaveOne`/`Update`
assertions) passes unmodified against `driver/mysql`.

---

### P3: Conflict/lock/pagination/upsert syntax mapped correctly

**User Story**: As a `driver/mysql` maintainer, I want MySQL error codes and lock/pagination/
upsert SQL correctly mapped to golem's dialect-agnostic contracts, so error handling and
query behavior match what `driver/postgres` callers already expect.

**Why P3**: Necessary for full conformance (P1) but each piece is independently small and
mechanical once P2's round-trip strategy exists.

**Acceptance Criteria**:

1. WHEN a write violates a `Unique` constraint THEN `IsConflict` SHALL return `true` for MySQL
   error 1062, and `mapError`-equivalent logic SHALL wrap it as `golem.ErrDuplicateKey`.
2. WHEN a write violates a `ForeignKey` constraint THEN the adapter SHALL map it to
   `golem.ErrForeignKeyViolation` (MySQL errors 1451/1452 — delete/insert respectively).
3. WHEN `.ForUpdate()`/`.ForShare()` run inside a `golem.Tx` THEN the compiled SQL SHALL end with
   `FOR UPDATE`/`FOR SHARE` (optionally `NOWAIT`/`SKIP LOCKED`, MySQL 8.0.1+).
4. WHEN `q.Limit(x).Offset(y)` are set THEN the compiled SQL SHALL end with `LIMIT x OFFSET y`
   (same shape as Postgres — no translation needed here).
5. WHEN `SaveOne`/`SaveMany` issue their upsert-shaped write THEN the compiled SQL SHALL use
   `INSERT ... ON DUPLICATE KEY UPDATE` where applicable.

**Independent Test**: `internal/dialecttest`'s `ConflictDetection`/`Locking` subtest groups pass
against `driver/mysql`, with `NoKeyUpdate`/`KeyShare` reported as `SKIP`.

---

## Edge Cases

- WHEN a `golem.ColumnType` kind has no exact MySQL equivalent (`BOOLEAN` → `TINYINT(1)`, `UUID` →
  `CHAR(36)`) THEN `Bind`/`Scan` SHALL convert transparently — callers never see the underlying
  MySQL representation.
- WHEN `golem.UUID()`-typed data is written as `CHAR(36)` THEN it SHALL round-trip as the same
  string value on read (this adapter never generates DDL — AD-012 — so the actual column type in
  a real database is the operator's responsibility; `Bind`/`Scan` just need to handle whatever a
  `CHAR(36)` column hands back, mirroring how `driver/postgres`'s conformance schema is
  hand-written DDL the harness itself doesn't generate).
- WHEN `Insert`'s follow-up `SELECT`-by-PK runs immediately after `INSERT`+`LAST_INSERT_ID()`
  inside a transaction THEN it SHALL see its own uncommitted write (same-transaction visibility) —
  no special handling needed beyond using the same connection/transaction for both statements.
- WHEN `NOWAIT`/`SKIP_LOCKED` are requested against a MariaDB target older than 10.6 THEN behavior
  is explicitly best-effort/unverified (Out of Scope) — the adapter still compiles the SQL MySQL
  8+ expects; a caller targeting older MariaDB may get a syntax error from their database, not a
  golem-level error.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| MYSQL-01 | P1: Passes M15 conformance | Design | Pending |
| MYSQL-02 | P1: Passes M15 conformance | Design | Pending |
| MYSQL-03 | P1: Passes M15 conformance | Design | Pending |
| MYSQL-04 | P2: Insert/Update without RETURNING | Design | Pending |
| MYSQL-05 | P2: Insert/Update without RETURNING | Design | Pending |
| MYSQL-06 | P2: Insert/Update without RETURNING | Design | Pending |
| MYSQL-07 | P3: Conflict/lock/pagination/upsert | Design | Pending |
| MYSQL-08 | P3: Conflict/lock/pagination/upsert | Design | Pending |
| MYSQL-09 | P3: Conflict/lock/pagination/upsert | Design | Pending |
| MYSQL-10 | P3: Conflict/lock/pagination/upsert | Design | Pending |
| MYSQL-11 | P3: Conflict/lock/pagination/upsert | Design | Pending |

**Coverage:** 11 total, 0 mapped to tasks, 11 unmapped ⚠️ (expected — Design/Tasks phases haven't run yet)

---

## Success Criteria

- [ ] `driver/mysql`'s unit tests (mocked, via `DATA-DOG/go-sqlmock` — MySQL's equivalent of
      `pgxmock`) hit 100% statement coverage, matching every other package's bar in this repo
- [ ] `driver/mysql/conformance_integration_test.go` passes `task test-integration` against a real
      MySQL 8+ container, with `NoKeyUpdate`/`KeyShare` reporting `SKIP`, not `PASS`/`FAIL`
- [ ] No changes needed to `golem`, `entity`, `repository`, `query`, `op`, `join`, `relation`,
      `internal/stmt`, or `internal/dialecttest` — if M16 needs one, that's a signal M15's
      "dialect-agnostic core" premise (AD-015/AD-016) had a gap, and it gets its own AD entry
      explaining what and why
