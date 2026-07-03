package entity

// Index configures an index declared via entity.Table.Index.
type Index struct {
	name   string
	unique bool
}

// Name sets the index name.
func (b *Index) Name(name string) *Index {
	b.name = name
	return b
}

// Unique marks the index as a unique index.
func (b *Index) Unique() *Index {
	b.unique = true
	return b
}

// ResolvedName returns the name set via Name(), or "".
func (b *Index) ResolvedName() string {
	return b.name
}

// IsUnique returns true if Unique() was called.
func (b *Index) IsUnique() bool {
	return b.unique
}

