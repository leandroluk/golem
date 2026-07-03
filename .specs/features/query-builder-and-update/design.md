# Query Builder + Save/Update (scoped) Design

**Spec**: `.specs/features/query-builder-and-update/spec.md`
**Status**: Approved

---

## Architecture Overview

`op.Eq(fieldPtr, value)` produces an opaque `op.Condition` holding the raw field pointer + value — it does NOT resolve a column name itself (it has no access to entity metadata). Resolution happens later, inside `repository`, using the same offset-based field-pointer resolution `entity` already has (promoted to an exported `entity.ResolveField` so `query`/`repository` can reuse it instead of duplicating the reflection trick).

`query.Query[T]`/`query.Update[T]` are built internally by `repository` (never directly by end users — matches the README's own pattern: users only ever receive one via a criteria callback). Each holds the same zero-value `*T` its owning `Repository[T]` already has, so it can resolve `Where`/`Set`'s field pointers the moment `FindMany`/`FindOne`/`UpdateOne`/`UpdateMany` builds SQL — no deferred-finalize dance needed here (unlike `entity.Builder`), because `Where`/`Set` conditions are consumed immediately after the criteria callback returns, not chained further.

```mermaid
graph TD
    A["repo.FindOne(ctx, func(t,q){ q.Where(op.Eq(&t.ID, 5)) })"] --> B["query.New(zero, meta) -> q"]
    B --> C["criteria(zero, q)"]
    C --> D["q.conditions = [{fieldPtr:&zero.ID, value:5}]"]
    D --> E["repository resolves each condition: entity.ResolveField(zero, fieldPtr) -> fieldName -> meta.Columns lookup -> columnName"]
    E --> F["whereColumns=[\"id\"], whereValues=[5]"]
    F --> G["conn.Dialect().Select(ctx, conn, table, whereColumns, whereValues)"]
    G --> H["postgres: SELECT * FROM table WHERE \"id\"=$1"]
    H --> I["0 rows -> ErrNotFound; 1 row -> scanRow -> T"]
```

---

## Components

### `entity.ResolveField` (promotes the existing unexported `resolveField`)

- **Location**: `entity/entity.go` (modify: export the existing function, keep its body identical)
- **Interface**: `func ResolveField(zero any, fieldPtr any) (string, error)`
- Existing internal callers (`Builder.Col`/`PrimaryKey`/`ForeignKey`) call the exported name now; behavior unchanged.

### `op` package (new)

```go
package op

// Condition is a single "column compares to a literal value" predicate.
// fieldPtr is resolved to a column name later, by whichever package
// consumes it (repository) — op itself has no entity metadata.
type Condition struct {
    FieldPtr any
    Value    any
}

func Eq(fieldPtr any, value any) Condition {
    return Condition{FieldPtr: fieldPtr, Value: value}
}
```

(Only `Eq` this pass — see spec.md's scope table for what's deferred.)

### `query` package (new)

```go
package query

// Query is received by a FindMany/FindOne criteria callback. Where's
// conditions are ANDed. Built internally by repository — never constructed
// directly by end users.
type Query[T any] struct {
    conditions []op.Condition
}

func New[T any]() *Query[T] { return &Query[T]{} }

func (q *Query[T]) Where(conditions ...op.Condition) {
    q.conditions = append(q.conditions, conditions...)
}

func (q *Query[T]) Conditions() []op.Condition { return q.conditions }
```

```go
// Update is received by an UpdateOne/UpdateMany criteria callback.
type Update[T any] struct {
    conditions []op.Condition
    sets       []SetClause
}

type SetClause struct {
    FieldPtr any
    Value    any
}

func NewUpdate[T any]() *Update[T] { return &Update[T]{} }

func (u *Update[T]) Where(conditions ...op.Condition) { u.conditions = append(u.conditions, conditions...) }
func (u *Update[T]) Set(fieldPtr any, value any)      { u.sets = append(u.sets, SetClause{FieldPtr: fieldPtr, Value: value}) }
func (u *Update[T]) Conditions() []op.Condition       { return u.conditions }
func (u *Update[T]) Sets() []SetClause                { return u.sets }
```

Note: `Query`/`Update` don't need the `zero *T` themselves — the field-pointer values captured inside `op.Condition`/`SetClause` are already resolved against whatever zero-value instance `repository` passed into the criteria callback (same pattern as `entity.Builder`: the callback's `t *T` parameter IS the zero value, so `&t.ID` inside the user's callback is already an address into it). `repository` just needs to keep using that SAME zero-value pointer both to invoke the callback and to later resolve every captured `FieldPtr` via `entity.ResolveField`.

### `golem.Dialect` — remove `FindByID`, add `Select`/`Update`

```go
// dialect.go — Bind/Scan/Insert unchanged; FindByID REMOVED

// Select returns every row matching the AND-ed column=value pairs (empty
// whereColumns/whereValues means "no filter, all rows").
Select(ctx context.Context, conn Conn, table string, whereColumns []string, whereValues []driver.Value) ([]map[string]any, error)

// Update runs UPDATE ... SET ... WHERE ... RETURNING *, returning every
// updated row (0+).
Update(ctx context.Context, conn Conn, table string, setColumns []string, setValues []driver.Value, whereColumns []string, whereValues []driver.Value) ([]map[string]any, error)
```

### `postgres.dialect` — implement `Select`/`Update`, remove `FindByID`

- `Select`: `SELECT * FROM "table"` + (if `len(whereColumns) > 0`) `WHERE "c1"=$1 AND "c2"=$2...`; `pgx.CollectRows(rows, pgx.RowToMap)` (plural — 0+ rows, not `CollectOneRow`).
- `Update`: `UPDATE "table" SET "c1"=$1,"c2"=$2 WHERE "c3"=$3 AND "c4"=$4 RETURNING *` (SET placeholders numbered first, then WHERE placeholders continue the sequence); `pgx.CollectRows`.
- Remove `buildFindByIDSQL`/`FindByID`, add `buildSelectSQL`/`buildUpdateSQL` (pure, unit-testable, same pattern as `buildInsertSQL`).

### `repository.Repository[T]` — remove `FindByID`, add `FindMany`/`FindOne`/`SaveOne`/`UpdateOne`/`UpdateMany`

- `FindMany`/`FindOne`: build `zero := new(T)`-equivalent (actually need a real *T to hand to the criteria callback — same as `entity.New`'s `var zero T; fn(&zero, ...)` pattern), invoke each criteria func against it and a `*query.Query[T]`, collect `Conditions()`, resolve each via `entity.ResolveField(zero, cond.FieldPtr)` → `meta.Columns` lookup → column name, call `Dialect.Select`, scan every returned row. `FindOne` = same as `FindMany` but takes the first result and returns `golem.ErrNotFound` if the slice is empty (and errors if the underlying entity metadata is otherwise fine — no special composite-PK restriction here, since there's no PK-specific logic at all, just arbitrary equality conditions).
- `SaveOne`: whereColumns/whereValues = `meta.PrimaryKey` columns read off `i`'s current field values (via the existing `FieldName` lookup already used by `Insert`/`scanRow`); setColumns/setValues = every OTHER (non-PK) column's current value on `i` (zero-valued non-PK fields ARE sent this time — unlike `Insert`, an update explicitly setting a field back to its zero value is a normal, expected case, not a "let the DB default apply" signal). Calls `Dialect.Update`, expects exactly 1 row back (0 → `golem.ErrNotFound`, per AC-6 not explicitly stated but consistent with treating "the row you meant to update is gone" as a not-found case).
- `UpdateOne`/`UpdateMany`: build a `*query.Update[T]` the same way `FindMany` builds a `*query.Query[T]`; resolve `Where`'s conditions AND `Set`'s clauses (both need `entity.ResolveField` + column lookup) into the `Dialect.Update` call. `UpdateOne` expects exactly 1 row (0 → `ErrNotFound`), `UpdateMany` returns whatever N rows matched (0 is NOT an error for `Many` — matches `FindMany`'s empty-result-is-fine semantics, contrasted with the `One` variants which treat zero as not-found).

---

## Tech Decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| `op.Condition`/`query.Query`/`query.Update` don't resolve field pointers themselves | Resolution happens once, inside `repository`, via `entity.ResolveField` | Avoids duplicating the offset-reflection trick in 3 packages; `op`/`query` stay simple data carriers |
| `entity.ResolveField` exported (was `resolveField`) | Promote, don't duplicate | DRY — same mechanism `entity.Builder` already uses |
| `FindByID` removed entirely (not deprecated/kept alongside) | Clean removal from `Dialect`, `repository`, and the example | Explicit user direction — `FindOne(Where(op.Eq(pk, id)))` fully replaces it, keeping it around would be redundant surface (project's own stated constraint: justify every method against what already exists) |
| `SaveOne` works for composite PK | No restriction (unlike the old single-column-only `FindByID`) | `SaveOne`'s WHERE is always built from `meta.PrimaryKey` (however many columns), not user-supplied — no ambiguity to restrict against |
| `Select`/`Update` Dialect methods take flat `(columns, values)` pairs, ANDed | No predicate tree / AST | Same YAGNI reasoning as AD-020 — only `op.Eq` exists, so "AND of equalities" is the entire expressiveness needed; revisit when `Or`/`Gt`/etc. land |
| Zero rows on `UpdateOne`/`SaveOne` | `golem.ErrNotFound` | Consistent with `FindOne`'s "nothing matched" signal — same sentinel, same meaning, no new error type needed |
