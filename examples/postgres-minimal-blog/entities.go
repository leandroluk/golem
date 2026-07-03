// Package main declares the entity schema for a minimal blog example
// (users, post, category, post_to_category) matching testdata/schema.sql
// verbatim. A later task adds main() in a separate main.go in this same
// package/directory to actually run against a Postgres database.
package main

import (
	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
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

type PostToCategory struct {
	PostID     int64
	CategoryID int64
}

var UserEntity = entity.New[User](func(t *User, b *entity.Builder) {
	b.TableName("users")
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Name, golem.VARCHAR(50))
	b.Col(&t.Email, golem.VARCHAR(50))
	b.PrimaryKey(&t.ID)
})

var PostEntity = entity.New[Post](func(t *Post, b *entity.Builder) {
	// table name defaults to "post" (lowercased struct name) — matches schema.sql, no override needed
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.OwnerUserID, golem.BIGINT()).Name("owner_user_id")
	b.Col(&t.Title, golem.VARCHAR(50))
	b.Col(&t.Content, golem.TEXT())
	b.PrimaryKey(&t.ID)
	b.ForeignKey(&t.OwnerUserID, UserEntity)
})

var CategoryEntity = entity.New[Category](func(t *Category, b *entity.Builder) {
	// table name defaults to "category" — matches schema.sql
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Name, golem.VARCHAR(50))
	b.PrimaryKey(&t.ID)
})

var PostToCategoryEntity = entity.New[PostToCategory](func(t *PostToCategory, b *entity.Builder) {
	b.TableName("post_to_category")
	b.Col(&t.PostID, golem.BIGINT()).Name("post_id")
	b.Col(&t.CategoryID, golem.BIGINT()).Name("category_id")
	b.PrimaryKey(&t.PostID, &t.CategoryID)
	b.ForeignKey(&t.PostID, PostEntity)
	b.ForeignKey(&t.CategoryID, CategoryEntity)
})
