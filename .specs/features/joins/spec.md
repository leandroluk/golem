# Spec — Joins (Milestone 6)

## Features

- Support dynamic SQL `JOIN` compilation in the Postgres dialect.
- Types of Joins supported:
  - `join.Inner`
  - `join.Left`
  - `join.Right`
  - `join.Full`
- Automatic prefixing of table names in compiled queries to prevent ambiguity (e.g. `"users"."id"` instead of `"id"`).
- Automatic injection of soft-delete conditions (`deleted_at IS NULL`) for joined tables unless `.WithDeleted()` is explicitly specified.

## API Example
```go
users, err := repo.FindMany(ctx, func(u *User, q0 *query.Query[User]) {
	join.Inner(q0, PostEntity, func(p *Post, q1 *query.Join[Post]) {
		q1.On(&p.OwnerUserID, &u.ID)
		q1.Where(op.Eq(&p.Title, "Hello, Golem!"))
	})
})
```
