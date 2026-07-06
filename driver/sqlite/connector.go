package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/leandroluk/golem"
	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

var newSQLDB = func(dsn string) (sqlIface, error) {
	return sql.Open("sqlite", dsn)
}

// connector is the sqlite.Connector implementation returned by New. It
// resolves the DSN, opens a *sql.DB, bounds its pool to a single
// connection, and forces a real round-trip via Ping so open failures
// (e.g. an unwritable path) surface immediately from Connect() rather than
// on first query.
type connector struct {
	opts *Options
	db   sqlIface
}

var _ golem.Connector = (*connector)(nil)

// Connect resolves the DSN, opens a *sql.DB, bounds the pool to exactly one
// connection, and Pings it immediately. The pool is bounded to one
// connection for two independent reasons that converge on the same fix:
// SQLite is single-writer (a larger pool just serializes behind SQLITE_BUSY
// retries anyway), and a ":memory:" database (even with cache=shared)
// depends on at least one connection staying open — bounding the pool
// removes any risk of the pool silently opening a second, divergent
// connection under load. On any failure, closes the pool if it was opened
// and returns a descriptive error — never panics. Logs lifecycle events
// (connect success/failure) via opts.Logger (falling back to
// golem.DefaultLogger()) when opts.Logging is true; logs nothing when
// opts.Logging is false.
func (c *connector) Connect() (golem.Dialect, error) {
	dsn := resolveDSN(c.opts)

	db, err := newSQLDB(dsn)
	if err != nil {
		c.log(golem.LogLevelError, "connect failed", map[string]any{"error": err.Error()})
		return nil, fmt.Errorf("sqlite: connect: %w", err)
	}

	if sqlDB, ok := db.(*sql.DB); ok {
		sqlDB.SetMaxOpenConns(1)
	}

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		c.log(golem.LogLevelError, "connect failed", map[string]any{"error": err.Error()})
		return nil, fmt.Errorf("sqlite: connect: %w", err)
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
