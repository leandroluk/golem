# Design — Hooks (Milestone 7)

## Architecture

- **`HookBuilder[T]`**: Chainable registration API on `*Entity[T]`. Internal slots are stored as field callback pointers on `Entity[T]`.
- **Interception of Conflicts**: `golem.Dialect` interface exposes `IsConflict(err error) bool` to identify database integrity constraint violations (like unique constraint violation).
- **Wiring in Repository**: `repository.Repository[T]` triggers the hooks:
  - `Insert` executes `BeforeCreate` -> DB insert query -> `AfterCreate` (on success) or `OnConflictCreate` (if conflict occurs).
  - `SaveOne` executes `BeforeUpdate` -> DB update query -> `AfterUpdate`/`OnConflictUpdate`.
  - `UpdateOne`/`UpdateMany` execute `AfterUpdate` for each returned updated row.
  - `Delete` executes `BeforeDelete` -> DB delete -> `AfterDelete`/`OnConflictDelete` for each row.
