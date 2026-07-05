# golem

**Vision:** A type-safe, TypeORM-inspired ORM for Go, built with generics and field-pointer references instead of struct tags or code generation.
**For:** Go backend developers who want TypeORM-like ergonomics (entities, repositories, query builder, relations) without reflection-by-tag magic or a codegen step.
**Solves:** Existing Go ORMs force a trade-off — heavy struct tags (gorm, bun) or a codegen pipeline (ent, sqlboiler). `golem` avoids both: entities are declared with plain structs, and every mapping (columns, keys, relations, hooks, query criteria) is expressed via pointers to struct fields, checked by the Go compiler.

## Goals

- Type-safe CRUD and querying with zero `interface{}`/tag parsing at runtime
- Minimal conceptual surface: no hidden relation types (`ManyToMany`/`JoinTable`) — a join table is just another entity with two `ForeignKey`s
- Escape hatches for anything the builder can't express (`Exec` for raw SQL) instead of growing the builder indefinitely
- Multi-dialect ready from day one: entities and column types (`golem.ColumnType`) are dialect-agnostic, so adding Postgres/MySQL/MSSQL/Oracle later doesn't require rewriting entity declarations (see AD-015 in `STATE.md`)

## Tech Stack

**Core:**

- Language: Go 1.25
- Module: `github.com/leandroluk/golem` (standalone repo — this project *is* the ORM, not a monorepo subpath; port of a design originally drafted as `gox/orm` inside the `gox` monorepo)
- First driver: `github.com/leandroluk/golem/driver/postgres` (others — MySQL, MSSQL, Oracle — added later behind the same `golem.Dialect` contract, effort varies: MySQL/SQLite closer to ANSI SQL, MSSQL/Oracle diverge more)

**Key dependencies:** `github.com/jackc/pgx/v5` for the Postgres adapter; core package stays zero-dependency (each adapter is the only place a driver dependency is expected).

## Scope

**v1 includes** (all designed and documented in `README.md`):

- `DataSource` + connector setup (`golem.NewDataSource`, `postgres.New`, named data sources) and the `golem.Dialect` contract adapters implement for value bind/scan
- Dialect-agnostic `golem.ColumnType` set (`golem.BIGINT()`, `golem.VARCHAR(n)`, `golem.UUID()`, `golem.JSON()`, etc.)
- Entity/schema declaration (`entity.New` + `entity.Table`: `Col`, `PrimaryKey`, `Unique`, `Index`, `ForeignKey`, `TableName`/`SchemaName`, `CreateDate`/`UpdateDate`/`DeleteDate`, `.Default`/`.DefaultFunc`)
- Relations expressed as plain entities with `ForeignKey` (including many-to-many via an explicit junction entity — no dedicated relation type)
- Fluent hooks (`entity.AddHook(Entity).BeforeCreate(...).AfterCreate(...)` etc., all `Before/After/OnConflict × Create/Update/Delete`)
- `Repository[T]` CRUD: `Insert`/`InsertMany`, `SaveOne`/`SaveMany`, `Update`, `Delete`/`Restore`, `FindMany`/`FindOne`, `Count`/`Exists`
- Query builder (`query.Query[T]`: `Select`/`Where`/`OrderBy`/`Limit`/`Offset`), `query.Update[T]` (`Where`+`Set`), `query.Count[T]` (`Where`)
- Joins (`golem/join`: `Inner`/`Left`/`Right`/`Full`, `query.Join[T]` with `On`/`Where`)
- Transactions (`dataSource.Transaction`) and the `golem.Conn` abstraction shared by `*DataSource` and `golem.Tx`
- Raw SQL escape hatch (`golem.Conn.Exec` → `golem.Result` with `Next`/`Scan`/`RowsAffected`; `Repository[T].Exec` → `[]T`)
- Typed sentinel errors (`golem.ErrNotFound`, `golem.ErrDuplicateKey`, `golem.ErrForeignKeyViolation`, extensible)
- Soft delete (`DeleteDate`) with default filtering on every `Where`-based builder, `.WithDeleted()` opt-out, and `Restore`

**Explicitly out of scope:**

- Migrations / schema synchronization — entities describe runtime mapping, not DDL source of truth; use an external tool (Liquibase, Flyway, goose, etc.)
- Navigational eager-load relation fields (TypeORM-style `question.categories` auto-populated) — deferred to a future `Preload`/`With` query helper, not a relation type
- Aggregations (`GroupBy`/`Sum`/`Avg`/`Having`) and pessimistic locking (`FOR UPDATE`) — not needed yet (YAGNI), revisit if a real use case appears
- Struct tags and code generation, by design

## Constraints

- Zero dependencies in the core package, heavy use of generics, convention-over-configuration defaults
- Every new builder method must justify why it can't be expressed with an existing one (avoid API surface growth) — see `STATE.md` for the pattern of rejected alternatives (`Manager`, `ManyToMany`/`JoinTable`, two-phase `entity.New()+Define()`)
- Releases stay on major version `0` forever — never cut a `v1.0.0` or higher. Go's module system requires any `v2+` module to carry a `/v2`, `/v3`, ... suffix on its import path (`github.com/leandroluk/golem/v2`), which would force every consumer to change their import on any future breaking change. Staying on `0.x.y` keeps `github.com/leandroluk/golem` as the import path permanently. Breaking changes bump the minor (`0.MAJOR.MINOR` in spirit, `0.x.y` in `go.mod`/tag syntax) instead of the major; see AD-033.

## Provenance

This project's design (vision, milestones, all architectural decisions) was originally drafted at `gox/orm/.specs` as a subpath of the `gox` monorepo, then moved to implement standalone in this repo (`golem`) instead. `.specs/project/STATE.md` decisions AD-001 through AD-017 predate this repo but still apply; only import paths changed (`github.com/leandroluk/gox/orm` → `github.com/leandroluk/golem`, `orm.*` symbol references → `golem.*`).


