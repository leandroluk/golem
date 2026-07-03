# Schema Declaration (M2) Specification

## Status: ✅ DONE

---

## Goals

- [x] `entity.New[T](func(t *T, b *entity.Table) {...})` builds a `*entity.Entity[T]` from field-pointer declarations (no struct tags)
- [x] `entity.Table.Col(fieldPtr any, t golem.ColumnType) *entity.Column` maps a struct field to a column
- [x] `entity.Table.PrimaryKey(fieldPtrs ...any)` declares a PK, single or composite
- [x] `entity.Table.ForeignKey(fieldPtr any, target *entity.Entity[J])` declares a FK (two-arg form only)
- [x] `entity.Table.TableName(name string)` / `SchemaName(name string)` override defaults
- [x] `entity.Table.Unique(fieldPtrs ...any)` declares a unique constraint (single or composite)
- [x] `entity.Table.Index(fieldPtrs ...any) *entity.Index` declares an index (with optional `.Name()`/`.Unique()`)
- [x] `entity.Table.CreateDate(fieldPtr any)` marks the create-timestamp field
- [x] `entity.Table.UpdateDate(fieldPtr any)` marks the update-timestamp field
- [x] `entity.Table.DeleteDate(fieldPtr any)` marks the soft-delete timestamp field
- [x] `entity.Column.Name(name string)` overrides the column name
- [x] `entity.Column.Nullable()` marks the column as nullable
- [x] `entity.Column.Default(value any)` sets a literal default value
- [x] `entity.Column.DefaultFunc(fn func() (any, error))` sets a computed default
- [x] `golem.BIGINT()`, `golem.INT()`, `golem.VARCHAR(n)`, `golem.TEXT()`, `golem.BOOLEAN()`, `golem.TIMESTAMPTZ()`, `golem.UUID()`, `golem.JSON()` — full ColumnType constructor set

## Acceptance Criteria

1. WHEN `entity.New[User](...)` declares `Col(&t.ID, golem.BIGINT())`, `Col(&t.Name, golem.VARCHAR(50))`, `PrimaryKey(&t.ID)` THEN the resulting `*entity.Entity[User]` SHALL expose the correct table name (`"user"`), column name→type mapping, and PK field list — resolved by field-pointer-to-struct-field matching via memory offset, not struct tags.
2. WHEN two different struct fields of the same Go type are passed to `Col`/`PrimaryKey`/`ForeignKey` THEN the entity SHALL correctly distinguish them by field IDENTITY (memory offset), not by type or declaration order.
3. WHEN `ForeignKey(&t.OwnerUserID, UserEntity)` is declared THEN the entity SHALL record that `OwnerUserID` references another entity's primary key.
4. WHEN `PrimaryKey(&t.QuestionID, &t.CategoryID)` (composite) is declared THEN the entity SHALL record both fields as the PK, in the given order.
5. WHEN `TableName`/`SchemaName` are not called THEN the entity SHALL default to the lowercased struct name and leave schema unset.
6. WHEN `Unique(&t.Email)` is declared THEN `EntityMeta.Uniques` SHALL record `[["email"]]`.
7. WHEN `Index(&t.Email).Name("idx_email").Unique()` is declared THEN `EntityMeta.Indexes` SHALL record the column name, index name, and unique flag.
8. WHEN `CreateDate`/`UpdateDate`/`DeleteDate` are declared THEN `EntityMeta.CreateDateField`/`UpdateDateField`/`DeleteDateField` SHALL hold the corresponding struct field names.
9. WHEN `Col(&t.Name, golem.TEXT()).Nullable()` is declared THEN `ColumnMeta.Nullable` SHALL be `true`.
10. WHEN `Col(&t.Name, golem.TEXT()).Default("anon")` is declared THEN `ColumnMeta.HasDefault` SHALL be `true` and `ColumnMeta.Default` SHALL be `"anon"`.
11. WHEN `Col(&t.Name, golem.TEXT()).DefaultFunc(fn)` is declared THEN `ColumnMeta.DefaultFunc` SHALL be non-nil and return the expected value.

## Success Criteria

- [x] `go test ./column/... ./entity/... ./index/... ./...` passes — all packages green
- [x] Build clean: `go build ./...` — no errors


