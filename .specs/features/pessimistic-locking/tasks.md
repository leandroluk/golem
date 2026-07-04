# Pessimistic Locking (M14) Tasks

**Design**: `.specs/features/pessimistic-locking/design.md`
**Status**: All done (implemented directly, single interlocking feature)

---

## T1: `query.Query[T]` lock methods — DONE

`query/lock.go`: `LockStrength`/`LockWait` enums, `ForUpdate`/`ForNoKeyUpdate`/`ForShare`/`ForKeyShare`. 100% coverage (`query/lock_test.go`).

## T2: `internal/stmt` + `driver/postgres` dialect — DONE

`stmt.LockClause{Strength, Wait}` on `stmt.Select`. `driver/postgres/dialect.go`'s `lockClauseSQL` renders the trailing `FOR ... [NOWAIT|SKIP LOCKED]` clause (after `ORDER BY`/`LIMIT`/`OFFSET`, matching Postgres grammar). 100% coverage (`driver/postgres/lock_sql_test.go`).

## T3: `Repository[T].FindMany` wiring + Tx guard — DONE

Reads `q.GetLockStrength()`/`.GetLockWait()`, builds `stmt.Select.Lock`, returns an error before compiling/querying if a lock is requested and `r.conn` isn't a `golem.Tx`. `FindOne` inherits this for free (calls `FindMany`). 100% coverage on the new branch (`repository/lock_test.go`).

## T4: Example + integration tests — DONE

`.examples/postgres-minimal-blog/main_integration_test.go`: `TestBlogExample_PessimisticLocking_ForUpdateBlocksConcurrentLocker` (real two-transaction blocking proof) and `TestBlogExample_ForUpdate_OutsideTransaction_ReturnsError` (guard proof), both against real Postgres.

## T5: README.md — DONE

New "Pessimistic Locking" section (between Aggregations and Raw SQL), Contents ToC entry, Implementation Status checkbox — this closes out ROADMAP.md's M1-M14, the full milestone list as of this writing.
