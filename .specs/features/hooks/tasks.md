# Tasks — Hooks (Milestone 7)

- [x] Define hooks struct on `Entity[T]` and `Trigger*` wrapper methods.
- [x] Implement chainable `HookBuilder[T]` in `entity/hook.go`.
- [x] Add duplicate registration panic checks.
- [x] Add `IsConflict(err)` to the `golem.Dialect` interface.
- [x] Implement conflict checking in the Postgres dialect.
- [x] Wire hook execution in Repository write paths (`Insert`, `SaveOne`, `UpdateOne`/`Many`, `Delete`).
- [x] Create hook unit and integration tests.
