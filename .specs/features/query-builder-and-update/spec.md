# Query Builder + Save/Update (scoped) Specification

**Continues:** `.specs/features/repository-core-crud/` (M3), pulls in the minimum of M4 (Query Builder) needed to replace `FindByID`.

## Scope decision (SPEC_DEVIATION from ROADMAP.md's full M4/M5)

Per user direction: drop `Repository[T].FindByID` entirely — `FindOne`/`FindMany` (with a `Where(op.Eq(&t.ID, id))` criteria) already cover that case, so keeping a separate PK-only method is redundant surface. Also scoped down from the full M4/M5 feature lists to exactly what closes basic CRUD for `postgres`-style usage:

**In scope this pass:**
- `op.Eq(fieldPtr, value) op.Condition` — the ONE comparison operator (others — `Gt`/`Gte`/`Lt`/`Lte`/`In`/`Like`/`Or`/`Not` — grow on demand later, per AD-017's own precedent)
- `query.Query[T].Where(conditions ...op.Condition)` — AND semantics only (no `Select`/`OrderBy`/`Limit`/`Offset` yet)
- `query.Update[T].Where(...)` + `.Set(fieldPtr, value)`
- `Repository[T].FindMany(ctx, criteria ...func(t *T, q *query.Query[T])) ([]T, error)`
- `Repository[T].FindOne(ctx, criteria ...func(t *T, q *query.Query[T])) (T, error)` — `golem.ErrNotFound` if no match
- `Repository[T].SaveOne(ctx, *T) (T, error)` — re-persists an existing instance by its current PK value(s); works for composite PK (unlike the old `FindByID`, which only worked single-column — this is a strict improvement)
- `Repository[T].UpdateOne(ctx, func(t *T, u *query.Update[T])) (T, error)` / `UpdateMany(ctx, ...) ([]T, error)`

**Removed this pass:** `Repository[T].FindByID`, `golem.Dialect.FindByID`. Superseded by `FindOne`.

**Deferred (tracked in STATE.md Todos):** `Delete`/`Restore` (needs `entity.Table.DeleteDate`, not built yet — separate prerequisite), `Count`/`Exists`, `SaveMany`, `op.Gt/Gte/Lt/Lte/In/Like/Or/Not`, `query.Query[T].Select/OrderBy/Limit/Offset`, `.WithDeleted()` (no soft-delete exists yet).

## Acceptance Criteria

1. WHEN `repo.FindOne(ctx, func(t *T, q *query.Query[T]) { q.Where(op.Eq(&t.ID, 5)) })` matches exactly one row THEN it SHALL return that row scanned into `T`.
2. WHEN no row matches a `FindOne` criteria THEN it SHALL return `golem.ErrNotFound`.
3. WHEN `repo.FindMany(ctx, func(t *T, q *query.Query[T]) { q.Where(op.Eq(&t.OwnerUserID, 1)) })` matches N rows THEN it SHALL return all N, scanned.
4. WHEN `FindMany`/`FindOne` is called with NO criteria THEN it SHALL return all rows in the table (no `WHERE` clause).
5. WHEN multiple `op.Eq` conditions are passed to one `Where(...)` call THEN they SHALL be ANDed together.
6. WHEN `repo.SaveOne(ctx, i)` is called with `i`'s PK field(s) already populated (single or composite) THEN it SHALL `UPDATE` every non-PK column by that PK, returning the updated row.
7. WHEN `repo.UpdateOne(ctx, func(t *T, u *query.Update[T]) { u.Where(op.Eq(&t.ID, 5)); u.Set(&t.Name, "x") })` matches exactly one row THEN it SHALL update it and return the result; if zero rows match, return `golem.ErrNotFound`.
8. WHEN `repo.UpdateMany(...)` matches N rows THEN it SHALL update and return all N.
9. WHEN any of the above receives a driver/SQL error THEN it SHALL wrap and return it, never panic.

## Success Criteria

- [ ] `.examples/postgres/main.go` uses `FindOne` instead of the removed `FindByID`, still round-trips correctly
- [ ] `go test ./op/... ./query/... ./repository/...` pass (unit, fakes)
- [ ] Integration test (real Postgres) covers `FindMany`/`FindOne`/`SaveOne`/`UpdateOne`/`UpdateMany`
