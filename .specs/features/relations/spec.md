# Relations (M11) Specification

## Status: ✅ DONE

---

## Goals

- [x] `entity.Table.ForeignKey(fieldPtr any, target any, opts ...*relation.ForeignKeyOptions)` — 3rd argument variadic, backward-compatible with the existing 2-arg form
- [x] `relation.ForeignKeyOptions` fluent builder exposing the full chain documented in README.md: `Cascade`, `OnDelete`, `OnUpdate`, `Deferrable`, `CreateForeignKeyConstraints`, `Lazy`, `Eager`, `Persistence`, `OrphanedRowAction`
- [x] `ForeignKey`'s `target` argument is actually read (previously a no-op bug — `entity/table.go` never even type-asserted it) — captures the target's table name and single-column primary key
- [x] `entity.ForeignKeysReferencing(targetTable string) []FKRegistration` — a package-level registry populated as a side effect of `entity.New` running `ForeignKey`, indexed by the PARENT (target) side, so a parent-side operation can discover which child entities point at it (the declaration direction is the opposite)
- [x] `Repository[T].Delete` applies every cascade-actionable `OnDelete` action (`Cascade`, `SetNull`, `Restrict`) registered against the entity being deleted, atomically (auto-opens a transaction when 1+ actionable FK exists and the current `Conn` isn't already a `Tx`)
- [x] `Repository[T].Delete`'s cascade honors the CHILD's own soft-delete configuration (soft-deletes the child if it has a `DeleteDate` field, hard-deletes otherwise; `Restrict`'s existence check also excludes already-soft-deleted child rows)
- [x] `Restrict` returns `golem.ErrForeignKeyViolation` (M10 sentinel, reused) when 1+ non-deleted child row still references the parent being deleted
- [x] Options with no coherent runtime meaning in golem's architecture (`Cascade*`, `Persistence`, `OrphanedRowAction`, `CreateForeignKeyConstraints`, `Deferrable`) are accepted and stored on `ForeignKeyMeta.Options`, but have **no runtime effect** — see design.md for why
- [x] `OnUpdate` is accepted/stored but not wired to any Repository[T] method yet (no operation currently changes a row's PK value) — documented as a known gap, not silently dropped
- [x] `Eager` is accepted/stored; wiring it to actual automatic preloading is M12's job, not M11's

## Acceptance Criteria

1. WHEN `ForeignKey(&t.OwnerUserID, UserEntity)` (2-arg form) is declared THEN it SHALL behave exactly as before (default `ForeignKeyOptions`, `CreateForeignKeyConstraints`/`Persistence` default `true`, everything else its zero value).
2. WHEN `ForeignKey(&t.OwnerUserID, UserEntity, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))` is declared THEN `ForeignKeyMeta.Options.ResolvedOnDelete()` SHALL be `relation.OnDeleteCascade`, and `entity.ForeignKeysReferencing("users")` SHALL include an entry for this FK.
3. WHEN `target` doesn't implement `Describe() EntityMeta` THEN `ForeignKey` SHALL panic with a descriptive message (loud failure, matching `ResolveField`'s existing panic-on-bad-input convention elsewhere in this package).
4. WHEN `target`'s primary key is composite (2+ columns) THEN `ForeignKey` SHALL panic — composite-PK FK targets aren't supported (a single `fieldPtr` can't express a composite FK column set).
5. WHEN `Repository[T].Delete` deletes a row and an entity has `ForeignKey(..., OnDelete(OnDeleteCascade))` pointing at `T`, with no soft-delete on the child THEN the matching child rows SHALL be hard-deleted.
6. WHEN that child entity has a `DeleteDate` field THEN the cascade SHALL soft-delete it instead (`UPDATE ... SET deletedAtCol = now()`), not hard-delete it.
7. WHEN `OnDeleteSetNull` is configured THEN matching child rows SHALL have the FK column set to `NULL`, not be deleted.
8. WHEN `OnDeleteRestrict` is configured and 1+ non-soft-deleted child row still references the parent THEN `Delete` SHALL return an error satisfying `errors.Is(err, golem.ErrForeignKeyViolation)`, and the parent row SHALL NOT be deleted.
9. WHEN `OnDeleteRestrict` is configured and 0 matching child rows exist (or all are already soft-deleted) THEN `Delete` SHALL proceed normally.
10. WHEN `OnDeleteDefault`/`OnDeleteNoAction` (or no `ForeignKey` at all references `T`) THEN `Delete` SHALL behave exactly as before M11 (no extra transaction, no cascade queries).
11. WHEN 1+ cascade-actionable FK exists and the `Repository[T]`'s `Conn` is a plain `*DataSource` (not already inside a `Tx`) THEN `Delete` SHALL open its own transaction for the cascade + parent delete, committing on success and rolling back on any error.
12. WHEN `Repository[T]`'s `Conn` is already a `golem.Tx` THEN `Delete` SHALL reuse it (no nested `Begin`), leaving commit/rollback ownership with whoever opened that `Tx`.

## Success Criteria

- [x] `go test ./relation/... ./entity/... ./repository/...` — all green, `relation` package at 100% statement coverage, every new function in `entity`/`repository` for this feature at 100% (verified via `go tool cover -func`)
- [x] `go build ./...` — no errors
- [x] `.examples/postgres-minimal-blog`: `Post`'s `ForeignKey` to `User` now uses `OnDelete(relation.OnDeleteCascade)`; `TestBlogExample_CascadeDeleteUser_DeletesTheirPosts` (real Postgres, `task test-integration`) verifies deleting a `User` cascades into deleting their `Post` rows
