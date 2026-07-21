// Command sqlite is a runnable demo of golem against SQLite
// using the User/Post/Category/PostToCategory entities declared
// in entities.go.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/driver/sqlite"
	"github.com/leandroluk/golem/internal/op"
	"github.com/leandroluk/golem/internal/query"
	"github.com/leandroluk/golem/internal/repository"
)

const defaultDSN = "golem_example.db"

// migration is split into individual statements for consistency with
// every other .examples/ entry — modernc.org/sqlite's Exec can usually run
// several ;-separated statements at once, but splitting avoids relying on
// that (and matches driver/sqlite's own SetMaxOpenConns(1) single-writer
// model, one statement at a time).
var migration = []string{
	`DROP TABLE IF EXISTS post_to_category`,
	`DROP TABLE IF EXISTS post`,
	`DROP TABLE IF EXISTS category`,
	`DROP TABLE IF EXISTS users`,
	`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name VARCHAR(50) NOT NULL,
		email VARCHAR(50) NOT NULL
	)`,
	`CREATE TABLE post (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_user_id INTEGER NOT NULL,
		title VARCHAR(50) NOT NULL,
		content TEXT NOT NULL,
		FOREIGN KEY (owner_user_id) REFERENCES users(id)
	)`,
	`CREATE TABLE category (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name VARCHAR(50) NOT NULL
	)`,
	`CREATE TABLE post_to_category (
		post_id INTEGER NOT NULL,
		category_id INTEGER NOT NULL,
		PRIMARY KEY (post_id, category_id),
		FOREIGN KEY (post_id) REFERENCES post(id),
		FOREIGN KEY (category_id) REFERENCES category(id)
	)`,
}

func resolveDSN() string {
	if dsn := os.Getenv("GOLEM_SQLITE_TEST_DSN"); dsn != "" {
		return dsn
	}
	return defaultDSN
}

func mustValue[V any](v V, err error) V {
	if err != nil {
		panic(err)
	}
	return v
}

func mustExec(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	dsn := resolveDSN()
	dataSource := mustValue(golem.NewDataSource(sqlite.New(func(o *sqlite.Options) { o.Path = dsn })))
	mustExec(dataSource.Connect())
	defer dataSource.Close()

	fmt.Println("Connected to SQLite.")
	ctx := context.Background()
	for _, stmt := range migration {
		mustValue(dataSource.Exec(ctx, stmt))
	}

	user := mustValue(repository.Get(dataSource, UserEntity).Insert(ctx, &User{
		Name:  "John Doe",
		Email: "john.doe@email.com",
	}))

	fmt.Printf("Inserted user: %+v\n", user)

	posts := mustValue(repository.Get(dataSource, PostEntity).InsertMany(ctx,
		&Post{OwnerUserID: user.ID, Title: "Hello, Golem!", Content: "My first post using golem."},
		&Post{OwnerUserID: user.ID, Title: "A Second Post", Content: "Another post, still using golem."},
	))
	fmt.Printf("Inserted %d posts: %+v\n", len(posts), posts)

	categories := mustValue(repository.Get(dataSource, CategoryEntity).InsertMany(ctx,
		&Category{Name: "Announcements"},
		&Category{Name: "Tutorials"},
	))
	fmt.Printf("Inserted %d categories: %+v\n", len(categories), categories)

	postToCategories := mustValue(repository.Get(dataSource, PostToCategoryEntity).InsertMany(ctx,
		&PostToCategory{PostID: posts[0].ID, CategoryID: categories[0].ID},
		&PostToCategory{PostID: posts[1].ID, CategoryID: categories[1].ID},
	))
	fmt.Printf("Inserted %d post-to-category links: %+v\n", len(postToCategories), postToCategories)

	foundUser := mustValue(repository.Get(dataSource, UserEntity).FindOne(ctx, func(u *User, q *query.Query[User]) {
		q.Where(op.Eq(&u.ID, user.ID))
	}))
	fmt.Printf("Round-trip FindOne(user.ID=%d) => %+v\n", user.ID, foundUser)
	fmt.Println("Done.")
}
