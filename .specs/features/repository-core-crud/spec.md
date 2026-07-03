# Repository Core CRUD (M3, scoped) Specification

**Driving use case:** `examples/postgres-minimal-blog` — insert users/posts/categories/junction rows, read a few back to prove the relations round-trip through real Postgres.

## Scope decision (SPEC_DEVIATION from ROADMAP.md's full M3 list)

Same rationale as `schema-declaration/spec.md`: scoped to exactly what the driving example needs.

**Deferred (tracked in STATE.md Todos):** `SaveOne`/`SaveMany`, `UpdateOne`/`UpdateMany`, `Delete`/`Restore` (no `DeleteDate` exists on any example entity in this pass), `Count`/`Exists`, `FindMany`/`FindOne` (query builder, M4 territory), composite-PK `FindByID` (the only composite-PK entity in the example, `PostToCategory`, is only ever inserted, never fetched by key).

## Goals

- [ ] `repository.Get[T](conn golem.Conn, e *entity.Entity[T]) *Repository[T]`
- [ ] `Repository[T].Insert(ctx, *T) (T, error)` — single-row insert, returns the row with PK (and any DB-generated values) populated back, via Postgres `RETURNING *`
- [ ] `Repository[T].InsertMany(ctx, ...*T) ([]T, error)` — same, N rows (simplest correct implementation: N sequential `Insert` calls in this pass — no batched multi-row `INSERT` optimization yet)
- [ ] `Repository[T].FindByID(ctx, id any) (T, error)` — single-column PK only in this pass; fetches and scans one row

## Out of Scope (this pass)

| Feature | Reason |
| --- | --- |
| `SaveOne`/`SaveMany` | Example never re-persists an in-memory instance after insert |
| `UpdateOne`/`UpdateMany` | No update use case in the example |
| `Delete`/`Restore` | No `DeleteDate` on any example entity |
| `Count`/`Exists` | Not exercised |
| `FindMany`/`FindOne` (query builder) | M4 — needs `op`/`query` packages not built yet |
| Composite-PK `FindByID` | `PostToCategory` (the only composite-PK entity) is insert-only in the example |
| Batched multi-row `INSERT` | `InsertMany` as N×`Insert` is correct and sufficient; optimize only if a real perf need appears |

## Acceptance Criteria

1. WHEN `repository.Get(dataSource, UserEntity).Insert(ctx, &User{Name: "...", Email: "..."})` is called against a connected `DataSource` THEN system SHALL execute an `INSERT ... RETURNING *`, bind each non-PK field via `Dialect.Bind`, and return a `User` with `ID` (and every other column) populated from the returned row via `Dialect.Scan`.
2. WHEN `Insert` is called for `Post` (which has a `ForeignKey` column `OwnerUserID`) THEN system SHALL bind `OwnerUserID` as a plain column value (the FK's target is metadata for future use — e.g. cascade, M2 continuation — not required for the INSERT itself to work).
3. WHEN `InsertMany(ctx, a, b)` is called THEN system SHALL insert both rows (via sequential `Insert` calls) and return both populated results in the same order as given.
4. WHEN `FindByID(ctx, id)` is called for an entity with a single-column PK THEN system SHALL execute a `SELECT * WHERE <pk column> = $1`, scan the one row via `Dialect.Scan`, and return it.
5. WHEN `FindByID` finds no matching row THEN system SHALL return `golem.ErrNotFound` (introducing this one sentinel now, ahead of full M10, since `FindByID` cannot report "not found" any other way that doesn't string-match).
6. WHEN any of the above operations receives a driver/SQL error THEN system SHALL return it wrapped with context (never panic).

## Success Criteria

- [ ] `examples/postgres-minimal-blog/main.go` runs against a real (dockerized) Postgres: creates a user, two posts (one per... at least one) owned by that user, two categories, and junction rows linking posts to categories; reads at least one entity back via `FindByID` to prove the round-trip
- [ ] `go test ./repository/...` unit tests pass (fake `Conn`/`Dialect`)
- [ ] An integration test (behind the `integration` build tag, reusing `docker-compose.test.yml`) runs the same flow against real Postgres


