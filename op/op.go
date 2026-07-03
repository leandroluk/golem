package op

// Condition represents a predicate node in the query builder. It carries
// field pointer references and comparison/logical metadata, which is resolved
// to actual column names inside repository by offset arithmetic.
type Condition struct {
	FieldPtr      any
	Value         any
	Op            string      // "eq", "gt", "gte", "lt", "lte", "like", "in", "or", "not"
	SubConditions []Condition // For logical "or" / "not"
}

// Eq builds an equality Condition: column == value.
func Eq(fieldPtr any, value any) Condition {
	return Condition{FieldPtr: fieldPtr, Value: value, Op: "eq"}
}

// Gt builds a greater-than Condition: column > value.
func Gt(fieldPtr any, value any) Condition {
	return Condition{FieldPtr: fieldPtr, Value: value, Op: "gt"}
}

// Gte builds a greater-than-or-equal Condition: column >= value.
func Gte(fieldPtr any, value any) Condition {
	return Condition{FieldPtr: fieldPtr, Value: value, Op: "gte"}
}

// Lt builds a less-than Condition: column < value.
func Lt(fieldPtr any, value any) Condition {
	return Condition{FieldPtr: fieldPtr, Value: value, Op: "lt"}
}

// Lte builds a less-than-or-equal Condition: column <= value.
func Lte(fieldPtr any, value any) Condition {
	return Condition{FieldPtr: fieldPtr, Value: value, Op: "lte"}
}

// In builds an IN list Condition: column IN (values...).
func In(fieldPtr any, values ...any) Condition {
	return Condition{FieldPtr: fieldPtr, Value: values, Op: "in"}
}

// Like builds a LIKE string Condition: column LIKE value.
func Like(fieldPtr any, value any) Condition {
	return Condition{FieldPtr: fieldPtr, Value: value, Op: "like"}
}

// Or builds a logical OR combination of conditions.
func Or(conditions ...Condition) Condition {
	return Condition{Op: "or", SubConditions: conditions}
}

// Not builds a logical negation of the given condition.
func Not(condition Condition) Condition {
	return Condition{Op: "not", SubConditions: []Condition{condition}}
}

// Order represents a single field sorting descriptor.
type Order struct {
	FieldPtr any
	Desc     bool
}

// Asc builds an ascending sort order for the given field pointer.
func Asc(fieldPtr any) Order {
	return Order{FieldPtr: fieldPtr, Desc: false}
}

// Desc builds a descending sort order for the given field pointer.
func Desc(fieldPtr any) Order {
	return Order{FieldPtr: fieldPtr, Desc: true}
}

