package golem

// Conn is a sealed marker interface implemented only by types in this
// package (DataSource today, Tx later).
type Conn interface {
	isConn()
	// Dialect returns the active Dialect for this connection. This is the
	// mechanism repository (a separate package) uses to reach the Dialect
	// given only a Conn value.
	Dialect() Dialect
}

