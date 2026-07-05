// Package mysql is golem's MySQL/MariaDB adapter — it implements
// golem.Dialect (value bind/scan, statement compilation/execution) on top
// of github.com/go-sql-driver/mysql via database/sql, and golem.Connector
// (via mysql.New) so a golem.DataSource can connect to a real MySQL 8+ (or
// best-effort MariaDB) instance.
//
// Nothing in the rest of golem imports this package — a *golem.DataSource
// only ever holds the golem.Dialect/golem.Connector interfaces, so this
// adapter is the only place a MySQL-specific dependency is allowed to live.
package mysql

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

// Options configures a MySQL/MariaDB connection. Either DSN or the discrete
// fields (Host, Port, User, Password, Database, TLSConfig) may be provided.
// If both are provided, non-zero discrete fields override only the
// corresponding parts of the DSN; the rest of the DSN is preserved.
type Options struct {
	DSN       string
	Host      string
	Port      int
	User      string
	Password  string
	Database  string
	TLSConfig string // mysql.Config.TLSConfig: "", "true", "false", "skip-verify", "preferred", or a name registered via mysql.RegisterTLSConfig
	Logging   bool
	Logger    golem.Logger
}

// New builds a golem.Option that wires a MySQL/MariaDB Connector configured
// by the given function. This is the ONLY supported way end users configure
// a MySQL DataSource — see README.md examples.
func New(configure func(*Options)) golem.Option {
	opts := &Options{}
	configure(opts)
	return golem.WithConnector(&connector{opts: opts})
}
