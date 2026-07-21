package golem

// Option configures a DataSource at construction time via NewDataSource.
type Option func(*dataSourceConfig)

// dataSourceConfig accumulates the settings applied by Option values before
// NewDataSource builds the DataSource.
type dataSourceConfig struct {
	name      string
	connector Connector
	parser    Parser
}

// DataSourceName sets the DataSource's name. If never called, NewDataSource
// defaults the name to "default".
func DataSourceName(name string) Option {
	return func(c *dataSourceConfig) { c.name = name }
}

// WithConnector wires the Connector a DataSource delegates its lifecycle to.
// Adapter packages (postgres.New, future mysql.New, ...) call this
// internally — it is exported so adapters outside this module tree can use
// it, but end users never call it directly (they call postgres.New(...)
// instead, which returns an Option built from this).
func WithConnector(c Connector) Option {
	return func(cfg *dataSourceConfig) { cfg.connector = c }
}

// CustomParser overrides the Parser a DataSource uses to convert struct
// field values to/from driver.Value. fn receives DefaultParser and returns
// the Parser to use -- return fn's argument unchanged to keep the default,
// or wrap it (decorator) to extend it while falling back to it. If
// CustomParser is never passed, NewDataSource uses DefaultParser as-is.
func CustomParser(fn func(defaultParser Parser) Parser) Option {
	return func(cfg *dataSourceConfig) { cfg.parser = fn(DefaultParser) }
}
