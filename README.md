<div align="center">
  <img src="docs/assets/banner.png" alt="Golem - Type Safe ORM for Go" />
</div>

<br/>

<div align="center">
  <a href="https://github.com/leandroluk/golem/actions">
    <img src="https://github.com/leandroluk/golem/actions/workflows/ci.yml/badge.svg?branch=master" alt="Build Status" />
  </a>
  <a href="https://codecov.io/gh/leandroluk/golem">
    <img src="https://img.shields.io/codecov/c/github/leandroluk/golem/master.svg" alt="Coverage Status" />
  </a>
  <a href="https://pkg.go.dev/github.com/leandroluk/golem">
    <img src="https://pkg.go.dev/badge/github.com/leandroluk/golem.svg" alt="Go Doc" />
  </a>
  <a href="https://github.com/leandroluk/golem/releases">
    <img src="https://img.shields.io/github/release/leandroluk/golem.svg?style=flat-square" alt="Release" />
  </a>
</div>

<br/>

📚 **[Read the Documentation](https://leandroluk.github.io/golem)**

---

## Contents
- [Contents](#contents)
- [Getting started](#getting-started)
- [Implementation Status](#implementation-status)
- [Next Steps](#next-steps)
- [About the Project](#about-the-project)
- [Documentation](#documentation)
  - [Connecting to Postgres](#connecting-to-postgres)
  - [Declaring schemas](#declaring-schemas)
  - [Many-to-many relations (junction entity)](#many-to-many-relations-junction-entity)
  - [Repository (CRUD)](#repository-crud)
  - [Query Builder](#query-builder)
  - [Joins](#joins)
  - [Preload / Eager Loading](#preload--eager-loading)
  - [Aggregations](#aggregations)
  - [Pessimistic Locking](#pessimistic-locking)
  - [Raw SQL (escape hatch)](#raw-sql-escape-hatch)
  - [Errors](#errors)
  - [Migrations](#migrations)
  - [Custom logger](#custom-logger)
- [Contributors](#contributors)
- [License](#license)

---

## Getting started

```go
package main

import (
  "github.com/leandroluk/golem"
  postgres "github.com/leandroluk/golem/driver/postgres"
)

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func(o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  if err := dataSource.Connect(); err != nil {
    panic(err)
  }
}
```

See [Documentation](#documentation) below for the full API (entities, repositories, query builder, joins, transactions, raw SQL).

---

## Implementation Status

- [x] M1 - Foundation (`DataSource`, `golem.Conn`, `golem.Dialect`, Postgres adapter)
- [x] M2 - Schema Declaration (`entity.Table`, `golem.ColumnType`)
- [x] M3 - Repository Core CRUD (`Insert`/`InsertMany`, `FindOne`/`FindMany` — no dedicated `FindByID`, see AD-022)
- [x] M4 - Query Builder & Read Paths
- [x] M5 - Update/Count Builders
- [x] M6 - Joins
- [x] M7 - Hooks
- [x] M8 - Transactions
- [x] M9 - Raw SQL
- [x] M10 - Typed Errors
- [x] M11 - Relations (`ForeignKeyOptions` + Cascade)
- [x] M12 - Preload / Eager Loading
- [x] M13 - Aggregations
- [x] M14 - Pessimistic Locking
- [x] M15 - Cross-Dialect Conformance Suite
- [x] M16 - MySQL / MariaDB Adapter
- [x] M17 - SQLite Adapter
- [x] M18 - SQL Server (MSSQL) Adapter
- [x] M19 - Oracle Adapter

Adapter set is considered **done** at these 5 databases (Postgres, MySQL, SQLite, SQL Server,
Oracle) — see "Next Steps" below.

See `.specs/project/ROADMAP.md` for the full milestone breakdown.

---

## Next Steps

M1-M14 (everything above) targets Postgres only. `golem.ColumnType`/`golem.Dialect` were designed
dialect-agnostic from day one (AD-015 in `.specs/project/STATE.md`) specifically so adding another
database is "write a new `driver/*` package," not "redesign the core." `.specs/project/ROADMAP.md`
now has concrete planned milestones for every database in [INSIGHT.md](INSIGHT.md), in this order:

- **M15** ✅ — `internal/dialecttest`, a cross-dialect conformance test suite every adapter below
  must pass; `driver/postgres` is its first caller, verified against real Postgres
- **M16** ✅ — `driver/mysql` (MySQL 8+ primary, MariaDB best-effort), verified against real MySQL
- **M17** ✅ — `driver/sqlite` (`modernc.org/sqlite`, pure Go/no cgo), verified against a real
  in-memory database — no Docker service needed, the one adapter that genuinely doesn't need one
- **M18** ✅ — `driver/mssql` (`microsoft/go-mssqldb`, pure database/sql), verified against a real
  SQL Server 2025 container — `OUTPUT INSERTED.*`, `@pN` placeholders, table-hint locking
- **M19** ✅ — `driver/oracle` (`github.com/sijms/go-ora/v2`, pure Go/no cgo), verified against a
  real Oracle 23ai Free container — multi-round-trip Insert/Update (MySQL-shaped, not
  single-`RETURNING`), `:N` placeholders, `NUMBER`-family scan disambiguation

**M20 (IBM Db2) and M21 (Snowflake) were both dropped from scope** (see AD-051/AD-052 in
`.specs/project/STATE.md`). Db2 was the only adapter that couldn't stay pure-Go
(`github.com/ibmdb/go_ibm_db` needs the IBM CLI driver's native DLL/shared library at runtime, no
viable pure-Go alternative exists) and has a small, enterprise/legacy-niche real-world footprint
(~0.7-0.8% relational-database market share). Snowflake's only free local test target (a DuckDB-based
emulator) turned out too broken on basic operations (parameterized queries, pagination, several core
types) to trust, and Snowflake's standard tables don't enforce `UNIQUE`/`FOREIGN KEY` constraints at
all — a real, permanent divergence from every other adapter. After two consecutive milestones landing
on "increasingly exotic database, increasingly awkward adapter shape, no free way to verify," the
adapter set was deliberately capped at the 5 common, widely-used databases above — the same scope
philosophy mainstream ORMs like TypeORM follow, rather than chasing exhaustive database coverage. Any
future adapter needs its own fresh scope justification, not an assumption that M20/M21 resume.

**M18 (SQL Server) note:** pulling `mcr.microsoft.com/mssql/server` can silently fail or hang on
some networks/ISPs/corporate DNS setups — the registry's blob storage backend
(`*.data.mcr.microsoft.com`) may be unreachable even when `mcr.microsoft.com` itself resolves fine,
and there's no Docker Hub mirror to fall back to. If `docker pull` fails, check your DNS/network
setup first. For one setup, installing and enabling the **Cloudflare One (WARP) client** was the
only thing that fixed it — your mileage may vary depending on what's actually blocking the CDN
backend in your environment.
No other adapters are planned. The adapter set is deliberately capped at the 5 common databases
above (AD-051/AD-052) — CockroachDB, BigQuery, Redshift, ClickHouse, Db2, and Snowflake are all out
of scope, not a backlog.

`INSIGHT.md` (repo root) is the type-mapping research this list drew from while the adapters above
were being built — kept as reference material, including its Db2/Snowflake sections from before
those were dropped.

## Performance

Golem foi projetado com otimizações inspiradas nos ORMs de mais alta performance do mercado (como o [BreezeORM](https://github.com/nelthaarion/breezeorm)), porém mantendo um design 100% *Type-Safe*.

- **Zero-Allocation Scanner**: O processamento de linhas vindas do banco de dados usa um *Plan* pré-compilado dinamicamente para cada entidade e mapeia diretamente os valores em memória usando `unsafe.Pointer`. Isso evita o gargalo do Garbage Collector com `reflect.Value`, entregando instâncias da sua Struct em **~80ns por operação** com apenas **1 alocação de memória** (a própria struct retornada).
- **Lock-Free Caches**: Caches de metadados internos (*Entity Registries* e *Data Source Registries*) operam em concorrência extrema através de padrões lock-free baseados em `atomic.Pointer` e Copy-on-Write, garantindo zero latência de trava (*lock contention*) mesmo sob carga massiva.

---

## About the Project

A type-safe, TypeORM-inspired ORM for Go, built with generics and field-pointer references instead of
struct tags or code generation. Entities are declared with plain structs, and every mapping (columns,
keys, relations, hooks, query criteria) is expressed via pointers to struct fields, checked by the Go
compiler — no reflection-by-tag magic, no codegen step.

Multi-dialect ready from day one: entities and column types (`golem.ColumnType`) are dialect-agnostic,
so adding Postgres/MySQL/MSSQL/Oracle later doesn't require rewriting entity declarations. Postgres is
the First driver (`github.com/leandroluk/golem/driver/postgres`, via `jackc/pgx/v5`).

Migrations/schema synchronization are explicitly out of scope — entities describe runtime mapping, not
a DDL source of truth. Use an external tool (Liquibase, Flyway, goose, etc.).

See `.specs/project/PROJECT.md` for the full vision/scope and `.specs/project/STATE.md` for the
history of design decisions (AD-001 through AD-017).

---

## Documentation

### Connecting to Postgres

```go
package ex

import (
  "fmt"

  "github.com/leandroluk/golem"
  postgres "github.com/leandroluk/golem/driver/postgres"
)

func main() {
  // starts a named DataSource instance
  dataSource, err := golem.NewDataSource(
    // defaults to "default" if not passed
    golem.DataSourceName("example"),
    // the connector is required — no usable DataSource without one
    postgres.New(func (o *postgres.Options) {
      // connect using a DSN
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"

      // or connect using discrete properties
      o.Host = "localhost"
      o.Port = 5432
      o.User = "postgres"
      o.Password = "1234"
      o.Database = "db"
      o.SSLMode = "disable"

      // whether executed queries get logged
      o.Logging = true
    }),
  )
  if err != nil {
    panic("could not build data source: " + err.Error())
  }
  defer dataSource.Close()

  if err := dataSource.Connect(); err != nil {
    panic("could not connect to database: " + err.Error())
  }

  fmt.Println("connected to database")

  // ...
}
```

> **Note**: if both DSN and discrete properties are set, each non-zero discrete property overrides only its
> corresponding part of the DSN — see `driver/postgres/dsn.go`'s `resolveDSN` for the exact precedence rules.

> **Retrieving a `*DataSource` from elsewhere** — `NewDataSource` registers the instance under its name (see
> above), so any other part of your program can fetch it back without threading a reference through manually:
>
> ```go
> ds, err := golem.GetDataSource("example") // or golem.GetDataSource() for the "default"-named one
> if err != nil {
>   panic(err) // golem.ErrDataSourceNotFound if that name was never created, or was already Close()'d
> }
> ```

> **`MustNewDataSource`** — panics instead of returning an error, for call sites (`main()`, test setup, a DI
> container's provider constructor) where a failed `DataSource` is unrecoverable and you'd just `panic(err)`
> yourself anyway:
>
> ```go
> dataSource := golem.MustNewDataSource(
>   postgres.New(func(o *postgres.Options) { o.DSN = dsn }),
> )
> defer dataSource.Close()
> ```

### Declaring schemas

> `golem.ColumnType` (`golem.BIGINT()`, `golem.VARCHAR(50)`, `golem.TEXT()`, `golem.BOOLEAN()`, `golem.DATETIME()`,
> `golem.UUID()`, `golem.JSON()`, etc. — full set: `BOOLEAN`, `SMALLINT`, `INTEGER`, `BIGINT`, `DECIMAL`,
> `FLOAT`, `CHAR`, `VARCHAR`, `TEXT`, `DATE`, `DATETIME`, `TIME`, `BLOB`, `UUID`, `JSON`) is dialect-agnostic —
> it doesn't turn into DDL (this `golem` doesn't generate schema, see the Migrations section) and doesn't
> depend on which adapter you connected. It's only a semantic id so the adapter knows how to bind (Go →
> driver) and scan (driver → Go) that value, since `database/sql` alone can't handle exotic types (UUID,
> JSONB, array, ENUM...) consistently across dialects. Each adapter (`postgres`, and in the future
> `mysql`/`mssql`/etc.) implements this contract:
>
>  type Dialect interface {
>    Bind(t golem.ColumnType, value any) (driver.Value, error)
>    Scan(t golem.ColumnType, raw any, dest any) error
>  }
>
> This keeps the entity 100% portable across dialects — only the `DataSource`/adapter chosen at runtime
> decides the actual dialect; the entity declaration never changes.

`entity.Table` (received inside `entity.New`'s callback) exposes:

scope:
  table:
    - `TableName(name string)`: table name; defaults to the struct name (e.g. `User` -> `"user"`)
    - `SchemaName(name string)`: table schema; defaults to whichever schema is currently selected on the connection
    - `PrimaryKey(fieldPtrs ...any)`: primary key; accepts 1+ fields (composite)
    - `Unique(fieldPtrs ...any)`: `UNIQUE` constraint; accepts 1+ fields (composite) — like `PrimaryKey`/`ForeignKey`, lives outside `Col` because uniqueness can span more than one column
    - `Index(fieldPtrs ...any) *entity.Index`: secondary index over 1+ fields
  column:
    - `Col(fieldPtr any, type golem.ColumnType) *entity.Column`: maps a struct field to a column with an explicit type
    - `ForeignKey(fieldPtr any, target *entity.Entity[T], opts ...*relation.ForeignKeyOptions)`: foreign key pointing at another entity's PK
    - `CreateDate(fieldPtr any) *entity.Column`: marks the field as "created at"; filled in automatically with the insert's timestamp
    - `UpdateDate(fieldPtr any) *entity.Column`: marks the field as "updated at"; filled in automatically with each update's timestamp
    - `DeleteDate(fieldPtr any) *entity.Column`: marks the field as "soft-deleted at" (a non-nil value means the row is deleted); `Repository[T]`'s `Delete`/`Restore` and every `Where`-capable criteria start filtering deleted rows by default — see `.WithDeleted()` in the Query Builder section

`*entity.Column` (returned by `Col`/`CreateDate`/`UpdateDate`/`DeleteDate`) chains:

- `.Name(name string)`: column name; defaults to the struct field's name
- `.Nullable()`: allows `NULL`
- `.Default(value any)`: default value (or dialect expression), becomes DDL/constraint in the database
- `.DefaultFunc(fn func() (any, error))`: like `.Default()`, but the value is computed in Go code at
  insert time instead of becoming a database expression — useful for UUIDs, slugs, a value derived from
  another field, etc. Only used when the field is still zero-valued; a returned error cancels the insert.
  (`CreateDate`/`UpdateDate` already fill in the operation's timestamp on their own — no separate
  `.AutoNow()` needed, since marking a field as "created at" and not wanting that would make no sense)

`*entity.Index` (returned by `Index`) chains:

- `.Name(name string)`: index name; defaults to a generated one (`idx_<table>_<columns>`)
- `.Unique()`: unique index

```go
package ex

import (
  "context"

  "github.com/leandroluk/golem"
  relation "github.com/leandroluk/golem/relation"
  entity "github.com/leandroluk/golem/entity"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/driver/postgres"
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

type Post struct {
  ID          int64
  CreatedAt   time.Time
  UpdatedAt   time.Time
  DeletedAt   *time.Time
  OwnerUserID int64
  Title       string
  Content     string
  Published   bool
}

type Message struct {
  ID           int64
  CreatedAt    time.Time
  UpdatedAt    time.Time
  DeletedAt    *time.Time
  Content      string
  SenderUserID int64
  PostID       int64
}

var UserEntity = entity.New[User](func(t *User, b *entity.Table) {
  // table name; if omitted, uses the struct name (e.g. "User")
  b.TableName("users")
  // table schema; if omitted, uses whichever schema is currently selected on
  // the connection (e.g. Postgres's "search_path", or the connector's default schema)
  b.SchemaName("public")

  // maps the columns in the database with their explicit types
  b.Col(&t.ID, golem.BIGINT())
  // column name; if omitted, uses the struct field's name (e.g. "Name")
  b.Col(&t.Name, golem.VARCHAR(50)).Name("full_name")
  b.Col(&t.Email, golem.VARCHAR(50))
  b.Col(&t.Age, golem.INTEGER())
  b.Col(&t.CreatedAt, golem.DATETIME())
  b.Col(&t.UpdatedAt, golem.DATETIME())
  b.Col(&t.DeletedAt, golem.DATETIME()).Nullable().Default(nil)

  // accepts several properties
  b.PrimaryKey(&t.ID)
  // lives outside Col because it may be a combination of fields (composite unique)
  b.Unique(&t.Email)

  // declares special fields, i.e. created-at/updated-at timestamp fields
  b.CreateDate(&t.CreatedAt)
  b.UpdateDate(&t.UpdatedAt)
  b.DeleteDate(&t.DeletedAt).Nullable().Default(nil)
})


var PostEntity = entity.New[Post](func(t *Post, b *entity.Table) {
  b.Col(&t.ID, golem.BIGINT())
  b.Col(&t.CreatedAt, golem.DATETIME())
  b.Col(&t.UpdatedAt, golem.DATETIME())
  b.Col(&t.DeletedAt, golem.DATETIME()).Nullable().Default(nil)
  b.Col(&t.OwnerUserID, golem.BIGINT())
  b.Col(&t.Title, golem.VARCHAR(50))
  b.Col(&t.Content, golem.TEXT())
  b.Col(&t.Published, golem.BOOLEAN())

  b.PrimaryKey(&t.ID)
  // secondary index: speeds up queries filtering by OwnerUserID (e.g. "a user's posts")
  b.Index(&t.OwnerUserID)
  // the third parameter is optional. ForeignKeyOptions only has OnDelete — it's the only
  // option with real effect given how golem is built: with no DDL (no migrations, see
  // AD-012) and no navigational field for an in-memory attached relation (see AD-001/
  // AD-024), Cascade/Persistence/OrphanedRowAction/CreateForeignKeyConstraints/Deferrable/
  // Lazy/Eager/OnUpdate would never have real behavior to hang off of — cut instead of
  // being merely accepted and ignored (see AD-032 in .specs/project/STATE.md)
  b.ForeignKey(&t.OwnerUserID, UserEntity, relation.NewForeignKeyOptions().
    // when a user is deleted, their posts are actually deleted too
    // (Repository[T].Delete consults a global FK registry and applies this)
    // other options are:
    // - relation.OnDeleteDefault (does nothing at the golem level; if a real DB constraint exists outside golem, it decides)
    // - relation.OnDeleteRestrict (blocks the delete with golem.ErrForeignKeyViolation if a referencing post exists)
    // - relation.OnDeleteSetNull
    // - relation.OnDeleteNoAction (same effect as OnDeleteDefault at the golem level)
    OnDelete(relation.OnDeleteCascade))

  // declares special fields, i.e. created-at/updated-at timestamp fields
  b.CreateDate(&t.CreatedAt)
  b.UpdateDate(&t.UpdatedAt)
  b.DeleteDate(&t.DeletedAt).Nullable().Default(nil)
})

var MessageEntity = entity.New[Message](func(t *Message, b *entity.Table) {
  b.Col(&t.ID, golem.BIGINT())
  b.Col(&t.CreatedAt, golem.DATETIME())
  b.Col(&t.UpdatedAt, golem.DATETIME())
  b.Col(&t.DeletedAt, golem.DATETIME()).Nullable().Default(nil)
  b.Col(&t.Content, golem.TEXT())
  b.Col(&t.SenderUserID, golem.BIGINT())
  b.Col(&t.PostID, golem.BIGINT())

  b.PrimaryKey(&t.ID)
  b.ForeignKey(&t.SenderUserID, UserEntity)
  b.ForeignKey(&t.PostID, PostEntity)

  b.CreateDate(&t.CreatedAt)
  b.UpdateDate(&t.UpdatedAt)
  b.DeleteDate(&t.DeletedAt).Nullable().Default(nil)
})

// entity.AddHook(Entity) returns a chainable builder (same style as ForeignKeyOptions/JoinTable):
// each method fixes one hook slot, with no need for a wrapper type per hook (no BeforeCreateHook,
// AfterCreateHook, etc). Available slots, all sharing the same signature
// func(ctx context.Context, i *T, conn golem.Conn) error:
//  - (Before|After|OnConflict)Create
//  - (Before|After|OnConflict)Update
//  - (Before|After|OnConflict)Delete
//
// every hook must return an error, and a returned error cancels the operation.
// all of them run inside the same transaction as the operation that triggered them (conn), so an
// error from any one of them rolls back everything — including the very insert/update/delete that
// triggered the hook.

var _ = entity.AddHook(UserEntity).
  // example hook before creating a user:
  BeforeCreate(func (ctx context.Context, i *User, conn golem.Conn) error {
    fmt.Println("before creating user ", i.Name)
    return nil
  }).
  // example side-query inside the hook, in the same transaction (conn) that created the user:
  AfterCreate(func (ctx context.Context, i *User, conn golem.Conn) error {
    repo := repository.Get(conn, UserEntity)
    count, err := repo.Count(ctx)
    fmt.Println("total users now:", count)
    return err
  })

/**
 * Note: registering the same hook slot twice triggers a panic naming the slot, e.g:
 * entity.AddHook(UserEntity).
 *   BeforeCreate(func (ctx context.Context, i *User, conn golem.Conn) error {
 *     fmt.Println("before creating user ", i.Name)
 *     return nil
 *   }).
 *   BeforeCreate(func (ctx context.Context, i *User, conn golem.Conn) error {
 *     fmt.Println("before creating user ", i.Name)
 *     return nil
 *   })
 */

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(UserEntity, PostEntity, MessageEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()
}

```

### Custom field types (Valuer/Scanner)

A struct field doesn't have to be a plain Go scalar. If its type implements `database/sql/driver.Valuer`,
golem calls `Value()` to get the value to write instead of using the field's raw Go value as-is; if it
implements `sql.Scanner`, golem calls `Scan(raw)` to populate it instead of trying a direct assignment.
Both are the standard library's own contracts — golem has zero knowledge of, or dependency on, whatever
type implements them:

```go
type Money struct {
  Cents int64
}

func (m Money) Value() (driver.Value, error) {
  return m.Cents, nil
}

func (m *Money) Scan(src any) error {
  v, ok := src.(int64)
  if !ok {
    return fmt.Errorf("Money: cannot scan %T", src)
  }
  m.Cents = v
  return nil
}

type Order struct {
  ID    int64
  Total Money // stored as a plain BIGINT column, wrapped in Go
}

var OrderEntity = entity.New(func(o *Order, b *entity.Table) {
  b.Col(&o.ID, golem.BIGINT())
  b.Col(&o.Total, golem.BIGINT())
  b.PrimaryKey(&o.ID)
})
```

This is the seam a dirty-tracking/optional-field wrapper (a `gonest.Accessor[T]`-shaped type, or your own)
plugs into: implement `Value()`/`Scan()` on a small adapter type wrapping it, use that adapter type as the
struct field instead of the wrapper directly, and golem round-trips it exactly like any other column, no
extra configuration needed.

### Many-to-many relations (junction entity)

> Inspired by https://typeorm.io/docs/relations/relations, but without the `@JoinTable`/`@JoinColumn`
> concept: the junction table is always a plain entity, with two foreign keys. That's literally what the
> database does under the hood, so hiding it behind a parallel API (`ManyToMany` + `JoinTable`) makes no
> sense — `ForeignKey` already covers this case.

```go
package ex

import (
  "context"

  "github.com/leandroluk/golem"
  entity "github.com/leandroluk/golem/entity"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/driver/postgres"
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

var CategoryEntity = entity.New[Category](func(t *Category, b *entity.Table) {
  b.Col(&t.ID, golem.BIGINT())
  b.Col(&t.Name, golem.VARCHAR(50))

  b.PrimaryKey(&t.ID)
})

var QuestionEntity = entity.New[Question](func(t *Question, b *entity.Table) {
  b.Col(&t.ID, golem.BIGINT())
  b.Col(&t.Title, golem.VARCHAR(50))
  b.Col(&t.Text, golem.TEXT())

  b.PrimaryKey(&t.ID)
})

// since QuestionToCategory references QuestionEntity/CategoryEntity but neither of them references
// QuestionToCategory back, there's no initialization cycle — the entities can be passed in directly
var QuestionToCategoryEntity = entity.New[QuestionToCategory](func(t *QuestionToCategory, b *entity.Table) {
  b.Col(&t.QuestionID, golem.BIGINT())
  b.Col(&t.CategoryID, golem.BIGINT())

  // composite primary key
  b.PrimaryKey(&t.QuestionID, &t.CategoryID)

  b.ForeignKey(&t.QuestionID, QuestionEntity)
  b.ForeignKey(&t.CategoryID, CategoryEntity)
})

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(QuestionEntity, CategoryEntity, QuestionToCategoryEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  // no automatic collection cascade: each entity is inserted explicitly, and the junction row is
  // just another normal insert — no "save the whole graph" magic under the hood. everything runs
  // inside a single transaction so the question is never left "orphaned" if the junction insert fails
  ctx := context.Background()

  // repository.Get only needs the connection (golem.Tx here, *golem.DataSource outside a transaction) —
  // both implement golem.Conn
  err = dataSource.Transaction(ctx, func(tx golem.Tx) error {
    categories, err := repository.Get(tx, CategoryEntity).InsertMany(ctx,
      &Category{Name: "ORMs"},
      &Category{Name: "Programming"},
    )
    if err != nil {
      return err
    }

    question, err := repository.Get(tx, QuestionEntity).Insert(ctx, &Question{
      Title: "How do I ask about golem?",
      Text:  "Where can I get golem-related questions answered?",
    })
    if err != nil {
      return err
    }

    _, err = repository.Get(tx, QuestionToCategoryEntity).InsertMany(ctx,
      &QuestionToCategory{QuestionID: question.ID, CategoryID: categories[0].ID},
      &QuestionToCategory{QuestionID: question.ID, CategoryID: categories[1].ID},
    )
    return err
  })
  if err != nil {
    panic(err)
  }
}
```

> **Note**: to load `question.Categories` as a ready-made collection (like TypeORM's eager/lazy loading),
> the idea is to solve that later via a query helper (`Preload`/`With` over `QuestionToCategoryEntity`),
> not as a new relation type. This keeps the relations API to just `ForeignKey` (one-to-many/many-to-one/
> one-to-one), which is what actually exists in the database.

### Repository (CRUD)

> No generic `Manager`: only `Repository[T]` exists, always bound to a single entity (equivalent to
> TypeORM's `dataSource.getRepository(User)`, without a parallel `dataSource.manager`). Transactions are
> the `DataSource`'s responsibility.
>
> `repository.Get[T any](conn golem.Conn, e *entity.Entity[T]) *Repository[T]` takes a `golem.Conn` —
> an interface implemented by both `*golem.DataSource` and `golem.Tx`. Outside a transaction, pass the
> `dataSource`; inside one, pass the `tx` — the repository doesn't know (and doesn't need to know) which one it is.

```go
package ex

import (
  "context"
  "fmt"

  "github.com/leandroluk/golem"
  op "github.com/leandroluk/golem/op"
  query "github.com/leandroluk/golem/query"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/driver/postgres"
)

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(UserEntity, PostEntity, MessageEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  ctx := context.Background()

  // repository.Get[T] infers T from UserEntity's type (*entity.Entity[User]), no need to write
  // repository.Get[User](...). dataSource implements golem.Conn, so it runs on its pool
  users := repository.Get(dataSource, UserEntity)

  // Insert: always 1 new entity, returned with its PK filled in
  user, err := users.Insert(ctx, &User{Name: "John Doe", Email: "john.doe@email.com", Age: 30})
  if err != nil {
    panic(err)
  }

  // SaveOne: I already have the runtime instance (came from Insert above) — persists it again by PK
  user.Age = 31
  user, err = users.SaveOne(ctx, &user)
  if err != nil {
    panic(err)
  }

  // lookup by primary key: FindOne + op.Eq (no dedicated FindByID, see AD-022)
  found, err := users.FindOne(ctx, func(t *User, q *query.Query[User]) {
    q.Where(op.Eq(&t.ID, user.ID))
  })
  if err != nil {
    panic(err)
  }

  // Find/FindOne take criteria — detailed in the "Query Builder" section below
  admins, err := users.FindMany(ctx /*, criteria here */)
  if err != nil {
    panic(err)
  }

  // Count/Exists take their own criteria type (query.Count[T], Where only) — detailed in the "Query
  // Builder" section below; with no argument at all, they count/check the whole table
  total, err := users.Count(ctx)
  if err != nil {
    panic(err)
  }
  fmt.Println("total users:", total)

  // User has DeleteDate declared, so this is a soft delete (sets DeletedAt), not a row deletion
  if err := users.Delete(ctx, &found); err != nil {
    panic(err)
  }

  // Restore undoes it: clears DeletedAt again
  if err := users.Restore(ctx, &found); err != nil {
    panic(err)
  }

  // Transactions live on DataSource, not on Repository. Inside the callback, tx (which also
  // implements golem.Conn) replaces dataSource when building repository.Get
  err = dataSource.Transaction(ctx, func(tx golem.Tx) error {
    if _, err := repository.Get(tx, PostEntity).Insert(ctx, &Post{OwnerUserID: user.ID, Title: "first post"}); err != nil {
      return err
    }
    _, err := repository.Get(tx, MessageEntity).Insert(ctx, &Message{SenderUserID: user.ID, Content: "first message"})
    return err
  })
  if err != nil {
    panic(err)
  }

  _ = admins
}
```

> **Quick reference** — `Repository[T]` methods:
>
> | Method | Returns | Description |
> | :- | :- | :- |
> | `Insert(ctx, e *T) (T, error)` | 1 row | inserts 1 new entity, returned with its PK filled in |
> | `InsertMany(ctx, entities ...*T) ([]T, error)` | N rows | inserts several at once |
> | `SaveOne(ctx, e *T) (T, error)` | 1 row | re-persists a runtime instance you already have (e.g. from a previous `Insert`/`FindOne`), by PK |
> | `SaveMany(ctx, entities ...*T) ([]T, error)` | N rows | like `SaveOne`, for several instances at once |
> | `Update(ctx, criteria func(t *T, u *query.Update[T])) ([]T, error)` | N rows | updates directly in the database by criteria (`Where`+`Set`), no runtime instance needed; 0 matched rows is not an error — there's no separate `UpdateOne`/`UpdateMany`, both did exactly the same query |
> | `Delete(ctx, entities ...*T) error` | — | delete by PK; if the entity has `DeleteDate`, sets the timestamp (soft delete) instead of removing the row |
> | `Restore(ctx, entities ...*T) error` | — | undoes a soft delete (clears `DeleteDate`) by PK; a no-op if `DeleteDate` isn't declared |
> | `FindMany(ctx, criteria ...func(t *T, q *query.Query[T])) ([]T, error)` | N rows | optional criteria; without one, brings back the whole table. Detailed in the Query Builder section |
> | `FindOne(ctx, criteria ...func(t *T, q *query.Query[T])) (T, error)` | 1 row | same as `FindMany`, capped to 1 |
> | `Count(ctx, criteria ...func(t *T, c *query.Count[T])) (int64, error)` | count | optional criteria (`Where` only); without one, counts the whole table |
> | `Exists(ctx, criteria ...func(t *T, c *query.Count[T])) (bool, error)` | bool | shortcut for `Count > 0` without fetching a row, same criteria as `Count` |
>
> `dataSource.Transaction(ctx, func(tx golem.Tx) error {...})` opens the transaction; any
> `repository.Get(tx, Entity)` created inside the callback runs on it (`tx` implements `golem.Conn`, just
> like `dataSource`). If the callback returns an error, the whole transaction is rolled back.

### Query Builder

> Criteria via a declarative closure: the callback receives `t *T` (the same field pointer as `Col`/
> `ForeignKey`/`PrimaryKey`) and `q *query.Query[T]`. `Where` conditions are variadic with AND semantics.
> Call order inside the callback doesn't matter — the query is only built once the callback returns.
>
> If the entity has `DeleteDate` (soft delete), every query filters out deleted rows by default (an
> implicit `WHERE deleted_at IS NULL`) — like every ORM with soft delete does. `.WithDeleted()` turns that
> filter off for that query. It exists on any builder with `Where` underneath: `query.Query[T]`
> (`FindMany`/`FindOne`), `query.Count[T]` (`Count`/`Exists`), `query.Update[T]` (`Update`)
> and `query.Join[T]` (inside `join.*`). On entities without `DeleteDate`, `.WithDeleted()` is a no-op.

```go
package ex

import (
  "context"
  "fmt"

  "github.com/leandroluk/golem"
  op "github.com/leandroluk/golem/op"
  query "github.com/leandroluk/golem/query"
  postgres "github.com/leandroluk/golem/driver/postgres"
  repository "github.com/leandroluk/golem/repository"
)

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(UserEntity, PostEntity, MessageEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  ctx := context.Background()

  // repository.Get[T] infers T from UserEntity's type (*entity.Entity[User]), no need to write
  // repository.Get[User](...). dataSource implements golem.Conn, so it runs on its pool
  userRepo := repository.Get(dataSource, UserEntity)

  // Insert: always 1 new entity, returned with its PK filled in
  user, err := userRepo.Insert(ctx, &User{Name: "John Doe", Email: "john.doe@email.com", Age: 30})
  if err != nil {
    panic(err)
  }

  // SaveOne: I already have the runtime instance (came from Insert above) — persists it again by PK
  user.Age = 31
  user, err = userRepo.SaveOne(ctx, &user)
  if err != nil {
    panic(err)
  }

  // SaveMany: same idea, but for several runtime instances at once (variadic, like InsertMany)
  users, err := userRepo.SaveMany(ctx, &user)
  if err != nil {
    panic(err)
  }
  _ = users

  // Update: no instance at all — just Where+Set, updates directly in the database. There's no
  // separate UpdateOne/UpdateMany — the criteria can match 1 row or several, the method is the
  // same, and 0 matched rows is not an error.
  updated, err := userRepo.Update(ctx, func(t *User, u *query.Update[User]) {
    u.Where(op.Eq(&t.ID, user.ID))
    u.Set(&t.Name, "John Doe")
    u.Set(&t.Email, "john.doe@email.com")
    u.Set(&t.Age, 30)
  })
  if err != nil {
    panic(err)
  }
  _ = updated

  // same method, criteria that can match (and update) more than one row
  users, err = userRepo.Update(ctx, func(t *User, u *query.Update[User]) {
    u.Where(op.Eq(&t.Age, 30))
    u.Set(&t.Age, 31)
  })
  if err != nil {
    panic(err)
  }
  _ = users

  found, err := userRepo.FindOne(ctx, func (t *User, q *query.Query[User]) {
    q.Select(&t.Name, &t.Email, &t.Age)
    q.Where(
      // other conditions available here: op.Or, op.In, op.Like, etc — op.Not(op.In(...)) composes negation, no dedicated NotIn
      op.Eq(&t.Name, "John Doe"),
      op.Eq(&t.Email, "john.doe@email.com"),
      op.Eq(&t.Age, 30),
    )
    q.OrderBy(op.Desc(&t.ID))
    q.Limit(10)
    q.Offset(0)
    // without this, users with DeletedAt set wouldn't show up in this FindOne
    q.WithDeleted()
  })
  if err != nil {
    panic(err)
  }

  admins, err := userRepo.FindMany(ctx, func (t *User, q *query.Query[User]) {
    // if q.Select isn't passed, every column is always returned
    q.Where(
      // other conditions available here: op.Or, op.In, op.Like, etc — op.Not(op.In(...)) composes negation, no dedicated NotIn
      op.Eq(&t.Name, "John Doe"),
      op.Eq(&t.Email, "john.doe@email.com"),
      op.Eq(&t.Age, 30),
    )
    // declaration order doesn't matter, the query is built once the function returns
    q.Limit(10)
    q.Offset(0)
    q.OrderBy(op.Desc(&t.ID))
  })
  if err != nil {
    panic(err)
  }
  _ = admins

  // Count/Exists take their own criteria type (query.Count[T], Where only); with no argument at
  // all, they count/check the whole table
  adultCount, err := userRepo.Count(ctx, func (t *User, c *query.Count[User]) {
    c.Where(op.Gte(&t.Age, 18))
  })
  if err != nil {
    panic(err)
  }
  fmt.Println("total adult users:", adultCount)

  if err := userRepo.Delete(ctx, &found); err != nil {
    panic(err)
  }

  // Transactions live on DataSource, not on Repository. Inside the callback, tx (which also
  // implements golem.Conn) replaces dataSource when building repository.Get
  err = dataSource.Transaction(ctx, func(tx golem.Tx) error {
    if _, err := repository.Get(tx, PostEntity).Insert(ctx, &Post{OwnerUserID: user.ID, Title: "first post"}); err != nil {
      return err
    }
    _, err := repository.Get(tx, MessageEntity).Insert(ctx, &Message{SenderUserID: user.ID, Content: "first message"})
    return err
  })
  if err != nil {
    panic(err)
  }
}
```

### Joins

> `golem/join` package: `join.Inner`/`join.Left`/`join.Right`/`join.Full` (SQL's `JOIN` names — `Left`/
> `Right`/`Full` are already "outer" by definition, no need for a separate generic `Outer`). Each one
> takes the outer `*query.Query[T]` (to register itself on it), the entity on the side entering the join
> (`T` inferred from it, same as `repository.Get`), and a callback for the new side. We name the builders
> `q0` (outer), `q1` (join), etc, to make each level explicit.
>
> `q1.On(fieldPtr, fieldPtr)` compares column to column (both sides are field addresses) — different from
> `op.Eq(fieldPtr, value)`, which compares a column to a literal value in `Where`. That's why it's a
> separate method instead of reusing `op.Eq` for both cases. `q1.Where(...)` also exists, to filter the
> side that entered the join (with normal `op.*`, literal values) without mixing it with the outer query's `Where`.

```go
package ex

import (
  "context"

  "github.com/leandroluk/golem"
  op "github.com/leandroluk/golem/op"
  join "github.com/leandroluk/golem/join"
  query "github.com/leandroluk/golem/query"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/driver/postgres"
)

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(UserEntity, PostEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  ctx := context.Background()

  // users who have at least 1 published post (INNER: only rows matching the On condition make it in)
  users, err := repository.Get(dataSource, UserEntity).FindMany(ctx, func (u *User, q0 *query.Query[User]) {
    join.Inner(q0, PostEntity, func (p *Post, q1 *query.Join[Post]) {
      q1.On(&p.OwnerUserID, &u.ID)
      q1.Where(op.Eq(&p.Published, true))
    })
    q0.Where(op.Eq(&u.Name, "John Doe"))
  })
  if err != nil {
    panic(err)
  }
  _ = users
}
```

### Preload / Eager Loading

> `repository.Preload[T, J](ctx, repo, items, targetEntity, criteria...)` fetches the rows related to
> `items` (the result of a previous `FindMany`/`FindOne`) and returns a `map[any][]J` grouped by the
> relation key — it NEVER attaches the result back onto `items` (there's no field like `user.Posts
> []Post` on the struct; see AD-001/AD-024 in `.specs/project/STATE.md` — this is deliberate, entities
> stay plain structs, with no navigational field).
>
> The join column is discovered automatically from the `ForeignKey` already declared between the two
> entities (works in both directions: `Preload(ctx, userRepo, users, PostEntity)` — loads each user's
> posts — or `Preload(ctx, postRepo, posts, UserEntity)` — loads each post's owner). `criteria` accepts
> the same `func(j *J, q *query.Query[J])` as `FindMany` (Where/OrderBy/Limit/Offset/WithDeleted),
> always combined (AND) with the join filter `Preload` already builds on its own.
>
> There's no flag like `Eager(true)` that automatically triggers `Preload` inside `FindMany`/`FindOne` —
> there's no way to return data of different types (`J` varies by FK) hidden behind the fixed
> `([]T, error)` signature without heavy reflection or breaking the API. Always call `repository.Preload`
> explicitly. Details: `.specs/features/preload-eager-loading/design.md`.

```go
package ex

import (
  "context"
  "fmt"

  "github.com/leandroluk/golem"
  op "github.com/leandroluk/golem/op"
  query "github.com/leandroluk/golem/query"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/driver/postgres"
)

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(UserEntity, PostEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  ctx := context.Background()
  userRepo := repository.Get(dataSource, UserEntity)

  users, err := userRepo.FindMany(ctx, func (u *User, q *query.Query[User]) {
    q.Where(op.Eq(&u.Name, "John Doe"))
  })
  if err != nil {
    panic(err)
  }

  // fetches the posts for each user returned above, grouped by User.ID
  postsByUserID, err := repository.Preload(ctx, userRepo, users, PostEntity, func (p *Post, q *query.Query[Post]) {
    q.Where(op.Eq(&p.Published, true))
    q.OrderBy(op.Desc(&p.ID))
  })
  if err != nil {
    panic(err)
  }
  for _, u := range users {
    fmt.Printf("%s has %d published posts\n", u.Name, len(postsByUserID[u.ID]))
  }
}
```

### Aggregations

> `repository.Aggregate[T, R](ctx, repo, func(t *T, res *R, a *query.Aggregate[T, R]) {...}) ([]R, error)` —
> same principle as `Preload`: `R` is any struct (doesn't need `entity.New`), resolved by field pointer
> against `t` (source, `T`) and `res` (destination, `R`), no tags.
>
> `a.GroupBy(&t.Field, &res.Field)` marks a grouping column, loading the value into the destination
> field. `a.Sum`/`a.Avg`/`a.Count` take `(sourceFieldPtr, destFieldPtr)` — the aggregate reads from `T`,
> writes to `R`. `a.CountAll(&res.Field)` is `COUNT(*)`, with no source column. `a.Where(...)` filters
> BEFORE grouping (resolved against `T`, same as `FindMany`); `a.Having(...)` filters AFTER grouping —
> each `Having` condition's `FieldPtr` must point to an `R` field already registered via
> `Sum`/`Avg`/`Count`/`CountAll` (you can't `HAVING` over a plain `GroupBy` column in this version).
> `a.OrderBy(...)` accepts both `GroupBy` and aggregate fields.
>
> `Sum`/`Avg` always come back as `float64` on the destination (the Postgres driver forces a `CAST(...
> AS DOUBLE PRECISION)` — without it, `SUM`/`AVG` over an integer column becomes `NUMERIC` in Postgres,
> which the driver doesn't decode directly as `float64`). `MIN`/`MAX` don't exist in this version (on
> purpose, not an oversight — they'd make sense on non-numeric columns like text/date, where the cast to
> `DOUBLE PRECISION` would break; see `.specs/features/aggregations/design.md`).

```go
package ex

import (
  "context"
  "fmt"

  "github.com/leandroluk/golem"
  op "github.com/leandroluk/golem/op"
  query "github.com/leandroluk/golem/query"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/driver/postgres"
)

type CategoryTotal struct {
  Category string
  Total    float64
  Count    int64
}

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(PostEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  ctx := context.Background()
  postRepo := repository.Get(dataSource, PostEntity)

  // total (and count) of posts per category, only categories with more than 1 post
  totals, err := repository.Aggregate(ctx, postRepo, func (p *Post, res *CategoryTotal, a *query.Aggregate[Post, CategoryTotal]) {
    a.GroupBy(&p.Category, &res.Category)
    a.Sum(&p.Views, &res.Total)
    a.CountAll(&res.Count)
    a.Where(op.Eq(&p.Published, true))
    a.Having(op.Gt(&res.Count, int64(1)))
    a.OrderBy(op.Desc(&res.Total))
  })
  if err != nil {
    panic(err)
  }
  for _, t := range totals {
    fmt.Printf("%s: %d posts, %.0f views\n", t.Category, t.Count, t.Total)
  }
}
```

### Pessimistic Locking

> `query.Query[T]` gains `.ForUpdate()`, `.ForNoKeyUpdate()`, `.ForShare()`, `.ForKeyShare()` — they
> become `SELECT ... FOR UPDATE`/`FOR NO KEY UPDATE`/`FOR SHARE`/`FOR KEY SHARE` on Postgres. Each one
> accepts an optional `query.LockWaitNoWait` or `query.LockWaitSkipLocked` (default: blocks until the row unlocks).
>
> **Locking a read outside a transaction is a no-op disguised as success** — the lock releases as soon
> as the isolated statement finishes, so `FindMany`/`FindOne` with `.ForUpdate()` (or any variant) return
> an error if `conn` isn't a `golem.Tx` (`repository.Get(dataSource, ...)` directly doesn't count; it
> must be `repository.Get(tx, ...)` inside `dataSource.Transaction(ctx, func(tx golem.Tx) error {...})`).
>
> Doesn't apply to `repository.Aggregate` — Postgres itself doesn't allow `FOR UPDATE` together with
> aggregate functions.

```go
package ex

import (
  "context"

  "github.com/leandroluk/golem"
  op "github.com/leandroluk/golem/op"
  query "github.com/leandroluk/golem/query"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/driver/postgres"
)

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(UserEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  ctx := context.Background()

  // read-then-write pattern: lock the row, decide what to do based on it, update it,
  // all inside the same transaction — no other transaction can lock/read this row
  // (with FOR UPDATE too) until this transaction commits or rolls back.
  err = dataSource.Transaction(ctx, func (tx golem.Tx) error {
    userRepo := repository.Get(tx, UserEntity)

    user, err := userRepo.FindOne(ctx, func (u *User, q *query.Query[User]) {
      q.Where(op.Eq(&u.ID, 42))
      q.ForUpdate() // or q.ForUpdate(query.LockWaitSkipLocked) to skip already-locked rows
    })
    if err != nil {
      return err
    }

    _, err = userRepo.Update(ctx, func (u *User, upd *query.Update[User]) {
      upd.Where(op.Eq(&u.ID, user.ID))
      upd.Set(&u.Name, "safely updated")
    })
    return err
  })
  if err != nil {
    panic(err)
  }
}
```

### Raw SQL (escape hatch)

> No builder covers 100% of the cases, so there needs to be an escape hatch to raw SQL, at two levels:
>
> `Exec` is part of `golem.Conn` (so it works the same on `*golem.DataSource` and on `golem.Tx` inside a
> transaction) — it runs any statement and returns a `golem.Result`, because different queries return
> different things (a `SELECT` has rows to iterate, an `UPDATE` without `RETURNING` only has an affected
> count, an `UPDATE ... RETURNING` has both). `golem.Result`:
>
>  type Result interface {
>    // advances to the next row (like sql.Rows.Next); false once rows run out or there are none
>    Next() bool
>    // current row as column→value; only valid after Next() == true
>    Scan() (map[string]any, error)
>    // rows affected by the statement; doesn't depend on Next/Scan, works even without RETURNING
>    RowsAffected() (int64, error)
>  }
>
> `Repository[T].Exec` runs a raw read query but scans the result into type `T` (returns `[]T` directly,
> no `Result` needed), reusing the column→field mapping that already exists (the same name declared via
> `Col(...).Name(...)`) — no tags or manual scanning needed.

```go
package ex

import (
  "context"
  "fmt"

  "github.com/leandroluk/golem"
  repository "github.com/leandroluk/golem/repository"
  postgres "github.com/leandroluk/golem/driver/postgres"
)

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      o.DSN = "postgres://postgres:1234@localhost:5432/db?sslmode=disable"
    }),
    golem.Entities(UserEntity),
  )
  if err != nil {
    panic(err)
  }
  defer dataSource.Close()

  ctx := context.Background()

  // Exec: returns golem.Result (golem.Conn: dataSource or tx, either works). RowsAffected works even
  // without RETURNING; Next/Scan only yield rows if the statement returns any (here, via RETURNING)
  result, err := dataSource.Exec(ctx, "UPDATE users SET age = age + 1 WHERE id = $1 RETURNING *", 1)
  if err != nil {
    panic(err)
  }
  affected, err := result.RowsAffected()
  if err != nil {
    panic(err)
  }
  fmt.Println("rows affected:", affected)

  for result.Next() {
    row, err := result.Scan()
    if err != nil {
      panic(err)
    }
    fmt.Println(row)
  }

  // Exec on Repository[T]: same idea, but scans the result into the entity's type
  users, err := repository.Get(dataSource, UserEntity).Exec(ctx, "SELECT * FROM users WHERE age > $1", 18)
  if err != nil {
    panic(err)
  }
  _ = users
}
```

> **Additional quick reference**:
>
> | Method | Where | Returns | Description |
> | :- | :- | :- | :- |
> | `Exec(ctx, sql string, args ...any) (golem.Result, error)` | `golem.Conn` (`DataSource`/`Tx`) | `Result` (`Next`/`Scan`/`RowsAffected`) | raw statement, no bound type |
> | `Exec(ctx, sql string, args ...any) ([]T, error)` | `Repository[T]` | N rows | raw read, scanned into type `T` |

### Errors

> Sentinels in `golem` (`golem.Err*`) for the most common cases — always check with `errors.Is`, never
> compare the string message (each dialect/driver speaks differently). The adapter (`postgres`, etc.) is
> what translates the driver's native error (e.g. Postgres's SQLSTATE `23505`) into the matching
> sentinel, preserving the original error underneath via `%w` — so `errors.Unwrap`/`errors.As` can still
> reach the driver's native error when the sentinel isn't granular enough.
>
> initial set (most common; the list grows as more dialects/cases get mapped, without breaking what
> already exists):
>
>  - `golem.ErrNotFound`: no matching row found (`FindOne`/`SaveOne` with no match — `Update` never triggers this, 0 matched rows is not an error)
>  - `golem.ErrDuplicateKey`: `Unique` constraint violation (single or composite)
>  - `golem.ErrForeignKeyViolation`: `ForeignKey` violation (points at something that doesn't exist, or deletes something still referenced)
>
> if the driver returns an error that doesn't match any mapped sentinel yet, the original error surfaces
> unchanged (never forced into a generic "unknown" sentinel) — only what's already been mapped becomes a sentinel.

```go
package ex

import (
  "context"
  "errors"
  "fmt"

  "github.com/leandroluk/golem"
  "github.com/leandroluk/golem/op"
  "github.com/leandroluk/golem/query"
  repository "github.com/leandroluk/golem/repository"
)

func handle(ctx context.Context, userRepo *repository.Repository[User]) {
  user, err := userRepo.FindOne(ctx, func(t *User, q *query.Query[User]) {
    q.Where(op.Eq(&t.ID, 999))
  })
  if errors.Is(err, golem.ErrNotFound) {
    fmt.Println("user does not exist")
    return
  }
  if err != nil {
    panic(err) // a real infra error (connection dropped, etc.)
  }

  _, err = userRepo.Insert(ctx, &User{Email: user.Email})
  if errors.Is(err, golem.ErrDuplicateKey) {
    fmt.Println("email already registered")
    return
  }
  if err != nil {
    panic(err)
  }
}
```

### Migrations

> Out of scope, on purpose. Entities here describe runtime mapping/behavior, not a source of truth for
> DDL — schema (creating/altering tables, versioning, rollback) is left to an external tool (Liquibase,
> Flyway, goose, etc.). No automatic "synchronize" like TypeORM's dev mode.

### Custom logger

```go
package ex

import (
  "github.com/leandroluk/golem"
  postgres "github.com/leandroluk/golem/driver/postgres"
)

type Logger struct {}

// ensures the Logger struct implements the golem.Logger interface
var _ golem.Logger = (*Logger)(nil)

func (l *Logger) Log(level golem.LogLevel, msg string, args map[string]any) {
  raw, _ := json.Marshal(args)
  str := string(raw)
  switch level {
  case golem.LogLevelDebug: fmt.Println("[DEBUG]: " + msg, str)
  case golem.LogLevelInfo: fmt.Println("[INFO]: " + msg, str)
  case golem.LogLevelWarn: fmt.Println("[WARN]: " + msg, str)
  case golem.LogLevelError: fmt.Println("[ERROR]: " + msg, str)
  default: panic("unknown log level " + level.String())
  }
}
func (l *Logger) Debug(msg string, args map[string]any) { l.Log(golem.LogLevelDebug, msg, args) }
func (l *Logger) Info(msg string, args map[string]any)  { l.Log(golem.LogLevelInfo,  msg, args) }
func (l *Logger) Warn(msg string, args map[string]any)  { l.Log(golem.LogLevelWarn,  msg, args) }
func (l *Logger) Error(msg string, args map[string]any) { l.Log(golem.LogLevelError, msg, args) }

func main() {
  dataSource, err := golem.NewDataSource(
    postgres.New(func (o *postgres.Options) {
      // sets which logger is used; defaults to an implementation using fmt.Println
      o.Logger = &Logger{}
    })
  )
}
```

---

## Contributors
Thanks to all the people who contribute! [[Contribute](CONTRIBUTING.md)]

---

## License
MIT License – see [LICENSE](LICENSE) file for details.
