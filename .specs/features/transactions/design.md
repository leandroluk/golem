# Design — Transactions (Milestone 8)

## Architecture

- **`golem.Tx`**: Sealed interface extending `golem.Conn`.
- **`golem.TxConn`**: Database-agnostic transaction connection interface (exposing `Commit` and `Rollback`) implemented by driver package transaction wrappers.
- **`txImpl`**: Internal struct in package `golem` satisfying `golem.Tx` and delegates calls to the underlying `TxConn`. Exposes `Underlying() any`.
- **Dialect `Begin`**: `golem.Dialect` interface has `Begin(ctx, conn) (TxConn, error)` to initialize transactions on the database connector.
- **Dynamic Query Routing**: Dialects implement `getExecutor(conn golem.Conn)` to route executions to either the transaction `pgx.Tx` or the connection pool `pgxpool.Pool` depending on connection type.
