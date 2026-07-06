# Query builder

Criteria are built via a declarative closure: the callback receives `t *T` (the same field pointer style as `Col`/`ForeignKey`/`PrimaryKey`) and a builder (`q *query.Query[T]`, `u *query.Update[T]`, or `c *query.Count[T]`, depending which `Repository[T]` method you're calling). Call order inside the callback doesn't matter — the query is only built once the callback returns.

```go
found, err := userRepo.FindOne(ctx, func(t *User, q *query.Query[User]) {
	q.Select(&t.Name, &t.Email, &t.Age)
	q.Where(
		op.Eq(&t.Name, "John Doe"),
		op.Eq(&t.Email, "john.doe@email.com"),
		op.Gte(&t.Age, 18),
	)
	q.OrderBy(op.Desc(&t.ID))
	q.Limit(10)
	q.Offset(0)
})
```

## `Where` and `op.*`

`Where(conditions ...op.Condition)` is variadic with **AND** semantics — every condition passed must match. Available conditions:

| Function | SQL equivalent |
| --- | --- |
| `op.Eq(fieldPtr, value)` | `column = value` |
| `op.Gt(fieldPtr, value)` | `column > value` |
| `op.Gte(fieldPtr, value)` | `column >= value` |
| `op.Lt(fieldPtr, value)` | `column < value` |
| `op.Lte(fieldPtr, value)` | `column <= value` |
| `op.In(fieldPtr, values...)` | `column IN (values...)` |
| `op.Like(fieldPtr, value)` | `column LIKE value` |
| `op.Or(conditions...)` | `(cond1 OR cond2 OR ...)` |
| `op.Not(condition)` | `NOT (condition)` |

There's no dedicated `Ne`/`NotIn` — compose negation instead: `op.Not(op.Eq(&t.Status, "banned"))`, `op.Not(op.In(&t.ID, 1, 2, 3))`.

`op.Or`/`op.Not` nest normally, since they're themselves `op.Condition`:

```go
q.Where(
	op.Eq(&t.Active, true),
	op.Or(
		op.Eq(&t.Role, "admin"),
		op.Gte(&t.Age, 18),
	),
)
// active = true AND (role = 'admin' OR age >= 18)
```

## Ordering

`OrderBy(orders ...op.Order)` — `op.Asc(fieldPtr)` / `op.Desc(fieldPtr)`:

```go
q.OrderBy(op.Desc(&t.CreatedAt), op.Asc(&t.Name))
```

## Pagination

```go
q.Limit(10)
q.Offset(20)
```

## Selecting specific columns

`Select(fieldPtrs ...any)` — without it, every column is returned. With it, only the listed fields are populated (the rest stay zero-valued on the returned struct).

```go
q.Select(&t.ID, &t.Name)
```

## Soft delete: `.WithDeleted()`

If the entity has `DeleteDate` declared (see [Declaring schemas](schema.md)), every query filters out deleted rows by default — an implicit `WHERE deleted_at IS NULL`. `.WithDeleted()` turns that filter off for that one query.

It exists on any builder with `Where` underneath: `query.Query[T]` (`FindMany`/`FindOne`), `query.Count[T]` (`Count`/`Exists`), `query.Update[T]` (`Update`), and `query.Join[T]` (inside `join.*` — see [Joins](joins.md)). On entities without `DeleteDate`, `.WithDeleted()` is a no-op.

```go
// without this, users with DeletedAt set wouldn't show up here
found, err := userRepo.FindOne(ctx, func(t *User, q *query.Query[User]) {
	q.Where(op.Eq(&t.ID, 42))
	q.WithDeleted()
})
```

## Update criteria

`query.Update[T]` combines `Where` (same `op.*` set above) with `Set(fieldPtr, value)`:

```go
updated, err := userRepo.Update(ctx, func(t *User, u *query.Update[User]) {
	u.Where(op.Eq(&t.ID, user.ID))
	u.Set(&t.Name, "John Doe")
	u.Set(&t.Age, 31)
})
```

See [Repository](repository.md) for the full CRUD surface, [Joins](joins.md) for combining queries across entities, and [Pessimistic locking](locking.md) for `.ForUpdate()` and friends.
