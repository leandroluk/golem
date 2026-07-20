package golem

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/leandroluk/golem/internal/must"
)

// DataSource owns the connect/close lifecycle for one logical database
// connection, and holds the Dialect the active Connector produces once
// connected.
type DataSource struct {
	name      string
	connector Connector
	dialect   Dialect
	connected bool
}

var _ Conn = (*DataSource)(nil)

func (*DataSource) isConn() { return }

// dataSourceRegistry holds every live DataSource by name, so GetDataSource
// can look one up from anywhere without the caller threading it through
// manually. Registered by NewDataSource, deregistered by Close.
var (
	dataSourceRegistryMu sync.Mutex
	dataSourceRegistry   atomic.Pointer[map[string]*DataSource]
)

func init() {
	m := make(map[string]*DataSource)
	dataSourceRegistry.Store(&m)
}

// NewDataSource builds a DataSource from the given options and registers it
// under its name (see DataSourceName; defaults to "default") for later
// retrieval via GetDataSource. Returns an error if no Connector was supplied
// via any option (e.g. postgres.New(...) is required — there is no usable
// DataSource without one), or if a DataSource is already registered under
// the same name.
func NewDataSource(opts ...Option) (*DataSource, error) {
	cfg := &dataSourceConfig{name: "default"}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.connector == nil {
		return nil, fmt.Errorf("golem: no connector configured (pass e.g. postgres.New(...) to NewDataSource)")
	}

	dataSourceRegistryMu.Lock()
	defer dataSourceRegistryMu.Unlock()
	oldMap := *dataSourceRegistry.Load()
	if _, exists := oldMap[cfg.name]; exists {
		return nil, fmt.Errorf("golem: a DataSource named %q is already registered", cfg.name)
	}

	newMap := make(map[string]*DataSource, len(oldMap)+1)
	for k, v := range oldMap {
		newMap[k] = v
	}
	ds := &DataSource{name: cfg.name, connector: cfg.connector}
	newMap[cfg.name] = ds
	dataSourceRegistry.Store(&newMap)
	return ds, nil
}

// MustNewDataSource is like NewDataSource but panics instead of returning an
// error — a convenience for call sites (main(), test setup, a Provider
// constructor) where a failed DataSource is unrecoverable and the caller
// would just panic on the error anyway.
func MustNewDataSource(opts ...Option) *DataSource {
	return must.Value(NewDataSource(opts...))
}

// GetDataSource returns the DataSource registered under name (defaults to
// "default" when omitted; optionalName only ever uses its first element,
// the usual Go idiom for a single optional argument). Returns
// ErrDataSourceNotFound wrapped with the name if no DataSource was ever
// created with that name via NewDataSource, or if it was already Close()'d.
func GetDataSource(optionalName ...string) (*DataSource, error) {
	name := "default"
	if len(optionalName) > 0 {
		name = optionalName[0]
	}

	m := *dataSourceRegistry.Load()
	ds, ok := m[name]
	if !ok {
		return nil, fmt.Errorf("golem: %w: %q", ErrDataSourceNotFound, name)
	}
	return ds, nil
}

// Name returns the DataSource's configured name.
func (ds *DataSource) Name() string { return ds.name }

// Dialect returns the active Dialect established by the last successful
// Connect(). Returns nil if never connected.
func (ds *DataSource) Dialect() Dialect { return ds.dialect }

// Connect establishes the connection via the configured Connector and stores
// the resulting Dialect. Idempotent: calling Connect again on an already-
// connected DataSource is a no-op that returns nil. On failure, the
// DataSource is NOT marked connected, so a corrected retry can succeed.
func (ds *DataSource) Connect() error {
	if ds.connected {
		return nil
	}
	dialect, err := ds.connector.Connect()
	if err != nil {
		return err
	}
	ds.dialect = dialect
	ds.connected = true
	return nil
}

// Close releases the connection via the configured Connector and deregisters
// ds from GetDataSource's registry (freeing its name for reuse by a future
// NewDataSource call). Deregistration always happens, even if ds was never
// connected; the underlying Connector.Close() call is a safe no-op in that
// case. Idempotent if called twice.
func (ds *DataSource) Close() error {
	dataSourceRegistryMu.Lock()
	oldMap := *dataSourceRegistry.Load()
	if oldMap[ds.name] == ds {
		newMap := make(map[string]*DataSource, len(oldMap)-1)
		for k, v := range oldMap {
			if k != ds.name {
				newMap[k] = v
			}
		}
		dataSourceRegistry.Store(&newMap)
	}
	dataSourceRegistryMu.Unlock()

	if !ds.connected {
		return nil
	}
	err := ds.connector.Close()
	ds.connected = false
	return err
}

// Transaction executes fn inside a transaction. If fn returns a non-nil error,
// the transaction is rolled back; if fn returns nil, the transaction is committed.
// If fn panics, the transaction is rolled back and the panic is propagated.
func (ds *DataSource) Transaction(ctx context.Context, fn func(tx Tx) error) error {
	dialect := ds.Dialect()
	if dialect == nil {
		return fmt.Errorf("golem: cannot start transaction on disconnected data source")
	}
	txConn, err := dialect.Begin(ctx, ds)
	if err != nil {
		return err
	}
	tx := NewTx(dialect, txConn)

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback(ctx)
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

// Exec executes a raw SQL statement on the data source.
func (ds *DataSource) Exec(ctx context.Context, sql string, args ...any) (Result, error) {
	dialect := ds.Dialect()
	if dialect == nil {
		return nil, fmt.Errorf("golem: cannot execute query on disconnected data source")
	}
	rows, affected, err := dialect.ExecRaw(ctx, ds, sql, args)
	if err != nil {
		return nil, err
	}
	return &rawResult{rows: rows, rowsAffected: affected, currentIndex: -1}, nil
}
