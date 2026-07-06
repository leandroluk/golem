// Package mssql is golem's SQL Server adapter — it implements golem.Dialect
// (value bind/scan, statement compilation/execution) on top of
// github.com/microsoft/go-mssqldb via database/sql, and golem.Connector (via
// mssql.New) so a golem.DataSource can connect to a real SQL Server 2019+
// instance.
//
// Nothing in the rest of golem imports this package — a *golem.DataSource
// only ever holds the golem.Dialect/golem.Connector interfaces, so this
// adapter is the only place a SQL Server-specific dependency is allowed to
// live.
package mssql

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

// Options configures a SQL Server connection. Either DSN or the discrete
// fields (Host, Port, User, Password, Database) may be provided. If both
// are provided, non-zero discrete fields override only the corresponding
// parts of the DSN; the rest of the DSN is preserved.
type Options struct {
	DSN      string
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Logging  bool
	Logger   golem.Logger
}

// New builds a golem.Option that wires a SQL Server Connector configured by
// the given function. This is the ONLY supported way end users configure a
// SQL Server DataSource — see README.md examples.
func New(configure func(*Options)) golem.Option {
	opts := &Options{}
	configure(opts)
	return golem.WithConnector(&connector{opts: opts})
}
