// Package main declares the entity schema for a minimal blog example
// (users, post, category, post_to_category) matching the migration DDL in
// main.go verbatim, and (in main.go) runs a walkthrough of golem's full
// feature set against a real Postgres instance.
package main

import (
	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
	"github.com/leandroluk/golem/relation"
)

// User is a blog author.
type User struct {
	ID    int64
	Name  string
	Email string
}

// Post belongs to a User (OwnerUserID) and can be tagged with 0+ Categories
// via PostToCategory.
type Post struct {
	ID          int64
	OwnerUserID int64
	Title       string
	Content     string
}

// Category tags a Post via PostToCategory.
type Category struct {
	ID   int64
	Name string
}

// PostToCategory is the many-to-many junction entity between Post and
// Category — a plain entity with two ForeignKeys, no dedicated relation
// type (see .specs/project/STATE.md AD-001).
type PostToCategory struct {
	PostID     int64
	CategoryID int64
}

// UserEntity is User's schema declaration.
var UserEntity = entity.New[User](func(t *User, b *entity.Table) {
	b.TableName("users")
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Name, golem.VARCHAR(50))
	b.Col(&t.Email, golem.VARCHAR(50))
	b.PrimaryKey(&t.ID)
})

// PostEntity is Post's schema declaration.
var PostEntity = entity.New[Post](func(t *Post, b *entity.Table) {
	// table name defaults to "post" (lowercased struct name) — matches schema.sql, no override needed
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.OwnerUserID, golem.BIGINT()).Name("owner_user_id")
	b.Col(&t.Title, golem.VARCHAR(50))
	b.Col(&t.Content, golem.TEXT())
	b.PrimaryKey(&t.ID)
	// deleting a User cascades into deleting their Posts (real runtime
	// behavior — see Repository[T].Delete / .specs/features/relations).
	b.ForeignKey(&t.OwnerUserID, UserEntity, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))
})

// CategoryEntity is Category's schema declaration.
var CategoryEntity = entity.New[Category](func(t *Category, b *entity.Table) {
	// table name defaults to "category" — matches schema.sql
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Name, golem.VARCHAR(50))
	b.PrimaryKey(&t.ID)
})

// PostToCategoryEntity is PostToCategory's schema declaration.
var PostToCategoryEntity = entity.New[PostToCategory](func(t *PostToCategory, b *entity.Table) {
	b.TableName("post_to_category")
	b.Col(&t.PostID, golem.BIGINT()).Name("post_id")
	b.Col(&t.CategoryID, golem.BIGINT()).Name("category_id")
	b.PrimaryKey(&t.PostID, &t.CategoryID)
	b.ForeignKey(&t.PostID, PostEntity)
	b.ForeignKey(&t.CategoryID, CategoryEntity)
})
