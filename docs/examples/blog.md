# Example — Blog

A full walkthrough of a small blog domain (`User`, `Post`, `Category`, and the `PostToCategory` many-to-many junction), exercising every feature covered in the guides. The same code runs unchanged against any of the [5 supported adapters](../guides/adapters.md) — only the imported `driver/*` package and its connection options change.

The full runnable versions of this example (one per adapter, each with its own migration and integration tests) live under `.examples/{postgres,mysql,mssql,sqlite,oracle}` in the repository.

## Entities

```go
package main

import (
	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
	"github.com/leandroluk/golem/relation"
)

type User struct {
	ID    int64
	Name  string
	Email string
}

type Post struct {
	ID          int64
	OwnerUserID int64
	Title       string
	Content     string
}

type Category struct {
	ID   int64
	Name string
}

// PostToCategory is the many-to-many junction entity between Post and
// Category — a plain entity with two foreign keys, no dedicated relation
// type. See guides/relations.md.
type PostToCategory struct {
	PostID     int64
	CategoryID int64
}

var UserEntity = entity.New[User](func(t *User, b *entity.Table) {
	b.TableName("users")
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Name, golem.VARCHAR(50))
	b.Col(&t.Email, golem.VARCHAR(50))
	b.PrimaryKey(&t.ID)
})

var PostEntity = entity.New[Post](func(t *Post, b *entity.Table) {
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.OwnerUserID, golem.BIGINT()).Name("owner_user_id")
	b.Col(&t.Title, golem.VARCHAR(50))
	b.Col(&t.Content, golem.TEXT())
	b.PrimaryKey(&t.ID)
	// deleting a User cascades into deleting their Posts
	b.ForeignKey(&t.OwnerUserID, UserEntity, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))
})

var CategoryEntity = entity.New[Category](func(t *Category, b *entity.Table) {
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Name, golem.VARCHAR(50))
	b.PrimaryKey(&t.ID)
})

var PostToCategoryEntity = entity.New[PostToCategory](func(t *PostToCategory, b *entity.Table) {
	b.TableName("post_to_category")
	b.Col(&t.PostID, golem.BIGINT()).Name("post_id")
	b.Col(&t.CategoryID, golem.BIGINT()).Name("category_id")
	b.PrimaryKey(&t.PostID, &t.CategoryID)
	b.ForeignKey(&t.PostID, PostEntity)
	b.ForeignKey(&t.CategoryID, CategoryEntity)
})
```

## Connecting

```go
import (
	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/driver/postgres"
)

dataSource, err := golem.NewDataSource(
	postgres.New(func(o *postgres.Options) {
		o.DSN = "postgres://golem:golem@localhost:5432/golem_test?sslmode=disable"
	}),
)
if err != nil {
	panic(err)
}
defer dataSource.Close()

if err := dataSource.Connect(); err != nil {
	panic(err)
}
```

Swap to any other adapter by changing the import and `Options`:

```go
import "github.com/leandroluk/golem/driver/mysql"

mysql.New(func(o *mysql.Options) { o.DSN = "golem:golem@tcp(localhost:3306)/golem_test?parseTime=true" })
```

```go
import "github.com/leandroluk/golem/driver/sqlite"

sqlite.New(func(o *sqlite.Options) { o.Path = ":memory:" })
```

See [Connecting](../guides/connecting.md) for every adapter's options.

## Insert

```go
ctx := context.Background()
userRepo := repository.Get(dataSource, UserEntity)
postRepo := repository.Get(dataSource, PostEntity)
categoryRepo := repository.Get(dataSource, CategoryEntity)

user, err := userRepo.Insert(ctx, &User{Name: "John Doe", Email: "john.doe@email.com"})

posts, err := postRepo.InsertMany(ctx,
	&Post{OwnerUserID: user.ID, Title: "Hello, Golem!", Content: "My first post."},
	&Post{OwnerUserID: user.ID, Title: "A Second Post", Content: "Another one."},
)

categories, err := categoryRepo.InsertMany(ctx,
	&Category{Name: "Announcements"},
	&Category{Name: "Tutorials"},
)

_, err = repository.Get(dataSource, PostToCategoryEntity).InsertMany(ctx,
	&PostToCategory{PostID: posts[0].ID, CategoryID: categories[0].ID},
	&PostToCategory{PostID: posts[1].ID, CategoryID: categories[1].ID},
)
```

## Find, join, and preload

```go
// find by primary key
found, err := userRepo.FindOne(ctx, func(u *User, q *query.Query[User]) {
	q.Where(op.Eq(&u.ID, user.ID))
})

// join: users who have at least 1 post
usersWithPosts, err := userRepo.FindMany(ctx, func(u *User, q0 *query.Query[User]) {
	join.Inner(q0, PostEntity, func(p *Post, q1 *query.Join[Post]) {
		q1.On(&p.OwnerUserID, &u.ID)
	})
})

// preload: each user's posts, grouped by User.ID — no navigational field on User itself
postsByUserID, err := repository.Preload(ctx, userRepo, usersWithPosts, PostEntity)
for _, u := range usersWithPosts {
	fmt.Printf("%s has %d posts\n", u.Name, len(postsByUserID[u.ID]))
}
```

See [Joins](../guides/joins.md) and [Preload](../guides/preload.md).

## Aggregations

```go
type UserPostCount struct {
	UserID int64
	Count  int64
}

counts, err := repository.Aggregate(ctx, postRepo, func(p *Post, res *UserPostCount, a *query.Aggregate[Post, UserPostCount]) {
	a.GroupBy(&p.OwnerUserID, &res.UserID)
	a.CountAll(&res.Count)
	a.OrderBy(op.Desc(&res.Count))
})
```

See [Aggregations](../guides/aggregations.md).

## Cascade delete

Deleting `user` also deletes their posts — no manual cleanup needed, per the `OnDeleteCascade` declared on `Post.OwnerUserID`'s foreign key:

```go
if _, err := userRepo.Delete(ctx, func(u *User, d *query.Delete[User]) {
	d.Where(op.Eq(&u.ID, user.ID))
}); err != nil {
	panic(err)
}

deletedUser, err := userRepo.FindOne(ctx, func(u *User, q *query.Query[User]) {
	q.Where(op.Eq(&u.ID, user.ID))
})
// deletedUser == nil, err == nil — not found is not an error

cascadedPost, err := postRepo.FindOne(ctx, func(p *Post, q *query.Query[Post]) {
	q.Where(op.Eq(&p.ID, posts[0].ID))
})
// cascadedPost == nil, err == nil — cascade-deleted along with the user
```

See [Relations & cascades](../guides/relations.md).

## Typed errors

```go
// ErrForeignKeyViolation: Post.OwnerUserID pointing at a user that doesn't exist
_, err = postRepo.Insert(ctx, &Post{OwnerUserID: 999_999_999, Title: "Orphan", Content: "..."})
// errors.Is(err, golem.ErrForeignKeyViolation) == true

// ErrDuplicateKey: same composite PK (post_id, category_id) inserted twice
ptcRepo := repository.Get(dataSource, PostToCategoryEntity)
link := &PostToCategory{PostID: posts[0].ID, CategoryID: categories[0].ID}
_, _ = ptcRepo.Insert(ctx, link)
_, err = ptcRepo.Insert(ctx, link)
// errors.Is(err, golem.ErrDuplicateKey) == true
```

See [Typed errors](../guides/errors.md).

## Pessimistic locking

```go
err = dataSource.Transaction(ctx, func(tx golem.Tx) error {
	locked, err := repository.Get(tx, UserEntity).FindOne(ctx, func(u *User, q *query.Query[User]) {
		q.Where(op.Eq(&u.ID, user.ID))
		q.ForUpdate()
	})
	if err != nil {
		return err
	}
	_, err = repository.Get(tx, UserEntity).Update(ctx, func(u *User, upd *query.Update[User]) {
		upd.Where(op.Eq(&u.ID, locked.ID))
		upd.Set(&u.Name, "locked and updated")
	})
	return err
})
```

See [Pessimistic locking](../guides/locking.md).

## Transactions

```go
err = dataSource.Transaction(ctx, func(tx golem.Tx) error {
	newUser, err := repository.Get(tx, UserEntity).Insert(ctx, &User{Name: "Jane", Email: "jane@email.com"})
	if err != nil {
		return err
	}
	_, err = repository.Get(tx, PostEntity).Insert(ctx, &Post{OwnerUserID: newUser.ID, Title: "First post", Content: "..."})
	return err
})
// both inserts commit together, or neither does
```

## Raw SQL

```go
result, err := dataSource.Exec(ctx, "UPDATE users SET name = $1 WHERE id = $2", "Updated Name", user.ID)
affected, _ := result.RowsAffected()

rawUsers, err := userRepo.Exec(ctx, "SELECT * FROM users WHERE name LIKE $1", "J%")
```

See [Raw SQL](../guides/raw-sql.md) — placeholder syntax differs per adapter, see [Supported databases](../guides/adapters.md).
