package golem

// ColumnType is an opaque, dialect-agnostic semantic column-type identifier.
// Dialects read it via the accessor methods; the zero-value represents an
// unrecognized / unset type.
type ColumnType struct {
	kind      string
	precision int // only meaningful for kinds that carry a precision, e.g. DECIMAL, FLOAT
	scale     int // only meaningful for kinds that carry a scale, e.g. DECIMAL
	length    int // only meaningful for kinds that carry a length, e.g. CHAR, VARCHAR
}

// Kind returns the dialect-agnostic type identifier (e.g. "bigint", "varchar").
func (c ColumnType) Kind() string { return c.kind }

// Precision returns the precision for numeric types (DECIMAL, FLOAT); zero for others.
func (c ColumnType) Precision() int { return c.precision }

// Scale returns the scale for DECIMAL; zero for other types.
func (c ColumnType) Scale() int { return c.scale }

// Length returns the max length for CHAR / VARCHAR; zero for other types.
func (c ColumnType) Length() int { return c.length }

