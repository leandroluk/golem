package golem

import "fmt"

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

func (*DataSource) isConn() {}

// NewDataSource builds a DataSource from the given options. Returns an error
// if no Connector was supplied via any option (e.g. postgres.New(...) is
// required — there is no usable DataSource without one).
func NewDataSource(opts ...Option) (*DataSource, error) {
	cfg := &dataSourceConfig{name: "default"}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.connector == nil {
		return nil, fmt.Errorf("golem: no connector configured (pass e.g. postgres.New(...) to NewDataSource)")
	}
	return &DataSource{name: cfg.name, connector: cfg.connector}, nil
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

// Close releases the connection via the configured Connector. Safe no-op if
// never connected. Idempotent if called twice.
func (ds *DataSource) Close() error {
	if !ds.connected {
		return nil
	}
	err := ds.connector.Close()
	ds.connected = false
	return err
}
