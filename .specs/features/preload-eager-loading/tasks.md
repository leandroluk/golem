# Preload / Eager Loading (M12) Tasks

**Design**: `.specs/features/preload-eager-loading/design.md`
**Status**: All done (implemented directly, single interlocking feature)

---

## T1: `repository.Preload[T, J any]` + `resolvePreloadJoin` — DONE

`repository/preload.go`. Reuses `entity.ForeignKeysReferencing` (M11), `Repository[T]`'s existing unexported helpers (`buildWherePredicate`, `fieldToColumn`, `applySoftDeleteFilter`, `scanRow`, `resolveFieldPtrAny`). Both FK directions (parent→children, child→parent) supported via one function. 100% statement coverage: `repository/preload_test.go` (15 cases — both directions, empty items, no-FK error, composite-PK error, criteria filter, soft-delete filter, key dedup, bad field-pointer criteria error, scan error, compile/query error propagation, ordering).

## T2: Example + integration test — DONE

`.examples/postgres/main_integration_test.go`: `TestBlogExample_Preload_LoadsPostsPerUser` — 2 users, 3 posts (2+1 split), verifies `repository.Preload(ctx, userRepo, users, PostEntity)` groups correctly against real Postgres.

## T3: README.md — DONE

New "Preload / Eager Loading" section (between Joins and Raw SQL), Contents ToC entry, Implementation Status checklist (M11/M12 marked done, M10 checkbox fixed — it had been left unchecked after M10 actually shipped).
