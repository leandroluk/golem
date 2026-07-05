package mysql

import (
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestResolveDSN_DSNOnly(t *testing.T) {
	in := "alice:secret@tcp(dbhost:3306)/mydb"
	got, err := resolveDSN(&Options{DSN: in})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("resolveDSN produced an unparsable DSN %q: %v", got, err)
	}
	if cfg.User != "alice" {
		t.Errorf("user = %q, want alice", cfg.User)
	}
	if cfg.Passwd != "secret" {
		t.Errorf("password = %q, want secret", cfg.Passwd)
	}
	if cfg.Addr != "dbhost:3306" {
		t.Errorf("addr = %q, want dbhost:3306", cfg.Addr)
	}
	if cfg.DBName != "mydb" {
		t.Errorf("db = %q, want mydb", cfg.DBName)
	}
	if !cfg.ParseTime {
		t.Error("expected ParseTime to be forced true")
	}
}

func TestResolveDSN_FieldsOnly(t *testing.T) {
	o := &Options{
		Host:     "myhost",
		Port:     3307,
		User:     "bob",
		Password: "pw123",
		Database: "appdb",
	}
	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("resolveDSN produced an unparsable DSN %q: %v", got, err)
	}
	if cfg.User != "bob" {
		t.Errorf("user = %q, want bob", cfg.User)
	}
	if cfg.Passwd != "pw123" {
		t.Errorf("password = %q, want pw123", cfg.Passwd)
	}
	if cfg.Addr != "myhost:3307" {
		t.Errorf("addr = %q, want myhost:3307", cfg.Addr)
	}
	if cfg.DBName != "appdb" {
		t.Errorf("db = %q, want appdb", cfg.DBName)
	}
	if cfg.Net != "tcp" {
		t.Errorf("net = %q, want tcp", cfg.Net)
	}
}

func TestResolveDSN_BothSet_PartialOverride_Database(t *testing.T) {
	in := "alice:secret@tcp(dbhost:3306)/olddb"
	o := &Options{DSN: in, Database: "newdb"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("resolveDSN produced an unparsable DSN %q: %v", got, err)
	}
	if cfg.DBName != "newdb" {
		t.Errorf("db = %q, want newdb (field should override)", cfg.DBName)
	}
	if cfg.User != "alice" {
		t.Errorf("user = %q, want alice (preserved from DSN)", cfg.User)
	}
	if cfg.Passwd != "secret" {
		t.Errorf("password = %q, want secret (preserved from DSN)", cfg.Passwd)
	}
	if cfg.Addr != "dbhost:3306" {
		t.Errorf("addr = %q, want dbhost:3306 (preserved from DSN)", cfg.Addr)
	}
}

func TestResolveDSN_BothSet_PartialOverride_HostOnly_PreservesPort(t *testing.T) {
	in := "alice:secret@tcp(dbhost:3306)/mydb"
	o := &Options{DSN: in, Host: "newhost"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("resolveDSN produced an unparsable DSN %q: %v", got, err)
	}
	if cfg.Addr != "newhost:3306" {
		t.Errorf("addr = %q, want newhost:3306 (host overridden, port preserved)", cfg.Addr)
	}
}

func TestResolveDSN_BothSet_PartialOverride_PortOnly_PreservesHost(t *testing.T) {
	in := "alice:secret@tcp(dbhost:3306)/mydb"
	o := &Options{DSN: in, Port: 9999}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("resolveDSN produced an unparsable DSN %q: %v", got, err)
	}
	if cfg.Addr != "dbhost:9999" {
		t.Errorf("addr = %q, want dbhost:9999 (port overridden, host preserved)", cfg.Addr)
	}
}

func TestResolveDSN_BothSet_TLSConfig(t *testing.T) {
	in := "alice:secret@tcp(dbhost:3306)/mydb"
	o := &Options{DSN: in, TLSConfig: "skip-verify"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("resolveDSN produced an unparsable DSN %q: %v", got, err)
	}
	if cfg.TLSConfig != "skip-verify" {
		t.Errorf("tls config = %q, want skip-verify", cfg.TLSConfig)
	}
}

func TestResolveDSN_NoDiscreteHostOrPort_NoAddrOverride(t *testing.T) {
	in := "alice:secret@tcp(dbhost:3306)/mydb"
	o := &Options{DSN: in, Database: "newdb"}

	got, err := resolveDSN(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "dbhost:3306") {
		t.Errorf("expected addr to remain untouched, got %q", got)
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

	_, err := resolveDSN(&Options{DSN: "this is not a valid dsn @@@ ((("})
	if err == nil {
		t.Fatal("expected error for malformed DSN, got nil")
	}
	if err.Error() == "" {
		t.Fatal("expected a descriptive (non-empty) error message")
	}
}

func TestResolveDSN_FieldsOnly_NoHostOrPort_UsesDefaults(t *testing.T) {
	got, err := resolveDSN(&Options{User: "bob"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("resolveDSN produced an unparsable DSN %q: %v", got, err)
	}
	if cfg.User != "bob" {
		t.Errorf("user = %q, want bob", cfg.User)
	}
}

func TestResolveDSN_HostOnly_NoDSN_DefaultsPort(t *testing.T) {
	got, err := resolveDSN(&Options{Host: "myhost"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("resolveDSN produced an unparsable DSN %q: %v", got, err)
	}
	if cfg.Addr != "myhost:3306" {
		t.Errorf("addr = %q, want myhost:3306 (default port)", cfg.Addr)
	}
}

func TestResolveDSN_PortOnly_NoDSN_DefaultsHost(t *testing.T) {
	got, err := resolveDSN(&Options{Port: 3307})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("resolveDSN produced an unparsable DSN %q: %v", got, err)
	}
	if cfg.Addr != "127.0.0.1:3307" {
		t.Errorf("addr = %q, want 127.0.0.1:3307 (default host)", cfg.Addr)
	}
}

func TestSplitHostPort(t *testing.T) {
	cases := []struct {
		addr     string
		wantHost string
		wantPort string
	}{
		{"", "", ""},
		{"host:1234", "host", "1234"},
		{"malformed", "malformed", ""},
	}
	for _, tc := range cases {
		host, port := splitHostPort(tc.addr)
		if host != tc.wantHost || port != tc.wantPort {
			t.Errorf("splitHostPort(%q) = (%q, %q), want (%q, %q)", tc.addr, host, port, tc.wantHost, tc.wantPort)
		}
	}
}
