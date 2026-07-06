# Pessimistic Locking (M14) Design

**Spec**: `.specs/features/pessimistic-locking/spec.md`
**Status**: Approved

---

## API shape

`query.Query[T]` gains 4 chainable methods (`query/lock.go`), each a thin wrapper over one shared unexported `lock(strength, wait...)`:

```go
func (q *Query[T]) ForUpdate(wait ...LockWait) *Query[T]
func (q *Query[T]) ForNoKeyUpdate(wait ...LockWait) *Query[T]
func (q *Query[T]) ForShare(wait ...LockWait) *Query[T]
func (q *Query[T]) ForKeyShare(wait ...LockWait) *Query[T]
```

`LockStrength`/`LockWait` are string-backed enums (matches the existing `relation.OnDeleteAction`-style convention). Calling a `For*` method twice on the same `Query[T]` replaces the previous choice (last call wins) rather than accumulating — unlike `Where`/`OrderBy`, only one lock clause can ever apply to a single `SELECT`, so "accumulate" wouldn't mean anything here.

---

## Why the Tx-or-error guard exists

`SELECT ... FOR UPDATE` run outside an explicit transaction is valid SQL — Postgres wraps every standalone statement in an implicit transaction, takes the lock, and releases it the instant that statement finishes. The row is never actually held locked across any subsequent statement, which is the entire point of pessimistic locking (protect a read-then-write sequence from a concurrent writer). A caller who writes `repo.FindOne(ctx, func(...){ q.ForUpdate() })` against a bare `*DataSource` gets no error and no useful locking — a silent footgun.

`Repository[T].FindMany` (which `FindOne` calls) checks `q.GetLockStrength() != ""` and, if so, type-asserts `r.conn.(golem.Tx)`; on failure, returns an error before compiling or issuing any query. This forces the correct usage pattern (`dataSource.Transaction(ctx, func(tx golem.Tx) error { repository.Get(tx, Entity).FindOne(...).ForUpdate() ... })`) to be the only one that works, rather than documenting it as a "please remember" convention.

---

## Why `Aggregate` doesn't get locking

Postgres rejects `SELECT ... FOR UPDATE` when the query has `GROUP BY`, a HAVING clause, or any aggregate function in the target list ("`FOR UPDATE` is not allowed with aggregate functions") — this isn't a golem design choice, it's a hard Postgres restriction. `repository.Aggregate` never gained a `Lock` field on its generated `stmt.Select` at all; there's nothing to support.

---

## Real-blocking verification, not just SQL-shape verification

Every other milestone's example integration test asserts an outcome (a row exists/doesn't, a value changed). Pessimistic locking's entire value proposition is a *runtime concurrency property* — two overlapping transactions actually serialize on the same row — which SQL-shape assertions (unit tests checking `sql == "... FOR UPDATE"`) can't prove by themselves. `TestBlogExample_PessimisticLocking_ForUpdateBlocksConcurrentLocker` (`.examples/postgres/main_integration_test.go`) runs two real transactions from two goroutines against real Postgres, channel-synchronized:

1. Goroutine 1 opens a transaction, runs `FindOne(...).ForUpdate()`, signals "locked" over a channel, then blocks on a second channel waiting for permission to commit.
2. Main goroutine waits for the "locked" signal, then starts goroutine 2, which opens its own transaction and runs the same `FindOne(...).ForUpdate()` against the identical row.
3. Main sleeps briefly (300ms) — long enough for goroutine 2 to have issued its query and (if locking works) be blocked on it — then asserts goroutine 2's result channel has NOT received anything yet (a non-blocking `select`/`default`).
4. Main signals goroutine 1 to finish (commit, releasing the lock).
5. Main asserts goroutine 2's result channel receives within a generous timeout (5s) afterward.

This proves actual row-level blocking, not just that the generated SQL contains the right keywords.
