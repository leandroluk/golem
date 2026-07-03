package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/leandroluk/golem"
)

// connector is the postgres.Connector implementation returned by New. It
// resolves the DSN, opens a pgxpool.Pool, and forces a real round-trip via
// Ping so connection failures surface immediately from Connect() rather than
// on first query.
type connector struct {
	opts *Options
	pool *pgxpool.Pool
}

var _ golem.Connector = (*connector)(nil)

// Connect resolves the DSN, opens a pgxpool.Pool, and Pings it immediately
// (pgxpool.New itself is lazy and does NOT fail on a bad host/credentials —
// only Ping forces a real round-trip, which is required so Connect() surfaces
// unreachable-host/bad-auth errors immediately instead of on first query).
// On any failure, closes the pool if it was opened and returns a descriptive
// error — never panics. Logs lifecycle events (connect success/failure) via
// opts.Logger (falling back to golem.DefaultLogger()) when opts.Logging is
// true; logs nothing when opts.Logging is false.
func (c *connector) Connect() (golem.Dialect, error) {
	dsn, err := resolveDSN(c.opts)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		c.log(golem.LogLevelError, "connect failed", map[string]any{"error": err.Error()})
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		c.log(golem.LogLevelError, "connect failed", map[string]any{"error": err.Error()})
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}

	c.pool = pool
	c.log(golem.LogLevelInfo, "connected", nil)
	return &dialect{pool: pool}, nil
}

// Close releases the pool. pgxpool.Pool.Close() has no error return, so this
// always returns nil. Safe to call even if Connect never succeeded (c.pool
// would be nil).
func (c *connector) Close() error {
	if c.pool != nil {
		c.pool.Close()
		c.log(golem.LogLevelInfo, "closed", nil)
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
