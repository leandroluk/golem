package postgres

import "github.com/leandroluk/golem"

// Options configures a Postgres connection. Either DSN or the discrete
// fields (Host, Port, User, Password, Database, SSLMode) may be provided.
// If both are provided, non-zero discrete fields override only the
// corresponding parts of the DSN; the rest of the DSN is preserved.
type Options struct {
	DSN      string
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	Logging  bool
	Logger   golem.Logger
}

// New builds a golem.Option that wires a Postgres Connector configured by
// the given function. This is the ONLY supported way end users configure a
// Postgres DataSource — see README.md examples.
func New(configure func(*Options)) golem.Option {
	opts := &Options{}
	configure(opts)
	return golem.WithConnector(&connector{opts: opts})
}
