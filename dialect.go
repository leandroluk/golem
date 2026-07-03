package golem

import (
	"context"
	"database/sql/driver"
)

// Dialect knows how to bind a Go value to a driver.Value for a given
// ColumnType, and scan a raw driver value back into a Go destination.
// Adapters (postgres, and later mysql/mssql/oracle) implement this.
type Dialect interface {
	Bind(t ColumnType, value any) (driver.Value, error)
	Scan(t ColumnType, raw any, dest any) error

	// Insert executes an INSERT for the given table/columns/values and
	// returns the resulting row (e.g. via RETURNING * on Postgres) as a
	// column-name-keyed map. conn is accepted for future Tx support (M8) —
	// this pass's adapters are not required to use it (a real
	// implementation may use its own stored connection/pool instead).
	Insert(ctx context.Context, conn Conn, table string, columns []string, values []driver.Value) (map[string]any, error)

	// FindByID fetches one row by a single-column primary key. found is
	// false (with a nil error) when no row matches.
	FindByID(ctx context.Context, conn Conn, table string, pkColumn string, id driver.Value) (map[string]any, bool, error)
}
