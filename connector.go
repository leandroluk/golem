package golem

// Connector is the seam adapters implement to plug into DataSource's
// connect/close lifecycle. Connect returns the Dialect this connector
// produces once connected.
type Connector interface {
	Connect() (Dialect, error)
	Close() error
}
