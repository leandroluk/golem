package golem

// ColumnType is an opaque, dialect-agnostic semantic column-type identifier.
type ColumnType struct {
	kind   string
	length int // only meaningful for kinds that carry a length, e.g. VARCHAR
}
