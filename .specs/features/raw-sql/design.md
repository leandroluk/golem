# Design — Raw SQL (Milestone 9)

## Architecture

- **`golem.Result`**: Sealed cursor iterator interface.
- **`rawResult`**: Concrete implementation wrapping returning row maps `[]map[string]any` and a count pointer.
- **`Dialect.ExecRaw`**: Exposes query executions on the database dialect, collecting resulting row maps and parsing command tags.
- **`Repository.Exec`**: Executes SQL on the connection, loops over resulting cursors and maps returned fields onto `T` using the existing column-field reflection scanning.
