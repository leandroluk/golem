package postgres

import (
	"net/url"
	"strings"
	"testing"
)

// parseForAssert parses a DSN string produced by resolveDSN so tests can
// assert on individual components rather than doing brittle whole-string
// comparisons.
func parseForAssert(t *testing.T, dsn string) *url.URL {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("resolveDSN produced an unparsable DSN %q: %v", dsn, err)
	}
	return u
}

func TestResolveDSN_DSNOnly(t *testing.T) {
	in := "postgres://alice:secret@dbhost:5432/mydb?sslmode=disable"
	got, err := resolveDSN(&Options{DSN: in})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.User.Username() != "alice" {
		t.Errorf("user = %q, want alice", u.User.Username())
	}
	if pw, _ := u.User.Password(); pw != "secret" {
		t.Errorf("password = %q, want secret", pw)
	}
	if u.Hostname() != "dbhost" {
		t.Errorf("host = %q, want dbhost", u.Hostname())
	}
	if u.Port() != "5432" {
		t.Errorf("port = %q, want 5432", u.Port())
	}
	if strings.TrimPrefix(u.Path, "/") != "mydb" {
		t.Errorf("db = %q, want mydb", u.Path)
	}
	if u.Query().Get("sslmode") != "disable" {
		t.Errorf("sslmode = %q, want disable", u.Query().Get("sslmode"))
	}
}

func TestResolveDSN_FieldsOnly(t *testing.T) {
	o := &Options{
		Host:     "myhost",
		Port:     5433,
		User:     "bob",
		Password: "pw123",
		Database: "appdb",
		SSLMode:  "require",
	}
	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.User.Username() != "bob" {
		t.Errorf("user = %q, want bob", u.User.Username())
	}
	if pw, _ := u.User.Password(); pw != "pw123" {
		t.Errorf("password = %q, want pw123", pw)
	}
	if u.Hostname() != "myhost" {
		t.Errorf("host = %q, want myhost", u.Hostname())
	}
	if u.Port() != "5433" {
		t.Errorf("port = %q, want 5433", u.Port())
	}
	if strings.TrimPrefix(u.Path, "/") != "appdb" {
		t.Errorf("db = %q, want appdb", u.Path)
	}
	if u.Query().Get("sslmode") != "require" {
		t.Errorf("sslmode = %q, want require", u.Query().Get("sslmode"))
	}
}

func TestResolveDSN_BothSet_PartialOverride_Database(t *testing.T) {
	// DSN says "olddb", but the Database field says "newdb" -> field wins.
	// Everything else (user, password, host, port, sslmode) must come from
	// the DSN untouched.
	in := "postgres://alice:secret@dbhost:5432/olddb?sslmode=disable"
	o := &Options{DSN: in, Database: "newdb"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if strings.TrimPrefix(u.Path, "/") != "newdb" {
		t.Errorf("db = %q, want newdb (field should override)", u.Path)
	}
	if u.User.Username() != "alice" {
		t.Errorf("user = %q, want alice (preserved from DSN)", u.User.Username())
	}
	if pw, _ := u.User.Password(); pw != "secret" {
		t.Errorf("password = %q, want secret (preserved from DSN)", pw)
	}
	if u.Hostname() != "dbhost" {
		t.Errorf("host = %q, want dbhost (preserved from DSN)", u.Hostname())
	}
	if u.Port() != "5432" {
		t.Errorf("port = %q, want 5432 (preserved from DSN)", u.Port())
	}
	if u.Query().Get("sslmode") != "disable" {
		t.Errorf("sslmode = %q, want disable (preserved from DSN)", u.Query().Get("sslmode"))
	}
}

func TestResolveDSN_BothSet_PartialOverride_HostOnly_PreservesPort(t *testing.T) {
	// Only Host is set alongside the DSN; Port == 0 must NOT override the
	// port that's already present in the DSN.
	in := "postgres://alice:secret@dbhost:5432/mydb?sslmode=disable"
	o := &Options{DSN: in, Host: "newhost"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.Hostname() != "newhost" {
		t.Errorf("host = %q, want newhost (field should override)", u.Hostname())
	}
	if u.Port() != "5432" {
		t.Errorf("port = %q, want 5432 (preserved from DSN since Port==0)", u.Port())
	}
}

func TestResolveDSN_BothSet_PartialOverride_SSLMode(t *testing.T) {
	in := "postgres://alice:secret@dbhost:5432/mydb?sslmode=disable"
	o := &Options{DSN: in, SSLMode: "verify-full"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.Query().Get("sslmode") != "verify-full" {
		t.Errorf("sslmode = %q, want verify-full", u.Query().Get("sslmode"))
	}
	if strings.TrimPrefix(u.Path, "/") != "mydb" {
		t.Errorf("db = %q, want mydb (preserved from DSN)", u.Path)
	}
}

func TestResolveDSN_NeitherSet(t *testing.T) {
	_, err := resolveDSN(&Options{})
	if err == nil {
		t.Fatal("expected error when neither DSN nor discrete fields are set, got nil")
	}
	if err.Error() == "" {
		t.Fatal("expected a descriptive (non-empty) error message")
	}
}

func TestResolveDSN_MalformedDSN(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("resolveDSN panicked on malformed DSN: %v", r)
		}
	}()

	_, err := resolveDSN(&Options{DSN: "postgres://[::1"})
	if err == nil {
		t.Fatal("expected error for malformed DSN, got nil")
	}
	if err.Error() == "" {
		t.Fatal("expected a descriptive (non-empty) error message")
	}
}

