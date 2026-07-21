# Joins

The `github.com/leandroluk/golem` package provides `golem.JoinInner`/`Left`/`Right`/`Full` — SQL's join names (`Left`/`Right`/`Full` are already "outer" by definition, so there's no separate generic `Outer`).

Each function takes the outer `*golem.Query[T]` (to register itself on it), the entity on the joining side (`J` inferred from it, same as `golem.NewRepository`), and a callback for the new side's builder. We name the parameters `t`/`q0` (outer), `j`/`q1` (join) to keep each level explicit.

```go
package main

import (
	"context"

	"github.com/leandroluk/golem"
)

func example(ctx context.Context, dataSource *golem.DataSource) ([]User, error) {
	// users who have at least 1 published post
	// (INNER: only rows matching the On condition make it in)
	return golem.NewRepository(dataSource, UserEntity).FindMany(ctx, func(u *User, q0 *golem.Query[User]) {
		golem.JoinInner(q0, PostEntity, func(p *Post, q1 *golem.Join[Post]) {
			q1.On(&p.OwnerUserID, &u.ID)
			q1.Where(golem.Eq(&p.Published, true))
		})
		q0.Where(golem.Eq(&u.Name, "John Doe"))
	})
}
```

## `On` vs `Where`

`q1.On(fieldPtr, fieldPtr)` compares **column to column** — both arguments are field addresses. This is different from `golem.Eq(fieldPtr, value)`, which compares a column to a literal value in `Where`; that's why joining uses a separate method instead of reusing `golem.Eq` for both cases.

`q1.Where(...)` filters the side that entered the join (with the normal `golem.*` set — see [Query builder](query-builder.md)) without mixing it into the outer query's `Where`.

## Join types

| Function | SQL |
| --- | --- |
| `golem.JoinInner(q0, Entity, fn)` | `INNER JOIN` |
| `golem.JoinLeft(q0, Entity, fn)` | `LEFT JOIN` |
| `golem.JoinRight(q0, Entity, fn)` | `RIGHT JOIN` |
| `golem.JoinFull(q0, Entity, fn)` | `FULL JOIN` |

Multiple joins can be registered on the same outer query — call `golem.JoinInner`/etc. more than once inside the same `FindMany`/`FindOne` closure.

## Soft delete inside a join

`golem.Join[T]` also has `.WithDeleted()`, same meaning as everywhere else — see [Query builder](query-builder.md#soft-delete-withdeleted).

See [Preload](preload.md) for loading related rows without a SQL join (as a separate, grouped query instead).
