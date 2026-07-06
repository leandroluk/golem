# Declaring schemas

Entities are plain Go structs — no tags, no code generation. `entity.New[T](func(t *T, b *entity.Table) {...})` declares the mapping in a callback: `t` is a pointer to a zero-value `T` used purely to take field addresses, and `b` is the builder that records what each address means.

```go
package main

import (
	"time"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
	"github.com/leandroluk/golem/relation"
)

type User struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	Name      string
	Email     string
	Age       uint8
}

var UserEntity = entity.New[User](func(t *User, b *entity.Table) {
	// table name; if omitted, uses the lowercased struct name (e.g. "User" -> "user")
	b.TableName("users")
	// table schema; if omitted, uses whichever schema is currently selected on the connection
	b.SchemaName("public")

	b.Col(&t.ID, golem.BIGINT())
	// column name; if omitted, uses the struct field's name (e.g. "Name")
	b.Col(&t.Name, golem.VARCHAR(50)).Name("full_name")
	b.Col(&t.Email, golem.VARCHAR(50))
	b.Col(&t.Age, golem.INTEGER())
	b.Col(&t.CreatedAt, golem.DATETIME())
	b.Col(&t.UpdatedAt, golem.DATETIME())
	b.Col(&t.DeletedAt, golem.DATETIME()).Nullable().Default(nil)

	b.PrimaryKey(&t.ID)
	// lives outside Col because uniqueness/PKs can span more than one column (composite)
	b.Unique(&t.Email)

	// declares special fields — filled in automatically by Repository[T]
	b.CreateDate(&t.CreatedAt)
	b.UpdateDate(&t.UpdatedAt)
	b.DeleteDate(&t.DeletedAt).Nullable().Default(nil)
})
```

## `golem.ColumnType`

`golem.BIGINT()`, `golem.VARCHAR(50)`, `golem.TEXT()`, `golem.BOOLEAN()`, `golem.DATETIME()`, `golem.UUID()`, `golem.JSON()`, etc. — the full set is `BOOLEAN`, `SMALLINT`, `INTEGER`, `BIGINT`, `DECIMAL`, `FLOAT`, `CHAR`, `VARCHAR`, `TEXT`, `DATE`, `DATETIME`, `TIME`, `BLOB`, `UUID`, `JSON`.

`ColumnType` is dialect-agnostic — it never turns into DDL (golem doesn't generate schema, see [Migrations](../README.md#migrations)) and doesn't depend on which adapter you connected. It's only a semantic id so the adapter knows how to bind (Go → driver) and scan (driver → Go) that value, since `database/sql` alone can't handle exotic types (UUID, JSON, ENUM, …) consistently across dialects. Each adapter implements:

```go
type Dialect interface {
	Bind(t golem.ColumnType, value any) (driver.Value, error)
	Scan(t golem.ColumnType, raw any, dest any) error
	// ... statement compilation/execution
}
```

This keeps the entity 100% portable across dialects — only the connector chosen at `NewDataSource` time decides the actual database; the entity declaration never changes. See [Supported databases](adapters.md) for how each type maps per dialect.

## `entity.Table` reference

Scope — table-level (lives outside `Col` because it can span more than one column, or has no single field to attach to):

| Method | Description |
| --- | --- |
| `TableName(name string)` | table name; defaults to the struct name |
| `SchemaName(name string)` | table schema; defaults to whichever schema is currently selected on the connection |
| `PrimaryKey(fieldPtrs ...any)` | primary key; accepts 1+ fields (composite) |
| `Unique(fieldPtrs ...any)` | `UNIQUE` constraint; accepts 1+ fields (composite) |
| `Index(fieldPtrs ...any) *entity.Index` | secondary index over 1+ fields |

Scope — column-level:

| Method | Description |
| --- | --- |
| `Col(fieldPtr any, t golem.ColumnType) *entity.Column` | maps a struct field to a column with an explicit type |
| `ForeignKey(fieldPtr any, target *entity.Entity[T], opts ...*relation.ForeignKeyOptions)` | foreign key pointing at another entity's PK |
| `CreateDate(fieldPtr any) *entity.Column` | marks the field as "created at" — filled in automatically with the insert's timestamp |
| `UpdateDate(fieldPtr any) *entity.Column` | marks the field as "updated at" — filled in automatically with each update's timestamp |
| `DeleteDate(fieldPtr any) *entity.Column` | marks the field as "soft-deleted at" — a non-nil value means the row is deleted; see [Query builder](query-builder.md)'s `.WithDeleted()` |

`*entity.Column` (returned by `Col`/`CreateDate`/`UpdateDate`/`DeleteDate`) chains:

| Method | Description |
| --- | --- |
| `.Name(name string)` | column name; defaults to the struct field's name |
| `.Nullable()` | allows `NULL` |
| `.Default(value any)` | default value (or dialect expression) |
| `.DefaultFunc(fn func() (any, error))` | computed in Go code at insert time (UUIDs, slugs, a value derived from another field, …) — only applied when the field is still zero-valued; a returned error cancels the insert |

`*entity.Index` (returned by `Index`) chains:

| Method | Description |
| --- | --- |
| `.Name(name string)` | index name; defaults to a generated one (`idx_<table>_<columns>`) |
| `.Unique()` | unique index |

## Foreign keys

`ForeignKey`'s third (optional) parameter, `relation.ForeignKeyOptions`, only exposes `OnDelete` — the one option with real runtime effect given golem's design (no DDL generation, no navigational fields for in-memory attached relations):

```go
type Post struct {
	ID          int64
	OwnerUserID int64
	Title       string
	Content     string
}

var PostEntity = entity.New[Post](func(t *Post, b *entity.Table) {
	b.Col(&t.ID, golem.BIGINT())
	b.Col(&t.OwnerUserID, golem.BIGINT())
	b.Col(&t.Title, golem.VARCHAR(50))
	b.Col(&t.Content, golem.TEXT())
	b.PrimaryKey(&t.ID)

	b.ForeignKey(&t.OwnerUserID, UserEntity, relation.NewForeignKeyOptions().
		// when a user is deleted, their posts are actually deleted too
		// (Repository[T].Delete consults a global FK registry and applies this)
		OnDelete(relation.OnDeleteCascade))
})
```

`OnDelete` accepts:

| Value | Behavior |
| --- | --- |
| `relation.OnDeleteCascade` | deleting the parent also deletes (or soft-deletes) referencing children |
| `relation.OnDeleteSetNull` | deleting the parent sets the FK column to `NULL` on referencing children |
| `relation.OnDeleteRestrict` | blocks the delete with `golem.ErrForeignKeyViolation` if a referencing row exists |
| `relation.OnDeleteDefault` / `relation.OnDeleteNoAction` | no golem-level behavior; if a real DB constraint exists outside golem, it decides |

See [Relations & cascades](relations.md) for the full picture, including many-to-many junction entities.

## Hooks

`entity.AddHook(Entity)` returns a chainable builder — one method per hook slot, no wrapper type per hook:

```go
var _ = entity.AddHook(UserEntity).
	BeforeCreate(func(ctx context.Context, i *User, conn golem.Conn) error {
		fmt.Println("before creating user", i.Name)
		return nil
	}).
	AfterCreate(func(ctx context.Context, i *User, conn golem.Conn) error {
		repo := repository.Get(conn, UserEntity)
		count, err := repo.Count(ctx)
		fmt.Println("total users now:", count)
		return err
	})
```

Available slots (all sharing the signature `func(ctx context.Context, i *T, conn golem.Conn) error`):

- `(Before|After|OnConflict)Create`
- `(Before|After|OnConflict)Update`
- `(Before|After|OnConflict)Delete`

Every hook must return an error, and a returned error cancels the operation. All hooks run inside the same transaction as the operation that triggered them (`conn`), so an error from any one of them rolls back everything — including the very insert/update/delete that triggered the hook. Registering the same slot twice on the same entity panics, naming the slot.
