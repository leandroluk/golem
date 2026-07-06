# Aggregations (M13) Tasks

**Design**: `.specs/features/aggregations/design.md`
**Status**: All done (implemented directly, single interlocking feature)

---

## T1: `internal/stmt` — `Projection`, `Select.GroupBy`/`.Having`, `AggregateComparison` — DONE

`internal/stmt/stmt.go`: `Projection{Column, Func, Alias}`, `Select` gains `Projections []Projection`/`GroupBy []string`/`Having Predicate`, new `AggregateComparison` predicate variant.

## T2: `driver/postgres` dialect — DONE

`aggregateSQLFunc`/`projectionSQL` helpers; `CompileSelect` renders `Projections`/`GroupBy`/`Having` when present; `compilePredicate` gains an `AggregateComparison` case. 100% coverage on all new code (`driver/postgres/aggregate_sql_test.go`) — the only uncovered line in `CompileSelect` is a pre-existing `Where`/`Join.Where` error-propagation gap that predates M13.

## T3: `query.Aggregate[T, R]` — DONE

`query/aggregate.go`: fluent builder, mirrors `query.Query[T]`'s accumulate-don't-replace pattern. 100% coverage (`query/aggregate_test.go`).

## T4: `repository.Aggregate[T, R]` — DONE

`repository/aggregate.go`: resolves `T`/`R` field pointers, builds the `stmt.Select` plan, executes, scans results. Refactored `scanRow`'s field-assignment logic into a shared `assignFieldValue` helper (reused here) — pure extraction, no behavior change (existing `scanRow` tests still pass unchanged). 100% coverage on `Aggregate` itself (`repository/aggregate_test.go`, 20 cases).

## T5: Example + integration test — DONE

`.examples/postgres/main_integration_test.go`: `TestBlogExample_Aggregate_PostCountPerUser` — `GroupBy`+`CountAll`+`Where`+`Having` against real Postgres.

## T6: README.md — DONE

New "Aggregations" section (between Preload and Raw SQL), Contents ToC entry, Implementation Status checkbox.
