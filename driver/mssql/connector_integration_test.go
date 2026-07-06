//go:build integration

package mssql

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/leandroluk/golem"
)

const defaultTestDSN = "sqlserver://sa:Golem_Test_2026!@localhost:51433?database=master"

// testDSN returns the DSN to use for integration tests, honoring
// GOLEM_MSSQL_TEST_DSN when set (matching the Taskfile.yml's
// test-integration target), falling back to the same default used there
// otherwise.
func testDSN() string {
	if dsn := os.Getenv("GOLEM_MSSQL_TEST_DSN"); dsn != "" {
		return dsn
	}
	return defaultTestDSN
}

// unreachableHostDSN swaps the port for one nothing listens on, so Connect
// fails fast (connection refused) instead of succeeding.
func unreachableHostDSN(base string) string {
	return strings.Replace(base, ":51433", ":59997", 1)
}

// badCredentialsDSN swaps in a wrong password, keeping host/port/db intact.
func badCredentialsDSN(base string) string {
	return strings.Replace(base, "sa:Golem_Test_2026!@", "sa:WrongPassword123!@", 1)
}

// spyLogger is a test Logger that records every entry it receives.
type spyLogger struct {
	mu      sync.Mutex
	entries []string
}

var _ golem.Logger = (*spyLogger)(nil)

func (s *spyLogger) record(level, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, level+": "+msg)
}

func (s *spyLogger) Debug(msg string, args map[string]any) { s.record("debug", msg) }
func (s *spyLogger) Info(msg string, args map[string]any)  { s.record("info", msg) }
func (s *spyLogger) Warn(msg string, args map[string]any)  { s.record("warn", msg) }
func (s *spyLogger) Error(msg string, args map[string]any) { s.record("error", msg) }

func (s *spyLogger) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func TestConnector_ConnectAndClose_Success(t *testing.T) {
	dsn := testDSN()

	ds, err := golem.NewDataSource(New(func(o *Options) {
		o.DSN = dsn
	}), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}

	if err := ds.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}

	if err := ds.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

func TestConnector_Connect_UnreachableHost(t *testing.T) {
	dsn := unreachableHostDSN(testDSN())

	ds, err := golem.NewDataSource(New(func(o *Options) {
		o.DSN = dsn
	}), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	defer ds.Close()

	err = ds.Connect()
	if err == nil {
		t.Fatal("expected Connect to return an error for an unreachable host, got nil")
	}
	if err.Error() == "" {
		t.Fatal("expected a descriptive error, got empty error string")
	}
	t.Logf("unreachable host error: %v", err)
}

func TestConnector_Connect_BadCredentials(t *testing.T) {
	dsn := badCredentialsDSN(testDSN())

	ds, err := golem.NewDataSource(New(func(o *Options) {
		o.DSN = dsn
	}), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	defer ds.Close()

	err = ds.Connect()
	if err == nil {
		t.Fatal("expected Connect to return an error for bad credentials, got nil")
	}
	if err.Error() == "" {
		t.Fatal("expected a descriptive error, got empty error string")
	}
	t.Logf("bad credentials error: %v", err)
}

func TestConnector_LoggingEnabled_SpyReceivesEntries(t *testing.T) {
	dsn := testDSN()
	spy := &spyLogger{}

	ds, err := golem.NewDataSource(New(func(o *Options) {
		o.DSN = dsn
		o.Logging = true
		o.Logger = spy
	}), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}

	if err := ds.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	if err := ds.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if got := spy.count(); got < 1 {
		t.Fatalf("expected spy logger to receive at least 1 entry, got %d", got)
	}
}

func TestConnector_LoggingDisabled_SpyReceivesNoEntries(t *testing.T) {
	dsn := testDSN()
	spy := &spyLogger{}

	ds, err := golem.NewDataSource(New(func(o *Options) {
		o.DSN = dsn
		o.Logger = spy
		// o.Logging left at zero value (false)
	}), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}

	if err := ds.Connect(); err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	if err := ds.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if got := spy.count(); got != 0 {
		t.Fatalf("expected spy logger to receive 0 entries when Logging is disabled, got %d", got)
	}
}
