// Package stmt defines AST nodes representing SQL statements and logical
// query predicates. It is an internal package used by repository and dialect
// adapters, not exposed to end users.
package stmt

import "database/sql/driver"

// Predicate represents a node in a WHERE condition tree.
type Predicate interface {
	isPredicate()
}

// Comparison represents a field-to-value or column-to-value comparison.
type Comparison struct {
	Column string
	Op     string // "eq", "gt", "gte", "lt", "lte", "like", "in", "is_null"
	Value  any    // driver.Value or []driver.Value for "in"
}

func (Comparison) isPredicate() {}

// Logical represents a logical combination (AND, OR) of nested predicates.
type Logical struct {
	Op         string // "and", "or"
	Predicates []Predicate
}

func (Logical) isPredicate() {}

// Not represents a logical negation of a nested predicate.
type Not struct {
	Predicate Predicate
}

func (Not) isPredicate() {}

// OrderElement represents a single column ordering clause.
type OrderElement struct {
	Column string
	Desc   bool
}

// OnCondition represents column = column equality in JOIN ON clause.
type OnCondition struct {
	LeftCol  string
	RightCol string
}

// Join represents a single JOIN clause in SELECT statement.
type Join struct {
	Type  string // "inner", "left", "right", "full"
	Table string
	On    []OnCondition
	Where Predicate
}

// Select represents a SELECT statement plan.
type Select struct {
	Table   string
	Columns []string // Empty means "*"
	Where   Predicate
	OrderBy []OrderElement
	Limit   *int
	Offset  *int
	Count   bool // If true, projects COUNT(*) instead of Columns
	Joins   []Join
}


// Delete represents a DELETE statement plan.
type Delete struct {
	Table string
	Where Predicate
}

// Insert represents an INSERT statement plan.
type Insert struct {
	Table   string
	Columns []string
	Values  []driver.Value
}

// UpdateClause represents a single SET column = value clause.
type UpdateClause struct {
	Column string
	Value  driver.Value
}

// Update represents an UPDATE statement plan.
type Update struct {
	Table string
	Sets  []UpdateClause
	Where Predicate
}

