// Command mysql is a runnable demo of golem against a real MySQL
// database, using the User/Post/Category/PostToCategory entities declared
// in entities.go.
//
// Run it against the Dockerized test MySQL:
//
//	docker compose -f docker-compose.test.yml up -d --wait
//	go run ./.examples/mysql
//	docker compose -f docker-compose.test.yml down
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/driver/mysql"
	"github.com/leandroluk/golem/internal/op"
	"github.com/leandroluk/golem/internal/query"
	"github.com/leandroluk/golem/internal/repository"
)

// defaultDSN matches the Taskfile.yml's GOLEM_MYSQL_TEST_DSN default exactly.
// parseTime=true is required by the mysql driver to scan into time.Time.
const defaultDSN = "golem:golem@tcp(localhost:53306)/golem_test?parseTime=true"

// migration is split into individual statements — go-sql-driver/mysql
// rejects multiple commands in one Exec unless multiStatements=true is set
// on the DSN (not set here), so each statement is run separately, same
// reasoning as every other example in .examples/.
var migration = []string{
	`DROP TABLE IF EXISTS post_to_category`,
	`DROP TABLE IF EXISTS post`,
	`DROP TABLE IF EXISTS category`,
	`DROP TABLE IF EXISTS users`,
	`CREATE TABLE users (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(50) NOT NULL,
		email VARCHAR(50) NOT NULL
	)`,
	`CREATE TABLE post (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		owner_user_id BIGINT NOT NULL,
		title VARCHAR(50) NOT NULL,
		content TEXT NOT NULL,
		FOREIGN KEY (owner_user_id) REFERENCES users(id)
	)`,
	`CREATE TABLE category (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
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

// resolveDSN returns GOLEM_MYSQL_TEST_DSN from the environment if set, otherwise
// falls back to defaultDSN. Shared by main() and the integration test so
// both resolve the connection string identically.
func resolveDSN() string {
	if dsn := os.Getenv("GOLEM_MYSQL_TEST_DSN"); dsn != "" {
		// Ensure parseTime=true is present if testing with environment variable
		return dsn + "?parseTime=true"
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

	dataSource := mustValue(golem.NewDataSource(mysql.New(func(o *mysql.Options) { o.DSN = dsn })))

	mustExec(dataSource.Connect())

	defer dataSource.Close()

	fmt.Println("Connected to MySQL.")

	ctx := context.Background()

	for _, stmt := range migration {
		mustValue(dataSource.Exec(ctx, stmt))
	}

	// id columns are AUTO_INCREMENT — leaving ID unset (zero) lets repository.Insert
	// omit it from the INSERT entirely, so MySQL assigns it.
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
