# Repository Must* Variants Specification

**Driving use case:** callers who already treat a `Repository[T]` error as unrecoverable (setup/seed
scripts, tests, code paths already inside a `recover()` boundary) currently must write
`res, err := repo.Insert(...); if err != nil { panic(err) }` by hand at every call site.

## Scope

Add a `Must`-prefixed sibling for every existing error-returning `Repository[T]` method. Each
`Must*` method has the identical parameters and identical non-error return value(s) as its
counterpart, drops `error` from the return signature, and panics (via `internal/must`) instead of
returning a non-nil error.

## Goals

- [ ] `MustInsert(ctx, *T) T` — wraps `Insert`
- [ ] `MustInsertMany(ctx, ...*T) []T` — wraps `InsertMany`
- [ ] `MustFindMany(ctx, ...criteria) []T` — wraps `FindMany`
- [ ] `MustFindOne(ctx, ...criteria) *T` — wraps `FindOne`
- [ ] `MustSaveOne(ctx, *T) *T` — wraps `SaveOne`
- [ ] `MustSaveMany(ctx, ...*T) []T` — wraps `SaveMany`
- [ ] `MustUpdate(ctx, criteria) []T` — wraps `Update`
- [ ] `MustDelete(ctx, criteria) []T` — wraps `Delete`
- [ ] `MustRestore(ctx, ...*T)` — wraps `Restore` (no non-error return, so just `must.Exec`)
- [ ] `MustCount(ctx, ...criteria) int64` — wraps `Count`
- [ ] `MustExists(ctx, ...criteria) bool` — wraps `Exists`
- [ ] `MustExec(ctx, sql, ...args) []T` — wraps `Exec`

## Non-goals

- No new behavior — every `Must*` body is a 1-line delegation (`must.Value(r.X(...))` or
  `must.Exec(r.X(...))`), zero duplicated logic.
- No change to the non-`Must` methods' signatures or behavior.
- `golem.go` needs no new reexport — `golem.Repository[T]` is a type alias
  (`type Repository[T any] = repository.Repository[T]`), so new methods on
  `repository.Repository[T]` are automatically visible as `golem.Repository[T]` methods.

## Acceptance Criteria

1. WHEN any `Must*` method's wrapped call succeeds THEN system SHALL return exactly what the
   wrapped method returned (minus the nil error).
2. WHEN any `Must*` method's wrapped call returns a non-nil error THEN system SHALL panic with
   that error (via `must.Value`/`must.Exec`, i.e. `panic(err)`), not a wrapped/annotated variant.
3. WHEN `task coverage` runs THEN every `Must*` method SHALL be covered by both a success-path and
   a panic-path unit test, keeping the repo's 100.0% coverage bar (`internal/must` panic paths are
   already covered by table-driven tests using a `fakeDialect` that can be told to error).

## Out of Scope

| Feature | Reason |
|---|---|
| `Must` variant of `Exec`'s underlying `conn.Exec` (raw connection-level) | Only `Repository[T]` methods are in scope, per user request |
| Doc comments explaining panic semantics on the non-`Must` methods | Existing docs stay as-is; only `Must*` methods gain new doc comments |
