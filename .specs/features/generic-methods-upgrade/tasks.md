# M29 — Go 1.27 Generic Methods Upgrade Tasks

## Task Breakdown

### Phase 1: Toolchain & Core Engine Upgrades
- [x] **T1: Toolchain update**
  - Update `go.mod` to `go 1.27`.
  - Gate: `go version` confirms Go 1.27+ and `go mod tidy` succeeds.

- [x] **T2: Query Join Methods & Eliminate `internal/join`**
  - Implement `InnerJoin`, `LeftJoin`, `RightJoin`, and `FullJoin` on `query.Query[T]`.
  - Migrate tests from `internal/join` into `internal/query` or root tests.
  - Remove directory `internal/join`.
  - Gate: `go test ./internal/query/...` green.

- [x] **T3: Repository Methods (`Aggregate`, `MustAggregate`, `Preload`, `MustPreload`)**
  - Refactor `internal/repository/aggregate.go` to attach `Aggregate[R any]` and `MustAggregate[R any]` as methods on `*Repository[T]`.
  - Refactor `internal/repository/preload.go` to attach `Preload[J any]` and `MustPreload[J any]` as methods on `*Repository[T]`.
  - Update `aggregate_test.go` and `preload_test.go`.
  - Gate: `go test ./internal/repository/...` green.

- [x] **T4: Entity Hooks (`Entity[T].Hooks()`) & Type-Safe `Table.ForeignKey[J any]`**
  - Add `(e *Entity[T]) Hooks() *HookBuilder[T]`.
  - Update `(b *Table) ForeignKey[J any](fieldPtr any, target *Entity[J], opts ...*relation.ForeignKeyOptions)`.
  - Remove runtime describer assertion and panics.
  - Gate: `go test ./internal/entity/...` green.

- [x] **T5: Scanner Plan Method (`Plan.ScanFromMap[T any]`)**
  - Turn `ScanFromMap` into receiver method on `*Plan`.
  - Update scanner tests.
  - Gate: `go test ./internal/scanner/...` green.

### Phase 2: Public API & Call Sites Modernization
- [x] **T6: Modernize `golem.go` root exports and `golem_test.go`**
  - Remove `JoinInner`, `JoinLeft`, `JoinRight`, `JoinFull`.
  - Remove `RunAggregate`, `Preload`, `AddHook`.
  - Update `golem_test.go` to exercise the new receiver methods.
  - Gate: `go test . -short` green.

- [x] **T7: Modernize Dialect Tests, Drivers, and Examples**
  - Update all call sites in `internal/dialecttest/` (`joins.go`, `preload.go`, `aggregations.go`, etc.).
  - Update `.examples/` codebases.
  - Gate: `go test ./... -short` green.

### Phase 3: Documentation & Repository Sync
- [x] **T8: Documentation Overhaul**
  - Update `README.md` root documentation with the new fluent method calls.
  - Update `docs/guides/joins.md`, `docs/guides/aggregations.md`, `docs/guides/preload.md`, `docs/guides/schema.md`, `docs/guides/repository.md`, and `docs/examples/blog.md`.
  - Gate: No occurrences of deprecated free functions in markdown files.

- [x] **T9: Specs, State, Commit and Push**
  - Update `ROADMAP.md` (add M29) and `STATE.md` (record AD-062).
  - Verify 100% test coverage with `task coverage` or `go test ./... -short`.
  - Commit following conventional commit convention (`feat!: upgrade to Go 1.27 generic methods and eliminate standalone functions`).
  - Push branch to `origin`.
