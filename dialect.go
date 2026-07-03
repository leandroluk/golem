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

	// Select returns every row matching the AND-ed column=value pairs.
	// Empty whereColumns/whereValues means no filter (all rows).
	Select(ctx context.Context, conn Conn, table string, whereColumns []string, whereValues []driver.Value) ([]map[string]any, error)

	// Update runs UPDATE ... SET ... WHERE ... RETURNING *, returning
	// every updated row (0+).
	Update(ctx context.Context, conn Conn, table string, setColumns []string, setValues []driver.Value, whereColumns []string, whereValues []driver.Value) ([]map[string]any, error)
}
