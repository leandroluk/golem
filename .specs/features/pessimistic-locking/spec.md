# Pessimistic Locking (M14) Specification

## Status: ✅ DONE

---

## Goals

- [x] `query.Query[T]` gains `.ForUpdate(wait ...LockWait)`, `.ForNoKeyUpdate(...)`, `.ForShare(...)`, `.ForKeyShare(...)` — map to Postgres's `SELECT ... FOR {UPDATE|NO KEY UPDATE|SHARE|KEY SHARE}`
- [x] Each accepts an optional `query.LockWaitNoWait`/`query.LockWaitSkipLocked` (default: block, Postgres's own default)
- [x] `Repository[T].FindMany`/`FindOne` reject a locked query when the current `Conn` isn't a `golem.Tx` — locking outside a real transaction is a no-op that looks like it worked (the lock releases the instant the implicit single-statement transaction ends), so it's a loud error, not silent nothing
- [x] `repository.Aggregate` doesn't support locking at all — Postgres itself rejects `FOR UPDATE` combined with aggregate functions/`GROUP BY`, so there's nothing to wire up

## Acceptance Criteria

1. WHEN `q.ForUpdate()` is declared and `FindMany`/`FindOne` runs with `conn` being a `golem.Tx` THEN the compiled SQL SHALL end with `FOR UPDATE`.
2. WHEN `q.ForShare(query.LockWaitNoWait)` is declared THEN the compiled SQL SHALL end with `FOR SHARE NOWAIT`.
3. WHEN a lock is requested and `conn` is NOT a `golem.Tx` (e.g. a bare `*DataSource`) THEN `FindMany`/`FindOne` SHALL return an error without issuing any query.
4. WHEN two concurrent transactions both run `FindOne(...).ForUpdate()` against the same row THEN the second SHALL block until the first transaction commits or rolls back (verified against real Postgres, not just SQL text).
5. WHEN `LIMIT`/`OFFSET`/`ORDER BY` are also present THEN the lock clause SHALL be the last thing in the generated SQL (Postgres grammar requires this).

## Success Criteria

- [x] `go test ./query/... ./driver/postgres/... ./repository/...` — green, every new function at 100% statement coverage
- [x] `go build ./...` — no errors
- [x] `.examples/postgres`: `TestBlogExample_PessimisticLocking_ForUpdateBlocksConcurrentLocker` proves real blocking behavior against Postgres (two goroutines, two transactions, channel-synchronized); `TestBlogExample_ForUpdate_OutsideTransaction_ReturnsError` proves the outside-a-Tx guard
