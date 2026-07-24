# Insert Returns `*T` Specification

**Driving observation:** every single-row `Repository[T]` method returns `*T` (`FindOne`,
`SaveOne`) except `Insert`, which returned a bare `T` — an inconsistency spotted while adding
`Must*` variants (`repository-must-variants`): `MustInsert` had to return `T` while `MustFindOne`/
`MustSaveOne` return `*T`, exposing the asymmetry.

## Scope

`Repository[T].Insert` changes from `(T, error)` to `(*T, error)`. `MustInsert` follows,
`T` → `*T`. `InsertMany`/`MustInsertMany` keep returning `[]T` (unchanged) — they already
dereference each `Insert` result into the slice, so only their internal plumbing (not their public
signature) changes.

## Goals

- [ ] `Insert(ctx, *T) (*T, error)` — on failure, returns `(nil, err)` instead of `(zero, err)`
- [ ] `MustInsert(ctx, *T) *T`
- [ ] `InsertMany` internals updated to dereference `Insert`'s `*T` result into its `[]T` slice
- [ ] Every call site across the repo (`.examples/*`, `internal/dialecttest/*`, README,
      `docs/guides/repository.md`) updated for the new pointer type

## Non-goals

- No behavior change beyond the return type — same hooks (`BeforeCreate`/`AfterCreate`/
  `OnConflictCreate`), same SQL, same error conditions.
- `InsertMany`, `FindMany`, `SaveMany`, `Update`, `Delete`, `Exec` keep returning `[]T` (value
  elements) — only single-row methods return `*T`. `Count`/`Exists` keep their scalar returns.

## Acceptance Criteria

1. WHEN `Insert` succeeds THEN system SHALL return a non-nil `*T` populated the same way the old
   `T` value was.
2. WHEN `Insert` fails (validation, hook, conflict, or driver error) THEN system SHALL return
   `(nil, err)`.
3. WHEN `task coverage` runs THEN `Insert`/`MustInsert`/`InsertMany`/`MustInsertMany` SHALL keep
   100.0% coverage.

## Out of Scope

| Feature | Reason |
|---|---|
| Changing `InsertMany`'s public signature | Already `[]T`, consistent with every other Many-method; no reason to make it `[]*T` |
