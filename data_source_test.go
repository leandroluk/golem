package golem

import (
	"errors"
	"testing"
)

// dsFakeConnector is a uniquely-named fake Connector for this test file
// (distinct from fakeConnector in connector_test.go) because it needs call
// counters and a configurable error, which the sibling fake doesn't have.
type dsFakeConnector struct {
	connectCalls int
	closeCalls   int
	connectErr   error
	dialect      Dialect
}

func (f *dsFakeConnector) Connect() (Dialect, error) {
	f.connectCalls++
	if f.connectErr != nil {
		return nil, f.connectErr
	}
	return f.dialect, nil
}

func (f *dsFakeConnector) Close() error {
	f.closeCalls++
	return nil
}

var _ Connector = (*dsFakeConnector)(nil)

func TestNewDataSource_NoConnector(t *testing.T) {
	ds, err := NewDataSource()
	if err == nil {
		t.Fatal("expected error when no connector configured, got nil")
	}
	if ds != nil {
		t.Fatalf("expected nil DataSource, got %v", ds)
	}
}

func TestNewDataSource_WithConnector_DefaultName(t *testing.T) {
	fake := &dsFakeConnector{dialect: fakeDialect{}}
	ds, err := NewDataSource(WithConnector(fake))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ds == nil {
		t.Fatal("expected non-nil DataSource")
	}
	if ds.Name() != "default" {
		t.Fatalf("expected name %q, got %q", "default", ds.Name())
	}
}

func TestNewDataSource_WithName(t *testing.T) {
	fake := &dsFakeConnector{dialect: fakeDialect{}}
	ds, err := NewDataSource(DataSourceName("example"), WithConnector(fake))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ds.Name() != "example" {
		t.Fatalf("expected name %q, got %q", "example", ds.Name())
	}
}

func TestDataSource_Connect_Success(t *testing.T) {
	wantDialect := fakeDialect{}
	fake := &dsFakeConnector{dialect: wantDialect}
	ds, err := NewDataSource(WithConnector(fake))
	if err != nil {
		t.Fatalf("unexpected error building DataSource: %v", err)
	}

	if err := ds.Connect(); err != nil {
		t.Fatalf("unexpected error from Connect: %v", err)
	}
	if fake.connectCalls != 1 {
		t.Fatalf("expected connectCalls == 1, got %d", fake.connectCalls)
	}
	if ds.dialect != wantDialect {
		t.Fatalf("expected ds.dialect to be the fake's returned Dialect")
	}
	if !ds.connected {
		t.Fatal("expected ds.connected to be true after successful Connect")
	}
}

func TestDataSource_Connect_Idempotent(t *testing.T) {
	fake := &dsFakeConnector{dialect: fakeDialect{}}
	ds, err := NewDataSource(WithConnector(fake))
	if err != nil {
		t.Fatalf("unexpected error building DataSource: %v", err)
	}

	if err := ds.Connect(); err != nil {
		t.Fatalf("unexpected error from first Connect: %v", err)
	}
	if err := ds.Connect(); err != nil {
		t.Fatalf("unexpected error from second Connect: %v", err)
	}
	if fake.connectCalls != 1 {
		t.Fatalf("expected connectCalls to stay 1 after second Connect, got %d", fake.connectCalls)
	}
}

func TestDataSource_Connect_ErrorThenRetrySucceeds(t *testing.T) {
	wantErr := errors.New("boom")
	fake := &dsFakeConnector{connectErr: wantErr}
	ds, err := NewDataSource(WithConnector(fake))
	if err != nil {
		t.Fatalf("unexpected error building DataSource: %v", err)
	}

	if err := ds.Connect(); !errors.Is(err, wantErr) {
		t.Fatalf("expected Connect to return %v, got %v", wantErr, err)
	}
	if ds.connected {
		t.Fatal("expected ds.connected to stay false after failed Connect")
	}

	// Fix the fake and retry.
	fake.connectErr = nil
	fake.dialect = fakeDialect{}
	if err := ds.Connect(); err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if !ds.connected {
		t.Fatal("expected ds.connected to be true after successful retry")
	}
	if fake.connectCalls != 2 {
		t.Fatalf("expected connectCalls == 2 after retry, got %d", fake.connectCalls)
	}
}

func TestDataSource_Close_WithoutConnect(t *testing.T) {
	fake := &dsFakeConnector{dialect: fakeDialect{}}
	ds, err := NewDataSource(WithConnector(fake))
	if err != nil {
		t.Fatalf("unexpected error building DataSource: %v", err)
	}

	if err := ds.Close(); err != nil {
		t.Fatalf("expected nil error from Close without Connect, got %v", err)
	}
	if fake.closeCalls != 0 {
		t.Fatalf("expected closeCalls == 0, got %d", fake.closeCalls)
	}
}

func TestDataSource_Close_AfterConnect(t *testing.T) {
	fake := &dsFakeConnector{dialect: fakeDialect{}}
	ds, err := NewDataSource(WithConnector(fake))
	if err != nil {
		t.Fatalf("unexpected error building DataSource: %v", err)
	}
	if err := ds.Connect(); err != nil {
		t.Fatalf("unexpected error from Connect: %v", err)
	}

	if err := ds.Close(); err != nil {
		t.Fatalf("expected nil error from Close, got %v", err)
	}
	if fake.closeCalls != 1 {
		t.Fatalf("expected closeCalls == 1, got %d", fake.closeCalls)
	}
}

func TestDataSource_Close_Idempotent(t *testing.T) {
	fake := &dsFakeConnector{dialect: fakeDialect{}}
	ds, err := NewDataSource(WithConnector(fake))
	if err != nil {
		t.Fatalf("unexpected error building DataSource: %v", err)
	}
	if err := ds.Connect(); err != nil {
		t.Fatalf("unexpected error from Connect: %v", err)
	}

	if err := ds.Close(); err != nil {
		t.Fatalf("unexpected error from first Close: %v", err)
	}
	if err := ds.Close(); err != nil {
		t.Fatalf("unexpected error from second Close: %v", err)
	}
	if fake.closeCalls != 1 {
		t.Fatalf("expected closeCalls to stay 1 after second Close, got %d", fake.closeCalls)
	}
}

func TestDataSource_SatisfiesConn(t *testing.T) {
	var _ Conn = (*DataSource)(nil)
}

