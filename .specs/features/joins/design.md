# Design — Joins (Milestone 6)

## Architecture

- **`join` package**: Exposes `Inner`, `Left`, `Right`, and `Full` generic helpers which accept the parent query, target entity, and a join configuration callback.
- **Join registry**: Registrations populate type-erased `query.JoinData` inside `query.Query[T]` to avoid circular package imports.
- **Dialect compilation**: The Postgres dialect parses the query's joins list, generates `JOIN table ON keys` clauses, appends joined-table predicates to the `WHERE` clause with proper prefixing, and injects soft-delete clauses.
- **Reflection scanning**: Row scanning maps returned columns to the root entity's fields. Column projections are prefixed dynamically to avoid query result collisions.
