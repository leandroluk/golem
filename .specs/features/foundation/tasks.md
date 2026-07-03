# Foundation (M1) Tasks

**Design**: `.specs/features/foundation/design.md`
**Status**: Done — all 9 tasks complete, `make gate-full` passes (unit + real dockerized Postgres integration)

**SPEC_DEVIATION**: `docker-compose.test.yml`'s Postgres service was originally mapped to host port 5432; during T9 this conflicted with an unrelated Postgres instance already running locally on the machine. Remapped to host port 55432 (container-internal port unchanged) — see commit `fix(test): remap test Postgres to host port 55432`.

---

## Execution Plan

### Phase 1: Test infra (Sequential)

```
T1
```

### Phase 2: Root package primitives (Parallel OK)

```
       ┌→ T2 ─┐
T1 ────┼→ T3 ─┼──→ (Phase 3)
       └→ T4 ─┘
```

### Phase 3: Interfaces + adapter config (Parallel OK)

```
T3 ──→ T5 ─┐
T2 ──→ T7 ─┼──→ (Phase 4)
```

### Phase 4: DataSource + adapter dialect stub (Parallel OK)

```
T4, T5 ──→ T6 ─┐
T3, T5 ──→ T8 ─┼──→ T9
```

### Phase 5: Real Postgres wiring + integration test (Sequential)

```
T6, T7, T8, T2 ──→ T9
```

---

## Task Breakdown

### T1: Test infra — Taskfile.yml + docker-compose.test.yml

**What**: Root `Taskfile.yml` with `build`/`vet`/`test`/`test-integration`/`gate-quick`/`gate-full` targets, plus `docker-compose.test.yml` defining a disposable Postgres service for integration tests.
**Where**: `Taskfile.yml`, `docker-compose.test.yml`
**Depends on**: None
**Reuses**: N/A (first artifact)
**Requirement**: N/A (tooling)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `Taskfile.yml` has `build`, `vet`, `test`, `test-integration`, `gate-quick`, `gate-full` targets matching `.specs/codebase/TESTING.md`
- [x] `docker-compose.test.yml` defines a Postgres service with a healthcheck (`pg_isready`) so `up -d --wait` blocks until ready
- [x] `make -n gate-quick`/`make -n gate-full` dry-run shows the correct command sequence; `docker compose -f docker-compose.test.yml config` validates
- [x] SPEC_DEVIATION: `go vet ./...` exits non-zero ("no packages to vet") on a source-free module — a real gate-quick pass is deferred to T2 (first task adding source); this is a Go tooling quirk on an empty tree, not a Taskfile.yml defect

**Tests**: none
**Gate**: build (manual: `make gate-quick` succeeds)

**Commit**: `chore(test): add Taskfile.yml and docker-compose test infra`

---

### T2: `golem.Logger` + `LogLevel` [P] — ✅ Complete

**What**: `LogLevel` enum (`Debug`/`Info`/`Warn`/`Error` + `String()`), `Logger` interface, unexported default console logger.
**Where**: `logger.go`
**Depends on**: T1
**Reuses**: N/A
**Requirement**: N/A (interface shape only; behavior verified end-to-end in T9)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `LogLevel` constants + `String()` cover all 4 levels; SPEC_DEVIATION: `String()` itself never panics (returns `"unknown(N)"` fallback) — the panic-on-unmapped-value belongs to a *caller's* own switch (per README's custom-logger example), not to `String()`
- [x] `Logger` interface has exactly `Debug`/`Info`/`Warn`/`Error(msg string, args map[string]any)`
- [x] Default logger implements `Logger` via `fmt.Println`, never panics
- [x] Gate check passes: `make gate-quick` (verified via `go build/vet/test` directly, equivalent)
- [x] Test count: 4 unit tests pass

**Tests**: unit
**Gate**: quick

**Commit**: `feat(golem): add Logger interface and LogLevel enum`

---

### T3: `golem.ColumnType` stub [P] — ✅ Complete

**What**: Opaque `ColumnType` struct (unexported `kind` field), no exported constructor yet (M2 adds those).
**Where**: `columntype.go`
**Depends on**: T1
**Reuses**: N/A
**Requirement**: N/A (exists only so `Dialect` has a real param type to compile against)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `ColumnType` struct compiles with an unexported `kind string` field
- [x] In-package test constructs a fake `ColumnType{kind: "test"}` and asserts equality/zero-value behavior
- [x] Gate check passes: `make gate-quick` (verified via `go build/vet/test` directly, equivalent)
- [x] Test count: 2 unit tests pass

**Tests**: unit
**Gate**: quick

**Commit**: `feat(golem): add ColumnType stub for M1`

---

### T4: `golem.Conn` sealed marker interface [P] — ✅ Complete

**What**: `Conn` interface with a single unexported method (`isConn()`), sealing implementers to this package.
**Where**: `conn.go`
**Depends on**: T1
**Reuses**: N/A
**Requirement**: FOUND-11 (Conn exposes no speculative methods)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [x] `Conn` interface defined with only `isConn()` (unexported)
- [x] In-package test defines a fake type implementing `isConn()` and asserts `var _ Conn = (*fakeConn)(nil)` compiles
- [x] Gate check passes: `make gate-quick` (verified via `go build/vet/test` directly, equivalent)
- [x] Test count: 1 unit test passes

**Tests**: unit
**Gate**: quick

**Commit**: `feat(golem): add sealed Conn marker interface`

---

### T5: `golem.Dialect` + `golem.Connector` interfaces [P] — ✅ Complete

**What**: `Dialect` interface (`Bind`/`Scan`) and `Connector` interface (`Connect() (Dialect, error)`, `Close() error`).
**Where**: `dialect.go`, `connector.go`
**Depends on**: T3
**Reuses**: `ColumnType` (T3)
**Requirement**: FOUND-16 (Dialect shape: exactly `Bind`/`Scan`, nothing speculative)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `Dialect` interface: `Bind(t ColumnType, value any) (driver.Value, error)`, `Scan(t ColumnType, raw any, dest any) error` — nothing else
- [ ] `Connector` interface: `Connect() (Dialect, error)`, `Close() error`
- [ ] In-package test defines fake `Dialect`/`Connector` implementations and asserts they satisfy the interfaces
- [ ] Gate check passes: `make gate-quick`
- [ ] Test count: 1+ unit test passes

**Tests**: unit
**Gate**: quick

**Commit**: `feat(golem): add Dialect and Connector contracts`

---

### T6: `golem.DataSource` + `Option`/`NewDataSource`/`Connect`/`Close` [P] — ✅ Complete

**What**: `DataSource` struct, `Option` type, `DataSourceName`, `WithConnector`, `NewDataSource`, `(*DataSource) Connect()`/`Close()` with idempotency rules from design.md, `(*DataSource) isConn()` satisfying `Conn`.
**Where**: `datasource.go`, `options.go`
**Depends on**: T4, T5
**Reuses**: `Conn` (T4), `Connector` (T5)
**Requirement**: FOUND-01, FOUND-02, FOUND-03, FOUND-04, FOUND-05, FOUND-09 (golem-level: no connector supplied → error), FOUND-10, FOUND-11, FOUND-18

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `NewDataSource(opts ...Option) (*DataSource, error)` returns non-nil `*DataSource`/nil error on valid config; returns error if no `Connector` was supplied via any option
- [ ] `DataSourceName(name)` sets name; omitted → defaults to `"default"`
- [ ] `WithConnector(c Connector) Option` wires the connector (exported for adapters, not meant for direct end-user use)
- [ ] `Connect()` delegates to `Connector.Connect()`, stores the returned `Dialect`, is a no-op (returns nil) if already connected
- [ ] `Connect()` propagates connector errors verbatim (descriptive, no panic) on failure
- [ ] `Close()` delegates to `Connector.Close()`; no-op (returns nil) if never connected; idempotent if called twice
- [ ] `var _ Conn = (*DataSource)(nil)` compiles
- [ ] `var _ error` — `Connect()` failure does not mark the `DataSource` as connected (a retry after fixing config can succeed)
- [ ] Unit test uses a fake `Connector` to verify: success path, idempotent double-`Connect`, `Close`-without-`Connect`, double-`Close`, error propagation, default name `"default"`, and that the `Dialect` returned by the fake connector is retrievable internally (an unexported accessor or field is fine — same-package test)
- [ ] Gate check passes: `make gate-quick`
- [ ] Test count: 6+ unit tests pass

**Tests**: unit
**Gate**: quick

**Commit**: `feat(golem): add DataSource with Connect/Close lifecycle`

---

### T7: `postgres.Options` + `resolveDSN` [P] — ✅ Complete

**What**: `Options` struct (`DSN`, `Host`, `Port`, `User`, `Password`, `Database`, `SSLMode`, `Logging`, `Logger`) and `resolveDSN(o *Options) (string, error)` implementing the documented DSN/discrete-field precedence.
**Where**: `driver/postgres/postgres.go` (Options struct), `driver/postgres/dsn.go` (resolveDSN)
**Depends on**: T2
**Reuses**: `golem.Logger` (T2)
**Requirement**: FOUND-06, FOUND-07, FOUND-08, FOUND-09 (adapter-level: neither DSN nor fields set → error), edge case (malformed DSN)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] DSN-only config resolves to that DSN unchanged
- [ ] Fields-only config builds a valid DSN from discrete fields
- [ ] Both set: discrete fields override only the fields actually set on `Options`, DSN supplies the rest (partial override)
- [ ] Neither set: `resolveDSN` returns a descriptive configuration error
- [ ] Malformed DSN (invalid URL syntax): `resolveDSN` returns a descriptive error, no panic
- [ ] Gate check passes: `make gate-quick`
- [ ] Test count: 5+ table-driven unit tests pass

**Tests**: unit
**Gate**: quick

**Commit**: `feat(postgres): add Options and DSN precedence resolution`

---

### T8: `postgres.dialect` stub (implements `golem.Dialect`) [P] — ✅ Complete

**What**: Minimal `dialect` type whose `Bind`/`Scan` return a descriptive "unrecognized column type" error for any input (no real `ColumnType` set exists until M2).
**Where**: `driver/postgres/dialect.go`
**Depends on**: T3, T5
**Reuses**: `golem.Dialect`, `golem.ColumnType` (T3, T5)
**Requirement**: FOUND-17, FOUND-19

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `var _ golem.Dialect = (*dialect)(nil)` compiles
- [ ] `Bind`/`Scan` called with any fake `ColumnType` return a descriptive error (e.g. `"postgres: unrecognized column type"`), never panic
- [ ] Gate check passes: `make gate-quick`
- [ ] Test count: 2+ unit tests pass

**Tests**: unit
**Gate**: quick

**Commit**: `feat(postgres): add Dialect stub implementation`

---

### T9: `postgres.connector` + `postgres.New` (real Postgres wiring) — ✅ Complete

**What**: `connector` type implementing `golem.Connector` via `pgxpool` (resolves DSN via T7, opens the pool, runs a `SELECT 1` liveness check, wraps connect errors descriptively, logs lifecycle events through `Options.Logger`/default when `Options.Logging`); `New(configure func(*Options)) golem.Option` wiring it all through `golem.WithConnector`.
**Where**: `driver/postgres/connector.go`, `driver/postgres/postgres.go` (`New` func)
**Depends on**: T6, T7, T8, T2
**Reuses**: `golem.WithConnector`/`golem.DataSource` (T6), `resolveDSN`/`Options` (T7), `dialect` (T8), `golem.Logger` (T2)

**Requirement**: FOUND-01, FOUND-02, FOUND-03, FOUND-04, FOUND-12, FOUND-13, FOUND-14, FOUND-15 — end-to-end against real Postgres (FOUND-01–04 also unit-tested with a fake in T6; this task is the real-database proof the spec's Independent Test demands)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `golem.NewDataSource(postgres.New(...))` + `.Connect()` succeeds against the Dockerized Postgres from `docker-compose.test.yml`
- [ ] `.Connect()` against an unreachable host returns a descriptive error, does not hang/panic
- [ ] `.Connect()` against valid host but bad credentials returns a descriptive error, does not hang/panic
- [ ] `.Close()` after successful `.Connect()` releases the pool, returns nil
- [ ] `Options.Logging = true` with a custom `Logger` (test double appending to a slice) receives at least one entry across connect/close
- [ ] `Options.Logging` false/unset never invokes the logger
- [ ] All integration tests behind `//go:build integration`
- [ ] Gate check passes: `make gate-full`
- [ ] Test count: 6+ integration tests pass

**Tests**: integration
**Gate**: full

**Commit**: `feat(postgres): wire real pgxpool connector and New()`

---

## Parallel Execution Map

```
Phase 1 (Sequential):
  T1

Phase 2 (Parallel, after T1):
  ├── T2 [P]
  ├── T3 [P]
  └── T4 [P]

Phase 3 (Parallel, after T2/T3):
  ├── T5 [P]  (needs T3)
  └── T7 [P]  (needs T2)

Phase 4 (Parallel, after T4/T5/T3):
  ├── T6 [P]  (needs T4, T5)
  └── T8 [P]  (needs T3, T5)

Phase 5 (Sequential, after T6/T7/T8/T2):
  T9
```

**Parallelism constraint:** A task marked `[P]` must have ALL of these:
- No unfinished dependencies
- Required test type is parallel-safe (per TESTING.md Parallelism Assessment)
- No shared mutable state with other `[P]` tasks in the same phase

T9 is deliberately NOT `[P]` — it's `integration`-tested (Parallel-Safe: No per TESTING.md) and is the only task touching the shared Docker Postgres container.

---

## Task Granularity Check

| Task                               | Scope                                                                     | Status                                                            |
| ---------------------------------- | ------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| T1: Taskfile.yml + docker-compose  | 2 files, pure tooling                                                     | ✅ Granular                                                        |
| T2: Logger + LogLevel              | 1 file                                                                    | ✅ Granular                                                        |
| T3: ColumnType stub                | 1 file                                                                    | ✅ Granular                                                        |
| T4: Conn marker interface          | 1 file                                                                    | ✅ Granular                                                        |
| T5: Dialect + Connector interfaces | 2 small, cohesive files (both pure contracts, no logic)                   | ✅ Granular (OK as 2-in-1: both are 3-line interface declarations) |
| T6: DataSource + Options           | 2 files, one cohesive component (lifecycle + its config)                  | ✅ Granular                                                        |
| T7: postgres Options + resolveDSN  | 2 files, one cohesive component (config + its parsing)                    | ✅ Granular                                                        |
| T8: postgres dialect stub          | 1 file                                                                    | ✅ Granular                                                        |
| T9: postgres connector + New       | 2 files, one cohesive component (real connection + its public entrypoint) | ✅ Granular                                                        |

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows                                                                | Status  |
| ---- | ---------------------- | ---------------------------------------------------------------------------- | ------- |
| T1   | None                   | Phase 1, no incoming arrows                                                  | ✅ Match |
| T2   | T1                     | Phase 2, arrow from T1                                                       | ✅ Match |
| T3   | T1                     | Phase 2, arrow from T1                                                       | ✅ Match |
| T4   | T1                     | Phase 2, arrow from T1                                                       | ✅ Match |
| T5   | T3                     | Phase 3, arrow from T3                                                       | ✅ Match |
| T6   | T4, T5                 | Phase 4, arrows from T4 and T5                                               | ✅ Match |
| T7   | T2                     | Phase 3, arrow from T2                                                       | ✅ Match |
| T8   | T3, T5                 | Phase 4, arrows from T3 and T5                                               | ✅ Match |
| T9   | T6, T7, T8, T2         | Phase 5, arrows from T6, T7, T8 (T2 transitively via T7/T9's own logger use) | ✅ Match |

---

## Test Co-location Validation

| Task                            | Code Layer Created/Modified         | Matrix Requires | Task Says   | Status |
| ------------------------------- | ----------------------------------- | --------------- | ----------- | ------ |
| T1: Taskfile.yml/compose        | tooling (no code layer)             | —               | none        | ✅ OK   |
| T2: Logger                      | `golem` root package                | unit            | unit        | ✅ OK   |
| T3: ColumnType                  | `golem` root package                | unit            | unit        | ✅ OK   |
| T4: Conn                        | `golem` root package                | unit            | unit        | ✅ OK   |
| T5: Dialect/Connector           | `golem` root package                | unit            | unit        | ✅ OK   |
| T6: DataSource                  | `golem` root package                | unit            | unit        | ✅ OK   |
| T7: postgres Options/resolveDSN | `driver/postgres` (DSN resolution) | unit            | unit        | ✅ OK   |
| T8: postgres dialect stub       | `driver/postgres` (dialect stub)   | unit            | unit        | ✅ OK   |
| T9: postgres connector/New      | `driver/postgres` (real connector) | integration     | integration | ✅ OK   |

No task defers its required tests to "another task" — every task's `Done when` includes its own gate check.

---

## Tools for Execution

Every task above uses **NONE** for MCP and Skill — this is standard Go stdlib code (structs, interfaces, `net/url`, `pgxpool`, `testing`). No project MCP/skill adds value here (no framework-specific codegen, no external API to query). `context7` MCP will be used ad hoc only if `pgx/v5`/`pgxpool` API details need verification during T9 (already a documented dependency, not a new one).


