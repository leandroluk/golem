package golem

// Conn is a sealed marker interface implemented only by types in this
// package (DataSource today, Tx later). It intentionally exposes no public
// methods yet — later milestones (repository, transactions, raw SQL exec)
// grow it incrementally as real callers need something from it.
type Conn interface {
	isConn()
}
