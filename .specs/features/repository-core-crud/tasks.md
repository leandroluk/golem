# Schema Declaration + Repository Core CRUD Tasks

Covers both `.specs/features/schema-declaration/` and `.specs/features/repository-core-crud/` (delivered together, driven by `examples/postgres-minimal-blog`).

**Design**: `schema-declaration/design.md`, `repository-core-crud/design.md`
**Status**: Done — all 9 tasks complete, `make gate-full` passes (unit + real dockerized Postgres integration, including `examples/postgres-minimal-blog`)

**SPEC_DEVIATION found during T9**: `Repository[T].Insert` originally sent every declared column, including a zero-valued PK, which collided with `BIGSERIAL` defaults (caught via the real-Postgres integration test). Fixed in `repository/repository.go`: zero-valued fields are now omitted from the INSERT so DB-side defaults apply. See commit `fix(repository): omit zero-valued fields from Insert`.

---

## Execution Plan

```
Phase 1 (parallel):  T1 (ColumnType ctors) [P]   T2 (column.Builder) [P]   T7 (schema.sql infra) [P]
Phase 2 (sequential): T3 (entity package)  -- depends on T1, T2
Phase 3 (parallel):  T4 (Conn/Dialect/ErrNotFound core growth) [P]   T8 (example entities) [P] -- T4 depends on nothing new, T8 depends on T3+T1
Phase 4 (sequential): T5 (postgres dialect Insert/FindByID) -- depends on T4
Phase 5 (sequential): T6 (repository package) -- depends on T3, T4
Phase 6 (sequential): T9 (example main.go + integration test) -- depends on T5, T6, T7, T8
```

---

## Task Breakdown

### T1: `golem.BIGINT()`/`VARCHAR(n)`/`TEXT()` constructors [P] — ✅ Complete

**What**: Real `ColumnType` constructors. Add an unexported `length int` field to the existing `ColumnType` struct (`columntype.go`, additive only) and a new file with the three constructors.
**Where**: `columntype.go` (modify: add `length int` field only), `columntype_constructors.go` (new)
**Depends on**: None
**Tests**: unit (construct each, assert internal `kind`/`length` via in-package test)
**Gate**: quick

### T2: `column.Builder` [P] — ✅ Complete

**What**: New `column` package, `Builder` struct with `.Name(name string) *Builder`.
**Where**: `column/builder.go`
**Depends on**: None
**Tests**: unit
**Gate**: quick

### T3: `entity` package — field-pointer resolution + `Entity[T]`/`Builder`/`New`/`Describe`  — ✅ Complete

**What**: Full `entity` package per `schema-declaration/design.md`: `resolveField` (offset-matching), `Entity[T]`, `Builder` (`TableName`/`SchemaName`/`PrimaryKey`/`Col`/`ForeignKey`), `New[T]`, `EntityMeta`/`ColumnMeta`, `Describe()`.
**Where**: `entity/entity.go`, `entity/builder.go`
**Depends on**: T1 (`golem.ColumnType`), T2 (`column.Builder`)
**Tests**: unit — MUST include the "two same-Go-type fields resolved correctly by identity, not type/order" case from spec.md AC-2, plus composite `PrimaryKey`, `ForeignKey` metadata, and table/column name defaulting rules
**Gate**: quick

### T4: Core package growth — `Conn.Dialect()`, `Dialect.Insert`/`FindByID`, `golem.ErrNotFound` [P]  — ✅ Complete

**What**: Modify `conn.go` (add `Dialect() Dialect` to the `Conn` interface), `datasource.go` (implement it), `dialect.go` (add `Insert`/`FindByID` to the `Dialect` interface), new `errors.go` (`ErrNotFound`).
**Where**: `conn.go`, `datasource.go`, `dialect.go`, `errors.go` (new)
**Depends on**: None (independent of entity/column work)
**Tests**: unit — existing `datasource_test.go`/`dialect_test.go` fakes need updating to satisfy the grown interfaces (their fakes must gain the new methods or the package won't compile) — this is EXPECTED and must be done in this task, not treated as breakage
**Gate**: quick

### T5: `postgres.dialect` gains pool + real `Insert`/`FindByID`  — ✅ Complete

**What**: `connector.Connect()` returns `&dialect{pool: pool}` instead of the stateless stub; `dialect.Insert`/`dialect.FindByID` build parameterized SQL (double-quoted identifiers) and execute via `pgx.CollectOneRow(rows, pgx.RowToMap)`.
**Where**: `adapter/postgres/dialect.go` (modify), `adapter/postgres/connector.go` (modify)
**Depends on**: T4
**Tests**: unit for SQL-string-building helpers (pure functions, no DB); real execution is proven end-to-end in T9's integration test, not re-tested here in isolation
**Gate**: quick

### T6: `repository` package — `Get`/`Insert`/`InsertMany`/`FindByID`  — ✅ Complete

**What**: Full `repository` package per `repository-core-crud/design.md`, using `reflect` to move values between `*T` and `map[string]any`.
**Where**: `repository/repository.go`
**Depends on**: T3, T4
**Tests**: unit — fake `Conn`/`Dialect` (in-package or via a test-only fake implementing the grown interfaces), covering: `Insert` populates PK+all columns from the fake's returned row, `InsertMany` preserves order, `FindByID` returns `golem.ErrNotFound` when the fake reports not-found, `FindByID` on a composite-PK entity returns a descriptive (non-`ErrNotFound`) error
**Gate**: quick

### T7: `testdata/schema.sql` + `docker-compose.test.yml` mount [P]  — ✅ Complete

**What**: DDL for `users`/`post`/`category`/`post_to_category` (real FK constraints), mounted into the existing test Postgres service via `docker-entrypoint-initdb.d`.
**Where**: `testdata/schema.sql` (new), `docker-compose.test.yml` (modify: add volume mount)
**Depends on**: None
**Tests**: none (infra) — validated implicitly when T9's integration test runs against it
**Gate**: build (`docker compose -f docker-compose.test.yml config` validates)

### T8: `examples/postgres-minimal-blog` entity declarations [P]  — ✅ Complete

**What**: `User`/`Post`/`Category`/`PostToCategory` structs + their `entity.New[...]` declarations, matching `testdata/schema.sql`'s table/column names exactly (explicit `.TableName("users")`/`.TableName("post_to_category")`/`.Name("owner_user_id")` etc. where defaults wouldn't match).
**Where**: `examples/postgres-minimal-blog/entities.go`
**Depends on**: T3 (entity package), T1 (ColumnType)
**Tests**: unit — `go build` succeeding on this file IS the test (declarations either compile and resolve correctly or don't); one light test per entity asserting `Describe()` returns the expected table/column names is worth adding since it directly proves the naming-match with `schema.sql`
**Gate**: quick

### T9: `examples/postgres-minimal-blog/main.go` + integration test  — ✅ Complete

**What**: A runnable `main.go` demonstrating: insert a user, insert 2 posts owned by that user, insert 2 categories, insert `post_to_category` junction rows linking them, `FindByID` the user back to prove the round-trip. An integration test (`//go:build integration`) running the same flow programmatically (not just eyeballing `main.go` output) against the dockerized Postgres from T7's schema.
**Where**: `examples/postgres-minimal-blog/main.go`, `examples/postgres-minimal-blog/main_integration_test.go`
**Depends on**: T5, T6, T7, T8
**Tests**: integration
**Gate**: full (`make test-integration`)

---

## Parallel Execution Map

```
Phase 1: T1 [P], T2 [P], T7 [P]
Phase 2: T3 (needs T1, T2)
Phase 3: T4 [P] (independent), T8 [P] (needs T3, T1)
Phase 4: T5 (needs T4)
Phase 5: T6 (needs T3, T4)
Phase 6: T9 (needs T5, T6, T7, T8)
```
