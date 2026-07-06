# Aggregations

`repository.Aggregate[T, R](ctx, repo, func(t *T, res *R, a *query.Aggregate[T, R]) {...}) ([]R, error)` follows the same principle as [Preload](preload.md): `R` is any struct (doesn't need `entity.New`), resolved by field pointer against `t` (source, `T`) and `res` (destination, `R`) — no tags.

```go
package main

import (
	"context"
	"fmt"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
)

type CategoryTotal struct {
	Category string
	Total    float64
	Count    int64
}

func example(ctx context.Context, dataSource *golem.DataSource) error {
	postRepo := repository.Get(dataSource, PostEntity)

	// total views (and post count) per category, only categories with more than 1 post
	totals, err := repository.Aggregate(ctx, postRepo, func(p *Post, res *CategoryTotal, a *query.Aggregate[Post, CategoryTotal]) {
		a.GroupBy(&p.Category, &res.Category)
		a.Sum(&p.Views, &res.Total)
		a.CountAll(&res.Count)
		a.Where(op.Eq(&p.Published, true))
		a.Having(op.Gt(&res.Count, int64(1)))
		a.OrderBy(op.Desc(&res.Total))
	})
	if err != nil {
		return err
	}

	for _, t := range totals {
		fmt.Printf("%s: %d posts, %.0f views\n", t.Category, t.Count, t.Total)
	}
	return nil
}
```

## `query.Aggregate[T, R]` reference

| Method | Description |
| --- | --- |
| `GroupBy(sourceFieldPtr, destFieldPtr)` | groups by a source column, writes its value into the destination field |
| `Sum(sourceFieldPtr, destFieldPtr)` | `SUM(column)`, written to the destination field as `float64` |
| `Avg(sourceFieldPtr, destFieldPtr)` | `AVG(column)`, written to the destination field as `float64` |
| `Count(sourceFieldPtr, destFieldPtr)` | `COUNT(column)` |
| `CountAll(destFieldPtr)` | `COUNT(*)` — no source column |
| `Where(conditions ...op.Condition)` | filters **before** grouping, resolved against `T` — same `op.*` set as [Query builder](query-builder.md) |
| `Having(conditions ...op.Condition)` | filters **after** grouping — each condition's field pointer must point to an `R` field already registered via `Sum`/`Avg`/`Count`/`CountAll` |
| `OrderBy(orders ...op.Order)` | accepts both `GroupBy` and aggregate fields |
| `Limit(n)` / `Offset(n)` | pagination over the grouped result |
| `WithDeleted()` | includes soft-deleted rows — see [Query builder](query-builder.md#soft-delete-withdeleted) |

## Notes

- You can't `HAVING` over a plain `GroupBy` column in this version — `Having`'s field pointers must reference an aggregate destination field (`Sum`/`Avg`/`Count`/`CountAll`'s target), not a `GroupBy` target.
- `Sum`/`Avg` always come back as `float64` on the destination. On Postgres, the driver forces a `CAST(... AS DOUBLE PRECISION)` — without it, `SUM`/`AVG` over an integer column becomes `NUMERIC`, which doesn't decode directly as `float64`.
- `MIN`/`MAX` don't exist in this version, on purpose — they'd make sense on non-numeric columns (text, date) where the float cast above would break. Revisit only with per-column-type-aware casting.
