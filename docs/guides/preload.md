# Preload / eager loading

`repository.Preload[T, J](ctx, repo, items, targetEntity, criteria...)` fetches the rows related to `items` (the result of a previous `FindMany`/`FindOne`) and returns a `map[any][]J` grouped by the relation key. It **never** attaches the result back onto `items` — there's no field like `user.Posts []Post` on the struct. Entities stay plain structs, with no navigational field.

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

func example(ctx context.Context, dataSource *golem.DataSource) error {
	userRepo := repository.Get(dataSource, UserEntity)

	users, err := userRepo.FindMany(ctx, func(u *User, q *query.Query[User]) {
		q.Where(op.Eq(&u.Name, "John Doe"))
	})
	if err != nil {
		return err
	}

	// fetches the posts for each user returned above, grouped by User.ID
	postsByUserID, err := repository.Preload(ctx, userRepo, users, PostEntity, func(p *Post, q *query.Query[Post]) {
		q.Where(op.Eq(&p.Published, true))
		q.OrderBy(op.Desc(&p.ID))
	})
	if err != nil {
		return err
	}

	for _, u := range users {
		fmt.Printf("%s has %d published posts\n", u.Name, len(postsByUserID[u.ID]))
	}
	return nil
}
```

## How the join column is found

The join column is discovered automatically from the `ForeignKey` already declared between the two entities (see [Declaring schemas](schema.md#foreign-keys)) — it works in both directions:

- `Preload(ctx, userRepo, users, PostEntity)` — loads each user's posts (FK on `Post.OwnerUserID → User.ID`)
- `Preload(ctx, postRepo, posts, UserEntity)` — loads each post's owner (same FK, opposite direction)

## Criteria

`criteria` accepts the same `func(j *J, q *query.Query[J])` shape as `FindMany` — `Where`/`OrderBy`/`Limit`/`Offset`/`WithDeleted`, all from [Query builder](query-builder.md) — always combined (AND) with the join filter `Preload` builds on its own.

## Why there's no `Eager(true)` flag

Some ORMs let you mark a relation `eager: true` so it's automatically loaded inside `FindMany`/`FindOne`. Golem doesn't, on purpose: there's no way to return data of a different type (`J` varies per foreign key) hidden behind the fixed `([]T, error)` signature without heavy reflection or breaking the API. Always call `repository.Preload` explicitly, right after the `FindMany`/`FindOne` call whose results you want to enrich.
