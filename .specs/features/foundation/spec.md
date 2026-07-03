# Foundation (M1) Specification

## Problem Statement

Every later milestone (schema, repository, query builder, transactions) needs a live, configurable
connection to Postgres to operate against. Before any of that can be built, `golem` needs a
`DataSource` that can be constructed, connected, and closed, plus the `golem.Conn` abstraction that
future milestones (repository, transactions) will depend on.

## Goals

- [ ] `golem.NewDataSource(...)` + `postgres.New(...)` compiles and connects to a real Postgres instance
- [ ] `*golem.DataSource` satisfies `golem.Conn` so M3+ can build against the interface without changes later
- [ ] Connection configuration supports both DSN and discrete fields, with a documented precedence rule
- [ ] `golem.Dialect` exists as an interface and the Postgres adapter implements it, so M2 can build `golem.ColumnType` against a working contract

## Out of Scope

| Feature | Reason |
| --- | --- |
| Entities, `entity.Builder`, the actual `golem.ColumnType` set (`BIGINT`, `VARCHAR`, ...) | M2 — M1 only needs the `golem.Dialect` interface to exist and be implemented, not the types that use it |
| `Repository[T]`, query building, hooks, transactions | M3-M8 |
| Query-level logging (logging individual SQL statements) | No queries exist yet in M1; revisit once M3 (repository) introduces the first real SQL execution |
| `golem.Tx` implementing `golem.Conn` | M8 — M1 only needs `golem.Conn` to exist as an interface and `*DataSource` to satisfy it |
| MySQL/MSSQL/Oracle implementations of `golem.Dialect` | Future adapters (see ROADMAP.md "Future Considerations") — M1 only needs the contract shaped so they *can* be added later without changing it |
| A registry/lookup API for named data sources (`golem.DataSourceName("example")` is just a label in M1) | Not documented in README beyond naming; add later only if a real multi-datasource lookup need appears |

---

## User Stories

### P1: Connect to Postgres ⭐ MVP

**User Story**: As a Go developer using `golem`, I want to create a `DataSource` and connect it to Postgres, so that later features have a live connection to operate on.

**Why P1**: Nothing else in the roadmap (M2-M10) can be built or tested without a working connection.

**Acceptance Criteria**:

1. WHEN `golem.NewDataSource(postgres.New(func(o *postgres.Options) {...}))` is called with a valid configuration THEN system SHALL return a non-nil `*golem.DataSource` and a nil error.
2. WHEN `DataSource.Connect()` is called against a reachable Postgres instance THEN system SHALL establish a connection pool and return a nil error.
3. WHEN `DataSource.Connect()` is called against an unreachable host or with invalid credentials THEN system SHALL return a non-nil, descriptive error and SHALL NOT panic.
4. WHEN `DataSource.Close()` is called after a successful `Connect()` THEN system SHALL release the underlying connection pool and return a nil error.
5. WHEN no `golem.DataSourceName(...)` option is passed to `golem.NewDataSource` THEN system SHALL default the data source's name to `"default"`.

**Independent Test**: Point `postgres.New` at a real (or dockerized) Postgres instance, call `Connect()`, assert no error, run `SELECT 1` through the raw pool internally, call `Close()`, assert no error.

---

### P1: Configure the Postgres connection (DSN or discrete fields) ⭐ MVP

**User Story**: As a Go developer, I want to configure the Postgres connection either via a single DSN string or via discrete fields (host/port/user/password/database/sslmode), so that I can use whichever is more convenient for my deployment (e.g. env-injected DSN vs. individually-injected fields).

**Why P1**: Documented explicitly in the root README ("Exemplo usando postgres") as core connector behavior; both forms are shown as first-class.

**Acceptance Criteria**:

1. WHEN only `o.DSN` is set on `postgres.Options` THEN system SHALL connect using that DSN as-is.
2. WHEN only the discrete fields (`Host`, `Port`, `User`, `Password`, `Database`, `SSLMode`) are set THEN system SHALL build a connection string from them.
3. WHEN both `o.DSN` and one or more discrete fields are set THEN system SHALL prefer the discrete fields over the DSN for the fields that were set (partial override), matching the README note: "Se passar ambos (DSN + propriedades) então propriedades tem prioridade."
4. WHEN neither `o.DSN` nor any discrete field is set THEN system SHALL return a configuration error from `Connect()` (or from `postgres.New`, TBD at design time) rather than attempting to connect with an empty target.

**Independent Test**: Three separate `Connect()` calls (DSN-only, fields-only, both-set-with-one-field-overridden) against the same real Postgres instance all succeed and open a connection to the expected database.

---

### P2: `golem.Conn` abstraction exists and `*DataSource` satisfies it

**User Story**: As a `golem` contributor building M3+ (repository, transactions), I want `golem.Conn` to already exist as an interface with `*DataSource` as a compile-time-verified implementation, so that M3-M9 can be written against `golem.Conn` from day one without a later breaking refactor.

**Why P2**: Not user-facing on its own (no end-user calls `golem.Conn` directly in M1), but is a hard prerequisite for `repository.Get`, `Exec`, and `Transaction` signatures used everywhere from M3 onward.

**Acceptance Criteria**:

1. WHEN the `golem` package is compiled THEN a `var _ golem.Conn = (*golem.DataSource)(nil)` compile-time assertion SHALL pass.
2. WHEN `golem.Conn` is defined THEN it SHALL expose exactly the operations needed by later milestones and no more (no speculative methods) — at minimum whatever `repository.Get` and `Exec` need; grown incrementally as M3/M9 land, not front-loaded in M1.

**Independent Test**: `go build ./...` succeeds with the assertion in place; no method exists on `golem.Conn` that isn't yet consumed by any shipped milestone.

---

### P2: `golem.Dialect` contract exists and the Postgres adapter implements it

**User Story**: As a `golem` contributor building M2 (`golem.ColumnType`), I want a `golem.Dialect` interface for value bind/scan to already exist and be implemented by the Postgres adapter, so that column types can be declared as dialect-agnostic from the first entity onward instead of being tied to one adapter's package.

**Why P2**: Not directly user-facing in M1 (no `golem.ColumnType` exists yet to exercise it), but is the hard prerequisite that makes M2's entities portable across future adapters (MySQL/MSSQL/Oracle) without a rewrite — see AD-015 in `STATE.md`.

**Acceptance Criteria**:

1. WHEN `golem.Dialect` is defined THEN it SHALL expose `Bind(t golem.ColumnType, value any) (driver.Value, error)` and `Scan(t golem.ColumnType, raw any, dest any) error`, and nothing else speculative.
2. WHEN the Postgres adapter is compiled THEN a `var _ golem.Dialect = (*postgres.dialect)(nil)` (or equivalent) compile-time assertion SHALL pass.
3. WHEN a `DataSource` connects via the Postgres adapter THEN it SHALL hold a reference to that adapter's `golem.Dialect` implementation, retrievable by whatever internal mechanism M2/M3 need (exact accessor TBD at design time — not necessarily public API).
4. WHEN `golem.Dialect.Bind`/`Scan` are called with a `ColumnType` the adapter doesn't yet recognize THEN system SHALL return a descriptive error rather than panicking (future-proofs against `golem.ColumnType` growing before an adapter catches up).

**Independent Test**: `go build ./...` passes with the assertion in place; a minimal fake `golem.ColumnType` + value round-trips through the Postgres adapter's `Bind` then `Scan` without error (exact fake shape TBD at design time, since the real `ColumnType` set doesn't exist until M2).

---

### P2: Pluggable logger

**User Story**: As a Go developer, I want to plug in my own `golem.Logger` implementation (or fall back to a default console logger), so that I control how connection lifecycle events are surfaced in my application's logging pipeline.

**Why P2**: Documented in README ("Exemplo de criação de um log personalizado") as a first-class customization point, but not required for the MVP connection flow to work (default logger covers it).

**Acceptance Criteria**:

1. WHEN `o.Logger` is set to a custom `golem.Logger` implementation THEN system SHALL use it for all log output instead of the default.
2. WHEN `o.Logger` is not set THEN system SHALL use a default console logger (`fmt.Println`-based, per README).
3. WHEN `o.Logging` is `false` (or unset) THEN system SHALL NOT invoke the logger for connection lifecycle events.
4. WHEN `o.Logging` is `true` THEN system SHALL invoke the logger for at least connect/disconnect/error lifecycle events (query-level logging is out of scope until M3+).

**Independent Test**: Implement `golem.Logger` with a struct that appends to a slice; connect and close a `DataSource` with `o.Logging = true`; assert the slice received at least one entry.

---

## Edge Cases

- WHEN `postgres.Options.DSN` is malformed (invalid URL syntax) THEN system SHALL return a descriptive error from `Connect()`, not panic.
- WHEN `Connect()` is called a second time on an already-connected `DataSource` THEN behavior is TBD — flagged for Design phase (idempotent no-op vs. explicit error).
- WHEN `Close()` is called without a prior successful `Connect()` THEN system SHALL NOT panic (safe no-op or descriptive error, TBD at design time).
- WHEN `Close()` is called twice THEN system SHALL NOT panic (idempotent).
- WHEN the Postgres server accepts the TCP connection but rejects auth (bad user/password) THEN system SHALL surface that as a `Connect()` error, not hang or panic.

---

## Requirement Traceability

| Requirement ID | Story | Task | Status |
| --- | --- | --- | --- |
| FOUND-01 | P1: Connect to Postgres | T6, T9 | In Tasks |
| FOUND-02 | P1: Connect to Postgres | T6, T9 | In Tasks |
| FOUND-03 | P1: Connect to Postgres | T6, T9 | In Tasks |
| FOUND-04 | P1: Connect to Postgres | T6, T9 | In Tasks |
| FOUND-05 | P1: Connect to Postgres | T6 | In Tasks |
| FOUND-06 | P1: Configure connection | T7 | In Tasks |
| FOUND-07 | P1: Configure connection | T7 | In Tasks |
| FOUND-08 | P1: Configure connection | T7 | In Tasks |
| FOUND-09 | P1: Configure connection | T6, T7 | In Tasks |
| FOUND-10 | P2: golem.Conn abstraction | T4, T6 | In Tasks |
| FOUND-11 | P2: golem.Conn abstraction | T4, T6 | In Tasks |
| FOUND-12 | P2: Pluggable logger | T9 | In Tasks |
| FOUND-13 | P2: Pluggable logger | T9 | In Tasks |
| FOUND-14 | P2: Pluggable logger | T9 | In Tasks |
| FOUND-15 | P2: Pluggable logger | T9 | In Tasks |
| FOUND-16 | P2: golem.Dialect contract | T5, T9 | In Tasks |
| FOUND-17 | P2: golem.Dialect contract | T8 | In Tasks |
| FOUND-18 | P2: golem.Dialect contract | T6 | In Tasks |
| FOUND-19 | P2: golem.Dialect contract | T8 | In Tasks |

**ID format:** `FOUND-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 19 total, 19 mapped to tasks, 0 unmapped ✅

---

## Success Criteria

- [ ] A real Postgres instance can be connected to and closed via `golem.NewDataSource` + `postgres.New`, with zero panics across all documented edge cases
- [ ] `go build ./...` passes with `*golem.DataSource` satisfying `golem.Conn`
- [ ] A custom `golem.Logger` can fully replace the default logger with no code changes outside `postgres.Options`
- [ ] `go build ./...` passes with the Postgres adapter satisfying `golem.Dialect`, ready for M2 to define `golem.ColumnType` against it without touching M1 code
