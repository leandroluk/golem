package mysql

import (
	"fmt"
	"net"
	"strconv"

	"github.com/go-sql-driver/mysql"
)

// resolveDSN combines o.DSN and the discrete fields into one final MySQL
// connection string, using go-sql-driver/mysql's own DSN parser/builder
// (mysql.Config) rather than net/url — MySQL DSNs
// ("user:pass@tcp(host:port)/db?param=value") aren't net/url-parseable (no
// scheme, bracket syntax).
//
// Precedence rules mirror driver/postgres's resolveDSN:
//   - If only DSN is set, it is used as-is (re-serialized after parsing).
//   - If only discrete fields are set, a DSN is built from them.
//   - If both are set, each non-zero discrete field overrides only its
//     corresponding part of the DSN (partial override); all other parts of
//     the DSN are preserved untouched. Port == 0 is treated as "not set" so
//     it never clobbers a port already present in the DSN.
//   - If neither is set, a descriptive config error is returned.
//   - A malformed DSN (fails mysql.ParseDSN) returns a descriptive error and
//     never panics.
//
// ParseTime is always forced true: golem's Dialect contract expects
// DATE/DATETIME/TIME columns to Scan as time.Time, which go-sql-driver only
// does when ParseTime is enabled — this is an internal implementation
// requirement, not a user-facing option.
func resolveDSN(o *Options) (string, error) {
	hasDiscrete := o.Host != "" || o.Port != 0 || o.User != "" ||
		o.Password != "" || o.Database != "" || o.TLSConfig != ""

	if o.DSN == "" && !hasDiscrete {
		return "", fmt.Errorf("mysql: no DSN or connection fields provided")
	}

	var cfg *mysql.Config
	if o.DSN == "" {
		cfg = mysql.NewConfig()
	} else {
		parsed, err := mysql.ParseDSN(o.DSN)
		if err != nil {
			return "", fmt.Errorf("mysql: malformed DSN: %w", err)
		}
		cfg = parsed
	}

	if o.User != "" {
		cfg.User = o.User
	}
	if o.Password != "" {
		cfg.Passwd = o.Password
	}

	if o.Host != "" || o.Port != 0 {
		host, port := splitHostPort(cfg.Addr)
		if o.Host != "" {
			host = o.Host
		}
		if o.Port != 0 {
			port = strconv.Itoa(o.Port)
		}
		if host == "" {
			host = "127.0.0.1"
		}
		if port == "" {
			port = "3306"
		}
		cfg.Net = "tcp"
		cfg.Addr = net.JoinHostPort(host, port)
	}

	if o.Database != "" {
		cfg.DBName = o.Database
	}

	if o.TLSConfig != "" {
		cfg.TLSConfig = o.TLSConfig
	}

	cfg.ParseTime = true

	return cfg.FormatDSN(), nil
}

// splitHostPort splits an "host:port" address (as stored in mysql.Config.Addr)
// into its parts. Returns ("", "") for an empty addr; a malformed one
// (shouldn't happen for anything mysql.Config itself produced) degrades to
// treating the whole string as the host with no port.
func splitHostPort(addr string) (host, port string) {
	if addr == "" {
		return "", ""
	}
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, ""
	}
	return h, p
}
