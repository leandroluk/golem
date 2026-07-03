# Roadmap

**Current Milestone:** M14 - Pessimistic Locking
**Status:** M1-M13 done, M14 planned

Source of truth for behavior/API shape: `README.md` (this repo's root README). Each milestone below is atomic — buildable and
testable on its own, in dependency order (later milestones assume earlier ones work).

---

## M1 - Foundation

**Goal:** A `DataSource` can be created, connected, and closed against Postgres. No entities yet.
**Target:** `golem.NewDataSource(...)` + `postgres.New(...)` compiles and connects to a real Postgres instance.
**Status:** ✅ DONE — see `.specs/features/foundation/` (spec, design, tasks all Verified)

### Features

**`golem.Conn` + `DataSource`** - DONE

- `golem.NewDataSource(options...)`, `golem.DataSourceName(name)`
- `DataSource.Connect()` / `.Close()`
- `golem.Conn` interface (implemented later by both `*DataSource` and `golem.Tx`)
- `golem.Logger` interface + default console logger

**`internal/stmt` (minimal skeleton)** - PLANNED (deferred to M3, not needed until repository/query building starts)

- Internal AST package (not public API): `stmt.Select`, `stmt.Insert`, `stmt.Update`, `stmt.Delete`
- M1 scope only: table ref + PK-equality `Where` — **PK-equality means AND of 1+ column=value checks, not just 1 column**, since composite PKs already exist in the design (e.g. `QuestionToCategory` from M2, PK = `QuestionID`+`CategoryID`); `FindByID`/`SaveOne`/`Delete`/`Restore` in M3 must work against composite PKs from day one, not as a later upgrade. Full arbitrary predicate tree (OR, comparisons beyond equality), `Set`, and `Join` are added incrementally in M4/M5/M6 — see AD-016 in STATE.md

**`golem.Dialect` contract** - DONE

- Value-level: `Bind(t golem.ColumnType, value any) (driver.Value, error)`, `Scan(t golem.ColumnType, raw any, dest any) error`
- Statement-level, asymmetric by kind (not 4 symmetric methods — see AD-016):
  - `CompileSelect(s *stmt.Select) (sql string, args []any, err error)` — pure compile, 1 round-trip
  - `CompileDelete(s *stmt.Delete) (sql string, args []any, err error)` — pure compile, 1 round-trip, no row data
  - `Insert(ctx context.Context, conn golem.Conn, s *stmt.Insert) ([]map[string]any, error)` — execute, adapter picks round-trip strategy (matters for dialects without `RETURNING`, e.g. MySQL)
  - `Update(ctx context.Context, conn golem.Conn, s *stmt.Update) ([]map[string]any, error)` — same reasoning as `Insert`
- Every adapter (starting with `postgres`) implements it; `DataSource` holds the active `Dialect` for the connection it manages
- Enables `golem.ColumnType` (M2) to stay adapter-agnostic from day one

**Postgres adapter** - DONE (M1 scope: connect/close, DSN resolution, Dialect stub. Real `Bind`/`Scan` type recognition lands in M2 alongside `golem.ColumnType`'s real constructors)

- `postgres.New(func(*postgres.Options))`: DSN or discrete fields (host/port/user/password/db/sslmode), DSN+fields precedence rule (fields win)
- `o.Logging` / `o.Logger` wiring
- Implements `golem.Dialect`: Postgres-specific bind/scan (UUID, JSON/JSONB, arrays, timestamptz, etc.) plus `Insert`/`Update` via native `RETURNING` (1 round-trip, since Postgres has it)
- Driver: `github.com/jackc/pgx/v5` (already vendored in `go.mod`)

---

## M2 - Schema Declaration

**Goal:** Entities can be declared and their metadata (columns, keys, indexes) inspected — no persistence yet.
**Target:** `entity.New[User](func(t *User, b *entity.Table) {...})` builds valid metadata for all documented `entity.Table` methods.
**Status:** ✅ DONE — `entity.Table` completo: `Col` (com `Nullable`/`Default`/`DefaultFunc`), `PrimaryKey`, `ForeignKey`, `TableName`/`SchemaName`, `Unique`, `Index` (com `*entity.Index`), `CreateDate`/`UpdateDate`/`DeleteDate`. `golem.ColumnType` completo: `BIGINT`, `INT`, `VARCHAR`, `TEXT`, `BOOLEAN`, `TIMESTAMPTZ`, `UUID`, `JSON`. Pacote `index` criado. `entity.Column` completo.

### Features

**`entity.Table` (table scope)** - PARTIALLY DONE

- `TableName`, `SchemaName` (defaults: struct name, current connection schema) — DONE
- `PrimaryKey(fieldPtrs ...any)` (composite-capable) — DONE
- `Unique(fieldPtrs ...any)` (composite-capable, separate from `Col`) — DEFERRED (AD-021)
- `Index(fieldPtrs ...any) *entity.Index` (`.Name`, `.Unique`) — DEFERRED (AD-021)

**`golem.ColumnType` set** - PARTIALLY DONE

- `golem.BIGINT()`, `golem.VARCHAR(length)`, `golem.TEXT()` — DONE
- `golem.INT()`, `golem.BOOLEAN()`, `golem.TIMESTAMPTZ()`, `golem.UUID()`, `golem.JSON()` — DEFERRED, not needed by the driving example yet

**`entity.Table` (column scope) + `entity.Column`** - PARTIALLY DONE

- `Col(fieldPtr any, type golem.ColumnType) *entity.Column` — DONE
- `entity.Column`: `.Name` — DONE; `.Nullable`, `.Default(value any)`, `.DefaultFunc(func() (any, error))` — DEFERRED (AD-021)
- `CreateDate`/`UpdateDate`/`DeleteDate` — DEFERRED (AD-021, no soft-delete filtering yet either)
- `ForeignKey(fieldPtr any, target *entity.Entity[T])` two-arg form — DONE; `opts ...*relation.ForeignKeyOptions` full chain (`Cascade`, `OnDelete`, `OnUpdate`, `Deferrable`, `CreateForeignKeyConstraints`, `Lazy`, `Eager`, `Persistence`, `OrphanedRowAction`) — DEFERRED (AD-021)

---

## M3 - Repository Core CRUD

**Goal:** Entities can be inserted, re-saved, deleted/restored, and fetched by PK against a real table — **including entities with a composite PK** (e.g. `QuestionToCategory` from M2).
**Target:** `repository.Get(dataSource, UserEntity)` round-trips a row end to end; `repository.Get(dataSource, QuestionToCategoryEntity)` round-trips against a composite PK.
**Status:** ✅ DONE — see `.specs/features/repository-core-crud/` (spec, design, tasks all Verified)

### Features

**`repository.Get[T]`** - DONE

- `repository.Get[T any](conn golem.Conn, e *entity.Entity[T]) *Repository[T]`, `T` inferred from `e`

**Write paths** - PARTIALLY DONE

- `Insert(ctx, *T) (T, error)` / `InsertMany(ctx, ...*T) ([]T, error)` — DONE, via `Dialect.Insert` (direct SQL, not `stmt.Insert` — AD-020); zero-valued fields are omitted so DB-side defaults (e.g. `BIGSERIAL`) apply
- `SaveOne(ctx, *T) (T, error)` / `SaveMany(ctx, ...*T) ([]T, error)` — DEFERRED (AD-021)
- `Delete(ctx, ...*T) error` / `Restore(ctx, ...*T) error` — DEFERRED (AD-021, no `DeleteDate` support yet)

**Read paths (PK only)** - PARTIALLY DONE

- `FindByID(ctx, id any) (T, error)` — DONE for single-column PK only (composite-PK `FindByID` deferred, AD-021), via `Dialect.FindByID` (direct SQL); no soft-delete filtering yet (no `DeleteDate` support)

---

## M4 - Query Builder & Read Paths

**Goal:** Arbitrary filtered reads work: `FindMany`/`FindOne` with `Where`/`Select`/`OrderBy`/`Limit`/`Offset`.
**Target:** The `FindOne`/`FindMany` examples in `README.md` run against a real table.
**Depends on:** extends `stmt.Select` (M1/M3) from PK-equality-only to a full AND/OR/NOT predicate tree (AD-016).

### Features

**`query.Query[T]`** - DONE

- `Select(fieldPtrs ...any)`, `Where(conditions ...op.Condition)` (AND semantics), `OrderBy(...)`, `Limit`, `Offset`
- `.WithDeleted()` — disables the default soft-delete filter for this query

**`op` package** - DONE

- Comparisons: `op.Eq`, `op.Gt`, `op.Gte`, `op.Lt`, `op.Lte`, `op.In`, `op.Like` (exact set TBD, grows on demand)
- Logical: `op.Or(...)` (AND is implicit/variadic via `Where(...)` itself); `op.Not(condition)` composes over any condition instead of dedicated negated variants — `op.Not(op.In(...))` instead of a separate `NotIn`, `op.Not(op.Eq(...))` instead of `NotEq`, etc. (`NOT (x IN (...))` and `NOT IN` are semantically identical in SQL, so no functional gap)
- Ordering: `op.Asc`/`op.Desc` for `OrderBy`

**`Repository[T]` wiring** - DONE

- `FindMany(ctx, criteria ...func(t *T, q *query.Query[T])) ([]T, error)`
- `FindOne(ctx, criteria ...func(t *T, q *query.Query[T])) (T, error)`

---

## M5 - Update/Count Builders

**Goal:** Criteria-based updates and counts work without needing an in-memory instance.
**Target:** `UpdateOne`/`UpdateMany`/`Count`/`Exists` examples in README run against a real table.
**Depends on:** extends `stmt.Update` (M1/M3) with a `Set` clause + full predicate `Where`; `stmt.Select` reused (with a `COUNT(*)` projection mode) for `Count`/`Exists`.

### Features

**`query.Update[T]`** - DONE

- `Where(...)`, `Set(fieldPtr any, value any)`, `.WithDeleted()`

**`query.Count[T]`** - DONE

- `Where(...)`, `.WithDeleted()`

**`Repository[T]` wiring** - DONE

- `UpdateOne(ctx, func(t *T, u *query.Update[T])) (T, error)` / `UpdateMany(...) ([]T, error)`
- `Count(ctx, criteria ...func(t *T, c *query.Count[T])) (int64, error)` / `Exists(...) (bool, error)`

---

## M6 - Joins

**Goal:** Queries can join across entities for filtering, including the many-to-many-via-junction-entity pattern.
**Target:** The `join.Inner` example in README (users with a published post) runs against real tables.
**Depends on:** adds a `Join` list to `stmt.Select` (kind, target table, `On` predicate, joined-side `Where` predicate).

### Features

**`join` package** - DONE

- `join.Inner`/`join.Left`/`join.Right`/`join.Full`, each `(q *query.Query[T], target *entity.Entity[J], func(j *J, q1 *query.Join[J]))`
- `query.Join[T]`: `On(fieldPtr, fieldPtr)` (column-to-column), `Where(...)` (column-to-value), `.WithDeleted()`

---

## M7 - Hooks

**Goal:** Lifecycle hooks run inside the same transaction as the operation that triggered them.
**Status:** ✅ DONE — see `.specs/features/hooks/` (spec, design, tasks all Verified)

### Features

**Fluent hook builder** - DONE

- `entity.AddHook(Entity)` returns a chainable builder: `BeforeCreate`/`AfterCreate`/`OnConflictCreate` × `Create`/`Update`/`Delete` (9 total, all `func(ctx context.Context, i *T, conn golem.Conn) error`)
- Registering the same slot twice on the same entity panics with the slot name
- Hook errors cancel and roll back the triggering operation

---

## M8 - Transactions

**Goal:** `dataSource.Transaction` provides a real `golem.Tx` that both `repository.Get` and hooks can use interchangeably with `*DataSource`.
**Status:** ✅ DONE — see `.specs/features/transactions/` (spec, design, tasks all Verified)

### Features

**`golem.Tx`** - DONE

- `dataSource.Transaction(ctx, func(tx golem.Tx) error) error` — commits on nil, rolls back on error
- `golem.Tx` implements `golem.Conn` (so `repository.Get(tx, Entity)` works identically to `repository.Get(dataSource, Entity)`)
- v1 uses the driver/DB default isolation level only — no option to request `SERIALIZABLE`/`REPEATABLE READ`/etc. Configurable isolation level is a deferred idea (see STATE.md), not in M8 scope

---

## M9 - Raw SQL

**Goal:** Anything the builders can't express is still reachable.
**Status:** ✅ DONE — see `.specs/features/raw-sql/` (spec, design, tasks all Verified)

### Features

**`golem.Conn.Exec`** - DONE

- `Exec(ctx, sql string, args ...any) (golem.Result, error)`
- `golem.Result`: `Next() bool`, `Scan() (map[string]any, error)`, `RowsAffected() (int64, error)`

**`Repository[T].Exec`** - DONE

- `Exec(ctx, sql string, args ...any) ([]T, error)` — scans using the same column→field mapping as `Col`

---

## M10 - Typed Errors

**Goal:** Callers can branch on error kind without string-matching driver messages.
**Target:** `errors.Is(err, golem.ErrNotFound)` etc. work for the documented sentinel set.
**Status:** ✅ DONE

### Features

**Sentinel errors** - DONE

- `golem.ErrNotFound`, `golem.ErrDuplicateKey`, `golem.ErrForeignKeyViolation` (`errors.go`)
- Postgres adapter (`driver/postgres/dialect.go`'s `mapError`) maps SQLSTATE codes (`23505` → `ErrDuplicateKey`, `23503` → `ErrForeignKeyViolation`) and wraps with `%w` (Go 1.20+ multi-`%w`) so the native `*pgconn.PgError` stays reachable via `errors.As`
- Unmapped driver errors (including non-Postgres ones) pass through unchanged — no forced generic "unknown" sentinel
- Covered by `driver/postgres/errors_test.go` (unit, all 4 SQLSTATE/pass-through cases) and `.examples/postgres-minimal-blog`'s `TestBlogExample_TypedErrors` (integration, real constraint violations)

---

## M11 - Relations (`ForeignKeyOptions` + Cascade)

**Goal:** `entity.Table.ForeignKey` accepts the full `relation.ForeignKeyOptions` chain documented in README.md (`Cascade`, `OnDelete`, `OnUpdate`, `Deferrable`, `CreateForeignKeyConstraints`, `Lazy`, `Eager`, `Persistence`, `OrphanedRowAction`), and the options that have a runtime meaning (not just DDL/documentation) actually change `Repository[T]` write behavior.
**Target:** The `Post`/`User` example in README's Schema Declaration section (with the full options chain) compiles and its `OnDelete` cascade behavior is exercised against real Postgres.
**Status:** ✅ DONE — see `.specs/features/relations/` (spec, design, tasks all Verified)

### Features

**`relation` package** - DONE

- `relation.NewForeignKeyOptions()` fluent builder: `.Cascade(...)`, `.OnDelete(...)`, `.OnUpdate(...)`, `.Deferrable(...)`, `.CreateForeignKeyConstraints(bool)`, `.Lazy(bool)`, `.Eager(bool)`, `.Persistence(bool)`, `.OrphanedRowAction(...)` — 100% test coverage
- `entity.Table.ForeignKey(fieldPtr any, target any, opts ...*relation.ForeignKeyOptions)` — 3rd arg variadic, backward-compatible with the existing 2-arg form. Also fixes a pre-existing bug: `target` was previously never even type-asserted/read
- Since migrations/DDL are permanently out of scope (AD-012) and entities never carry a navigational relation field (AD-001/AD-024), only `OnDelete` has real runtime effect in this pass — `Cascade*`/`Persistence`/`OrphanedRowAction`/`CreateForeignKeyConstraints`/`Deferrable` are accepted and stored (never silently dropped) but have no runtime behavior; `Eager` is stored, wired to real preloading in M12; `OnUpdate` is stored, not yet wired to any operation (no PK-mutating operation exists) — see `.specs/features/relations/design.md` for the full reasoning (AD-027 in STATE.md)

**FK registry + cascade** - DONE

- `entity.ForeignKeysReferencing(targetTable string) []FKRegistration` — package-level registry, populated as a side effect of `entity.New`/`ForeignKey`, indexed by the parent (target) side
- `Repository[T].Delete` applies `OnDeleteCascade`/`OnDeleteSetNull`/`OnDeleteRestrict` for every FK registered against the entity being deleted, honoring the child's own soft-delete config, wrapped in an implicit transaction when needed (reuses an existing `Tx` instead of nesting)
- `OnDeleteRestrict` returns `golem.ErrForeignKeyViolation` (M10 sentinel, reused) when blocked

---

## M12 - Preload / Eager Loading

**Goal:** Related rows can be fetched alongside a parent query without a dedicated relation/navigational-collection type on the struct (keeps AD-001's "plain struct" stance; see AD-024).
**Target:** The `README.md`-documented `Preload` example runs against real tables.
**Status:** ✅ DONE — see `.specs/features/preload-eager-loading/` (spec, design, tasks all Verified)

### Features

**`repository.Preload[T, J any]`** - DONE

- `repository.Preload(ctx, r *Repository[T], items []T, target *entity.Entity[J], criteria ...func(*J, *query.Query[J])) (map[any][]J, error)` — join column auto-discovered from the M11 FK registry, works in either FK direction, criteria mirrors `FindMany`'s shape
- `ForeignKeyOptions.Eager(true)` is NOT auto-wired into `FindMany`/`FindOne` — hits a real Go-generics wall (variable-per-FK related type, fixed `([]T, error)` signature); accepted/stored metadata only, see design.md. Callers call `Preload` explicitly instead.

---

## M13 - Aggregations

**Goal:** `query.Query[T]` supports `GroupBy`/`Sum`/`Avg`/`Having` for read paths that need aggregate results instead of full rows.
**Target:** README examples using `GroupBy`/aggregate functions run against real tables.
**Status:** ✅ DONE — see `.specs/features/aggregations/` (spec, design, tasks all Verified)

### Features

**`repository.Aggregate[T, R any]`** - DONE

- `repository.Aggregate(ctx, r *Repository[T], fn func(t *T, res *R, a *query.Aggregate[T, R])) ([]R, error)` — `R` is a plain struct (not an `entity.Entity`), resolved by field-pointer offset like everywhere else
- `query.Aggregate[T, R]`: `GroupBy`, `Sum`/`Avg`/`Count(sourceFieldPtr, destFieldPtr)`, `CountAll(destFieldPtr)`, `Where` (pre-aggregation, against `T`), `Having` (post-aggregation, against `R`, must reference a registered aggregate field), `OrderBy`, `Limit`/`Offset`/`WithDeleted`
- `Sum`/`Avg` always yield `float64` (Postgres dialect casts to `DOUBLE PRECISION` so pgx never returns `pgtype.Numeric` for an integer column)
- `Min`/`Max` deliberately not included (would need per-column-type-aware casting; out of this pass's stated scope) — see design.md

---

## M14 - Pessimistic Locking

**Goal:** `query.Query[T]` supports `SELECT ... FOR UPDATE` (and dialect-appropriate variants) for read-then-write concurrency control.
**Target:** A `.ForUpdate()`-style example on `FindOne`/`FindMany` runs against real tables inside a transaction.
**Status:** PLANNED

---

## Future Considerations

- Additional adapters beyond Postgres — MySQL/SQLite are moderate effort (closer to ANSI SQL); MSSQL/Oracle are higher effort (syntax diverges more, see AD-015 in STATE.md). Oracle specifically needs an identifier-length decision (historically 30 bytes pre-12.2, 128 from 12.2+) — validate/truncate at entity-registration time vs. let the driver error surface as-is, TBD when that adapter is actually built
- Configurable transaction isolation level on `dataSource.Transaction` (v1 ships with driver/DB default only, see M8)


