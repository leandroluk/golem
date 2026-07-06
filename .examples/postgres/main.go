// Command postgres is a runnable demo of golem against a real Postgres
// database, using the User/Post/Category/PostToCategory entities declared
// in entities.go. The schema migration (DROP+CREATE) lives in this file's
// migration constant, run once at startup — no separate .sql file to keep
// in sync.
//
// Run it against the Dockerized test Postgres:
//
//	docker compose -f .docker/docker-compose.test.yml up -d --wait
//	go run ./.examples/postgres
//	docker compose -f .docker/docker-compose.test.yml down
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/driver/postgres"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
)

// defaultDSN matches the Taskfile.yml's GOLEM_POSTGRES_TEST_DSN default exactly.
const defaultDSN = "postgres://golem:golem@localhost:55432/golem_test?sslmode=disable"

// migration is split into individual statements — pgx's extended query
// protocol (what golem.DataSource.Exec uses) rejects multiple commands in
// a single prepared statement ("SQLSTATE 42601: cannot insert multiple
// commands into a prepared statement"), unlike psql's simple protocol.
var migration = []string{
	`DROP TABLE IF EXISTS "post_to_category"`,
	`DROP TABLE IF EXISTS "post"`,
	`DROP TABLE IF EXISTS "category"`,
	`DROP TABLE IF EXISTS "users"`,
	`CREATE TABLE "users" (
		"id" BIGSERIAL PRIMARY KEY,
		"name" VARCHAR(50) NOT NULL,
		"email" VARCHAR(50) NOT NULL
	)`,
	`CREATE TABLE "post" (
		"id" BIGSERIAL PRIMARY KEY,
		"owner_user_id" BIGINT NOT NULL REFERENCES "users"("id"),
		"title" VARCHAR(50) NOT NULL,
		"content" TEXT NOT NULL
	)`,
	`CREATE TABLE "category" (
		"id" BIGSERIAL PRIMARY KEY,
		"name" VARCHAR(50) NOT NULL
	)`,
	`CREATE TABLE "post_to_category" (
		"post_id" BIGINT NOT NULL REFERENCES "post"("id"),
		"category_id" BIGINT NOT NULL REFERENCES "category"("id"),
		PRIMARY KEY ("post_id", "category_id")
	)`,
}

// resolveDSN returns GOLEM_POSTGRES_TEST_DSN from the environment if set, otherwise
// falls back to defaultDSN. Shared by main() and the integration test so
// both resolve the connection string identically.
func resolveDSN() string {
	if dsn := os.Getenv("GOLEM_POSTGRES_TEST_DSN"); dsn != "" {
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

	dataSource := mustValue(golem.NewDataSource(postgres.New(func(o *postgres.Options) { o.DSN = dsn })))

	mustExec(dataSource.Connect())

	defer dataSource.Close()

	fmt.Println("Connected to Postgres.")

	ctx := context.Background()

	for _, stmt := range migration {
		mustValue(dataSource.Exec(ctx, stmt))
	}

	// id columns are BIGSERIAL — leaving ID unset (zero) lets repository.Insert
	// omit it from the INSERT entirely, so Postgres assigns it.
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
