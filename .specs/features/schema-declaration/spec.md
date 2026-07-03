# Schema Declaration (M2, scoped) Specification

**Driving use case:** `examples/postgres-minimal-blog` — `User` (1)→(N) `Post`, `Post` (N)↔(N) `Category` via junction entity `PostToCategory`.

## Scope decision (SPEC_DEVIATION from ROADMAP.md's full M2 list)

ROADMAP.md's M2 section lists a fuller surface (`Unique`, `Index`, `CreateDate`/`UpdateDate`/`DeleteDate`, full `relation.ForeignKeyOptions` chain). This spec scopes M2 down to exactly what `postgres-minimal-blog` needs, per explicit user direction ("gatilho pra implementar M2+M3" driven by that example, not a mandate to finish 100% of the roadmap bullet list in one pass).

**Deferred to a later continuation of M2 (tracked in STATE.md Todos):** `Unique`, `Index`, `CreateDate`/`UpdateDate`/`DeleteDate` (and therefore soft-delete filtering), `column.Builder.Default`/`.DefaultFunc`, the full `relation.ForeignKeyOptions` chain (`Cascade`/`OnDelete`/`OnUpdate`/`Deferrable`/`Lazy`/`Eager`/`Persistence`/`OrphanedRowAction`). None of these are exercised by the driving example.

## Goals

- [ ] `entity.New[T](func(t *T, b *entity.Builder) {...})` builds a `*entity.Entity[T]` from field-pointer declarations (no struct tags)
- [ ] `entity.Builder.Col(fieldPtr any, t golem.ColumnType) *column.Builder` maps a struct field to a column
- [ ] `entity.Builder.PrimaryKey(fieldPtrs ...any)` declares a PK, single or composite
- [ ] `entity.Builder.ForeignKey(fieldPtr any, target *entity.Entity[J])` declares a FK (two-arg form only — no `relation.ForeignKeyOptions` in this pass)
- [ ] `entity.Builder.TableName(name string)` / `SchemaName(name string)` override defaults (struct name lowercased / connection default)
- [ ] `golem.BIGINT()`, `golem.VARCHAR(n)`, `golem.TEXT()` exist as real `ColumnType` constructors (the minimum set the example needs)
- [ ] `column.Builder.Name(name string)` overrides the column name (default: struct field name)

## Out of Scope (this pass)

| Feature | Reason |
| --- | --- |
| `Unique`, `Index` | Not exercised by `postgres-minimal-blog` |
| `CreateDate`/`UpdateDate`/`DeleteDate`, soft-delete filtering | Not exercised — example entities have no timestamp/soft-delete columns |
| `column.Builder.Nullable`/`.Default`/`.DefaultFunc` | Not exercised — every column in the example is required, no defaults |
| `relation.ForeignKeyOptions` (Cascade/OnDelete/OnUpdate/Deferrable/Lazy/Eager/Persistence/OrphanedRowAction) | Not exercised — example uses plain FKs with DB-default behavior |
| `golem.UUID()`, `golem.JSON()`, `golem.BOOLEAN()`, `golem.TIMESTAMPTZ()`, `golem.INT()` | Not needed by the example's column types |

## Acceptance Criteria

1. WHEN `entity.New[User](...)` declares `Col(&t.ID, golem.BIGINT())`, `Col(&t.Name, golem.VARCHAR(50))`, `PrimaryKey(&t.ID)` THEN the resulting `*entity.Entity[User]` SHALL expose (internally) the correct table name (`"user"`, lowercased struct name, since `TableName` wasn't called), column name→type mapping, and PK field list — resolved via field-pointer-to-struct-field matching, not string tags.
2. WHEN two different struct fields are passed to `Col`/`PrimaryKey`/`ForeignKey` THEN the entity SHALL correctly distinguish them even if their Go types are identical (e.g. two `int64` fields) — resolution must be by field IDENTITY (memory offset within the zero-value struct), not by type or declaration order.
3. WHEN `ForeignKey(&t.OwnerUserID, UserEntity)` is declared THEN the entity SHALL record that `OwnerUserID` references `UserEntity`'s primary key column.
4. WHEN `PrimaryKey(&t.QuestionID, &t.CategoryID)` (composite) is declared THEN the entity SHALL record both fields as the PK, in the order given.
5. WHEN `TableName`/`SchemaName` are not called THEN the entity SHALL default the table name to the lowercased struct name and leave schema unset (adapter/connection default applies).

## Success Criteria

- [ ] `examples/postgres-minimal-blog` compiles its entity declarations (`UserEntity`, `PostEntity`, `CategoryEntity`, `PostToCategoryEntity`) against this API with zero reflection/tag workarounds in user code
- [ ] `go test ./entity/... ./column/...` passes (unit tests covering field-pointer resolution, including the same-Go-type-different-field edge case)
