// Command mssql is a runnable demo of golem against a real MSSQL
// database, using the User/Post/Category/PostToCategory entities declared
// in entities.go.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/driver/mssql"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
)

const defaultDSN = "sqlserver://sa:Golem_Test_2026!@localhost:51433?database=master"

// migration is split into individual statements for consistency with
// every other .examples/ entry, even though T-SQL batches (unlike pgx's
// extended protocol) can usually run several ;-separated statements per
// Exec — running each separately avoids relying on that.
var migration = []string{
	`IF OBJECT_ID('post_to_category', 'U') IS NOT NULL DROP TABLE post_to_category`,
	`IF OBJECT_ID('post', 'U') IS NOT NULL DROP TABLE post`,
	`IF OBJECT_ID('category', 'U') IS NOT NULL DROP TABLE category`,
	`IF OBJECT_ID('users', 'U') IS NOT NULL DROP TABLE users`,
	`CREATE TABLE users (
		id BIGINT IDENTITY(1,1) PRIMARY KEY,
		name VARCHAR(50) NOT NULL,
		email VARCHAR(50) NOT NULL
	)`,
	`CREATE TABLE post (
		id BIGINT IDENTITY(1,1) PRIMARY KEY,
		owner_user_id BIGINT NOT NULL,
		title VARCHAR(50) NOT NULL,
		content VARCHAR(MAX) NOT NULL,
		FOREIGN KEY (owner_user_id) REFERENCES users(id)
	)`,
	`CREATE TABLE category (
		id BIGINT IDENTITY(1,1) PRIMARY KEY,
		name VARCHAR(50) NOT NULL
	)`,
	`CREATE TABLE post_to_category (
		post_id BIGINT NOT NULL,
		category_id BIGINT NOT NULL,
		PRIMARY KEY (post_id, category_id),
		FOREIGN KEY (post_id) REFERENCES post(id),
		FOREIGN KEY (category_id) REFERENCES category(id)
	)`,
}

func resolveDSN() string {
	if dsn := os.Getenv("GOLEM_MSSQL_TEST_DSN"); dsn != "" {
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
	dataSource := mustValue(golem.NewDataSource(mssql.New(func(o *mssql.Options) { o.DSN = dsn })))
	mustExec(dataSource.Connect())
	defer dataSource.Close()

	fmt.Println("Connected to MSSQL.")
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
