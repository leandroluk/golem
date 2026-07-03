// Command postgres-minimal-blog is a runnable demo of golem against a real
// Postgres database, using the User/Post/Category/PostToCategory entities
// declared in entities.go (schema: .docker/testdata/schema.sql at the repo root).
//
// Run it against the Dockerized test Postgres:
//
//	docker compose -f .docker/docker-compose.test.yml up -d --wait
//	go run ./examples/postgres-minimal-blog
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

// defaultDSN matches the Taskfile.yml's GOLEM_TEST_DSN default exactly.
const defaultDSN = "postgres://golem:golem@localhost:55432/golem_test?sslmode=disable"

// resolveDSN returns GOLEM_TEST_DSN from the environment if set, otherwise
// falls back to defaultDSN. Shared by main() and the integration test so
// both resolve the connection string identically.
func resolveDSN() string {
	if dsn := os.Getenv("GOLEM_TEST_DSN"); dsn != "" {
		return dsn
	}
	return defaultDSN
}

func main() {
	dsn := resolveDSN()

	dataSource, err := golem.NewDataSource(
		postgres.New(func(o *postgres.Options) {
			o.DSN = dsn
		}),
	)
	if err != nil {
		panic(err)
	}

	if err := dataSource.Connect(); err != nil {
		panic(err)
	}
	defer dataSource.Close()

	fmt.Println("Connected to Postgres.")

	ctx := context.Background()

	// id columns are BIGSERIAL in .docker/testdata/schema.sql — leaving ID unset
	// (zero) lets repository.Insert omit it from the INSERT entirely, so
	// Postgres assigns it.
	user, err := repository.Get(dataSource, UserEntity).Insert(ctx, &User{
		Name:  "John Doe",
		Email: "john.doe@email.com",
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Inserted user: %+v\n", user)

	posts, err := repository.Get(dataSource, PostEntity).InsertMany(ctx,
		&Post{OwnerUserID: user.ID, Title: "Hello, Golem!", Content: "My first post using golem."},
		&Post{OwnerUserID: user.ID, Title: "A Second Post", Content: "Another post, still using golem."},
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Inserted %d posts: %+v\n", len(posts), posts)

	categories, err := repository.Get(dataSource, CategoryEntity).InsertMany(ctx,
		&Category{Name: "Announcements"},
		&Category{Name: "Tutorials"},
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Inserted %d categories: %+v\n", len(categories), categories)

	postToCategories, err := repository.Get(dataSource, PostToCategoryEntity).InsertMany(ctx,
		&PostToCategory{PostID: posts[0].ID, CategoryID: categories[0].ID},
		&PostToCategory{PostID: posts[1].ID, CategoryID: categories[1].ID},
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Inserted %d post-to-category links: %+v\n", len(postToCategories), postToCategories)

	foundUser, err := repository.Get(dataSource, UserEntity).FindOne(ctx, func(u *User, q *query.Query[User]) {
		q.Where(op.Eq(&u.ID, user.ID))
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Round-trip FindOne(user.ID=%d) => %+v\n", user.ID, foundUser)

	fmt.Println("Done.")
}

