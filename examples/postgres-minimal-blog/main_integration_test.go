//go:build integration

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/adapter/postgres"
	"github.com/leandroluk/golem/repository"
)

// TestBlogExample_FullFlow exercises the same flow as main(), step by step,
// against the real Dockerized test Postgres (see Makefile's test-integration
// target / docker-compose.test.yml), asserting real outcomes at each step.
func TestBlogExample_FullFlow(t *testing.T) {
	dsn := resolveDSN()

	dataSource, err := golem.NewDataSource(postgres.New(func(o *postgres.Options) {
		o.DSN = dsn
	}))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}

	if err := dataSource.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer dataSource.Close()

	ctx := context.Background()

	// id columns are BIGSERIAL — leaving ID unset (zero) lets repository.Insert
	// omit it from the INSERT entirely, so Postgres assigns it.
	user, err := repository.Get(dataSource, UserEntity).Insert(ctx, &User{
		Name:  "Leandro",
		Email: "leandroluk@gmail.com",
	})
	if err != nil {
		t.Fatalf("Insert(User) returned error: %v", err)
	}
	if user.ID == 0 {
		t.Fatalf("expected non-zero user.ID after Insert, got %d", user.ID)
	}

	posts, err := repository.Get(dataSource, PostEntity).InsertMany(ctx,
		&Post{OwnerUserID: user.ID, Title: "Hello, Golem!", Content: "My first post using golem."},
		&Post{OwnerUserID: user.ID, Title: "A Second Post", Content: "Another post, still using golem."},
	)
	if err != nil {
		t.Fatalf("InsertMany(Post) returned error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	for i, p := range posts {
		if p.ID == 0 {
			t.Fatalf("posts[%d].ID is zero, want non-zero", i)
		}
		if p.OwnerUserID != user.ID {
			t.Fatalf("posts[%d].OwnerUserID = %d, want %d", i, p.OwnerUserID, user.ID)
		}
	}

	categories, err := repository.Get(dataSource, CategoryEntity).InsertMany(ctx,
		&Category{Name: "Announcements"},
		&Category{Name: "Tutorials"},
	)
	if err != nil {
		t.Fatalf("InsertMany(Category) returned error: %v", err)
	}
	if len(categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(categories))
	}
	for i, c := range categories {
		if c.ID == 0 {
			t.Fatalf("categories[%d].ID is zero, want non-zero", i)
		}
	}

	if _, err := repository.Get(dataSource, PostToCategoryEntity).InsertMany(ctx,
		&PostToCategory{PostID: posts[0].ID, CategoryID: categories[0].ID},
		&PostToCategory{PostID: posts[1].ID, CategoryID: categories[1].ID},
	); err != nil {
		t.Fatalf("InsertMany(PostToCategory) returned error: %v", err)
	}

	foundUser, err := repository.Get(dataSource, UserEntity).FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID(user.ID) returned error: %v", err)
	}
	if foundUser.Name != user.Name {
		t.Errorf("foundUser.Name = %q, want %q", foundUser.Name, user.Name)
	}
	if foundUser.Email != user.Email {
		t.Errorf("foundUser.Email = %q, want %q", foundUser.Email, user.Email)
	}

	_, err = repository.Get(dataSource, UserEntity).FindByID(ctx, user.ID+1_000_000)
	if !errors.Is(err, golem.ErrNotFound) {
		t.Errorf("FindByID(nonexistent ID): expected errors.Is(err, golem.ErrNotFound), got %v", err)
	}
}
