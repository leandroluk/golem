package column

// Builder configures a single column declared via entity.Builder.Col. A
// future entity package task constructs it internally (unexported fields)
// and reads Name() back out after the user's callback finishes.
type Builder struct {
	name string
}

// Name overrides the column's name (default, applied elsewhere, is the
// lowercased struct field name).
func (b *Builder) Name(name string) *Builder {
	b.name = name
	return b
}

// ResolvedName returns the name set via Name(), or "" if it was never
// called (the caller — the entity package — applies its own default in
// that case).
func (b *Builder) ResolvedName() string {
	return b.name
}
