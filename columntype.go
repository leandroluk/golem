package golem

// ColumnType is an opaque, dialect-agnostic semantic column-type identifier.
// M2 will add exported constructors (BIGINT(), VARCHAR(n), UUID(), etc.)
// that set kind — none of those exist yet in this task.
type ColumnType struct {
	kind string
}
