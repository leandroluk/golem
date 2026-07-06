package mssql

import (
	"fmt"
	"net/url"
	"strconv"
)

// resolveDSN combines o.DSN and the discrete fields into one final SQL
// Server connection string. Confirmed via a real SQL Server probe (see
// design.md): the "sqlserver" driver's DSN format is
// "sqlserver://user:pass@host:port?database=dbname" — net/url-parseable,
// unlike MySQL's bracket syntax, with the database name in the query
// string (not the URL path, unlike Postgres).
//
// Precedence rules mirror driver/postgres's resolveDSN:
//   - If only DSN is set, it is used as-is (re-serialized after parsing).
//   - If only discrete fields are set, a DSN is built from them.
//   - If both are set, each non-zero discrete field overrides only its
//     corresponding part of the DSN (partial override); all other parts of
//     the DSN are preserved untouched. Port == 0 is treated as "not set" so
//     it never clobbers a port already present in the DSN.
//   - If neither is set, a descriptive config error is returned.
//   - A malformed DSN (fails net/url.Parse) returns a descriptive error and
//     never panics.
func resolveDSN(o *Options) (string, error) {
	hasDiscrete := o.Host != "" || o.Port != 0 || o.User != "" ||
		o.Password != "" || o.Database != ""

	if o.DSN == "" && !hasDiscrete {
		return "", fmt.Errorf("mssql: no DSN or connection fields provided")
	}

	if o.DSN == "" {
		return buildDSN(o), nil
	}

	u, err := url.Parse(o.DSN)
	if err != nil {
		return "", fmt.Errorf("mssql: malformed DSN: %w", err)
	}

	if !hasDiscrete {
		return u.String(), nil
	}

	existingUser := ""
	existingPassword := ""
	existingPasswordSet := false
	if u.User != nil {
		existingUser = u.User.Username()
		existingPassword, existingPasswordSet = u.User.Password()
	}

	finalUser := existingUser
	if o.User != "" {
		finalUser = o.User
	}
	finalPassword := existingPassword
	finalPasswordSet := existingPasswordSet
	if o.Password != "" {
		finalPassword = o.Password
		finalPasswordSet = true
	}

	if finalUser != "" || finalPasswordSet {
		if finalPasswordSet {
			u.User = url.UserPassword(finalUser, finalPassword)
		} else {
			u.User = url.User(finalUser)
		}
	} else {
		u.User = nil
	}

	host := u.Hostname()
	if o.Host != "" {
		host = o.Host
	}
	port := u.Port()
	if o.Port != 0 {
		port = strconv.Itoa(o.Port)
	}
	if port != "" {
		u.Host = host + ":" + port
	} else {
		u.Host = host
	}

	if o.Database != "" {
		q := u.Query()
		q.Set("database", o.Database)
		u.RawQuery = q.Encode()
	}

	return u.String(), nil
}

// buildDSN constructs a DSN string purely from the discrete fields on o.
// o.DSN is assumed to be empty when this is called.
func buildDSN(o *Options) string {
	u := &url.URL{Scheme: "sqlserver"}

	if o.User != "" {
		if o.Password != "" {
			u.User = url.UserPassword(o.User, o.Password)
		} else {
			u.User = url.User(o.User)
		}
	}

	host := o.Host
	if o.Port != 0 {
		host = host + ":" + strconv.Itoa(o.Port)
	}
	u.Host = host

	if o.Database != "" {
		q := u.Query()
		q.Set("database", o.Database)
		u.RawQuery = q.Encode()
	}

	return u.String()
}
