package postgres

import (
	"context"
	"database/sql/driver"
	"fmt"

	"github.com/leandroluk/golem"
)

// dialect is the M1 stub Postgres implementation of golem.Dialect. It exists
// so the contract compiles and is satisfied end-to-end; it has no real
// ColumnType set to bind/scan yet (that's a future milestone) so every call
// returns a descriptive error instead of panicking.
type dialect struct{}

var _ golem.Dialect = (*dialect)(nil)

func (dialect) Bind(t golem.ColumnType, value any) (driver.Value, error) {
	return nil, fmt.Errorf("postgres: unrecognized column type %v", t)
}

func (dialect) Scan(t golem.ColumnType, raw any, dest any) error {
	return fmt.Errorf("postgres: unrecognized column type %v", t)
}

func (dialect) Insert(ctx context.Context, conn golem.Conn, table string, columns []string, values []driver.Value) (map[string]any, error) {
	return nil, fmt.Errorf("postgres: Insert not yet implemented")
}

func (dialect) FindByID(ctx context.Context, conn golem.Conn, table string, pkColumn string, id driver.Value) (map[string]any, bool, error) {
	return nil, false, fmt.Errorf("postgres: FindByID not yet implemented")
}
