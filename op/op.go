package op

// Condition is a single "column compares to a literal value" predicate.
// FieldPtr is resolved to a column name later, by whichever package
// consumes it (repository) — op itself has no entity metadata and does no
// resolution.
type Condition struct {
	FieldPtr any
	Value    any
}

// Eq builds an equality Condition: the column fieldPtr resolves to must
// equal value.
func Eq(fieldPtr any, value any) Condition {
	return Condition{FieldPtr: fieldPtr, Value: value}
}
