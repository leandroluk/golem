# M29 — Go 1.27 Generic Methods Upgrade Design

## Architectural Changes

### 1. Folding `internal/join` into `internal/query`

In Go < 1.27, `query.Query[T]` could not declare a method taking `*entity.Entity[J]` with type parameter `J`. To prevent circular dependencies and work around the language limitation, `internal/join` was created with `Inner[T, J]`, `Left[T, J]`, etc.

With Go 1.27 generic methods:
- `query.Query[T]` directly provides `InnerJoin[J any]`, `LeftJoin[J any]`, `RightJoin[J any]`, and `FullJoin[J any]`.
- Each join method constructs `query.NewJoin[J]()`, executes the configuration callback, extracts column metadata via `target.Describe()`, and appends `JoinData`.
- All methods return `*Query[T]` to allow seamless fluent chaining.
- The entire `internal/join` package is removed.

### 2. Upgrading `Repository[T]` to Host `Aggregate` and `Preload`

`Repository[T]` already owns the database connection, dialect, entity metadata, and scanning logic. `Aggregate` and `Preload` were only free functions because they introduced `R` (result struct) and `J` (joined entity type) respectively.

- `Repository[T].Aggregate[R any](ctx context.Context, fn func(t *T, res *R, a *query.Aggregate[T, R])) ([]R, error)`
- `Repository[T].MustAggregate[R any](ctx context.Context, fn func(t *T, res *R, a *query.Aggregate[T, R])) []R`
- `Repository[T].Preload[J any](ctx context.Context, items []T, target *entity.Entity[J], criteria ...func(*J, *query.Query[J])) (map[any][]J, error)`
- `Repository[T].MustPreload[J any](ctx context.Context, items []T, target *entity.Entity[J], criteria ...func(*J, *query.Query[J])) map[any][]J`

The `Must*` variants follow the convention established in M27 (AD-060) using `internal/must.Value`.

### 3. Entity Lifecycle Hooks via `Entity[T].Hooks()`

`golem.AddHook(e)` was an awkward free function. In Go 1.27, `Entity[T]` provides:
```go
func (e *Entity[T]) Hooks() *HookBuilder[T] {
    return &HookBuilder[T]{entity: e}
}
```
Usage:
```go
UserEntity.Hooks().BeforeCreate(func(ctx context.Context, u *User, conn golem.Conn) error {
    ...
})
```

### 4. Compile-Time Type Safety for `Table.ForeignKey`

In `Table`:
```go
func (b *Table) ForeignKey[J any](fieldPtr any, target *Entity[J], opts ...*relation.ForeignKeyOptions)
```
Because `target` is now statically typed as `*Entity[J]`, invalid types cannot be passed at compile time, eliminating the runtime `target.(describer)` type assertion and corresponding panic branch.

### 5. `Plan.ScanFromMap` Method

`scanner.Plan` provides:
```go
func (p *Plan) ScanFromMap[T any](row map[string]any) (T, error)
```
Retains the existing zero-allocation fast-path scanning engine while turning the free function into an object-oriented method on the compiled plan.
