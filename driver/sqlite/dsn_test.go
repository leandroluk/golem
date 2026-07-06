package sqlite

import "testing"

func TestResolveDSN_DefaultsToMemory(t *testing.T) {
	dsn := resolveDSN(&Options{})
	want := "file::memory:?cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_time_format=sqlite"
	if dsn != want {
		t.Fatalf("expected %q, got %q", want, dsn)
	}
}

func TestResolveDSN_ExplicitMemory(t *testing.T) {
	dsn := resolveDSN(&Options{Path: ":memory:"})
	want := "file::memory:?cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_time_format=sqlite"
	if dsn != want {
		t.Fatalf("expected %q, got %q", want, dsn)
	}
}

func TestResolveDSN_FilePath_NoSharedCache(t *testing.T) {
	dsn := resolveDSN(&Options{Path: "/tmp/golem-test.db"})
	want := "file:/tmp/golem-test.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_time_format=sqlite"
	if dsn != want {
		t.Fatalf("expected %q, got %q", want, dsn)
	}
}
