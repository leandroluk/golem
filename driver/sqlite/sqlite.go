// Package sqlite is golem's SQLite adapter — it implements golem.Dialect
// (value bind/scan, statement compilation/execution) on top of
// modernc.org/sqlite (a pure-Go, cgo-free port of SQLite) via database/sql,
// and golem.Connector (via sqlite.New) so a golem.DataSource can connect to
// an embedded/in-memory SQLite database. No Docker service or network
// connection is ever required to use or test this adapter.
//
// Nothing in the rest of golem imports this package — a *golem.DataSource
// only ever holds the golem.Dialect/golem.Connector interfaces, so this
// adapter is the only place a SQLite-specific dependency is allowed to live.
package sqlite

import (
	"context"
	"database/sql"

	"github.com/leandroluk/golem"
)

// sqlIface abstracts *sql.DB's surface so DATA-DOG/go-sqlmock can substitute
// for it in unit tests. *sql.DB already implements this directly.
type sqlIface interface {
	PingContext(ctx context.Context) error
	Close() error
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// Options configures a SQLite connection. Path is a file path, or ":memory:"
// (the default when empty) for an ephemeral, in-process database.
type Options struct {
	Path    string
	Logging bool
	Logger  golem.Logger
}

// New builds a golem.Option that wires a SQLite Connector configured by the
// given function. This is the ONLY supported way end users configure a
// SQLite DataSource — see README.md examples.
func New(configure func(*Options)) golem.Option {
	opts := &Options{}
	configure(opts)
	return golem.WithConnector(&connector{opts: opts})
}
