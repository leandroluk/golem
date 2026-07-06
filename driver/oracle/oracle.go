// Package oracle is golem's Oracle Database adapter — it implements
// golem.Dialect (value bind/scan, statement compilation/execution) on top
// of github.com/sijms/go-ora/v2 via database/sql, and golem.Connector (via
// oracle.New) so a golem.DataSource can connect to a real Oracle 12c+
// instance.
//
// Nothing in the rest of golem imports this package — a *golem.DataSource
// only ever holds the golem.Dialect/golem.Connector interfaces, so this
// adapter is the only place an Oracle-specific dependency is allowed to
// live.
package oracle

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

// Options configures an Oracle connection. Either DSN or the discrete
// fields (Host, Port, User, Password, ServiceName) may be provided. If both
// are provided, non-zero discrete fields override only the corresponding
// parts of the DSN; the rest of the DSN is preserved. ServiceName plays the
// role Database plays on every other adapter — Oracle's connection unit is
// a service name / pluggable database, not a "database" in the Postgres/
// MySQL sense.
type Options struct {
	DSN         string
	Host        string
	Port        int
	User        string
	Password    string
	ServiceName string
	Logging     bool
	Logger      golem.Logger
}

// New builds a golem.Option that wires an Oracle Connector configured by
// the given function. This is the ONLY supported way end users configure
// an Oracle DataSource — see README.md examples.
func New(configure func(*Options)) golem.Option {
	opts := &Options{}
	configure(opts)
	return golem.WithConnector(&connector{opts: opts})
}
