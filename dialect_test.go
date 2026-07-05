package golem

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/leandroluk/golem/internal/stmt"
)

type fakeDialect struct{}

func (fakeDialect) Bind(t ColumnType, value any) (driver.Value, error) { return value, nil }
func (fakeDialect) Scan(t ColumnType, raw any, dest any) error         { return nil }

func (fakeDialect) Insert(ctx context.Context, conn Conn, s *stmt.Insert) (map[string]any, error) {
	return nil, nil
}

func (fakeDialect) Update(ctx context.Context, conn Conn, s *stmt.Update) ([]map[string]any, error) {
	return nil, nil
}

func (fakeDialect) CompileSelect(s *stmt.Select) (string, []any, error) {
	return "", nil, nil
}

func (fakeDialect) CompileDelete(s *stmt.Delete) (string, []any, error) {
	return "", nil, nil
}

func (fakeDialect) Query(ctx context.Context, conn Conn, sql string, args []any) ([]map[string]any, error) {
	return nil, nil
}

func (fakeDialect) Exec(ctx context.Context, conn Conn, sql string, args []any) (int64, error) {
	return 0, nil
}

func (fakeDialect) IsConflict(err error) bool {
	return false
}

func (fakeDialect) ExecRaw(ctx context.Context, conn Conn, sql string, args []any) ([]map[string]any, int64, error) {
	return nil, 0, nil
}

type fakeTx struct{}

func (fakeTx) Commit(ctx context.Context) error {
	return nil
}

func (fakeTx) Rollback(ctx context.Context) error {
	return nil
}

func (fakeDialect) Begin(ctx context.Context, conn Conn) (TxConn, error) {
	return &fakeTx{}, nil
}

var _ Dialect = (*fakeDialect)(nil)

func TestFakeDialect_Bind(t *testing.T) {
	d := fakeDialect{}
	got, err := d.Bind(ColumnType{}, "value")
	if err != nil {
		t.Fatalf("Bind returned error: %v", err)
	}
	if got != "value" {
		t.Fatalf("Bind = %v, want %v", got, "value")
	}
}

func TestFakeDialect_Scan(t *testing.T) {
	d := fakeDialect{}
	var dest any
	if err := d.Scan(ColumnType{}, "raw", &dest); err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
}
