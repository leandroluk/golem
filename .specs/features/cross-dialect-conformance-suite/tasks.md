# Cross-Dialect Conformance Suite (M15) Tasks

**Design**: `.specs/features/cross-dialect-conformance-suite/design.md`
**Status**: DONE. `task test-integration` verified green against real Postgres. Found and fixed 3
real bugs along the way (AD-037 in STATE.md): pgx-native types (`pgtype.Numeric`/`pgtype.Time`/
`[16]byte`/JSON object-or-array) leaking past the `Dialect` boundary, and `assignFieldValue` unable
to scan a non-NULL value into a nullable (`*X`) field.

All tasks are sequential (each depends on the previous one's types/entities existing) — no `[P]`
parallel tasks, executed directly, no sub-agent dispatch.

**Note on verification**: no Docker available in this environment. Every task's gate is
`go build -tags=integration ./... && go vet -tags=integration ./driver/postgres` (compiles, type-
checks) — NOT a real pass/fail run against Postgres. `task test-integration` (Docker required)
is the actual gate; the user runs it locally or CI runs it on push.

---

## Task Breakdown

### T1: Package skeleton — `Schema`/`Capabilities`/`LockCapabilities` + `Run` dispatch

**What**: `internal/dialecttest/dialecttest.go` — package doc, the three types from design.md,
`Run(t, connector, schema, caps)` that builds a uniquely-named `*golem.DataSource`
(`golem.DataSourceName(t.Name())`, deferred `Close`), connects it, and dispatches to each
subtest group via `t.Run(name, func(t *testing.T) {...})` (groups are stubs for now, filled in by
later tasks).
**Requirement**: CONF-01, CONF-02, CONF-03
**Gate**: `go build ./...`

**Done when**:
- [ ] Types match design.md exactly
- [ ] `Run` connects and defers `Close`, dispatches to (initially empty) group funcs
- [ ] `go build ./...` passes

---

### T2: Harness entities — `Widget`/`Deleted`/`Parent`/`Child`

**What**: `internal/dialecttest/entities.go` — 4 structs + `entity.New` mappings, table names
`conf_widget`/`conf_deleted`/`conf_parent`/`conf_child`. `Deleted` gets a `DeletedAt *time.Time`
+ `DeleteDate`. `Child` gets a cascade `ForeignKey` to `Parent` (`OnDelete(relation.OnDeleteCascade)`
by default — individual cascade subtests override per-case via their own entity variants if
`OnDeleteSetNull`/`OnDeleteRestrict` need separate FK configs, per M11's existing test pattern of
one child-table-per-cascade-mode).
**Depends on**: T1
**Reuses**: `.examples/postgres-minimal-blog/entities.go`'s style (plain structs, `entity.New`, no tags)
**Requirement**: CONF-03

**Done when**:
- [ ] `go build ./...` passes
- [ ] Entities compile with `entity.New`, no panics at package init (`go vet` catches nothing new)

---

### T3: `BindScanRoundTrip` + `CRUD` groups

**What**: `internal/dialecttest/crud.go` — `runBindScanRoundTrip` (insert one `Widget` covering
every `golem.ColumnType` kind used in T2's entities, `FindOne` it back, compare field-by-field)
and `runCRUD` (`Insert`/`InsertMany`/`SaveOne`/`SaveMany`/`Update`/`FindOne`/`FindMany`/`Count`/
`Exists`/`Delete` against `Widget`, hard delete since `Widget` has no `DeleteDate`).
**Depends on**: T2
**Reuses**: `repository/repository_test.go`'s assertion style
**Requirement**: CONF-01

**Done when**:
- [ ] `go vet -tags=integration ./internal/dialecttest` passes
- [ ] Wired into `Run`'s dispatch table (T1)

---

### T4: `SoftDelete` + `Cascade` groups

**What**: `internal/dialecttest/softdelete.go` (`Delete`/`Restore` + default-filter/`.WithDeleted()`
on `Deleted`) and `internal/dialecttest/cascade.go` (`OnDeleteCascade`/`SetNull`/`Restrict` between
`Parent`/`Child` — needs 3 `Child`-shaped entities, one per cascade mode, matching
`repository/repository_test.go`'s `newCascadeChildEntity` pattern).
**Depends on**: T2
**Requirement**: CONF-01

**Done when**:
- [ ] `go vet -tags=integration ./internal/dialecttest` passes
- [ ] Wired into `Run`'s dispatch table

---

### T5: `Joins` + `Preload` groups

**What**: `internal/dialecttest/joins.go` (`join.Inner` between `Parent`/`Child`) and
`internal/dialecttest/preload.go` (`repository.Preload` between `Parent`/`Child`).
**Depends on**: T2
**Requirement**: CONF-01

**Done when**:
- [ ] `go vet -tags=integration ./internal/dialecttest` passes
- [ ] Wired into `Run`'s dispatch table

---

### T6: `Aggregates` group

**What**: `internal/dialecttest/aggregates.go` — `repository.Aggregate` over `Widget`
(`GroupBy`/`Sum`/`CountAll`/`Having`), needs a numeric + a groupable string column on `Widget`
(already present from T2's bind/scan coverage columns — reuse, don't add new ones unless T2's
shape doesn't cover it).
**Depends on**: T2
**Requirement**: CONF-01

**Done when**:
- [ ] `go vet -tags=integration ./internal/dialecttest` passes
- [ ] Wired into `Run`'s dispatch table

---

### T7: `Locking` + `ConflictDetection` groups

**What**: `internal/dialecttest/locking.go` — one subtest per `LockCapabilities` field, each
`t.Skip`s if its capability is `false`; plus the outside-a-`Tx` error guard (always runs,
capability-independent). `internal/dialecttest/conflict.go` — `golem.ErrDuplicateKey` (insert
same PK/unique twice) and `golem.ErrForeignKeyViolation` (bad FK, and — reusing `Cascade`'s
`Restrict`-mode child from T4 — a blocked delete).
**Depends on**: T2, T4 (reuses its Restrict-mode entities)
**Requirement**: CONF-01, CONF-04, CONF-05

**Done when**:
- [ ] `go vet -tags=integration ./internal/dialecttest` passes
- [ ] Wired into `Run`'s dispatch table
- [ ] A capability set with every `LockCapabilities` field `false` produces only `t.Skip`s in the
      `Locking` group when manually traced through the code (no way to execute-verify without Docker)

---

### T8: `driver/postgres` conformance integration test

**What**: `driver/postgres/conformance_integration_test.go` (`//go:build integration`, matching
`connector_integration_test.go`'s convention) — `postgresConformanceSchema` (4
`CREATE TEMPORARY TABLE` statements matching T2's entities exactly) + `postgresCapabilities`
(every field `true`) + one test function calling `dialecttest.Run`.
**Depends on**: T1-T7 (needs every group to exist)
**Reuses**: `.examples/postgres-minimal-blog/main_integration_test.go`'s `TempPost` DDL pattern,
`connector_integration_test.go`'s `testDSN()`/`resolveDSN()` helpers
**Requirement**: CONF-01, CONF-02, CONF-06

**Done when**:
- [ ] `go build -tags=integration ./driver/postgres` passes
- [ ] `go vet -tags=integration ./driver/postgres` passes
- [ ] User (or CI) runs `task test-integration` and confirms green — flagged explicitly as
      unverified by me until that happens (no Docker available here)

---

### T9: Update living docs

**What**: `.specs/project/ROADMAP.md` M15 status → DONE with a real summary (matching M1-M14's
existing style); `.specs/project/STATE.md` — new AD recording the shipped design + a note in
"Current Work"; `README.md`'s Implementation Status checkbox for M15 → `[x]`.
**Depends on**: T8
**Requirement**: — (housekeeping, not a spec requirement)

**Done when**:
- [ ] ROADMAP.md M15 section matches the pattern of M1-M14's "DONE" sections
- [ ] STATE.md has a new AD entry
- [ ] README.md's M15 checkbox is checked
