# Relations (M11) Tasks

**Design**: `.specs/features/relations/design.md`
**Status**: All done (implemented directly, not sub-agent-delegated, given the interlocking design decisions — see design.md)

---

## T1: `relation` package — DONE

`ForeignKeyOptions` fluent builder + all 5 option-value enum types (`CascadeOption`, `OnDeleteAction`, `OnUpdateAction`, `DeferrableMode`, `OrphanedRowActionMode`). 100% statement coverage (`relation/relation_test.go`).

## T2: `entity.Table.ForeignKey` — fix unused `target`, add opts — DONE

3rd variadic `...*relation.ForeignKeyOptions` argument. `target` now actually type-asserted (`describer` interface) and its `EntityMeta` read — previously silently ignored. Panics on non-describer target or composite-PK target. `ForeignKeyMeta` gains `ColumnName`/`TargetTableName`/`TargetPrimaryKey`/`Options`. Tests: `entity/table_foreignkey_test.go`.

## T3: FK registry (`entity/fk_registry.go`) — DONE

`registerForeignKey`/`ForeignKeysReferencing`, populated from `Table.finalize()`. Tests: `entity/fk_registry_test.go`.

## T4: `Repository[T].Delete` cascade wiring — DONE

`cascadeActionable`, `beginCascadeTx`, `applyDeleteCascades`, `countValue` helpers in `repository/repository.go`. `Delete` rewritten to consult `entity.ForeignKeysReferencing`, apply Restrict/Cascade/SetNull, wrap in an implicit transaction when needed. 100% coverage on every new function (`repository/delete_cascade_test.go`, 21 test cases covering all 3 actionable actions × soft-delete/hard-delete child variants × every error-propagation path × the already-in-a-Tx reuse case).

## T5: Example + integration test — DONE

`.examples/postgres-minimal-blog/entities.go`: `Post`'s `ForeignKey` to `User` now uses `OnDelete(relation.OnDeleteCascade)`. `TestBlogExample_CascadeDeleteUser_DeletesTheirPosts` in `main_integration_test.go` verifies against real Postgres. README.md's `ForeignKeyOptions` example corrected (`OnDelete(OnDeleteDefault)` → `OnDelete(OnDeleteCascade)`, matching its own descriptive comment) and annotated with which options have real runtime effect.
