package mssql

import (
	"net/url"
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
	in := "sqlserver://alice:secret@dbhost:1433?database=mydb"
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
	if u.Port() != "1433" {
		t.Errorf("port = %q, want 1433", u.Port())
	}
	if u.Query().Get("database") != "mydb" {
		t.Errorf("database = %q, want mydb", u.Query().Get("database"))
	}
}

func TestResolveDSN_FieldsOnly(t *testing.T) {
	o := &Options{
		Host:     "myhost",
		Port:     1434,
		User:     "bob",
		Password: "pw123",
		Database: "appdb",
	}
	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.Scheme != "sqlserver" {
		t.Errorf("scheme = %q, want sqlserver", u.Scheme)
	}
	if u.User.Username() != "bob" {
		t.Errorf("user = %q, want bob", u.User.Username())
	}
	if pw, _ := u.User.Password(); pw != "pw123" {
		t.Errorf("password = %q, want pw123", pw)
	}
	if u.Hostname() != "myhost" {
		t.Errorf("host = %q, want myhost", u.Hostname())
	}
	if u.Port() != "1434" {
		t.Errorf("port = %q, want 1434", u.Port())
	}
	if u.Query().Get("database") != "appdb" {
		t.Errorf("database = %q, want appdb", u.Query().Get("database"))
	}
}

func TestResolveDSN_BothSet_PartialOverride_Database(t *testing.T) {
	in := "sqlserver://alice:secret@dbhost:1433?database=olddb"
	o := &Options{DSN: in, Database: "newdb"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.Query().Get("database") != "newdb" {
		t.Errorf("database = %q, want newdb (field should override)", u.Query().Get("database"))
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
	if u.Port() != "1433" {
		t.Errorf("port = %q, want 1433 (preserved from DSN)", u.Port())
	}
}

func TestResolveDSN_BothSet_PartialOverride_HostOnly_PreservesPort(t *testing.T) {
	in := "sqlserver://alice:secret@dbhost:1433?database=mydb"
	o := &Options{DSN: in, Host: "newhost"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.Hostname() != "newhost" {
		t.Errorf("host = %q, want newhost (field should override)", u.Hostname())
	}
	if u.Port() != "1433" {
		t.Errorf("port = %q, want 1433 (preserved from DSN since Port==0)", u.Port())
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

func TestResolveDSN_BothSet_OverrideUser_NoExistingUserInfo(t *testing.T) {
	in := "sqlserver://dbhost:1433?database=mydb"
	o := &Options{DSN: in, User: "carol"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.User.Username() != "carol" {
		t.Errorf("user = %q, want carol", u.User.Username())
	}
	if _, ok := u.User.Password(); ok {
		t.Error("expected no password set")
	}
}

func TestResolveDSN_BothSet_OverridePassword_PreservesUser(t *testing.T) {
	in := "sqlserver://alice:secret@dbhost:1433?database=mydb"
	o := &Options{DSN: in, Password: "newpass"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.User.Username() != "alice" {
		t.Errorf("user = %q, want alice (preserved)", u.User.Username())
	}
	if pw, _ := u.User.Password(); pw != "newpass" {
		t.Errorf("password = %q, want newpass (overridden)", pw)
	}
}

func TestResolveDSN_BothSet_NoUserOrPassword_ClearsUserInfo(t *testing.T) {
	in := "sqlserver://dbhost:1433?database=mydb"
	o := &Options{DSN: in, Host: "newhost"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.User != nil {
		t.Errorf("User = %v, want nil", u.User)
	}
	if u.Hostname() != "newhost" {
		t.Errorf("host = %q, want newhost", u.Hostname())
	}
}

func TestResolveDSN_BothSet_OverridePort(t *testing.T) {
	in := "sqlserver://alice:secret@dbhost:1433?database=mydb"
	o := &Options{DSN: in, Port: 9999}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.Port() != "9999" {
		t.Errorf("port = %q, want 9999", u.Port())
	}
}

func TestResolveDSN_BothSet_NoPort_NoOverride(t *testing.T) {
	in := "sqlserver://alice:secret@dbhost?database=mydb"
	o := &Options{DSN: in, Database: "newdb"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.Port() != "" {
		t.Errorf("port = %q, want empty", u.Port())
	}
	if u.Hostname() != "dbhost" {
		t.Errorf("host = %q, want dbhost", u.Hostname())
	}
}

func TestBuildDSN_UserOnly_NoPassword(t *testing.T) {
	got := buildDSN(&Options{User: "dave", Host: "h"})
	u := parseForAssert(t, got)
	if u.User.Username() != "dave" {
		t.Errorf("user = %q, want dave", u.User.Username())
	}
	if _, ok := u.User.Password(); ok {
		t.Error("expected no password set")
	}
}

func TestBuildDSN_NoUser(t *testing.T) {
	got := buildDSN(&Options{Host: "h"})
	u := parseForAssert(t, got)
	if u.User != nil {
		t.Errorf("User = %v, want nil", u.User)
	}
}

func TestBuildDSN_NoPort(t *testing.T) {
	got := buildDSN(&Options{Host: "h"})
	u := parseForAssert(t, got)
	if u.Port() != "" {
		t.Errorf("port = %q, want empty", u.Port())
	}
	if u.Hostname() != "h" {
		t.Errorf("host = %q, want h", u.Hostname())
	}
}

func TestBuildDSN_NoDatabase(t *testing.T) {
	got := buildDSN(&Options{Host: "h"})
	u := parseForAssert(t, got)
	if u.Query().Get("database") != "" {
		t.Errorf("database = %q, want empty", u.Query().Get("database"))
	}
}

func TestResolveDSN_MalformedDSN(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("resolveDSN panicked on malformed DSN: %v", r)
		}
	}()

	_, err := resolveDSN(&Options{DSN: "sqlserver://[::1"})
	if err == nil {
		t.Fatal("expected error for malformed DSN, got nil")
	}
	if err.Error() == "" {
		t.Fatal("expected a descriptive (non-empty) error message")
	}
}
