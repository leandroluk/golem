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

`ColumnType` is dialect-agnostic — it never turns into DDL (golem doesn't generate schema, see [Migrations](../README.md#migrations)) and doesn't depend on which adapter you connected. It's purely documentation/metadata read by things like OpenAPI generation, not something the read/write path dispatches on at runtime — repository's `Insert`/`FindMany`/etc. work directly off the field's Go type (via `internal/scanner`, reflect-based), not `ColumnType`. This keeps the entity 100% portable across dialects — only the connector chosen at `NewDataSource` time decides the actual database; the entity declaration never changes. See [Supported databases](adapters.md) for how each type maps per dialect.

## Custom field types (Parser)

A field doesn't have to be a plain Go scalar. Every `DataSource` has a `golem.Parser` — `ToSQL(fieldVal reflect.Value) (driver.Value, error)` / `FromSQL(dst reflect.Value, raw any) error` — that converts field values to/from `driver.Value`. `golem.DefaultParser` unless overridden via `golem.CustomParser` at `NewDataSource` time.

`DefaultParser` resolves, in this order:

1. `driver.Valuer`/`sql.Scanner`, if the field type implements them (the stdlib's own contracts)
2. a `Get() T` / `Set(T)` method pair found by NAME via reflection (duck-typing, not a fixed interface) — works for a generic wrapper type, any `T`, without it implementing anything golem defines. This is what lets a third-party dirty-tracking wrapper (e.g. `gonest.Accessor[T]`) work as a struct field directly:

   ```go
   type Order struct {
   	ID    int64
   	Total gonest.Accessor[int64] // Get()/Set(T) duck-typed automatically
   }

   var OrderEntity = entity.New(func(o *Order, b *entity.Table) {
   	b.Col(&o.ID, golem.BIGINT())
   	b.Col(&o.Total, golem.BIGINT())
   	b.PrimaryKey(&o.ID)
   })
   ```

3. pointer dereference (`nil` → `nil`, non-nil → recurse into the pointee)
4. native passthrough (`string`/`bool`/`time.Time`/`[]byte`/numeric — already `driver.Value`-compatible)
5. named/enum type reduced to its underlying kind
6. JSON fallback for anything else (maps, structs — jsonb columns)

Need something the default doesn't cover? Decorate it instead of replacing it wholesale:

```go
dataSource := golem.MustNewDataSource(
	postgres.New(func(o *postgres.Options) { o.DSN = dsn }),
	golem.CustomParser(func(base golem.Parser) golem.Parser {
		return myParser{base: base} // your own ToSQL/FromSQL, falling back to base for everything else
	}),
)
```

> **Note**: `golem.Dialect` used to expose its own `Bind(ColumnType, any) (driver.Value, error)`/
> `Scan(ColumnType, raw any, dest any) error` per adapter — removed (never called by the actual
> read/write path, dead code since before this feature existed). `golem.Parser`, resolved per
> `DataSource` and overridable via `CustomParser`, is the supported extension point today,
> adapter-agnostic by construction.

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
