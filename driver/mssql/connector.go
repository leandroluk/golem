package mssql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/leandroluk/golem"
	_ "github.com/microsoft/go-mssqldb" // registers the "sqlserver" database/sql driver
)

var newSQLDB = func(dsn string) (sqlIface, error) {
	return sql.Open("sqlserver", dsn)
}

// connector is the mssql.Connector implementation returned by New. It
// resolves the DSN, opens a *sql.DB, and forces a real round-trip via Ping
// so connection failures surface immediately from Connect() rather than on
// first query.
type connector struct {
	opts *Options
	db   sqlIface
}

var _ golem.Connector = (*connector)(nil)

// Connect resolves the DSN, opens a *sql.DB, and Pings it immediately
// (sql.Open itself is lazy and does NOT fail on a bad host/credentials —
// only Ping forces a real round-trip). On any failure, closes the pool if
// it was opened and returns a descriptive error — never panics. Logs
// lifecycle events (connect success/failure) via opts.Logger (falling back
// to golem.DefaultLogger()) when opts.Logging is true; logs nothing when
// opts.Logging is false.
func (c *connector) Connect() (golem.Dialect, error) {
	dsn, err := resolveDSN(c.opts)
	if err != nil {
		return nil, fmt.Errorf("mssql: %w", err)
	}

	db, err := newSQLDB(dsn)
	if err != nil {
		c.log(golem.LogLevelError, "connect failed", map[string]any{"error": err.Error()})
		return nil, fmt.Errorf("mssql: connect: %w", err)
	}

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		c.log(golem.LogLevelError, "connect failed", map[string]any{"error": err.Error()})
		return nil, fmt.Errorf("mssql: connect: %w", err)
	}

	c.db = db
	c.log(golem.LogLevelInfo, "connected", nil)
	return &dialect{db: db}, nil
}

// Close releases the pool, returning *sql.DB.Close()'s error. Safe to call
// even if Connect never succeeded (c.db would be nil).
func (c *connector) Close() error {
	if c.db != nil {
		err := c.db.Close()
		c.log(golem.LogLevelInfo, "closed", nil)
		return err
	}
	return nil
}

func (c *connector) log(level golem.LogLevel, msg string, args map[string]any) {
	if !c.opts.Logging {
		return
	}
	logger := c.opts.Logger
	if logger == nil {
		logger = golem.DefaultLogger()
	}
	switch level {
	case golem.LogLevelError:
		logger.Error(msg, args)
	default:
		logger.Info(msg, args)
	}
}
