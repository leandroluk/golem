package golem

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/leandroluk/golem/internal/stmt"
)

type mockConnector struct {
	err error
}

func (c *mockConnector) Connect() (Dialect, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &mockDialect{}, nil
}

func (c *mockConnector) Close() error { return nil }

type mockDialect struct {
	beginErr error
	execErr  error
}

func (d *mockDialect) Bind(t ColumnType, value any) (driver.Value, error)  { return nil, nil }
func (d *mockDialect) Scan(t ColumnType, raw any, dest any) error          { return nil }
func (d *mockDialect) CompileSelect(s *stmt.Select) (string, []any, error) { return "", nil, nil }
func (d *mockDialect) CompileInsert(s *stmt.Insert) (string, []any, error) { return "", nil, nil }
func (d *mockDialect) CompileUpdate(s *stmt.Update) (string, []any, error) { return "", nil, nil }
func (d *mockDialect) CompileDelete(s *stmt.Delete) (string, []any, error) { return "", nil, nil }
func (d *mockDialect) Query(ctx context.Context, conn Conn, sql string, args []any) ([]map[string]any, error) {
	return nil, nil
}
func (d *mockDialect) Exec(ctx context.Context, conn Conn, sql string, args []any) (int64, error) {
	return 0, nil
}
func (d *mockDialect) IsConflict(err error) bool { return false }
func (d *mockDialect) ExecRaw(ctx context.Context, conn Conn, sql string, args []any) ([]map[string]any, int64, error) {
	if d.execErr != nil {
		return nil, 0, d.execErr
	}
	return nil, 1, nil
}
func (d *mockDialect) Begin(ctx context.Context, conn Conn) (TxConn, error) {
	if d.beginErr != nil {
		return nil, d.beginErr
	}
	return &mockTxConn{}, nil
}
func (d *mockDialect) Insert(ctx context.Context, conn Conn, s *stmt.Insert) (map[string]any, error) {
	return nil, nil
}
func (d *mockDialect) Update(ctx context.Context, conn Conn, s *stmt.Update) ([]map[string]any, error) {
	return nil, nil
}

type mockTxConn struct{}

func (f *mockTxConn) Commit(ctx context.Context) error   { return nil }
func (f *mockTxConn) Rollback(ctx context.Context) error { return nil }

func TestDataSource_IsConn(t *testing.T) {
	var _ Conn = (*DataSource)(nil)
	var ds DataSource
	ds.isConn() // just to cover
}

// newTestDataSource builds a DataSource under a name unique to the calling
// test (so parallel/sequential NewDataSource calls across this file's tests
// never collide in the process-wide registry) and registers t.Cleanup to
// Close it, freeing the name again.
func newTestDataSource(t *testing.T, opts ...Option) *DataSource {
	t.Helper()
	ds, err := NewDataSource(append([]Option{DataSourceName(t.Name())}, opts...)...)
	if err != nil {
		t.Fatalf("NewDataSource: %v", err)
	}
	t.Cleanup(func() { ds.Close() })
	return ds
}

func TestNewDataSource_ErrorNoConnector(t *testing.T) {
	_, err := NewDataSource(DataSourceName(t.Name()))
	if err == nil {
		t.Fatal("expected error when no connector configured")
	}
}

func TestNewDataSource_DefaultName(t *testing.T) {
	ds, err := NewDataSource(WithConnector(&mockConnector{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ds.Close()
	if ds.Name() != "default" {
		t.Fatalf("Name() = %q, want default", ds.Name())
	}
}

func TestNewDataSource_CustomName(t *testing.T) {
	ds, err := NewDataSource(WithConnector(&mockConnector{}), DataSourceName("primary"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ds.Close()
	if ds.Name() != "primary" {
		t.Fatalf("Name() = %q, want primary", ds.Name())
	}
}

func TestNewDataSource_ErrorDuplicateName(t *testing.T) {
	ds, err := NewDataSource(WithConnector(&mockConnector{}), DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ds.Close()

	_, err = NewDataSource(WithConnector(&mockConnector{}), DataSourceName(t.Name()))
	if err == nil {
		t.Fatal("expected error registering a second DataSource under the same name")
	}
}

func TestMustNewDataSource_Success(t *testing.T) {
	ds := MustNewDataSource(WithConnector(&mockConnector{}), DataSourceName(t.Name()))
	defer ds.Close()
	if ds.Name() != t.Name() {
		t.Fatalf("Name() = %q, want %q", ds.Name(), t.Name())
	}
}

func TestMustNewDataSource_PanicsOnError(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when no connector configured")
		}
	}()
	MustNewDataSource(DataSourceName(t.Name()))
}

func TestDataSource_Connect_Error(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{err: errors.New("connect error")}))
	err := ds.Connect()
	if err == nil || err.Error() != "connect error" {
		t.Fatalf("expected connect error, got %v", err)
	}
}

func TestDataSource_Connect_IdempotentNoOp(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{}))
	if err := ds.Connect(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ds.Connect(); err != nil {
		t.Fatalf("expected no-op success on second Connect, got %v", err)
	}
}

func TestDataSource_Close_NeverConnected_NoOp(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{}))
	if err := ds.Close(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDataSource_Close_AfterConnect(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{}))
	ds.Connect()
	if err := ds.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDataSource_Exec_Success(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{}))
	ds.Connect()
	res, err := ds.Exec(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil Result")
	}
}

func TestDataSource_Transaction_Success(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{}))
	ds.Connect()
	err := ds.Transaction(context.Background(), func(tx Tx) error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDataSource_Transaction_FnError_Rollback(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{}))
	ds.Connect()
	fnErr := errors.New("fn error")
	err := ds.Transaction(context.Background(), func(tx Tx) error { return fnErr })
	if err != fnErr {
		t.Fatalf("expected fn error, got %v", err)
	}
}

func TestDataSource_Transaction_FnPanic_RollbackAndRepropagate(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{}))
	ds.Connect()

	defer func() {
		r := recover()
		if r != "boom" {
			t.Fatalf("expected panic 'boom' to propagate, got %v", r)
		}
	}()

	ds.Transaction(context.Background(), func(tx Tx) error { panic("boom") })
}

func TestDataSource_Transaction_ErrorDisconnected(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{}))
	err := ds.Transaction(context.Background(), func(tx Tx) error { return nil })
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDataSource_Transaction_ErrorBegin(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{}))
	ds.Connect()
	ds.dialect.(*mockDialect).beginErr = errors.New("begin error")
	err := ds.Transaction(context.Background(), func(tx Tx) error { return nil })
	if err == nil || err.Error() != "begin error" {
		t.Fatal("expected begin error")
	}
}

func TestDataSource_Exec_ErrorDisconnected(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{}))
	_, err := ds.Exec(context.Background(), "SELECT 1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDataSource_Exec_ErrorExec(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{}))
	ds.Connect()
	ds.dialect.(*mockDialect).execErr = errors.New("exec error")
	_, err := ds.Exec(context.Background(), "SELECT 1")
	if err == nil || err.Error() != "exec error" {
		t.Fatal("expected exec error")
	}
}

func TestGetDataSource_Success(t *testing.T) {
	ds := newTestDataSource(t, WithConnector(&mockConnector{}))
	got, err := GetDataSource(t.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ds {
		t.Fatalf("GetDataSource returned a different instance")
	}
}

func TestGetDataSource_DefaultName(t *testing.T) {
	ds, err := NewDataSource(WithConnector(&mockConnector{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ds.Close()

	got, err := GetDataSource()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ds {
		t.Fatalf("GetDataSource() returned a different instance than the \"default\" one")
	}
}

func TestGetDataSource_ErrorNotFound(t *testing.T) {
	_, err := GetDataSource(t.Name())
	if !errors.Is(err, ErrDataSourceNotFound) {
		t.Fatalf("expected ErrDataSourceNotFound, got %v", err)
	}
}

func TestGetDataSource_ErrorAfterClose(t *testing.T) {
	ds, err := NewDataSource(WithConnector(&mockConnector{}), DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ds.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = GetDataSource(t.Name())
	if !errors.Is(err, ErrDataSourceNotFound) {
		t.Fatalf("expected ErrDataSourceNotFound, got %v", err)
	}
}

func TestTxImpl_IsConn(t *testing.T) {
	var _ Conn = (*txImpl)(nil)
	var tx txImpl
	tx.isConn()
}

func TestTxImpl_Exec_Error(t *testing.T) {
	d := &mockDialect{execErr: errors.New("exec error")}
	tx := NewTx(d, &mockTxConn{})
	_, err := tx.Exec(context.Background(), "SELECT 1")
	if err == nil || err.Error() != "exec error" {
		t.Fatal("expected exec error")
	}
}

func TestTxImpl_Exec_Success(t *testing.T) {
	d := &mockDialect{}
	tx := NewTx(d, &mockTxConn{})
	res, err := tx.Exec(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil Result")
	}
}

func TestRawResult_Scan_OutOfBounds(t *testing.T) {
	res := &rawResult{rows: []map[string]any{}, currentIndex: -1}
	_, err := res.Scan()
	if err == nil || err.Error() != "golem: Scan called out of bounds or before Next" {
		t.Fatal("expected scan out of bounds err")
	}
}
