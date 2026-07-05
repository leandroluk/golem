package golem

import (
	"context"
	"database/sql/driver"

	"github.com/leandroluk/golem/internal/stmt"
)

// Dialect knows how to bind a Go value to a driver.Value for a given
// ColumnType, and scan a raw driver value back into a Go destination.
// It also provides statement compilation and execution mappings.
// Adapters (postgres, and later mysql/mssql/oracle) implement this.
type Dialect interface {
	Bind(t ColumnType, value any) (driver.Value, error)
	Scan(t ColumnType, raw any, dest any) error

	// Insert executes an INSERT for the given statement and returns the
	// resulting row as a column-name-keyed map.
	Insert(ctx context.Context, conn Conn, s *stmt.Insert) (map[string]any, error)

	// Update runs UPDATE for the given statement, returning all updated rows.
	Update(ctx context.Context, conn Conn, s *stmt.Update) ([]map[string]any, error)

	// CompileSelect compiles a Select statement plan to SQL and args.
	CompileSelect(s *stmt.Select) (sql string, args []any, err error)

	// CompileDelete compiles a Delete statement plan to SQL and args.
	CompileDelete(s *stmt.Delete) (sql string, args []any, err error)

	// Query executes a compiled query statement returning a slice of row maps.
	Query(ctx context.Context, conn Conn, sql string, args []any) ([]map[string]any, error)

	// Exec executes a compiled command query (returning rows affected count).
	Exec(ctx context.Context, conn Conn, sql string, args []any) (int64, error)

	// IsConflict returns true if the error represents a database integrity constraint violation.
	IsConflict(err error) bool

	// ExecRaw executes a raw SQL statement, returning the list of returned rows (if any) and rows affected count.
	ExecRaw(ctx context.Context, conn Conn, sql string, args []any) ([]map[string]any, int64, error)

	// Begin starts a new transaction on the database.
	Begin(ctx context.Context, conn Conn) (TxConn, error)
}
