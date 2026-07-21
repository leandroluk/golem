# Repository (CRUD)

There's no generic `Manager` — only `Repository[T]` exists, always bound to a single entity (equivalent to TypeORM's `dataSource.getRepository(User)`, without a parallel `dataSource.manager`). Transactions are the `DataSource`'s responsibility, not the repository's.

```go
repo := golem.NewRepository(dataSource, UserEntity)
```

`golem.NewRepository[T any](conn golem.Conn, e *golem.Entity[T]) *Repository[T]` takes a `golem.Conn` — an interface implemented by both `*golem.DataSource` and `golem.Tx`. Outside a transaction, pass the `dataSource`; inside one, pass the `tx`. `T` is inferred from `UserEntity`'s type (`*golem.Entity[User]`), no need to write `golem.NewRepository[User](...)`.

## Insert

```go
user, err := repo.Insert(ctx, &User{Name: "John Doe", Email: "john.doe@email.com", Age: 30})
if err != nil {
	panic(err)
}
// user.ID is now filled in
```

`InsertMany` does the same for several entities in one call:

```go
users, err := repo.InsertMany(ctx,
	&User{Name: "Alice", Email: "alice@email.com", Age: 28},
	&User{Name: "Bob", Email: "bob@email.com", Age: 25},
)
```

## Save

`SaveOne` re-persists a runtime instance you already have (from a previous `Insert`/`FindOne`), by primary key:

```go
user.Age = 31
user, err = repo.SaveOne(ctx, &user)
```

`SaveMany` does the same for several instances at once.

## Update

No runtime instance needed — `Update` takes `Where` + `Set` criteria and updates directly in the database:

```go
updated, err := repo.Update(ctx, func(t *User, u *golem.Update[User]) {
	u.Where(golem.Eq(&t.ID, user.ID))
	u.Set(&t.Name, "John Doe")
	u.Set(&t.Age, 31)
})
```

There's no separate `UpdateOne`/`UpdateMany` — the criteria can match one row or several, the method is the same, and matching zero rows is not an error.

## Find

```go
// lookup by primary key: FindOne + golem.Eq (no dedicated FindByID)
found, err := repo.FindOne(ctx, func(t *User, q *golem.Query[User]) {
	q.Where(golem.Eq(&t.ID, user.ID))
})

// FindMany — no criteria at all brings back the whole table
allUsers, err := repo.FindMany(ctx)

// FindMany with criteria — see Query builder for the full Where/OrderBy/Limit/Offset surface
admins, err := repo.FindMany(ctx, func(t *User, q *golem.Query[User]) {
	q.Where(golem.Eq(&t.Name, "John Doe"))
})
```

## Count / Exists

Both take their own criteria type (`golem.Count[T]`, `Where` only) — with no argument at all, they count/check the whole table:

```go
total, err := repo.Count(ctx)

adultCount, err := repo.Count(ctx, func(t *User, c *golem.Count[User]) {
	c.Where(golem.Gte(&t.Age, 18))
})

exists, err := repo.Exists(ctx, func(t *User, c *golem.Count[User]) {
	c.Where(golem.Eq(&t.Email, "john.doe@email.com"))
})
```

`Exists` is a shortcut for "count > 0" without fetching a row.

## Delete / Restore

```go
// if User has DeleteDate declared, this is a soft delete (sets it) — otherwise the row is removed
if err := repo.Delete(ctx, &found); err != nil {
	panic(err)
}

// undoes a soft delete: clears the DeleteDate field again
if err := repo.Restore(ctx, &found); err != nil {
	panic(err)
}
```

Both are variadic — pass several entities to delete/restore them all by their respective primary keys.

## Transactions

Transactions live on `DataSource`, not on `Repository`. Inside the callback, `tx` (which also implements `golem.Conn`) replaces `dataSource` when building `golem.NewRepository`:

```go
err = dataSource.Transaction(ctx, func(tx golem.Tx) error {
	if _, err := golem.NewRepository(tx, PostEntity).Insert(ctx, &Post{OwnerUserID: user.ID, Title: "first post"}); err != nil {
		return err
	}
	_, err := golem.NewRepository(tx, MessageEntity).Insert(ctx, &Message{SenderUserID: user.ID, Content: "first message"})
	return err
})
```

If the callback returns an error, the whole transaction is rolled back.

## Quick reference

| Method | Returns | Description |
| --- | --- | --- |
| `Insert(ctx, e *T) (T, error)` | 1 row | inserts 1 new entity, returned with its PK filled in |
| `InsertMany(ctx, entities ...*T) ([]T, error)` | N rows | inserts several at once |
| `SaveOne(ctx, e *T) (T, error)` | 1 row | re-persists a runtime instance you already have, by PK |
| `SaveMany(ctx, entities ...*T) ([]T, error)` | N rows | like `SaveOne`, for several instances at once |
| `Update(ctx, criteria func(t *T, u *golem.Update[T])) ([]T, error)` | N rows | updates directly in the database by criteria; 0 matched rows is not an error |
| `Delete(ctx, entities ...*T) error` | — | delete by PK; soft-deletes instead of removing the row when `DeleteDate` is declared |
| `Restore(ctx, entities ...*T) error` | — | undoes a soft delete by PK; a no-op if `DeleteDate` isn't declared |
| `FindMany(ctx, criteria ...func(t *T, q *golem.Query[T])) ([]T, error)` | N rows | optional criteria; no criteria returns the whole table |
| `FindOne(ctx, criteria ...func(t *T, q *golem.Query[T])) (T, error)` | 1 row | same as `FindMany`, capped to 1 |
| `Count(ctx, criteria ...func(t *T, c *golem.Count[T])) (int64, error)` | count | optional criteria (`Where` only) |
| `Exists(ctx, criteria ...func(t *T, c *golem.Count[T])) (bool, error)` | bool | shortcut for `Count > 0` |

See [Query builder](query-builder.md) for the full `Where`/`golem.*`/`OrderBy`/pagination surface used by `FindMany`, `FindOne`, `Update`, and `Count`.
