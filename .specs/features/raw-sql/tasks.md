# Tasks — Raw SQL (Milestone 9)

- [x] Define `golem.Result` interface and `rawResult` implementation in `conn.go`.
- [x] Add `Exec` to the `golem.Conn` interface.
- [x] Implement `DataSource.Exec` and `txImpl.Exec`.
- [x] Add `ExecRaw` to `golem.Dialect`.
- [x] Implement `ExecRaw` inside Postgres dialect collecting rows and mapping CommandTag.
- [x] Implement `Repository.Exec` executing raw queries and mapping rows through reflection.
- [x] Create raw SQL unit and integration tests.
