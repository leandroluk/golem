# Query builder

Criteria are built via a declarative closure: the callback receives `t *T` (the same field pointer style as `Col`/`ForeignKey`/`PrimaryKey`) and a builder (`q *golem.Query[T]`, `u *golem.Update[T]`, or `c *golem.Count[T]`, depending which `Repository[T]` method you're calling). Call order inside the callback doesn't matter — the query is only built once the callback returns.

```go
found, err := userRepo.FindOne(ctx, func(t *User, q *golem.Query[User]) {
	q.Select(&t.Name, &t.Email, &t.Age)
	q.Where(
		golem.Eq(&t.Name, "John Doe"),
		golem.Eq(&t.Email, "john.doe@email.com"),
		golem.Gte(&t.Age, 18),
	)
	q.OrderBy(golem.Desc(&t.ID))
	q.Limit(10)
	q.Offset(0)
})
```

## `Where` and `golem.*`

`Where(conditions ...golem.Condition)` is variadic with **AND** semantics — every condition passed must match. Available conditions:

| Function | SQL equivalent |
| --- | --- |
| `golem.Eq(fieldPtr, value)` | `column = value` |
| `golem.Gt(fieldPtr, value)` | `column > value` |
| `golem.Gte(fieldPtr, value)` | `column >= value` |
| `golem.Lt(fieldPtr, value)` | `column < value` |
| `golem.Lte(fieldPtr, value)` | `column <= value` |
| `golem.In(fieldPtr, values...)` | `column IN (values...)` |
| `golem.Like(fieldPtr, value)` | `column LIKE value` |
| `golem.Or(conditions...)` | `(cond1 OR cond2 OR ...)` |
| `golem.Not(condition)` | `NOT (condition)` |

There's no dedicated `Ne`/`NotIn` — compose negation instead: `golem.Not(golem.Eq(&t.Status, "banned"))`, `golem.Not(golem.In(&t.ID, 1, 2, 3))`.

`golem.Or`/`golem.Not` nest normally, since they're themselves `golem.Condition`:

```go
q.Where(
	golem.Eq(&t.Active, true),
	golem.Or(
		golem.Eq(&t.Role, "admin"),
		golem.Gte(&t.Age, 18),
	),
)
// active = true AND (role = 'admin' OR age >= 18)
```

## Ordering

`OrderBy(orders ...golem.Order)` — `golem.Asc(fieldPtr)` / `golem.Desc(fieldPtr)`:

```go
q.OrderBy(golem.Desc(&t.CreatedAt), golem.Asc(&t.Name))
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

It exists on any builder with `Where` underneath: `golem.Query[T]` (`FindMany`/`FindOne`), `golem.Count[T]` (`Count`/`Exists`), `golem.Update[T]` (`Update`), and `golem.Join[T]` (inside `join.*` — see [Joins](joins.md)). On entities without `DeleteDate`, `.WithDeleted()` is a no-op.

```go
// without this, users with DeletedAt set wouldn't show up here
found, err := userRepo.FindOne(ctx, func(t *User, q *golem.Query[User]) {
	q.Where(golem.Eq(&t.ID, 42))
	q.WithDeleted()
})
```

## Update criteria

`golem.Update[T]` combines `Where` (same `golem.*` set above) with `Set(fieldPtr, value)`:

```go
updated, err := userRepo.Update(ctx, func(t *User, u *golem.Update[User]) {
	u.Where(golem.Eq(&t.ID, user.ID))
	u.Set(&t.Name, "John Doe")
	u.Set(&t.Age, 31)
})
```

See [Repository](repository.md) for the full CRUD surface, [Joins](joins.md) for combining queries across entities, and [Pessimistic locking](locking.md) for `.ForUpdate()` and friends.
