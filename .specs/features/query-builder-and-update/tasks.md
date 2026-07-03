# Query Builder + Save/Update Tasks

**Design**: `.specs/features/query-builder-and-update/design.md`
**Status**: Draft

File naming in this repo: **snake_case** for multi-word filenames (`column_type.go`, `data_source.go`); single-word files need no underscore (`entity.go`, `query.go`, `op.go`). Do not rename existing files. Docker/test infra lives under `.docker/` (`.docker/docker-compose.test.yml`, `.docker/testdata/schema.sql`) — use `make gate-quick`/`make gate-full`/`make test-integration`, don't hand-roll docker commands.

---

## Execution Plan

```
Phase 1 (parallel): T1 (entity.ResolveField) [P]   T2 (op package) [P]   T4 (Dialect Select/Update, remove FindByID) [P]
Phase 2 (parallel): T3 (query package, needs T2) [P]   T5 (postgres dialect impl, needs T4) [P]
Phase 3 (sequential): T6 (repository: FindMany/FindOne/SaveOne/UpdateOne/UpdateMany, remove FindByID) — needs T1, T3, T4
Phase 4 (sequential): T7 (example: FindOne instead of FindByID + integration test) — needs T5, T6
```

---

## Task Breakdown

### T1: Export `entity.ResolveField` [P]

**What**: Rename the existing unexported `resolveField` to exported `ResolveField` in `entity/entity.go` (same body, same signature: `func ResolveField(zero any, fieldPtr any) (string, error)`). Update the 3 existing call sites in `entity/builder.go` (`Col`, `PrimaryKey`, `ForeignKey`) to call the new exported name.
**Where**: `entity/entity.go`, `entity/builder.go` (modify)
**Depends on**: None
**Tests**: unit — existing `entity` tests must still pass unchanged (pure rename, no behavior change); optionally add one direct test calling `entity.ResolveField` from outside the package to prove it's now reachable
**Gate**: quick

### T2: `op` package (`Eq`) [P]

**What**: New package `op` with `Condition` struct and `Eq(fieldPtr any, value any) Condition`.
**Where**: `op/op.go` (new)
**Depends on**: None
**Tests**: unit — `Eq(&someField, 5)` produces a `Condition` with matching `FieldPtr`/`Value`
**Gate**: quick

### T3: `query` package (`Query[T]`, `Update[T]`) [P]

**What**: `query.Query[T]` (`Where(...op.Condition)`, `Conditions()`), `query.Update[T]` (`Where`, `Set(fieldPtr, value)`, `Conditions()`, `Sets()`), plus unexported-to-users constructors `query.New[T]()`/`query.NewUpdate[T]()` used internally by `repository`.
**Where**: `query/query.go` (new)
**Depends on**: T2 (`op.Condition`)
**Tests**: unit — `Where` accumulates conditions across multiple calls (append, not replace); `Update.Set` accumulates `SetClause`s; both work with zero conditions/sets (empty slice, not nil-panic)
**Gate**: quick

### T4: `golem.Dialect` — remove `FindByID`, add `Select`/`Update` [P]

**What**: Modify `dialect.go`: remove `FindByID` from the `Dialect` interface, add `Select(ctx, conn, table string, whereColumns []string, whereValues []driver.Value) ([]map[string]any, error)` and `Update(ctx, conn, table string, setColumns []string, setValues []driver.Value, whereColumns []string, whereValues []driver.Value) ([]map[string]any, error)`. Patch every fake implementing `golem.Dialect` (`dialect_test.go`'s `fakeDialect`, and any other in-package fakes across `conn_test.go`/`connector_test.go`/`data_source_test.go` that reference `Dialect`) to match — remove their `FindByID` stub, add `Select`/`Update` stubs. Patch `adapter/postgres/dialect.go`'s real (non-test) `dialect` type similarly: remove its real `FindByID` (and `buildFindByIDSQL`) — a LATER task (T5) adds the real `Select`/`Update` bodies; for THIS task, minimal placeholder bodies are fine (`return nil, fmt.Errorf("postgres: Select not yet implemented")` etc.) so the module compiles.
**Where**: `dialect.go`, `dialect_test.go`, and whichever other `_test.go` fakes reference `Dialect` (read them first), `adapter/postgres/dialect.go`
**Depends on**: None
**Tests**: unit — whole-repo gate must stay green after this change (existing tests, minus anything that specifically tested `FindByID`, which should be REMOVED, not left broken)
**Gate**: quick

### T5: `postgres.dialect` real `Select`/`Update` [P]

**What**: Replace T4's placeholder `Select`/`Update` bodies in `adapter/postgres/dialect.go` with real SQL generation + execution (see design.md's exact SQL shapes), using `d.pool.Query` + `pgx.CollectRows(rows, pgx.RowToMap)` (plural — 0+ rows, not `CollectOneRow`). Extract `buildSelectSQL`/`buildUpdateSQL` as pure, unit-testable helpers (same pattern as the existing `buildInsertSQL`).
**Where**: `adapter/postgres/dialect.go` (modify)
**Depends on**: T4
**Tests**: unit for the SQL-building helpers (pure, no DB) — real execution proven in T7's integration test
**Gate**: quick

### T6: `repository` — remove `FindByID`, add `FindMany`/`FindOne`/`SaveOne`/`UpdateOne`/`UpdateMany`

**What**: Per design.md's exact behavior. `FindMany`/`FindOne` build a real `*T` zero value, run the criteria callback with a `*query.Query[T]`, resolve each `op.Condition`'s `FieldPtr` via `entity.ResolveField` + `r.meta.Columns` lookup to a column name, call `Dialect.Select`. `SaveOne` builds WHERE from `r.meta.PrimaryKey` (reading current field values off the input `*T` — reuse the same field-value-reading approach `Insert` uses, but for ALL non-PK columns this time, including zero-valued ones — no `IsZero` skip here, that's an `Insert`-only rule). `UpdateOne`/`UpdateMany` similarly via `*query.Update[T]`.
**Where**: `repository/repository.go` (modify), `repository/repository_test.go` (modify: remove `FindByID` tests, add new ones)
**Depends on**: T1, T3, T4
**Tests**: unit — fake `Dialect` covering: `FindMany` with/without criteria, `FindOne` found/not-found, `SaveOne` single and composite PK, `UpdateOne` found/not-found, `UpdateMany` N rows and 0 rows (not an error)
**Gate**: quick

### T7: Update example + integration test

**What**: `examples/postgres-minimal-blog/main.go`: replace the removed `FindByID` call with `FindOne(ctx, func(t *User, q *query.Query[User]) { q.Where(op.Eq(&t.ID, user.ID)) })`. `main_integration_test.go`: replace/extend the existing `FindByID`-based assertions with `FindOne`/`FindMany` equivalents, and add real-Postgres coverage for `SaveOne` (e.g. update the user's `Name`, save, re-fetch, confirm) and `UpdateOne`/`UpdateMany` (e.g. update a post's `Title` by criteria).
**Where**: `examples/postgres-minimal-blog/main.go`, `examples/postgres-minimal-blog/main_integration_test.go`
**Depends on**: T5, T6
**Tests**: integration
**Gate**: full (`make test-integration`)
