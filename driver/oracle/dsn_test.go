package oracle

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
	in := "oracle://alice:secret@dbhost:1521/FREEPDB1"
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
	if u.Port() != "1521" {
		t.Errorf("port = %q, want 1521", u.Port())
	}
	if strings.TrimPrefix(u.Path, "/") != "FREEPDB1" {
		t.Errorf("service name (path) = %q, want FREEPDB1", u.Path)
	}
}

func TestResolveDSN_FieldsOnly(t *testing.T) {
	o := &Options{
		Host:        "myhost",
		Port:        1522,
		User:        "bob",
		Password:    "pw123",
		ServiceName: "APPDB",
	}
	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.Scheme != "oracle" {
		t.Errorf("scheme = %q, want oracle", u.Scheme)
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
	if u.Port() != "1522" {
		t.Errorf("port = %q, want 1522", u.Port())
	}
	if strings.TrimPrefix(u.Path, "/") != "APPDB" {
		t.Errorf("service name (path) = %q, want APPDB", u.Path)
	}
}

func TestResolveDSN_BothSet_PartialOverride_ServiceName(t *testing.T) {
	in := "oracle://alice:secret@dbhost:1521/OLDPDB"
	o := &Options{DSN: in, ServiceName: "NEWPDB"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if strings.TrimPrefix(u.Path, "/") != "NEWPDB" {
		t.Errorf("service name = %q, want NEWPDB (field should override)", u.Path)
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
	if u.Port() != "1521" {
		t.Errorf("port = %q, want 1521 (preserved from DSN)", u.Port())
	}
}

func TestResolveDSN_BothSet_PartialOverride_HostOnly_PreservesPort(t *testing.T) {
	in := "oracle://alice:secret@dbhost:1521/FREEPDB1"
	o := &Options{DSN: in, Host: "newhost"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := parseForAssert(t, got)
	if u.Hostname() != "newhost" {
		t.Errorf("host = %q, want newhost (field should override)", u.Hostname())
	}
	if u.Port() != "1521" {
		t.Errorf("port = %q, want 1521 (preserved from DSN since Port==0)", u.Port())
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
	in := "oracle://dbhost:1521/FREEPDB1"
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
	in := "oracle://alice:secret@dbhost:1521/FREEPDB1"
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
	in := "oracle://dbhost:1521/FREEPDB1"
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
	in := "oracle://alice:secret@dbhost:1521/FREEPDB1"
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
	in := "oracle://alice:secret@dbhost/FREEPDB1"
	o := &Options{DSN: in, ServiceName: "NEWPDB"}

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

func TestBuildDSN_NoServiceName(t *testing.T) {
	got := buildDSN(&Options{Host: "h"})
	u := parseForAssert(t, got)
	if u.Path != "" {
		t.Errorf("path = %q, want empty", u.Path)
	}
}

func TestResolveDSN_MalformedDSN(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("resolveDSN panicked on malformed DSN: %v", r)
		}
	}()

	_, err := resolveDSN(&Options{DSN: "oracle://[::1"})
	if err == nil {
		t.Fatal("expected error for malformed DSN, got nil")
	}
	if err.Error() == "" {
		t.Fatal("expected a descriptive (non-empty) error message")
	}
}
