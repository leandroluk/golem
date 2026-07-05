# Cross-Dialect Conformance Suite (M15) Specification

## Status: Planned

## Problem Statement

M1-M14 proved golem's core (entity mapping, repository, query builder, joins, hooks,
transactions, relations/cascade, preload, aggregations, pessimistic locking) against Postgres
only. The behavior contracts these features rely on were never written down as a portable test
suite — they live scattered across `driver/postgres`'s unit tests (mocked dialect) and
`.examples/postgres-minimal-blog`'s integration tests (real Postgres, using that example's own
`User`/`Post`/`Message` schema). Nothing about either is reusable by a second adapter.

Without this, M16 (MySQL/MariaDB) would mean hand-copying `.examples/postgres-minimal-blog`'s ~10
integration test functions into a new example, adjusting them by hand, and hoping nothing drifts
as more adapters get added after that. AD-034 already decided this is worth avoiding — this spec
defines exactly what gets built.

## Goals

- [ ] A single, reusable test harness that any `golem.Connector` implementation can be run
      against, asserting the same behavioral guarantees `.examples/postgres-minimal-blog`
      currently proves only for Postgres
- [ ] `driver/postgres` is proven conformant by being the harness's first (and, until M16, only)
      caller — no behavior change to what's already verified, just relocated/reused
- [ ] A dialect that can't support some part of the contract (e.g. Snowflake's M21 locking, or a
      dialect missing `NO KEY UPDATE`) declares that up front and the harness skips exactly that
      part, rather than the caller forking the whole suite

## Out of Scope

| Item | Reason |
| --- | --- |
| Actually building a second adapter (MySQL/etc.) | That's M16+; this milestone only builds and proves the harness against the one adapter that already exists |
| Testing driver-specific connection plumbing (DSN parsing, pool options, logging) | Adapter-specific by nature (see `driver/postgres/dsn_test.go`, `connector_test.go`) — stays adapter-local, not part of the shared contract |
| A CLI or `go generate` step to scaffold a new adapter's test file | Out of scope for now; revisit if scaffolding boilerplate becomes painful once 2-3 adapters exist |

---

## User Stories

### P1: Run the shared suite against a real Connector ⭐ MVP

**User Story**: As someone building a new golem adapter, I want a single function I can call
with my `golem.Connector` that exercises every cross-cutting behavior guarantee (CRUD, joins,
soft delete, cascade, aggregates, locking, conflict detection, bind/scan for every
`golem.ColumnType`), so that passing it is sufficient evidence my adapter is conformant — I don't
need to invent my own integration tests from scratch.

**Why P1**: This is the entire point of M15. Everything else is in service of this.

**Acceptance Criteria**:

1. WHEN a new adapter's integration test file calls the harness's entrypoint with a working
   `golem.Connector` THEN the harness SHALL run every conformance case as Go subtests
   (`t.Run(...)`), each independently reportable pass/fail.
2. WHEN `driver/postgres`'s own integration test calls the same entrypoint THEN every case that
   currently passes via `.examples/postgres-minimal-blog` SHALL still pass, with no assertions
   weakened to make the harness generic.
3. WHEN the harness needs example entities (a single-PK entity, a soft-delete entity, a
   cascade-FK parent/child pair) THEN it SHALL define and own them itself — no dependency on
   `.examples/postgres-minimal-blog`'s `User`/`Post`/`Message` types.

**Independent Test**: Run the harness against `driver/postgres` under `-tags=integration`
targeting the existing Dockerized test Postgres; all conformance subtests pass.

---

### P2: Declare unsupported capabilities instead of failing

**User Story**: As an adapter author whose database genuinely can't do something the harness
checks (e.g. no row locking on an OLAP database, no `NO KEY UPDATE` equivalent on MySQL), I want
to declare that up front, so the harness skips exactly that case instead of me forking the whole
suite or the harness reporting a false failure.

**Why P2**: Not needed for `driver/postgres` (which supports everything M1-M14 built), but the
harness is worthless for M18-M21 if it can't express "this dialect doesn't do X" — designing that
escape hatch now, while there's only one real caller, is far cheaper than retrofitting it once
3 adapters already depend on the harness's shape.

**Acceptance Criteria**:

1. WHEN an adapter's capability declaration marks a lock strength (or another optional
   capability) as unsupported THEN the harness SHALL skip (via `t.Skip`, not silently omit) the
   subtests exercising it, and still run every other case.
2. WHEN `driver/postgres`'s capability declaration marks everything as supported THEN no subtest
   SHALL be skipped.

**Independent Test**: A fake/stub capability declaration that marks locking as fully unsupported
causes the harness's locking subtests to report `SKIP`, not `PASS` or `FAIL`, while CRUD subtests
still run and pass.

---

### P3: Conformance run is wired into `task test-integration`

**User Story**: As a maintainer, I want the conformance suite's `driver/postgres` run to execute
as part of the existing integration gate, so it can't silently bit-rot the way an opt-in-only
suite could.

**Why P3**: Convenience/CI hygiene, not required for the harness itself to exist or be usable.

**Acceptance Criteria**:

1. WHEN `task test-integration` runs THEN it SHALL include the conformance suite's
   `driver/postgres` invocation (no new Taskfile target needed if it's reachable via the existing
   `go test -tags=integration ./...` step).

---

## Edge Cases

- WHEN the harness's own example entities collide (table name, schema) with something already in
  the target test database THEN the harness SHALL use a distinct, harness-owned table-name
  prefix so it never collides with `.examples/postgres-minimal-blog`'s tables when both run
  against the same test database in the same CI job.
- WHEN a conformance subtest needs cleanup (rows inserted) THEN it SHALL clean up after itself
  (delete/truncate) so subtests don't leak state into each other or across repeated runs.
- WHEN the harness is run against a Connector that fails to even Connect() THEN it SHALL fail
  fast with a clear error identifying the Connect failure, not a confusing downstream failure in
  an unrelated subtest.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| CONF-01 | P1: Run the shared suite | Design | Pending |
| CONF-02 | P1: Run the shared suite | Design | Pending |
| CONF-03 | P1: Run the shared suite | Design | Pending |
| CONF-04 | P2: Declare unsupported capabilities | Design | Pending |
| CONF-05 | P2: Declare unsupported capabilities | Design | Pending |
| CONF-06 | P3: Wired into test-integration | Design | Pending |

**Coverage:** 6 total, 0 mapped to tasks, 6 unmapped ⚠️ (expected — Design/Tasks phases haven't run yet)

---

## Success Criteria

- [ ] `driver/postgres`'s integration test suite calls the harness and every case passes against
      real Postgres (`task test-integration`)
- [ ] The harness package itself has no import on `driver/postgres` or `.examples/*` (adapters
      depend on the harness, never the reverse)
- [ ] A future M16 (MySQL) adapter's integration test is expected to need only: build a
      `golem.Connector`, declare its capabilities, call the harness — no new assertions authored
      by hand for anything already covered here
