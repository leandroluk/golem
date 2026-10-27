# M29 — Go 1.27 Generic Methods Upgrade Specification

## Overview

Go 1.27 introduces generic methods on concrete types (`func (r Receiver[T]) Method[U any](...)`). Previously, any operation needing a type parameter beyond the receiver's had to be implemented as a standalone package-level function (e.g. `golem.JoinInner`, `golem.RunAggregate`, `golem.Preload`, `golem.AddHook`).

This milestone eliminates these standalone functions without backward compatibility, moving all generic operations to idiomatic receiver methods on `Query[T]`, `Repository[T]`, `Entity[T]`, `Table`, and `Plan`.

---

## Scope

1. **Bump toolchain**: `go.mod` updated to `go 1.27`.
2. **`Query[T]` Joins**:
   - Add receiver methods on `query.Query[T]`:
     - `InnerJoin[J any](target *entity.Entity[J], fn func(j *J, q1 *Join[J])) *Query[T]`
     - `LeftJoin[J any](target *entity.Entity[J], fn func(j *J, q1 *Join[J])) *Query[T]`
     - `RightJoin[J any](target *entity.Entity[J], fn func(j *J, q1 *Join[J])) *Query[T]`
     - `FullJoin[J any](target *entity.Entity[J], fn func(j *J, q1 *Join[J])) *Query[T]`
   - Delete `internal/join` package and `JoinInner`/`JoinLeft`/`JoinRight`/`JoinFull` in `golem.go`.
3. **`Repository[T]` Aggregate & Preload**:
   - Add receiver methods on `repository.Repository[T]`:
     - `Aggregate[R any](ctx context.Context, fn func(t *T, res *R, a *query.Aggregate[T, R])) ([]R, error)`
     - `MustAggregate[R any](ctx context.Context, fn func(t *T, res *R, a *query.Aggregate[T, R])) []R`
     - `Preload[J any](ctx context.Context, items []T, target *entity.Entity[J], criteria ...func(*J, *query.Query[J])) (map[any][]J, error)`
     - `MustPreload[J any](ctx context.Context, items []T, target *entity.Entity[J], criteria ...func(*J, *query.Query[J])) map[any][]J`
   - Remove standalone `RunAggregate` and `Preload` from `golem.go`.
4. **`Entity[T]` Hooks**:
   - Add receiver method on `entity.Entity[T]`:
     - `Hooks() *HookBuilder[T]`
   - Remove standalone `AddHook` from `golem.go` and `internal/entity`.
5. **`Table.ForeignKey` Type Safety**:
   - Upgrade `Table.ForeignKey` to generic method:
     - `ForeignKey[J any](fieldPtr any, target *Entity[J], opts ...*relation.ForeignKeyOptions)`
   - Eliminates `target any` and runtime interface assertions.
6. **`Plan.ScanFromMap`**:
   - Add receiver method on `scanner.Plan`:
     - `ScanFromMap[T any](row map[string]any) (T, error)`
7. **Documentation & Call Sites**:
   - Update `README.md`, `docs/`, examples, and dialect test suites.

---

## Acceptance Criteria

1. WHEN declaring joins on a query THEN caller SHALL chain `q.InnerJoin(...)`, `q.LeftJoin(...)`, `q.RightJoin(...)`, or `q.FullJoin(...)` directly on `q`.
2. WHEN running aggregate queries THEN caller SHALL call `repo.Aggregate[R](ctx, ...)` or `repo.MustAggregate[R](ctx, ...)`.
3. WHEN preloading relationships THEN caller SHALL call `repo.Preload[J](ctx, ...)` or `repo.MustPreload[J](ctx, ...)`.
4. WHEN registering entity hooks THEN caller SHALL call `entity.Hooks().BeforeCreate(...)`.
5. WHEN declaring foreign keys THEN `b.ForeignKey(&p.UserID, UserEntity)` SHALL be verified at compile time with generic type parameter `*Entity[J]`.
6. WHEN running tests THEN `task test` and `go test ./... -short` SHALL pass cleanly with 100% statement coverage maintained.
7. WHEN reviewing documentation THEN all guides in `docs/` and root `README.md` SHALL demonstrate the new method signatures with zero mentions of deprecated standalone functions.

---

## Out of Scope

- Interface methods with generic type parameters (forbidden by Go 1.27 language specification).
