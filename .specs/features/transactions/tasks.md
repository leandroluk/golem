# Tasks — Transactions (Milestone 8)

- [x] Define `golem.Tx`, `golem.TxConn` and internal `txImpl` wrappers.
- [x] Add `Begin(ctx, conn)` to `golem.Dialect`.
- [x] Implement `DataSource.Transaction(ctx, fn)`.
- [x] Implement `Begin` and `pgTx` inside Postgres driver dialect.
- [x] Redirect query/exec calls to the active transaction inside the Postgres driver dialect.
- [x] Create transaction unit tests and integration tests.
