# Schema Declaration (M2, scoped) Design

**Spec**: `.specs/features/schema-declaration/spec.md`
**Status**: Approved

---

## Architecture Overview

`entity.New[T]` builds a `var zero T`, hands `&zero` to the user's callback alongside a `*entity.Builder`. Every `Builder` method (`Col`, `PrimaryKey`, `ForeignKey`) receives a field pointer (`&t.SomeField`) and resolves it to the underlying struct field **by memory offset**, not by type or call order — this is what lets `gox/orm`-style APIs avoid struct tags. The resolved field name plus whatever else the method call carries (a `ColumnType`, "this is the PK", "this references entity X") gets stored in the `Entity[T]`'s internal metadata, later read by `repository.Get` via a small exported `Describe()` accessor (metadata is package-external-readable, construction is builder-only).

```mermaid
graph TD
    A["entity.New[User](fn)"] --> B["var zero User; base := unsafe address of &zero"]
    B --> C["fn(&zero, builder)"]
    C --> D["builder.Col(&zero.ID, golem.BIGINT())"]
    D --> E["resolveField: offset of fieldPtr - base -> match struct field by Offset"]
    E --> F["store columnMeta{FieldName:\"ID\", Name:\"id\", Type:BIGINT}"]
    F --> G["*entity.Entity[User] (opaque)"]
    G -->|Describe()| H["repository package reads EntityMeta"]
```

---

## Field-Pointer Resolution (the core mechanism)

```go
func resolveField(zero any, fieldPtr any) (string, error) {
    base := reflect.ValueOf(zero).Elem() // zero is *T, .Elem() is addressable
    baseAddr := base.UnsafeAddr()
    fieldAddr := reflect.ValueOf(fieldPtr).Pointer() // fieldPtr is *FieldType
    offset := uintptr(fieldAddr) - baseAddr

    t := base.Type()
    for i := 0; i < t.NumField(); i++ {
        if t.Field(i).Offset == offset {
            return t.Field(i).Name, nil
        }
    }
    return "", fmt.Errorf("entity: field pointer does not belong to %s", t.Name())
}
```

Correctness depends on `entity.New` passing the SAME `&zero` instance to both the user's callback (as `t`) and to every `Builder` method's internal offset math — never a copy. `Builder` holds `zero any` (the original `*T`) for this reason.

---

## Defaulting Rules (decided now, since they affect generated SQL — see repository-core-crud/design.md)

- **Table name**: `strings.ToLower(reflect.TypeOf(zero).Elem().Name())` (e.g. `User` → `"user"`) unless `.TableName(...)` overrides it.
- **Column name**: `strings.ToLower(fieldName)` (e.g. `Name` → `"name"`, `ID` → `"id"`) unless `.Name(...)` overrides it. Compound field names (`OwnerUserID`) are NOT auto-split to snake_case — declare `.Name("owner_user_id")` explicitly when that's wanted (same pattern the design already uses for `.Name("full_name")`). This keeps the default rule trivial (no NLP-ish word-splitting heuristic to get subtly wrong) and matches "every new builder method must justify itself" — an auto-snake_case algorithm is exactly the kind of hidden magic this project avoids elsewhere.
- **SQL generation always double-quotes identifiers** (`"user"."owner_user_id"`) using the exact stored name string — see repository-core-crud/design.md. This makes the default/override naming above the single source of truth with no separate Postgres-side case-folding to reason about.

---

## Components

### `entity.Entity[T]` + `entity.Builder` + `entity.New[T]`

- **Location**: `entity/entity.go`, `entity/builder.go`
- **Interfaces**:
  - `func New[T any](fn func(t *T, b *Builder)) *Entity[T]`
  - `(*Builder) TableName(name string)`
  - `(*Builder) SchemaName(name string)` (stored but unused until an adapter needs it — Postgres in this pass always uses the connection's default schema; storing it now avoids a breaking change later)
  - `(*Builder) PrimaryKey(fieldPtrs ...any)`
  - `(*Builder) Col(fieldPtr any, t golem.ColumnType) *column.Builder`
  - `(*Builder) ForeignKey(fieldPtr any, target *Entity[J]) ` — two-arg only, no `relation.ForeignKeyOptions` (deferred)
  - `(*Entity[T]) Describe() EntityMeta` — read-side accessor for `repository`
- **Dependencies**: `golem.ColumnType`, `column.Builder`
- **Reuses**: nothing pre-existing (new package)

### `column.Builder`

- **Location**: `column/builder.go`
- **Interfaces**: `(*Builder) Name(name string) *Builder`
- **Dependencies**: none
- **Reuses**: none
- Note: `.Nullable()`/`.Default()`/`.DefaultFunc()` deliberately NOT implemented this pass (Out of Scope in spec.md)

### `EntityMeta` (the entity/repository seam)

```go
package entity

type ColumnMeta struct {
    FieldName string
    Name      string
    Type      golem.ColumnType
}

type EntityMeta struct {
    TableName  string
    SchemaName string
    Columns    []ColumnMeta
    PrimaryKey []string // COLUMN names (not field names), in declared order
}

func (e *Entity[T]) Describe() EntityMeta { ... }
```

`repository` never touches `Entity[T]`'s internals directly — only `EntityMeta`, a plain value type, decoupling the two packages cleanly.

### `golem.ColumnType` real constructors (this pass's subset)

- **Location**: `columntype_constructors.go` (new file, alongside the existing M1 `columntype.go` stub — do not modify `columntype.go` itself)
- **Interfaces**: `func BIGINT() ColumnType`, `func VARCHAR(n int) ColumnType`, `func TEXT() ColumnType` — each just sets the existing unexported `kind` field (`"bigint"`, `"varchar(50)"`, `"text"`) plus, for `VARCHAR`, stores `n` (add an unexported `length int` field to the existing `ColumnType` struct — additive, doesn't break M1's stub usage)

---

## Tech Decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| Field resolution | Offset-matching via `unsafe`-free `reflect.Value.UnsafeAddr()` + `reflect.Value.Pointer()` comparison | Standard technique for field-pointer APIs (avoids struct tags/codegen per project's core pitch); no `unsafe` package import needed, pure `reflect` |
| Column name default | `strings.ToLower(fieldName)`, no auto snake_case splitting | Trivial, unsurprising rule; compound names need an explicit `.Name(...)` anyway (same pattern already established for column renames) |
| SQL identifier quoting | Always double-quote, using the exact stored name | Removes Postgres case-folding as a variable entirely — the stored string IS the wire identifier, always |
| `EntityMeta` value type vs. exposing `Entity[T]` internals | Plain value struct returned by `Describe()` | Keeps `entity`/`repository` decoupled; `Entity[T]`'s internal representation can still evolve (Unique/Index/etc. later) without breaking `repository` |
