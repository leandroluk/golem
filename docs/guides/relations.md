# Relations & cascades

Golem has exactly one relation primitive: `ForeignKey`, declared inside `golem.NewEntity` (see [Declaring schemas](schema.md#foreign-keys)). There's no `OneToMany`/`ManyToOne`/`OneToOne`/`ManyToMany` type hierarchy — those all reduce to a foreign key at the database level, so golem doesn't hide that behind a parallel API.

## `OnDelete` behavior

The third argument to `ForeignKey` is a `golem.ForeignKeyOptions`, whose only real option is `OnDelete`:

```go
b.ForeignKey(&t.OwnerUserID, UserEntity, golem.NewForeignKeyOptions().
	OnDelete(golem.OnDeleteCascade))
```

| Value | Behavior |
| --- | --- |
| `golem.OnDeleteCascade` | deleting the parent also deletes (or soft-deletes) referencing children |
| `golem.OnDeleteSetNull` | deleting the parent sets the FK column to `NULL` on referencing children |
| `golem.OnDeleteRestrict` | blocks the delete with `golem.ErrForeignKeyViolation` if a referencing row exists |
| `golem.OnDeleteDefault` / `golem.OnDeleteNoAction` | no golem-level behavior; a real DB constraint outside golem (if any) decides |

This is applied by `Repository[T].Delete` at the application layer — it consults a global FK registry built from every `golem.NewEntity` call, not by relying on the database's own `FOREIGN KEY ... ON DELETE CASCADE` DDL (golem never generates DDL — see [Migrations](../README.md#migrations)). If you want the database itself to also enforce this, declare the same behavior in your own migration tool.

Only `OnDelete` is exposed. Other concepts common in ORMs with a full relations API (cascade on save, orphan removal, lazy/eager toggles, `OnUpdate` cascades for mutable primary keys) were considered and cut rather than accepted-but-inert — entities have no navigational fields (see [Preload](preload.md)) and golem never generates DDL, so most of that machinery would have nothing real to attach to.

## Many-to-many relations (junction entity)

Inspired by [TypeORM's relations](https://typeorm.io/docs/relations/relations), but without the `@JoinTable`/`@JoinColumn` concept: the junction table is always a plain entity, with two foreign keys. That's literally what the database does under the hood, so golem doesn't hide it behind a parallel API — `ForeignKey` already covers this case.

```go
package main

import (
	"context"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/driver/postgres"
)

type Category struct {
	ID   int64
	Name string
}

type Question struct {
	ID    int64
	Title string
	Text  string
}

// junction table: no new concept, just a plain entity with two foreign keys
type QuestionToCategory struct {
	QuestionID int64
	CategoryID int64
}

var CategoryEntity = golem.NewEntity[Category](func(t *Category, b *golem.Table) {
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Name, golem.VARCHAR(50))
	b.PrimaryKey(&t.ID)
})

var QuestionEntity = golem.NewEntity[Question](func(t *Question, b *golem.Table) {
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.Title, golem.VARCHAR(50))
	b.Col(&t.Text, golem.TEXT())
	b.PrimaryKey(&t.ID)
})

// since QuestionToCategory references QuestionEntity/CategoryEntity but neither of
// them references it back, there's no initialization cycle
var QuestionToCategoryEntity = golem.NewEntity[QuestionToCategory](func(t *QuestionToCategory, b *golem.Table) {
	b.Col(&t.QuestionID, golem.BIGINT())
	b.Col(&t.CategoryID, golem.BIGINT())

	// composite primary key
	b.PrimaryKey(&t.QuestionID, &t.CategoryID)

	b.ForeignKey(&t.QuestionID, QuestionEntity)
	b.ForeignKey(&t.CategoryID, CategoryEntity)
})

func example(ctx context.Context, dataSource *golem.DataSource) error {
	// no automatic collection cascade: each entity is inserted explicitly, and the
	// junction row is just another normal insert — no "save the whole graph" magic.
	// everything runs inside a single transaction so the question is never left
	// "orphaned" if the junction insert fails
	return dataSource.Transaction(ctx, func(tx golem.Tx) error {
		categories, err := golem.NewRepository(tx, CategoryEntity).InsertMany(ctx,
			&Category{Name: "ORMs"},
			&Category{Name: "Programming"},
		)
		if err != nil {
			return err
		}

		question, err := golem.NewRepository(tx, QuestionEntity).Insert(ctx, &Question{
			Title: "How do I ask about golem?",
			Text:  "Where can I get golem-related questions answered?",
		})
		if err != nil {
			return err
		}

		_, err = golem.NewRepository(tx, QuestionToCategoryEntity).InsertMany(ctx,
			&QuestionToCategory{QuestionID: question.ID, CategoryID: categories[0].ID},
			&QuestionToCategory{QuestionID: question.ID, CategoryID: categories[1].ID},
		)
		return err
	})
}
```

> **Loading `question.Categories` as a ready-made collection** (like some ORMs' eager/lazy loading) is solved via a query helper — `Preload` over `QuestionToCategoryEntity` — not as a new relation type. See [Preload](preload.md).
