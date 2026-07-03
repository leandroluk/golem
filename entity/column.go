package entity

// Column configures a single column declared via entity.Table.Col.
type Column struct {
	name        string
	nullable    bool
	hasNullable bool
	defaultVal  any
	hasDefault  bool
	defaultFunc func() (any, error)
}

// Name overrides the column's name (default: lowercased struct field name).
func (b *Column) Name(name string) *Column {
	b.name = name
	return b
}

// Nullable marks the column as accepting NULL values.
func (b *Column) Nullable() *Column {
	b.nullable = true
	b.hasNullable = true
	return b
}

// Default sets a literal default value for the column.
func (b *Column) Default(value any) *Column {
	b.defaultVal = value
	b.hasDefault = true
	return b
}

// DefaultFunc sets a function that provides a default value for the column.
func (b *Column) DefaultFunc(fn func() (any, error)) *Column {
	b.defaultFunc = fn
	return b
}

// ResolvedName returns the name set via Name(), or "" if never called.
func (b *Column) ResolvedName() string {
	return b.name
}

// ResolvedNullable returns true if Nullable() was called.
func (b *Column) ResolvedNullable() bool {
	return b.nullable
}

// ResolvedDefault returns (value, true) if Default() was called, or (nil, false).
func (b *Column) ResolvedDefault() (any, bool) {
	return b.defaultVal, b.hasDefault
}

// ResolvedDefaultFunc returns the function set via DefaultFunc(), or nil.
func (b *Column) ResolvedDefaultFunc() func() (any, error) {
	return b.defaultFunc
}

