# Roadmap

**Current Milestone:** — (M1-M14 done, no milestone currently planned — see STATE.md's Deferred Ideas / Future Considerations below for candidates)
**Status:** M1-M14 done

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
**Status:** ✅ DONE — `entity.Table` completo: `Col` (com `Nullable`/`Default`/`DefaultFunc`), `PrimaryKey`, `ForeignKey` (com o chain completo de `relation.ForeignKeyOptions`, ver M11), `TableName`/`SchemaName`, `Unique`, `Index` (com `*entity.Index`), `CreateDate`/`UpdateDate`/`DeleteDate`. `golem.ColumnType` completo: `BOOLEAN`, `SMALLINT`, `INTEGER`, `BIGINT`, `DECIMAL`, `FLOAT`, `CHAR`, `VARCHAR`, `TEXT`, `DATE`, `DATETIME`, `TIME`, `BLOB`, `UUID`, `JSON`. Pacote `index` criado. `entity.Column` completo. (Itens antes listados como DEFERRED/AD-021 abaixo foram todos construídos em passes posteriores — texto de escopo original preservado como registro histórico do que M2 cobria no dia em que essa spec foi escrita; ver `.specs/project/STATE.md` pelas ADs de cada item.)

### Features

**`entity.Table` (table scope)** - DONE

- `TableName`, `SchemaName` (defaults: struct name, current connection schema) — DONE
- `PrimaryKey(fieldPtrs ...any)` (composite-capable) — DONE
- `Unique(fieldPtrs ...any)` (composite-capable, separate from `Col`) — DONE
- `Index(fieldPtrs ...any) *entity.Index` (`.Name`, `.Unique`) — DONE

**`golem.ColumnType` set** - DONE

- `golem.BOOLEAN()`, `golem.SMALLINT()`, `golem.INTEGER()`, `golem.BIGINT()`, `golem.DECIMAL(precision, scale)`, `golem.FLOAT(precision)`, `golem.CHAR(length)`, `golem.VARCHAR(length)`, `golem.TEXT()`, `golem.DATE()`, `golem.DATETIME()`, `golem.TIME()`, `golem.BLOB()`, `golem.UUID()`, `golem.JSON()`

**`entity.Table` (column scope) + `entity.Column`** - DONE

- `Col(fieldPtr any, type golem.ColumnType) *entity.Column` — DONE
- `entity.Column`: `.Name`, `.Nullable`, `.Default(value any)`, `.DefaultFunc(func() (any, error))` — DONE
- `CreateDate`/`UpdateDate`/`DeleteDate` (soft-delete filtering wired into every `Where`-capable builder, see M4/M5) — DONE
- `ForeignKey(fieldPtr any, target *entity.Entity[T], opts ...*relation.ForeignKeyOptions)` — DONE, full `relation.ForeignKeyOptions` chain (`Cascade`, `OnDelete`, `OnUpdate`, `Deferrable`, `CreateForeignKeyConstraints`, `Lazy`, `Eager`, `Persistence`, `OrphanedRowAction`) — see M11 for which options have real runtime effect vs. metadata-only

---

## M3 - Repository Core CRUD

**Goal:** Entities can be inserted, re-saved, deleted/restored, and fetched by PK against a real table — **including entities with a composite PK** (e.g. `QuestionToCategory` from M2).
**Target:** `repository.Get(dataSource, UserEntity)` round-trips a row end to end; `repository.Get(dataSource, QuestionToCategoryEntity)` round-trips against a composite PK.
**Status:** ✅ DONE — see `.specs/features/repository-core-crud/` (spec, design, tasks all Verified)

### Features

**`repository.Get[T]`** - DONE

- `repository.Get[T any](conn golem.Conn, e *entity.Entity[T]) *Repository[T]`, `T` inferred from `e`

**Write paths** - DONE

- `Insert(ctx, *T) (T, error)` / `InsertMany(ctx, ...*T) ([]T, error)` — via `Dialect.Insert`; zero-valued fields are omitted so DB-side defaults (e.g. `BIGSERIAL`) apply
- `SaveOne(ctx, *T) (T, error)` / `SaveMany(ctx, ...*T) ([]T, error)` — re-persists an existing runtime instance by PK (composite-PK capable)
- `Delete(ctx, ...*T) error` / `Restore(ctx, ...*T) error` — soft-deletes (sets `DeleteDate`) when the entity has one, hard-deletes otherwise; also runs `OnDelete` cascade (M11)

**Read paths** - DONE

- No dedicated `FindByID` — removed (AD-022), superseded by `FindOne(ctx, func(t *T, q *query.Query[T]) { q.Where(op.Eq(&t.ID, id)) })`, which works for composite PKs too
- Soft-delete filtering (`WHERE deletedAtCol IS NULL`) applied by default on every `Where`-capable read, `.WithDeleted()` opts out

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
**Target:** `Update`/`Count`/`Exists` examples in README run against a real table.
**Depends on:** extends `stmt.Update` (M1/M3) with a `Set` clause + full predicate `Where`; `stmt.Select` reused (with a `COUNT(*)` projection mode) for `Count`/`Exists`.

### Features

**`query.Update[T]`** - DONE

- `Where(...)`, `Set(fieldPtr any, value any)`, `.WithDeleted()`

**`query.Count[T]`** - DONE

- `Where(...)`, `.WithDeleted()`

**`Repository[T]` wiring** - DONE

- `Update(ctx, func(t *T, u *query.Update[T])) ([]T, error)` — no `UpdateOne`/`UpdateMany` split (collapsed, AD-031 in STATE.md — they built the identical query, zero rows affected is not an error)
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

**Goal:** `entity.Table.ForeignKey` accepts a `relation.ForeignKeyOptions` chain, and the options actually change `Repository[T]` write behavior (not just DDL/documentation).
**Target:** The `Post`/`User` example in README's Schema Declaration section compiles and its `OnDelete` cascade behavior is exercised against real Postgres.
**Status:** ✅ DONE — see `.specs/features/relations/` (spec, design, tasks all Verified; note: originally shipped with a 9-option chain, later trimmed to just `OnDelete` — see AD-032 in STATE.md; the spec/design/tasks docs describe the original 9-option shape as historical record, this ROADMAP section reflects the current, trimmed one)

### Features

**`relation` package** - DONE

- `relation.NewForeignKeyOptions()` builder: `.OnDelete(...)` only — 100% test coverage
- `entity.Table.ForeignKey(fieldPtr any, target any, opts ...*relation.ForeignKeyOptions)` — 3rd arg variadic, backward-compatible with the existing 2-arg form. Also fixed a pre-existing bug: `target` was previously never even type-asserted/read
- `Cascade`/`OnUpdate`/`Deferrable`/`CreateForeignKeyConstraints`/`Lazy`/`Eager`/`Persistence`/`OrphanedRowAction` were built in M11's first pass, then removed (AD-032): none of them ever had a path to real runtime effect given golem's architecture (no DDL generation, ever — AD-012; no navigational relation field, ever — AD-001/AD-024; `Eager` specifically decided to stay manual-only — AD-028) — kept only as long as there was some chance one might get wired up later, and once M12/M13 shipped without needing any of them, keeping them "accepted but silently inert" was assessed as a footgun (an API surface that looks like it does something but doesn't) rather than a placeholder worth its confusion cost

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
**Status:** ✅ DONE — see `.specs/features/pessimistic-locking/` (spec, design, tasks all Verified)

### Features

**`query.Query[T]` locking** - DONE

- `.ForUpdate(wait ...query.LockWait)`, `.ForNoKeyUpdate(...)`, `.ForShare(...)`, `.ForKeyShare(...)` — map to Postgres's `SELECT ... FOR {UPDATE|NO KEY UPDATE|SHARE|KEY SHARE} [NOWAIT|SKIP LOCKED]`
- `Repository[T].FindMany`/`FindOne` reject a locked query outside a real `golem.Tx` (locking outside a transaction is a no-op that looks like it worked)
- `repository.Aggregate` doesn't support locking — Postgres itself rejects `FOR UPDATE` combined with aggregates/`GROUP BY`
- Verified against real Postgres with an actual two-transaction blocking test, not just SQL-shape assertions

---

## M15 - Cross-Dialect Conformance Suite

**Goal:** Extract cross-cutting behavior contracts (bind/scan round-trip, CRUD, joins, soft delete, cascade, aggregates, locking, conflict detection) into a reusable, dialect-agnostic harness — so every adapter from M16 onward is verified against the identical guarantees instead of hand-copied ad hoc tests per adapter.
**Target:** `internal/dialecttest` exposes a single conformance entrypoint (`Run`); `driver/postgres` calls it from a new `conformance_integration_test.go`.
**Status:** ✅ DONE — `task test-integration` passes: `TestPostgres_Conformance` green against real Postgres, `.examples/postgres-minimal-blog`'s pre-existing suite unaffected. Along the way, the harness immediately found and fixed 3 real bugs in `driver/postgres`/`repository` that no prior test (unit or integration) had exercised — see AD-037.

### Features

**Conformance harness** (`internal/dialecttest`) - DONE

- `Run(t, schema, caps, opts ...golem.Option)` — builds a uniquely-named `*DataSource` (`t.Name()`, per AD-035), runs `schema`'s DDL once, dispatches to 9 subtest groups
- Own fixed logical schema (`Widget`/`Deleted`/`Parent`/`CascadeChild`/`SetNullChild`/`RestrictChild`, table-prefixed `conf_*`) — no dependency on `.examples/postgres-minimal-blog`'s entities; each adapter supplies its own dialect-correct `CREATE TEMPORARY TABLE` DDL for that fixed shape (harness never generates DDL, see AD-012)
- Bind/Scan round-trip for every `golem.ColumnType` kind (`BindScanRoundTrip` group)
- CRUD (`Insert`/`InsertMany`/`SaveOne`/`Update`/`FindOne`/`FindMany`/`Count`/`Exists`/`Delete`) + `Where`/`OrderBy`/`Limit`/`Offset` (`CRUD` group)
- Soft delete filter + `.WithDeleted()`/`Restore` (`SoftDelete` group, M12); cascade delete/set-null/restrict (`Cascade` group, M11, one physical child table per mode)
- `join.Inner` (`Joins` group, M6); `repository.Preload` (`Preload` group, M12); `repository.Aggregate` (`Aggregates` group, M13)
- Pessimistic locking (`Locking` group, M14) — `Capabilities.Locking` lets a dialect mark any strength/wait mode unsupported; the harness `t.Skip`s exactly that subtest instead of failing or silently omitting it
- `golem.ErrDuplicateKey`/`golem.ErrForeignKeyViolation` via `errors.Is` (`ConflictDetection` group, M10)

**`driver/postgres` as first caller** - DONE

- New `driver/postgres/conformance_integration_test.go` (own `//go:build integration` tag): `postgresConformanceSchema` (6 `CREATE TEMPORARY TABLE` statements) + `postgresCapabilities` (every field `true`) + `TestPostgres_Conformance` calling `dialecttest.Run`
- No changes to `.examples/postgres-minimal-blog` (stays a hand-written narrative example) or `driver/postgres/connector_integration_test.go` (adapter-specific connection plumbing, out of scope per spec.md)

**3 real bugs found and fixed** (AD-037) - DONE

- `pgtype.Numeric`/`pgtype.Time`/`[16]byte`/JSON object-or-array pgx-native types leaking past the `Dialect` boundary into `repository`'s generic row-scanning, breaking any plain (non-aggregate) read of a `DECIMAL`/`TIME`/`UUID`/`JSON` column — fixed with a new `normalizeRow`/`normalizeRows` step in `driver/postgres/dialect.go`
- `repository.assignFieldValue` couldn't scan a non-NULL raw value into a nullable (`*X`) Go field — affects any `*time.Time`/similar nullable field on a row where that column happens to be non-NULL; fixed by wrapping the value in a new `*X` when `X` matches/converts, instead of only handling the identical/`ConvertibleTo` cases

**Todo (not blocking this milestone, but real gaps to close before M16+)**:
- [ ] CI doesn't run `task test-integration` / any integration tests at all yet (`.github/workflows/ci.yml` only runs `-short` unit tests) — wiring that in means this conformance proof (and every future adapter's) runs on every push, not just whoever's machine happens to have Docker that day

Full spec/design/tasks: `.specs/features/cross-dialect-conformance-suite/`.

---

## M16 - MySQL/MariaDB Adapter

**Goal:** `driver/mysql` implements `golem.Dialect`/`golem.Connector` for MySQL 8+/MariaDB, passing the M15 conformance suite.
**Target:** `driver/mysql/conformance_integration_test.go` calling `internal/dialecttest.Run`, verified against a real MySQL 8 container.
**Status:** ✅ DONE — `task test-integration` passes: `TestMySQL_Conformance` green against real MySQL 8, Postgres's own suite unaffected. Found and fixed 2 real core gaps and a MySQL-specific row-normalization requirement along the way — see AD-038/AD-039 in STATE.md.

### Features

**`driver/mysql`** (`github.com/go-sql-driver/mysql` + `database/sql`) - DONE

- Bind/Scan per INSIGHT.md's type table (`TINYINT(1)` for BOOLEAN, `CHAR(36)` for UUID — no native UUID type); `Bind`/`Scan` remain unit-tested directly but are dead code in the real path, same as `driver/postgres` (AD-037)
- `Insert`/`Update` as multi-round-trip `Execute` (no `RETURNING`) — `Insert` resolves primary key values from `stmt.Insert.PrimaryKey` (new field, AD-038) + `LastInsertId()`, then reads the row back by PK; `Update` captures matching primary keys *before* running the `UPDATE` (via the new `stmt.Update.PrimaryKey`), since re-running the original `WHERE` afterward can't work when `Sets` modifies a column `Where` itself filters on
- `driver/mysql/dialect.go`'s `normalizeCell` (column-type-aware, using `rows.ColumnTypes()`) converts `[]byte` DECIMAL text to `float64` and `[]byte` TIME text to `time.Time` — MySQL's driver returns many different column types as the same ambiguous `[]byte` Go type, unlike Postgres's self-describing `pgtype.*` wrapper types (AD-039)
- `INSERT ... ON DUPLICATE KEY UPDATE`-shaped upsert path; `IsConflict`/`mapError` map MySQL errors 1062 (duplicate key), 1451/1452 (FK violation)
- Locking: `FOR UPDATE`/`FOR SHARE` supported (MySQL 8.0.1+ also `NOWAIT`/`SKIP LOCKED`); no `NO KEY UPDATE`/`KEY SHARE` equivalent — `lockClauseSQL`'s `default:` case errors on those two strengths, same pattern as Postgres's own unsupported-value handling
- `LIMIT ? OFFSET ?` pagination (same shape as Postgres, `?` placeholders instead of `$N`); backtick identifier quoting

**Docker/Taskfile wiring** - DONE

- `.docker/docker-compose.test.yml` gained a `mysql` service (port 53306); `Taskfile.yml`'s `test-integration` gained `GOLEM_MYSQL_TEST_DSN`
- `driver/mysql`'s conformance schema uses `CREATE TABLE IF NOT EXISTS` rather than `CREATE TEMPORARY TABLE` — MySQL doesn't support `FOREIGN KEY` constraints on temporary tables at all, and `ConflictDetection`'s `ErrForeignKeyViolation` case needs a real constraint to violate

Full spec/design/tasks: `.specs/features/mysql-adapter/`.

---

## M17 - SQLite Adapter

**Goal:** `driver/sqlite` — embedded/serverless, no Docker service needed for its own tests (the one adapter genuinely different from every other in this list).
**Target:** Conformance suite green against an in-memory/temp-file database, no `docker-compose` dependency.
**Status:** ✅ DONE — `TestSQLite_Conformance` green against a real in-memory SQLite database via `modernc.org/sqlite` (pure Go, no cgo). Zero Docker/Taskfile wiring needed — realizes this milestone's own stated goal. Every `Locking` field reports `false`/`SKIP` (first adapter where locking is entirely unsupported). Found that `golem.Dialect.Bind`/`Scan` being dead code (M15/AD-037's known fact) has a sharper consequence here than in M16: date/time values round-trip via the raw driver's own serialization, not this adapter's Bind formatting — see AD-04x.

### Features

**`driver/sqlite`** (`modernc.org/sqlite` + `database/sql`) - DONE

- Every `ColumnType` kind maps onto one of SQLite's 5 storage classes (mostly INTEGER/TEXT/REAL/BLOB); `Bind`/`Scan` mirror `driver/postgres`'s simple pass-through shape (including for `date`/`datetime`/`time` — see below), even though both remain unit-tested-only dead code in the real path, same as every other adapter
- `Insert`/`Update` via `RETURNING *` — SQLite (3.35+, confirmed via Context7 + web search against the bundled `modernc.org/sqlite` v1.53.0) supports `RETURNING` natively, so this adapter is architecturally closer to `driver/postgres`'s single-round-trip shape than to `driver/mysql`'s `LAST_INSERT_ID()` dance; `stmt.Insert/Update.PrimaryKey` (M16/AD-038) are read but unused, same as Postgres
- `resolveDSN` forces 3 non-optional settings via the DSN's query string: `_pragma=foreign_keys(1)` (SQLite defaults FK enforcement OFF), `_pragma=busy_timeout(5000)`, and `_time_format=sqlite` (see AD-04x — needed because `Bind` is never actually called)
- Connection pool is always bounded to `SetMaxOpenConns(1)` — SQLite's single-writer model, and required so a shared `:memory:` DSN (`file::memory:?cache=shared`) doesn't fragment across multiple pooled connections
- `IsConflict`/`mapError` use `modernc.org/sqlite`'s extended SQLite result codes (`Error.Code()`) — confirmed empirically (not just via docs) that the driver returns the extended code (2067/1555/787) by default, not the primary one (19)
- Locking: every strength (`update`/`no_key_update`/`share`/`key_share`) and every wait mode unconditionally errors — SQLite has no `SELECT ... FOR ...` clause of any kind, the first adapter where `Capabilities.Locking` is entirely `false`
- `INSERT ... ON CONFLICT (key) DO UPDATE SET` for upsert (same syntax family as Postgres); `LIMIT ? OFFSET ?` pagination; double-quote identifier quoting (Postgres-style, not MySQL's backticks)
- `AUTOINCREMENT` behavior: `INTEGER PRIMARY KEY` alone (SQLite's rowid alias) is sufficient and composes with `entity.PrimaryKey` with no changes needed — the stricter `AUTOINCREMENT` keyword (which prevents rowid reuse) wasn't needed for a fresh `:memory:` database per test run

**No Docker/Taskfile wiring** - DONE (by not existing)

- `driver/sqlite`'s own tests (unit and integration) open `sqlite.New(func(o *Options) { o.Path = ":memory:" })` directly — no `GOLEM_SQLITE_TEST_DSN` env var, no `.docker/docker-compose.test.yml` service, no `Taskfile.yml` change. `task test-integration`'s existing `go test -tags=integration ./...` already picks this adapter up.

---

## M18 - SQL Server (MSSQL) Adapter

**Goal:** `driver/mssql` passing conformance.
**Status:** ✅ DONE — `TestMSSQL_Conformance` green against a real SQL Server 2025 container (`microsoft/go-mssqldb`, pure database/sql, no cgo). Found 3 real bugs only a live server exposed (mocks caught none): DECIMAL columns scan as `[]byte` ASCII text (same class as MySQL's AD-039); `repository.Exists()`'s Count+Limit query needed `ORDER BY (SELECT NULL)` (a bare `COUNT(*)` can't `ORDER BY` a real column without `GROUP BY`); `go-mssqldb`'s `QueryContext` is lazy — constraint violations only surface via `rows.Err()` inside `collectRows`, not the immediate `QueryContext` error. See AD-045 in STATE.md.

### Features

**`driver/mssql`** (`github.com/microsoft/go-mssqldb` + `database/sql`) - DONE

- `OFFSET @pN ROWS FETCH NEXT @pM ROWS ONLY` pagination — hard syntax error without an `ORDER BY`; `stmt.Select` gained a new `PrimaryKey []string` field so `CompileSelect` can inject a default `ORDER BY <pk> ASC` when a caller paginates without one, or `ORDER BY (SELECT NULL)` for bare-aggregate (`Count`) queries where no real column can be ordered by
- No upsert path needed — golem never implemented upsert semantics on any adapter (a design-phase assumption about needing `MERGE INTO` was caught and corrected before implementation, see STATE.md AD-045); `SaveOne`/`SaveMany` route through the same `Update` as everywhere else
- Locking via table hints (`WITH (UPDLOCK, ROWLOCK[, NOWAIT|READPAST])` for "update", `WITH (HOLDLOCK, ROWLOCK...)` for "share") attached to `FROM`, not a trailing clause — the only adapter so far shaped this way; no `NO KEY UPDATE`/`KEY SHARE` equivalent, same stance as MySQL/SQLite
- `OUTPUT INSERTED.*` in place of `RETURNING` for the insert/update round-trip
- `@p1`/`@p2` numbered placeholders (not `?`/`$N`); `[bracket]`-quoted identifiers
- `normalizeCell` (column-type-aware via `rows.ColumnTypes()`) handles `UNIQUEIDENTIFIER` (mixed-endian byte layout) and `DECIMAL` (`[]byte` ASCII text → `float64`)
- `IsConflict`/`mapError` match `mssql.Error{Number}` (2627/2601 duplicate key, 547 FK/CHECK violation — golem has no CHECK concept, so 547 maps uniformly to `ErrForeignKeyViolation`)

**Docker/Taskfile wiring** - DONE

- `.docker/docker-compose.test.yml` gained an `mssql` service (`mcr.microsoft.com/mssql/server:2022-latest`, port 51433); `Taskfile.yml`'s `test-integration` gained `GOLEM_MSSQL_TEST_DSN`
- README.md's Next Steps section notes `mcr.microsoft.com`'s CDN backend can be unreachable on some networks (no Docker Hub mirror exists) — installing/enabling the Cloudflare One (WARP) client resolved it in one real case

Full spec/design/tasks: `.specs/features/mssql-adapter/`.

---

## M19 - Oracle Adapter

**Goal:** `driver/oracle` passing conformance.
**Status:** ✅ DONE — `TestOracle_Conformance` green against a real Oracle 23ai Free container (`gvenzl/oracle-free:23-slim`, `github.com/sijms/go-ora/v2`, pure Go/no cgo, targeting Oracle 12c+). Found 4 real issues only a live server exposed: unquoted DDL identifiers fold to UPPERCASE (opposite of quoteIdent's always-lowercase-preserving quoting), `golem.TIME()`'s Oracle mapping had to change from `INTERVAL DAY TO SECOND` to `TIMESTAMP` (go-ora can't bind a `time.Time` into an interval column), a `scale`-based int/float disambiguation rule broke `COUNT(*)` (shares FLOAT's "unconstrained NUMBER" sentinel scale — fixed by trying `ParseInt` first, unconditionally), and the default unbounded connection pool caused an intermittent stale-read race requiring `SetMaxOpenConns(1)`. See AD-046 in STATE.md.

### Features

**`driver/oracle`** (`github.com/sijms/go-ora/v2` + `database/sql`) - DONE

- `OFFSET y ROWS FETCH NEXT x ROWS ONLY` pagination — unlike M18/MSSQL, Oracle does NOT require an `ORDER BY` for this syntax (confirmed via probe), so no default-order-injection logic is needed; `stmt.Select.PrimaryKey` (M18) is read but unused here
- No upsert path — golem never implemented upsert semantics on any adapter (same finding as M18); `SaveOne`/`SaveMany` route through `Update`, shaped like `driver/mysql`'s multi-round-trip (plain `INSERT`/`UPDATE` + follow-up `SELECT * WHERE <primary key>`), NOT a single-round-trip `RETURNING`-style adapter — Oracle's `RETURNING ... INTO` needs one pre-typed out-bind per column, not viable generically; auto-generated `IDENTITY` primary keys are resolved via a single-column `RETURNING <pk> INTO :N` (`go_ora.Out{}` bind) since `sql.Result.LastInsertId()` is silently unsupported
- `VARCHAR2(36)` for UUID (design decision, not `RAW(16)`'s byte-format ambiguity); `TIMESTAMP` for TIME (design correction, not `INTERVAL DAY TO SECOND`)
- `FOR UPDATE`/`NOWAIT`/`SKIP LOCKED` supported, same trailing-clause shape as Postgres; no `FOR SHARE` (Oracle has none, a permanent SQL-dialect gap) and no `NO KEY UPDATE`/`KEY SHARE` (Postgres-specific)
- `:N` numbered placeholders; double-quoted identifiers
- `normalizeCell` handles Oracle's uniform `"NUMBER"` `DatabaseTypeName()` (every numeric kind, scanned as a Go string) by trying `strconv.ParseInt` first, falling back to `strconv.ParseFloat`
- `IsConflict`/`mapError` match `*network.OracleError{ErrCode}` (1 duplicate key, 2291/2292 FK violation — two distinct codes for insert/update-side vs. delete-side, unlike MSSQL's single 547)
- `db.SetMaxOpenConns(1)` — not a style choice, fixes a confirmed intermittent stale-read race under the default unbounded pool

**Docker/Taskfile wiring** - DONE

- `.docker/docker-compose.test.yml` gained an `oracle` service (`gvenzl/oracle-free:23-slim`, port 51521) — no Oracle Container Registry login needed, unlike Oracle's own official images (a real reachability win over M18's `mcr.microsoft.com` friction); `Taskfile.yml`'s `test-integration` gained `GOLEM_ORACLE_TEST_DSN`

Full spec/design/tasks: `.specs/features/oracle-adapter/`.

---

## M20 - IBM Db2 Adapter

**Goal:** `driver/db2` passing conformance.
**Status:** PLANNED

### Features

**`driver/db2`** - PLANNED

- `MERGE INTO` for upsert; `OFFSET`/`FETCH` pagination
- `CHAR(16) FOR BIT DATA` for UUID (no native UUID type)
- `FOR UPDATE WITH RS`/`WITH RR` locking clauses — another `stmt.LockClause` translation table, distinct from MSSQL's and Oracle's

---

## M21 - Snowflake (OLAP) Adapter — reduced scope

**Goal:** `driver/snowflake` passing a deliberately narrower conformance subset — this is the one adapter whose target workload (analytical/OLAP) doesn't fit every M1-M14 guarantee, so scope is explicitly smaller than every prior adapter, not a gap to close later.
**Status:** PLANNED

### Features

**`driver/snowflake`** - PLANNED

- No row-level locking at all (OLAP has no `SELECT ... FOR UPDATE` equivalent) — this adapter rejects every lock strength unconditionally; M14's `golem.Tx` guard already assumes a dialect *can* lock, so this is the first real test of a dialect that flatly can't
- No `CHECK` constraint support (per INSIGHT.md) — degrade gracefully rather than emit invalid DDL, if/when `entity.Table` grows a `CHECK` equivalent
- `MERGE INTO` for upsert; `VARIANT` for JSON; `TIMESTAMP_NTZ` for DATETIME
- Do not inherit M11's cascade-delete assumptions uncritically — OLAP schemas commonly skip FK constraints entirely for load performance; confirm a real use case wants cascade support here before building it

---

## Future Considerations

- Configurable transaction isolation level on `dataSource.Transaction` (v1 ships with driver/DB default only, see M8)
- M16-M21's exact order is a starting proposal (impact/effort: MySQL and SQLite first, MSSQL/Oracle/Db2 progressively higher effort, Snowflake last and reduced-scope) — re-order freely if a real consumer needs a specific adapter sooner; see AD-034 in STATE.md


