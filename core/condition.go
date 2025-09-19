// Package core provides the fundamental building blocks of the golem ORM.
// It defines abstractions for queries, models, schema handling, and drivers.
package core

// Condition represents a single clause in a query filter.
//
// A condition can target a specific field (FieldName) with a given operator
// (Eq, Gt, Like, In, etc.) and a comparison value. Conditions can also
// be nested using Children, enabling composition of complex logical
// expressions with AND, OR, and NOT.
//
// Example:
//
//	cond := (&Condition{FieldName: "age"}).Gt(18).
//		And((&Condition{FieldName: "status"}).Eq("active"))
//
// The above creates a condition equivalent to:
//
//	(age > 18) AND (status = "active")
type Condition struct {
	FieldName string       // The field/column name this condition applies to
	Operator  *Operator    // The comparison operator (Eq, Gt, Like, etc.)
	Value     any          // The comparison value
	Children  []*Condition // Nested conditions (for AND, OR, NOT expressions)
}

// And combines this condition with additional conditions using the logical AND operator.
func (c *Condition) And(conditions ...*Condition) *Condition {
	return &Condition{
		Operator: &OpAnd,
		Children: append([]*Condition{c}, conditions...),
	}
}

// Or combines this condition with additional conditions using the logical OR operator.
func (c *Condition) Or(conditions ...*Condition) *Condition {
	return &Condition{
		Operator: &OpOr,
		Children: append([]*Condition{c}, conditions...),
	}
}

// Not negates this condition using the logical NOT operator.
func (c *Condition) Not() *Condition {
	return &Condition{
		Operator: &OpNot,
		Children: []*Condition{c},
	}
}

// Nil sets this condition to check for NULL values (IS NULL).
func (c *Condition) Nil() *Condition {
	c.Operator = &OpNil
	c.Value = nil
	return c
}

// Eq sets this condition to check for equality (=).
func (c *Condition) Eq(v any) *Condition {
	c.Operator = &OpEq
	c.Value = v
	return c
}

// Gt sets this condition to check for "greater than" (>).
func (c *Condition) Gt(v any) *Condition {
	c.Operator = &OpGt
	c.Value = v
	return c
}

// Gte sets this condition to check for "greater than or equal" (>=).
func (c *Condition) Gte(v any) *Condition {
	c.Operator = &OpGte
	c.Value = v
	return c
}

// Lt sets this condition to check for "less than" (<).
func (c *Condition) Lt(v any) *Condition {
	c.Operator = &OpLt
	c.Value = v
	return c
}

// Lte sets this condition to check for "less than or equal" (<=).
func (c *Condition) Lte(v any) *Condition {
	c.Operator = &OpLte
	c.Value = v
	return c
}

// Like sets this condition to perform a pattern match (SQL LIKE / regex equivalent).
func (c *Condition) Like(v any) *Condition {
	c.Operator = &OpLike
	c.Value = v
	return c
}

// In sets this condition to check whether the field value is contained in the provided list.
func (c *Condition) In(values ...any) *Condition {
	c.Operator = &OpIn
	c.Value = values
	return c
}
