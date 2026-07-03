package postgres

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// resolveDSN combines o.DSN and the discrete fields into one final Postgres
// connection string.
//
// Precedence rules:
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
		o.Password != "" || o.Database != "" || o.SSLMode != ""

	if o.DSN == "" && !hasDiscrete {
		return "", fmt.Errorf("postgres: no DSN or connection fields provided")
	}

	if o.DSN == "" {
		return buildDSN(o), nil
	}

	u, err := url.Parse(o.DSN)
	if err != nil {
		return "", fmt.Errorf("postgres: malformed DSN: %w", err)
	}

	if !hasDiscrete {
		return u.String(), nil
	}

	// Determine the existing user/password from the DSN so we can override
	// only the pieces the caller actually set.
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
		u.Path = "/" + strings.TrimPrefix(o.Database, "/")
	}

	if o.SSLMode != "" {
		q := u.Query()
		q.Set("sslmode", o.SSLMode)
		u.RawQuery = q.Encode()
	}

	return u.String(), nil
}

// buildDSN constructs a DSN string purely from the discrete fields on o.
// o.DSN is assumed to be empty when this is called.
func buildDSN(o *Options) string {
	u := &url.URL{Scheme: "postgres"}

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
		u.Path = "/" + strings.TrimPrefix(o.Database, "/")
	}

	if o.SSLMode != "" {
		q := u.Query()
		q.Set("sslmode", o.SSLMode)
		u.RawQuery = q.Encode()
	}

	return u.String()
}
