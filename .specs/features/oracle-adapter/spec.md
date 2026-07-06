# Oracle Adapter (M19) Specification

## Status: DONE — `driver/oracle` passes the full M15 conformance suite against a real Oracle 23ai Free container (see design.md's "Corrections" section for 4 real bugs found and fixed during T10's verification run)

## Problem Statement

M15-M18 proved golem's dialect-agnostic core against 4 databases (Postgres, MySQL, SQLite, SQL
Server), each diverging from the others in at least one significant way (pagination syntax, upsert
syntax, locking syntax, `[]byte` type ambiguity). M19 is the fifth adapter, `driver/oracle`,
targeting **Oracle Database 12c+ only** (user decision — `OFFSET/FETCH` pagination requires 12c+;
older versions would need a materially different `ROWNUM`-subquery `CompileSelect` path, out of
scope). Uses `github.com/sijms/go-ora` (user decision — pure Go, no cgo, no Oracle Instant Client
dependency, keeping the same "no external native dependency" property `driver/sqlite` established
in M17 across every adapter this project builds itself). Oracle's `NUMBER`-only numeric type system
(no dedicated `BOOLEAN`/`SMALLINT`/`INTEGER`/`BIGINT` types — all map to `NUMBER(p)` per INSIGHT.md)
and its `RAW(16)`/`VARCHAR2(36)` UUID options make this the first adapter needing a real per-`NUMBER`
scan-side type decision (does `database/sql` return `NUMBER(1)` as the same `[]byte`/native type as
`NUMBER(19)`, or does `go-ora` distinguish?) — this must be confirmed empirically, not assumed from
docs, same discipline as every prior milestone's real-server probing.

Identifier length (Oracle 12.2+ raised the limit from 30 to 128 bytes) is explicitly **out of
scope** for validation — per user decision, no adapter validates identifier length today (YAGNI),
and Oracle's own `ORA-00972: identifier is too long` error is clear enough on its own.

## Goals

- [ ] `driver/oracle` implements `golem.Dialect`/`golem.Connector`, passing `internal/dialecttest.Run`
      (M15) against a real Oracle 12c+ instance (Docker)
- [ ] Every M1-M14 capability golem exposes (CRUD, joins, soft delete, cascade, hooks, transactions,
      raw SQL, typed errors, relations, preload, aggregates) works identically from the caller's
      point of view — same `Repository[T]`/`query.*` API, only the `driver/oracle` import changes
- [ ] Confirm empirically (not just from docs) how `go-ora` scans `NUMBER(p,s)` variants for each
      `golem.ColumnType` kind (`boolean`, `smallint`, `integer`, `bigint`, `decimal`), since Oracle's
      single `NUMBER` type family is the first case where 5 different `golem.ColumnType` kinds all
      map to variations of the exact same underlying SQL type

## Out of Scope

| Item | Reason |
| --- | --- |
| Oracle versions before 12c | User decision — `OFFSET/FETCH` pagination needs 12c+; pre-12c needs `ROWNUM`-subquery emulation, a materially different `CompileSelect` path not worth building for an EOL'd version line |
| Identifier length validation/truncation (30 bytes pre-12.2, 128 from 12.2+) | User decision — no adapter validates identifier length today (YAGNI); Oracle's own `ORA-00972` error is clear enough, matches every other adapter's stance of not pre-validating driver-level constraints |
| `entity.Table`-level `CHECK` constraints, computed columns, or any DDL generation | Same reasoning as M16-M18: golem never generates DDL (AD-012), no `golem.ColumnType`/`entity.Table` API models these yet |
| `godror`/ODPI-C (cgo) as an alternative driver | User decision — `sijms/go-ora` (pure Go) chosen instead, keeping the zero-native-dependency property established by `driver/sqlite` (M17) |
| Oracle's `JSON` native type (21c+) | `golem.ColumnType`'s `json` kind maps to `CLOB` per INSIGHT.md (the 12c+-compatible choice) — 21c's dedicated `JSON` type is not targeted, consistent with targeting 12c+ as the floor |

---

## User Stories

### P1: `driver/oracle` passes the M15 conformance suite ⭐ MVP

**User Story**: As someone using golem, I want a `driver/oracle` adapter that works exactly like
`driver/postgres`/`driver/mysql`/`driver/sqlite`/`driver/mssql` from the caller's side, so that
targeting Oracle means changing one import, not rewriting application code.

**Why P1**: Entire point of M19 — everything else is in service of making this true against a
database whose type system (`NUMBER`-only numerics) and pagination requirements (12c+ `OFFSET/FETCH`,
same `ORDER BY`-required shape M18 already solved generically via `stmt.Select.PrimaryKey`) differ
from every prior adapter.

**Acceptance Criteria**:

1. WHEN `driver/oracle/conformance_integration_test.go` calls `dialecttest.Run` with a real Oracle
   12c+ `golem.Connector` THEN every non-locking-gated subtest SHALL pass.
2. WHEN a `Capabilities.Locking` field has no clean Oracle equivalent THEN it SHALL be declared
   `false`, and the harness SHALL skip (not fail) those subtests.
3. WHEN the same `entity.New`-declared schema and `Repository[T]` calls that work against the 4
   existing adapters are pointed at a `driver/oracle`-backed `DataSource` THEN they SHALL produce
   the same results (modulo Out of Scope's DDL-feature gaps, which no test exercises).

**Independent Test**: `task test-integration` (extended to also stand up Oracle) runs
`driver/oracle`'s conformance suite green.

---

### P2: `NUMBER`-family scan/bind correctness across all numeric `ColumnType` kinds

**User Story**: As a `driver/oracle` maintainer, I want every numeric `golem.ColumnType` kind
(`boolean`, `smallint`, `integer`, `bigint`, `decimal`, `float`) to scan/bind correctly even though
Oracle represents all of them as `NUMBER` variants, so `Repository[T]`'s generic `assignFieldValue`
never sees an unexpected Go type.

**Why P2**: This is the one part of M19 with no direct precedent — M16 (MySQL) and M18 (SQL Server)
each had one ambiguous `[]byte` case (`DECIMAL`) fixed by column-type-aware `normalizeCell`, but
Oracle's numerics being uniformly `NUMBER` (no separate `INTEGER`/`SMALLINT`/`BIGINT` SQL type at
all) means this must be verified per-kind against a real database before committing to a design,
not assumed to work like Postgres/MySQL's typed-integer columns.

**Acceptance Criteria** (all confirmed via a real-server probe, AC3's exact mechanism corrected
during T10's real conformance run — see design.md's "Corrections" section):

1. WHEN a column is declared `NUMBER(1)` (BOOLEAN per INSIGHT.md) THEN the real `collectRows` path
   SHALL round-trip a Go `bool` correctly. Confirmed: `go-ora` returns every `NUMBER` column
   (regardless of precision/scale) as a Go `string` via generic `Scan(&any)`, NOT natively
   `int64`/`float64` — `normalizeCell` parses it via `strconv.ParseInt` into `int64`, which then
   flows through `repository.assignFieldValue`'s existing numeric→bool path (M16/AD-039).
2. WHEN a column is declared `NUMBER(5)`/`NUMBER(10)`/`NUMBER(19)` (SMALLINT/INTEGER/BIGINT) THEN
   scanning SHALL produce a Go `int64` — same `normalizeCell` `strconv.ParseInt`-first path as AC1.
3. WHEN a column is declared `NUMBER(p,s)` with `s > 0` (DECIMAL) or `FLOAT` THEN scanning SHALL
   produce a Go `float64` — `normalizeCell` falls back to `strconv.ParseFloat` whenever the column's
   string value isn't a plain integer. **Corrected during T10's real conformance run**: an initial
   design based on `sql.ColumnType.DecimalSize()`'s `scale` (0 vs non-zero) broke `SoftDelete`'s
   `Count` call, because a bare `COUNT(*)` reports the identical "unconstrained NUMBER" sentinel
   scale as a genuine `FLOAT` column — trying `ParseInt` first, regardless of scale, is what
   actually distinguishes them correctly (see design.md for the full analysis).

**Independent Test**: `internal/dialecttest`'s `BindScanRoundTrip` and `CRUD` subtest groups pass
unmodified against `driver/oracle`, covering every numeric `golem.ColumnType` kind used by the
harness's `Widget` schema.

---

### P3: Pagination, upsert, locking, and conflict/error mapping correctly wired

**User Story**: As a `driver/oracle` maintainer, I want `OFFSET/FETCH` pagination, `MERGE INTO`-based
constructs (if needed — confirm against P2's already-established "golem never implemented upsert"
finding from M18), row locking, and Oracle error mapping correctly wired, so query behavior and
error handling match what golem callers already expect.

**Why P3**: Necessary for full conformance (P1) but mechanical once P2's type-mapping questions are
settled. M18 already discovered `SaveOne`/`SaveMany` never needed `MERGE INTO`-based upsert on any
adapter (golem has no upsert semantics at all) — this must be re-confirmed true for M19 rather than
re-assumed from INSIGHT.md's UPSERT table entry a second time.

**Acceptance Criteria**:

1. WHEN `q.Limit(x).Offset(y)` are set THEN the compiled SQL SHALL end with `OFFSET y ROWS FETCH
   NEXT x ROWS ONLY` (Oracle 12c+ syntax, identical clause shape to M18/SQL Server). **[Corrected
   during design — see design.md]** Unlike SQL Server, Oracle does NOT require an `ORDER BY` for
   `OFFSET/FETCH` to be valid syntax (confirmed via a real-server probe) — `stmt.Select.PrimaryKey`
   (added in M18, core-schema) is read but never needs to force-inject a default `ORDER BY` here;
   `ORDER BY` is only emitted when the caller explicitly set `s.OrderBy`.
2. WHEN `.ForUpdate()` (or whichever strengths map cleanly — confirm `NOWAIT`/`SKIP LOCKED` support
   during design; Oracle has supported both since old versions) runs inside a `golem.Tx` THEN the
   compiled SQL SHALL include the correct `FOR UPDATE [NOWAIT | SKIP LOCKED]` clause — same
   trailing-clause shape as Postgres, not a table hint like SQL Server.
3. WHEN a write violates a `Unique` constraint THEN `IsConflict` SHALL return `true` for Oracle's
   ORA-00001, wrapped as `golem.ErrDuplicateKey`.
4. WHEN a write violates a `ForeignKey` constraint THEN the adapter SHALL map Oracle's ORA-02291
   (insert/update) and ORA-02292 (delete) to `golem.ErrForeignKeyViolation`.
5. **[Confirmed during design, same as M18's P2/AC3 correction]** `SaveOne`/`SaveMany` do NOT need
   upsert (`MERGE INTO`) — golem has never implemented upsert semantics on any adapter. They route
   through the same multi-round-trip `Update` as `Repository[T].Update`. No `MERGE INTO` code is
   needed for M19.

**Independent Test**: `internal/dialecttest`'s `CRUD`/`Locking`/`ConflictDetection` subtest groups
pass (or correctly `SKIP`, for any unsupported `Locking` strength) against `driver/oracle`.

---

## Edge Cases

- WHEN a `golem.ColumnType` kind has no exact Oracle type (`BOOLEAN`→`NUMBER(1)`,
  `SMALLINT`→`NUMBER(5)`, `INTEGER`→`NUMBER(10)`, `BIGINT`→`NUMBER(19)`, `DECIMAL(p,s)`→`NUMBER(p,s)`,
  `TEXT`→`CLOB`, `UUID`→`VARCHAR2(36)` (design decision), `JSON`→`CLOB`, `TIME`→`TIMESTAMP`
  (corrected during T10's real conformance run — `INTERVAL DAY TO SECOND` can't accept a bound
  `time.Time` from `go-ora`, see design.md) THEN `Bind`/`Scan` SHALL convert
  transparently — matching every prior
  adapter's stance that these remain implemented/unit-tested even though dead in the real
  Insert/Update/Query path (AD-037).
- WHEN pagination (`OFFSET/FETCH`) is requested without an explicit `OrderBy` THEN the adapter SHALL
  simply omit `ORDER BY` — confirmed via probe that Oracle's `OFFSET/FETCH` doesn't require one,
  unlike SQL Server. This is a real, confirmed divergence from M18, not a redesign of core schema.
- WHEN `repository.Exists()`'s `Count: true` query needs pagination-shaped `OFFSET/FETCH` without a
  real orderable column THEN the adapter SHALL omit `ORDER BY` entirely (not apply MSSQL's
  `ORDER BY (SELECT NULL)` idiom — confirmed via probe that Oracle rejects that too, with the same
  `ORA-00937` error a real `ORDER BY <column>` gets). Since `OFFSET/FETCH` never needs an `ORDER BY`
  here, omitting it is the actual fix, not a workaround.
- WHEN `golem.UUID()`-typed data is written THEN it SHALL use `VARCHAR2(36)` (decided during design,
  not `RAW(16)`) — confirmed via probe this scans as a plain string already matching golem's UUID
  contract, with no byte-format decision needed the way `RAW(16)` would require.
- WHEN `NOWAIT`/`SKIP LOCKED`-equivalent behavior has no clean Oracle mapping for a given lock
  strength THEN it SHALL be declared unsupported via `Capabilities.Locking`, not approximated.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| ORACLE-01 | P1: Passes M15 conformance | Tasks (T1-T3, T10) | Ready |
| ORACLE-02 | P1: Passes M15 conformance | Tasks (T10) | Ready |
| ORACLE-03 | P1: Passes M15 conformance | Tasks (T10) | Ready |
| ORACLE-04 | P2: NUMBER-family scan/bind correctness | Tasks (T4, T6) | Ready |
| ORACLE-05 | P2: NUMBER-family scan/bind correctness | Tasks (T6) | Ready |
| ORACLE-06 | P2: NUMBER-family scan/bind correctness | Tasks (T6) | Ready |
| ORACLE-07 | P3: Pagination/upsert/locking/conflict mapping | Tasks (T5) | Ready |
| ORACLE-08 | P3: Pagination/upsert/locking/conflict mapping | Tasks (T5) | Ready |
| ORACLE-09 | P3: Pagination/upsert/locking/conflict mapping | Tasks (T8) | Ready |
| ORACLE-10 | P3: Pagination/upsert/locking/conflict mapping | Tasks (T8) | Ready |

**Coverage:** 10 total, 10 mapped to tasks, 0 unmapped ✅

---

## Success Criteria

- [ ] `driver/oracle`'s unit tests (mocked, via `DATA-DOG/go-sqlmock` — same library as every other
      `database/sql`-based adapter) hit 100% statement coverage (or documented exceptions, same
      class as every other adapter's, applying `internal/must`/`internal/testutil` where genuinely
      applicable)
- [ ] `driver/oracle/conformance_integration_test.go` passes `task test-integration` against a real
      Oracle 12c+ container, with any genuinely-unsupported locking strength reporting `SKIP`
- [ ] No changes needed to `golem`, `entity`, `repository`, `query`, `op`, `join`, `relation`,
      `internal/stmt`, or `internal/dialecttest` — if M19 needs one, that's a signal the
      "dialect-agnostic core" premise (AD-015/AD-016) had a gap, and it gets its own AD entry
      explaining what and why
