# Relations (M11) Design

**Spec**: `.specs/features/relations/spec.md`
**Status**: Approved

---

## Why most `ForeignKeyOptions` fields have no runtime effect

README.md's `ForeignKeyOptions` chain is a direct port of TypeORM's relation options. TypeORM's semantics for `Cascade`/`Persistence`/`OrphanedRowAction` all assume the owning struct has a **navigational collection field** holding the related object(s) in memory (`post.owner = newUser`, `user.posts = [...]`), and that saving/removing the owning struct cascades onto whatever is attached to that field.

golem structurally doesn't have that field — `AD-001` (many-to-many is a plain junction entity, no relation type) and `AD-024` (Preload/With stays a separate query-level API, no `Posts []Post` struct field) both rule it out, on purpose, to keep entity structs "plain, zero magic." Without an attached in-memory object to cascade a write from, `Cascade*`/`Persistence`/`OrphanedRowAction` have nothing to act on. `CreateForeignKeyConstraints`/`Deferrable` describe DDL-time constraint behavior, and golem never emits DDL (`AD-012`) — informational only, for whoever manages the schema outside golem.

`OnDelete`/`OnUpdate` are the exception: they describe what happens to the **referencing** rows when the **referenced** row changes, which needs no attached object — just a way to find "who points at this row," which is exactly what the FK registry below provides. Only `OnDelete` is wired up in M11 (see Todos in STATE.md for `OnUpdate`).

---

## `relation` package

Standalone package, no dependency on `entity` (avoids a cycle — `entity.ForeignKeyMeta`/`FKRegistration` import `relation`, not the other way around).

```go
type ForeignKeyOptions struct { /* unexported fields */ }
func NewForeignKeyOptions() *ForeignKeyOptions // CreateForeignKeyConstraints=true, Persistence=true, everything else zero
func (o *ForeignKeyOptions) Cascade(opts ...CascadeOption) *ForeignKeyOptions
func (o *ForeignKeyOptions) OnDelete(a OnDeleteAction) *ForeignKeyOptions
func (o *ForeignKeyOptions) OnUpdate(a OnUpdateAction) *ForeignKeyOptions
func (o *ForeignKeyOptions) Deferrable(m DeferrableMode) *ForeignKeyOptions
func (o *ForeignKeyOptions) CreateForeignKeyConstraints(b bool) *ForeignKeyOptions
func (o *ForeignKeyOptions) Lazy(b bool) *ForeignKeyOptions
func (o *ForeignKeyOptions) Eager(b bool) *ForeignKeyOptions
func (o *ForeignKeyOptions) Persistence(b bool) *ForeignKeyOptions
func (o *ForeignKeyOptions) OrphanedRowAction(m OrphanedRowActionMode) *ForeignKeyOptions
func (o *ForeignKeyOptions) HasCascade(opt CascadeOption) bool // opt != CascadeAll
func (o *ForeignKeyOptions) Resolved*() // one getter per field, mirrors entity/column.go's ResolvedName() pattern
```

`Cascade(CascadeAll)` expands to setting every granular flag (`Insert`/`Update`/`Remove`/`SoftRemove`/`Recover`) — stored as `map[CascadeOption]bool`, accumulates across multiple `Cascade(...)` calls (doesn't replace).

---

## `entity.Table.ForeignKey` — fixing the unused-`target` bug + adding opts

Before M11, `ForeignKey(fieldPtr any, target any)` never even type-asserted `target` — `ForeignKeyMeta` only ever stored `FieldName`. M11 fixes this as a side effect of adding options:

```go
type describer interface { Describe() EntityMeta }

func (b *Table) ForeignKey(fieldPtr any, target any, opts ...*relation.ForeignKeyOptions) {
    fieldName, err := ResolveField(b.zero, fieldPtr)
    if err != nil { panic(err) }

    d, ok := target.(describer)
    if !ok { panic(/* target must be *entity.Entity[J] for some J */) }
    targetMeta := d.Describe()
    if len(targetMeta.PrimaryKey) != 1 { panic(/* composite-PK targets unsupported */) }

    o := relation.NewForeignKeyOptions()
    if len(opts) > 0 { o = opts[0] }

    b.pendingForeignKey = append(b.pendingForeignKey, pendingForeignKey{
        fieldName: fieldName, targetTableName: targetMeta.TableName,
        targetPrimaryKey: targetMeta.PrimaryKey[0], opts: o,
    })
}
```

`target.Describe()` is called at declaration time, which works because Go initializes package-level vars in dependency order — if `PostEntity`'s initializer references `UserEntity`, Go guarantees `UserEntity` is already initialized (its `finalize()` already ran) by the time `PostEntity`'s callback runs. The pre-existing `AD-002` cyclic-entity thunk pattern (`func() *Entity[T]`) isn't handled specially here — out of scope, not exercised by any current entity.

Resolution (column name lookup) is deferred to `Table.finalize()`, same pattern as `PrimaryKey`/`Unique`/`Index` — because the child's own column name (from `Col`'s deferred naming) isn't known until then. `finalize()` also calls `registerForeignKey(...)` for each resolved FK — this is the only place a `pendingForeignKey` becomes both a `ForeignKeyMeta` (this entity's own metadata) AND an `entity.FKRegistration` (the global registry, keyed by the OTHER entity's table name).

---

## The FK registry (`entity/fk_registry.go`)

Declaration direction (child declares `ForeignKey` pointing at parent) is the opposite of the direction cascade needs to query in (given a parent being deleted, "who points at me?"). A package-level registry bridges this:

```go
type FKRegistration struct {
    ChildTableName        string
    ChildColumn           string
    ChildDeleteDateColumn string // "" if the child has no soft-delete field
    TargetTableName       string
    Options               *relation.ForeignKeyOptions
}

var fkRegistry = map[string][]FKRegistration{} // keyed by TargetTableName, mutex-guarded

func ForeignKeysReferencing(targetTable string) []FKRegistration // returns a COPY, never the live slice
```

Populated as a side effect of `entity.New` (via `Table.finalize()`), read by `Repository[T].Delete`. Global, package-level state — acceptable here because entities are conventionally declared once, as package-level vars, at program init; the one caveat is test code that declares the same table name repeatedly across test runs within one test binary accumulates duplicate registrations, which is harmless (a duplicate cascade action against the same rows just affects 0 rows the second time) but something to be aware of if writing entity-heavy tests — use distinct table names per test (see `repository/delete_cascade_test.go`'s convention).

---

## Cascade execution (`repository/repository.go`'s `Delete`)

```
Delete(ctx, items...):
  fkRegs := entity.ForeignKeysReferencing(r.meta.TableName)   # once, outside the per-item loop
  for each item:
    BeforeDelete hook
    pkValue := reflect value of item's (single, by construction) PK field
    conn, commit, rollback := beginCascadeTx(fkRegs)          # no-op tx if nothing actionable
    applyDeleteCascades(conn, fkRegs, pkValue)                # Restrict checks first, then Cascade/SetNull
      -> error? rollback(); return err
    run the parent's own soft-delete-or-hard-delete (existing logic, now against `conn` not `r.conn`)
      -> error? rollback(); IsConflict hook; return err
    commit()
      -> error? return err
    AfterDelete hook
```

- **`beginCascadeTx`**: scans `fkRegs` for any `cascadeActionable` `OnDelete` (`Cascade`/`SetNull`/`Restrict` — `Default`/`NoAction` are not). If none, returns `r.conn` unchanged with no-op commit/rollback (zero behavior change, zero overhead, for the common case of an entity nothing points at). If the current `Conn` is already a `golem.Tx`, reuses it as-is (no nested `Begin` — Postgres doesn't support that on one connection anyway). Otherwise opens a real transaction via `r.conn.Dialect().Begin(ctx, r.conn)` + `golem.NewTx`.
- **`applyDeleteCascades`**: two passes over `fkRegs` — first every `Restrict` (fail before mutating anything if any child row still references the parent, excluding already-soft-deleted child rows via an `IS NULL` filter on `ChildDeleteDateColumn` when the child has one), then every `Cascade`/`SetNull` (soft-delete the child if it has `ChildDeleteDateColumn`, else hard-delete; `SetNull` always `UPDATE ... SET childCol = NULL`). Uses the existing `internal/stmt` AST + `Dialect.CompileSelect`/`CompileDelete`/`Update`/`Query`/`Exec` — no `Dialect` interface changes needed, every primitive already existed (`Count: true` on `stmt.Select`, `is_null` comparison op).
- Composite-PK entities are safe here even though `Delete` supports them generally: `ForeignKey` only accepts single-column-PK targets (see spec.md #4), so any `FKRegistration` found via `ForeignKeysReferencing` guarantees the entity being deleted has exactly one PK column — reading `pkValue` as a scalar is always correct when `len(fkRegs) > 0`.

---

## Tech Decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| Registry scope | Package-level global in `entity`, not per-`DataSource` | Entities are declared once as package vars; no existing per-DataSource entity registry to hang this off of (`golem.Entities(...)` from README is aspirational, never implemented) |
| `OnDelete`/`OnUpdate` split | Only `OnDelete` wired to real behavior in M11 | No Repository[T] operation currently changes a row's PK value — `OnUpdate` cascade has nothing to trigger from yet; documented as a known gap (STATE.md Todos), not silently dropped from the option surface |
| Restrict-then-mutate ordering | Two passes: all `Restrict` checks before any `Cascade`/`SetNull` mutation | Clearer intent (a rejected delete shouldn't even attempt partial cascade mutation) — though the transaction wrapping makes this a belt-and-suspenders choice, not a correctness requirement |
| Transaction wrapping | Only opens a transaction when 1+ FK is cascade-actionable | Zero behavior/perf change for the (still-common) case of an entity nothing references |
