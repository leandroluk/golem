// Package core provides the fundamental building blocks of the golem ORM.
// This file defines the set of supported operators used in query conditions.
package core

// Operator represents a comparison or logical operator used in a query condition.
//
// Operators can be logical (AND, OR, NOT) or value-based (EQ, GT, IN, etc.).
type Operator string

const (
	// Logical operators
	opAnd Operator = "AND"
	opOr  Operator = "OR"
	opNot Operator = "NOT"

	// Value-based operators
	opNil  Operator = "NIL"  // field IS NULL
	opEq   Operator = "EQ"   // field = value
	opGt   Operator = "GT"   // field > value
	opGte  Operator = "GTE"  // field >= value
	opLt   Operator = "LT"   // field < value
	opLte  Operator = "LTE"  // field <= value
	opLike Operator = "LIKE" // field LIKE pattern (SQL) or regex (NoSQL)
	opIn   Operator = "IN"   // field IN (value list)
)

// Public operator aliases exposed to users of the ORM.
//
// These variables reference the internal constants and are intended
// to be used when constructing conditions programmatically.
//
// Example:
//
//	cond := &core.Condition{FieldName: "age", Operator: &core.OpGt, Value: 18}
var (
	OpAnd  = opAnd
	OpOr   = opOr
	OpNot  = opNot
	OpNil  = opNil
	OpEq   = opEq
	OpGt   = opGt
	OpGte  = opGte
	OpLt   = opLt
	OpLte  = opLte
	OpLike = opLike
	OpIn   = opIn
)
