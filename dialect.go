package golem

import "database/sql/driver"

// Dialect knows how to bind a Go value to a driver.Value for a given
// ColumnType, and scan a raw driver value back into a Go destination.
// Adapters (postgres, and later mysql/mssql/oracle) implement this.
type Dialect interface {
	Bind(t ColumnType, value any) (driver.Value, error)
	Scan(t ColumnType, raw any, dest any) error
}
