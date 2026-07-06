package sqlite

// No //go:build integration tag: these tests exercise the real connector
// against a real (in-memory) SQLite database, but per TESTING.md's own rule
// ("anything that opens a real network connection is integration; anything
// else is unit") that isn't a network connection — same reasoning as
// conformance_test.go. Named "_lifecycle_" (not "_integration_") since it
// no longer carries that build tag.

import (
	"sync"
	"testing"

	"github.com/leandroluk/golem"
)

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
	ds, err := golem.NewDataSource(New(func(o *Options) {
		o.Path = ":memory:"
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

func TestConnector_Connect_UnwritablePath(t *testing.T) {
	ds, err := golem.NewDataSource(New(func(o *Options) {
		o.Path = "/nonexistent-directory-xyz/golem-test.db"
	}), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource returned error: %v", err)
	}
	defer ds.Close()

	err = ds.Connect()
	if err == nil {
		t.Fatal("expected Connect to return an error for an unwritable path, got nil")
	}
	if err.Error() == "" {
		t.Fatal("expected a descriptive error, got empty error string")
	}
	t.Logf("unwritable path error: %v", err)
}

func TestConnector_LoggingEnabled_SpyReceivesEntries(t *testing.T) {
	spy := &spyLogger{}

	ds, err := golem.NewDataSource(New(func(o *Options) {
		o.Path = ":memory:"
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
	spy := &spyLogger{}

	ds, err := golem.NewDataSource(New(func(o *Options) {
		o.Path = ":memory:"
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
