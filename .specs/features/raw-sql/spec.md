# Spec — Raw SQL (Milestone 9)

## Features

- Support raw SQL execution via `golem.Conn.Exec` returning a `golem.Result` cursor.
- `golem.Result` exposes:
  - `Next() bool`
  - `Scan() (map[string]any, error)`
  - `RowsAffected() (int64, error)`
- Support typed raw queries via `Repository[T].Exec` returning scanned `[]T` slices.

## API Example
```go
result, err := dataSource.Exec(ctx, "UPDATE users SET name = $1 WHERE id = $2 RETURNING *", "Alice Updated", 1)
for result.Next() {
	row, err := result.Scan()
	// ...
}

users, err := repository.Get(dataSource, UserEntity).Exec(ctx, "SELECT * FROM users WHERE active = $1", true)
```
