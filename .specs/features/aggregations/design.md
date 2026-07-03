# Aggregations (M13) Design

**Spec**: `.specs/features/aggregations/spec.md`
**Status**: Approved

---

## Why a separate result struct `R`, resolved the same way as everywhere else

A GROUP BY/aggregate query's result shape (grouped columns + aggregate values) never matches `T`'s own column set — `T` doesn't have a `TotalAmount float64` field just because someone wants to sum `Amount` grouped by `Category`. Rather than returning loosely-typed `[]map[string]any` (breaks the project's type-safety pitch) or requiring `R` to be declared via `entity.New` (heavy ceremony for a one-off read shape), `Aggregate` reuses the exact mechanism every other builder in this codebase already uses: field-pointer resolution by memory offset (`entity.ResolveField`/`resolveFieldPtrAny`), just applied to TWO zero values instead of one — `T`'s zero (the source) and `R`'s zero (the destination). Neither needs struct tags; `R` doesn't even need to be a "real" entity.

```go
type CategoryTotal struct { Category string; Total float64; Count int64 }

results, err := repository.Aggregate(ctx, postRepo, func(p *Post, res *CategoryTotal, a *query.Aggregate[Post, CategoryTotal]) {
    a.GroupBy(&p.Category, &res.Category)
    a.Sum(&p.Views, &res.Total)
    a.CountAll(&res.Count)
})
```

`Aggregate` is a free function, not a `Repository[T]` method, for the same reason `Preload`/`join.Inner` are: a method can't introduce a new type parameter (`R`) beyond its receiver's (`Repository[T]`).

---

## Why `Sum`/`Avg` force a `DOUBLE PRECISION` cast

Postgres promotes `SUM`/`AVG` of an integer column (`bigint`, `integer`, ...) to `NUMERIC` — pgx's default type registration decodes `NUMERIC` as `pgtype.Numeric`, not a native Go `float64`. Since `assignFieldValue` (the same reflect-based converter `scanRow` uses) only converts between *convertible* Go types, a `pgtype.Numeric` landing in a `float64` destination field would fail with a "not convertible" error — a correctness trap that would only surface at runtime, on specific column types.

Fix: every `SUM`/`AVG`/`COUNT` projection in `driver/postgres/dialect.go`'s `aggregateSQLFunc` is wrapped as `CAST(SUM(...) AS DOUBLE PRECISION)` (`COUNT`/`COUNT(*)` already return `bigint`, decoded natively as Go `int64`, no cast needed). This makes `R`'s `Sum`/`Avg` destination fields always expect `float64` (or anything reflect-convertible from it, e.g. `float32`) — documented in README.md and this design doc, not left as a surprise.

---

## Why `Min`/`Max` aren't included

Considered and dropped for this pass: `MIN`/`MAX` are valid over non-numeric columns too (text, timestamps, UUIDs, ...), where the blanket `DOUBLE PRECISION` cast used for `SUM`/`AVG` would be actively wrong (you can't cast a `MIN(varchar)` to a float). Supporting them correctly needs per-column-type-aware casting (or none at all, trusting pgx's native type mapping per column type) — real, but separate scope from what ROADMAP.md's M13 goal actually asked for (`GroupBy`/`Sum`/`Avg`/`Having`). Left out rather than shipped half-correct; revisit as a follow-up if a real need comes up (see Todos in STATE.md).

---

## `Having` must repeat the aggregate expression, not reference the alias

Unlike `ORDER BY` (where Postgres *does* allow referencing a `SELECT`-list alias), `HAVING` in Postgres (and standard SQL generally) must reference the actual aggregate expression again — `HAVING SUM(amount) > 100`, never `HAVING total > 100` even if `total` is the `SELECT`-list alias. `stmt.AggregateComparison{Func, Column, Op, Value}` (a new `stmt.Predicate` variant, alongside the existing `stmt.Comparison`) carries `Func`+`Column` so the dialect can rebuild the full expression (`CAST(SUM("amount") AS DOUBLE PRECISION)`) inside the `HAVING` clause, independent of whatever alias that same expression got in the `SELECT` list. `Aggregate` translates each `Having` condition's `R`-side `FieldPtr` back to the `T`-side `Func`+`Column` it was registered under via a small `resultFieldName -> aggProjection` map built while resolving `Sum`/`Avg`/`Count`/`CountAll` — this is also why `Having` can only reference already-registered aggregate fields, not arbitrary `R` fields (there'd be no `Func`+`Column` to rebuild the expression from).
