package oracle

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/leandroluk/golem"
	_ "github.com/sijms/go-ora/v2" // registers the "oracle" database/sql driver
)

// SetMaxOpenConns(1) is not optional — confirmed via a real, reproducible
// bug during T10's own verification run: with the default unbounded pool,
// a client-side FK-restrict check (repository.go's applyDeleteCascades,
// which runs inside a transaction that gets rolled back when the check
// finds a blocking row) intermittently left a LATER, unrelated query on a
// freshly-pooled connection unable to see an already-committed row from a
// different connection — a stale-read race, not a genuine data bug (the
// row was never touched; a follow-up Exists() call on that same row
// spuriously returned false). The failure was flaky (didn't reproduce on
// every run, and reproduced only when the full conformance suite ran, not
// isolated subtest slices), consistent with a real connection-pool race
// rather than a deterministic logic bug. Serializing every operation onto
// a single connection eliminated it across many repeated real-container
// runs. This mirrors driver/sqlite's own SetMaxOpenConns(1) precedent,
// though for an entirely different reason (SQLite's is about its
// single-writer file-locking model; this is about go-ora/Oracle session
// consistency under concurrent pooled connections).
var newSQLDB = func(dsn string) (sqlIface, error) {
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// connector is the oracle.Connector implementation returned by New. It
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
		return nil, fmt.Errorf("oracle: %w", err)
	}

	db, err := newSQLDB(dsn)
	if err != nil {
		c.log(golem.LogLevelError, "connect failed", map[string]any{"error": err.Error()})
		return nil, fmt.Errorf("oracle: connect: %w", err)
	}

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		c.log(golem.LogLevelError, "connect failed", map[string]any{"error": err.Error()})
		return nil, fmt.Errorf("oracle: connect: %w", err)
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
