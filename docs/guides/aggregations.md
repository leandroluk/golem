# Aggregations

`repo.Aggregate[R](ctx, func(t *T, res *R, a *golem.Aggregate[T, R]) {...}) ([]R, error)` (and its panic-on-error counterpart `repo.MustAggregate[R]`) follows the same principle as [Preload](preload.md): `R` is any struct (doesn't need `golem.NewTable`), resolved by field pointer against `t` (source, `T`) and `res` (destination, `R`) — no tags.

```go
package main

import (
	"context"
	"fmt"

	"github.com/leandroluk/golem"
)

type CategoryTotal struct {
	Category string
	Total    float64
	Count    int64
}

func example(ctx context.Context, dataSource *golem.DataSource) error {
	postRepo := golem.NewRepository(dataSource, PostEntity)

	// total views (and post count) per category, only categories with more than 1 post
	totals, err := postRepo.Aggregate(ctx, func(p *Post, res *CategoryTotal, a *golem.Aggregate[Post, CategoryTotal]) {
		a.GroupBy(&p.Category, &res.Category)
		a.Sum(&p.Views, &res.Total)
		a.CountAll(&res.Count)
		a.Where(golem.Eq(&p.Published, true))
		a.Having(golem.Gt(&res.Count, int64(1)))
		a.OrderBy(golem.Desc(&res.Total))
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

## `golem.Aggregate[T, R]` reference

| Method                                  | Description                                                                                                                                  |
| --------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `GroupBy(sourceFieldPtr, destFieldPtr)` | groups by a source column, writes its value into the destination field                                                                       |
| `Sum(sourceFieldPtr, destFieldPtr)`     | `SUM(column)`, written to the destination field as `float64`                                                                                 |
| `Avg(sourceFieldPtr, destFieldPtr)`     | `AVG(column)`, written to the destination field as `float64`                                                                                 |
| `Count(sourceFieldPtr, destFieldPtr)`   | `COUNT(column)`                                                                                                                              |
| `CountAll(destFieldPtr)`                | `COUNT(*)` — no source column                                                                                                                |
| `Where(conditions ...golem.Condition)`  | filters **before** grouping, resolved against `T` — same `golem.*` set as [Query builder](query-builder.md)                                  |
| `Having(conditions ...golem.Condition)` | filters **after** grouping — each condition's field pointer must point to an `R` field already registered via `Sum`/`Avg`/`Count`/`CountAll` |
| `OrderBy(orders ...golem.Order)`        | accepts both `GroupBy` and aggregate fields                                                                                                  |
| `Limit(n)` / `Offset(n)`                | pagination over the grouped result                                                                                                           |
| `WithDeleted()`                         | includes soft-deleted rows — see [Query builder](query-builder.md#soft-delete-withdeleted)                                                   |

## Notes

- `Sum` and `Avg` wrap SQL projections with `CAST(... AS DOUBLE PRECISION)` so destination fields in `R` can always be typed `float64`.
- `Having` conditions can only reference fields already declared via `Sum`/`Avg`/`Count`/`CountAll`.
