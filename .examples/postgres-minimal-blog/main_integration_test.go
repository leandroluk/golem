//go:build integration

package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/adapter/postgres"
	"github.com/leandroluk/golem/entity"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
)

// TestBlogExample_FullFlow exercises the same flow as main(), step by step,
// against the real Dockerized test Postgres (see Taskfile.yml's test-integration
// target / .docker/docker-compose.test.yml), asserting real outcomes at each step.
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
		Name:  "John Doe",
		Email: "john.doe@email.com",
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

	foundUser, err := repository.Get(dataSource, UserEntity).FindOne(ctx, func(u *User, q *query.Query[User]) {
		q.Where(op.Eq(&u.ID, user.ID))
	})
	if err != nil {
		t.Fatalf("FindOne(user.ID) returned error: %v", err)
	}
	if foundUser.Name != user.Name {
		t.Errorf("foundUser.Name = %q, want %q", foundUser.Name, user.Name)
	}
	if foundUser.Email != user.Email {
		t.Errorf("foundUser.Email = %q, want %q", foundUser.Email, user.Email)
	}

	_, err = repository.Get(dataSource, UserEntity).FindOne(ctx, func(u *User, q *query.Query[User]) {
		q.Where(op.Eq(&u.ID, user.ID+1_000_000))
	})
	if !errors.Is(err, golem.ErrNotFound) {
		t.Errorf("FindOne(nonexistent ID): expected errors.Is(err, golem.ErrNotFound), got %v", err)
	}
}

type TempPost struct {
	ID        int64
	Title     string
	DeletedAt *time.Time
}

var TempPostEntity = entity.New[TempPost](func(t *TempPost, b *entity.Builder) {
	b.TableName("temp_posts")
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Title, golem.VARCHAR(50))
	b.Col(&t.DeletedAt, golem.DATETIME()).Name("deleted_at")
	b.PrimaryKey(&t.ID)
	b.DeleteDate(&t.DeletedAt)
})

func TestBlogExample_DeleteCountAndExists(t *testing.T) {
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

	// Create a temporary table for this session
	_, err = dataSource.Dialect().Exec(ctx, dataSource, `CREATE TEMPORARY TABLE temp_posts (
		id SERIAL PRIMARY KEY,
		title VARCHAR(50) NOT NULL,
		deleted_at TIMESTAMPTZ
	)`, nil)
	if err != nil {
		t.Fatalf("failed to create temporary table: %v", err)
	}

	repo := repository.Get(dataSource, TempPostEntity)

	// Insert 2 rows
	inserted, err := repo.InsertMany(ctx,
		&TempPost{Title: "Post 1"},
		&TempPost{Title: "Post 2"},
	)
	if err != nil {
		t.Fatalf("InsertMany: %v", err)
	}
	if len(inserted) != 2 {
		t.Fatalf("expected 2 inserted posts")
	}

	// 1. Check Count and Exists
	cnt, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if cnt != 2 {
		t.Errorf("Count = %d, want 2", cnt)
	}

	hasRows, err := repo.Exists(ctx, func(p *TempPost, c *query.Count[TempPost]) {
		c.Where(op.Eq(&p.Title, "Post 1"))
	})
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !hasRows {
		t.Errorf("expected Exists to be true")
	}

	// 2. Soft-delete "Post 1"
	err = repo.Delete(ctx, &inserted[0])
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Check Count (should be 1 because soft-deleted is filtered out by default)
	cnt, err = repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count after delete: %v", err)
	}
	if cnt != 1 {
		t.Errorf("Count after delete = %d, want 1", cnt)
	}

	// Check Count with deleted (should be 2)
	cnt, err = repo.Count(ctx, func(p *TempPost, c *query.Count[TempPost]) {
		c.WithDeleted()
	})
	if err != nil {
		t.Fatalf("Count with deleted: %v", err)
	}
	if cnt != 2 {
		t.Errorf("Count with deleted = %d, want 2", cnt)
	}

	// 3. Restore "Post 1"
	err = repo.Restore(ctx, &inserted[0])
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Count should be 2 again
	cnt, err = repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count after restore: %v", err)
	}
	if cnt != 2 {
		t.Errorf("Count after restore = %d, want 2", cnt)
	}
}

