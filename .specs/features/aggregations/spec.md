# Aggregations (M13) Specification

## Status: ✅ DONE

---

## Goals

- [x] `repository.Aggregate[T, R any](ctx, r *Repository[T], fn func(t *T, res *R, a *query.Aggregate[T, R])) ([]R, error)` — free function (same generics constraint as `Preload`/`join.Inner`: a method can't add a new type parameter), R is a plain struct (not an `entity.Entity`), resolved by field-pointer offset like everywhere else
- [x] `query.Aggregate[T, R]`: `GroupBy(sourceFieldPtr, destFieldPtr)`, `Sum`/`Avg`/`Count(sourceFieldPtr, destFieldPtr)`, `CountAll(destFieldPtr)`, `Where(...op.Condition)` (pre-aggregation, resolved against `T`), `Having(...op.Condition)` (post-aggregation, resolved against `R`, must reference a previously-registered aggregate field), `OrderBy(...op.Order)` (resolved against `R`, either a GroupBy or aggregate field), `Limit`/`Offset`/`WithDeleted`
- [x] `Sum`/`Avg` results are always `float64` — the Postgres dialect casts them (`CAST(... AS DOUBLE PRECISION)`) so pgx never returns `pgtype.Numeric` for an integer column's SUM/AVG
- [x] `Min`/`Max` are explicitly NOT included in this pass (see design.md) — not an oversight, a scoping decision (ROADMAP.md's M13 goal only lists GroupBy/Sum/Avg/Having)

## Acceptance Criteria

1. WHEN `a.GroupBy(&t.Category, &res.Category)` + `a.Sum(&t.Amount, &res.Total)` are declared THEN the compiled SQL SHALL be `SELECT "category" AS "agg_0", CAST(SUM("amount") AS DOUBLE PRECISION) AS "agg_1" FROM ... GROUP BY "category"`, and each result row SHALL scan into `R.Category`/`R.Total`.
2. WHEN `a.CountAll(&res.Count)` is declared THEN the projection SHALL be `COUNT(*)`, with no source column.
3. WHEN `a.Having(op.Gt(&res.Total, 100.0))` is declared, where `res.Total` was registered via `Sum` THEN the compiled `HAVING` clause SHALL repeat the aggregate expression (`HAVING CAST(SUM("amount") AS DOUBLE PRECISION)>$N`), never the SQL alias (Postgres doesn't allow referencing SELECT-list aliases in `HAVING`, unlike `ORDER BY`).
4. WHEN a `Having`/`OrderBy` condition's `FieldPtr` resolves to an `R` field never registered via `GroupBy`/`Sum`/`Avg`/`Count`/`CountAll` THEN `Aggregate` SHALL return an error (for `Having` specifically — `OrderBy` accepts either GroupBy or aggregate fields).
5. WHEN `a.Where(...)` is declared THEN it SHALL filter rows before grouping (resolved against `T`, identical semantics to `FindMany`'s `Where`), combined with the entity's default soft-delete filter unless `.WithDeleted()` is called.
6. WHEN a source or destination field pointer doesn't belong to `T`'s (or `R`'s) zero value THEN `Aggregate` SHALL return an error (not panic) — consistent with how `Preload`/`FindMany` surface field-resolution failures.
7. WHEN a result row is missing one of the registered aliases THEN the corresponding `R` field SHALL be left at its zero value, not treated as an error.

## Success Criteria

- [x] `go test ./query/... ./repository/... ./driver/postgres/...` — green, every new function at 100% statement coverage
- [x] `go build ./...` — no errors
- [x] `.examples/postgres`: `TestBlogExample_Aggregate_PostCountPerUser` (real Postgres, `task test-integration`) verifies `GroupBy`+`CountAll`+`Where`+`Having` end to end
