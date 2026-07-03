package query

import "github.com/leandroluk/golem/op"

// Query is received by a FindMany/FindOne criteria callback. Where's
// conditions are ANDed. Built internally by repository — never constructed
// directly by end users.
type Query[T any] struct {
	conditions []op.Condition
}

// New builds an empty Query[T]. Used internally by repository.
func New[T any]() *Query[T] {
	return &Query[T]{}
}

// Where adds conditions (AND semantics), accumulating across multiple calls.
func (q *Query[T]) Where(conditions ...op.Condition) {
	q.conditions = append(q.conditions, conditions...)
}

// Conditions returns every condition accumulated so far.
func (q *Query[T]) Conditions() []op.Condition {
	return q.conditions
}

// SetClause is a single "set this column to this value" clause for Update.
type SetClause struct {
	FieldPtr any
	Value    any
}

// Update is received by an UpdateOne/UpdateMany criteria callback.
type Update[T any] struct {
	conditions []op.Condition
	sets       []SetClause
}

// NewUpdate builds an empty Update[T]. Used internally by repository.
func NewUpdate[T any]() *Update[T] {
	return &Update[T]{}
}

// Where adds conditions (AND semantics), accumulating across multiple calls.
func (u *Update[T]) Where(conditions ...op.Condition) {
	u.conditions = append(u.conditions, conditions...)
}

// Set adds a set clause, accumulating across multiple calls.
func (u *Update[T]) Set(fieldPtr any, value any) {
	u.sets = append(u.sets, SetClause{FieldPtr: fieldPtr, Value: value})
}

// Conditions returns every condition accumulated so far.
func (u *Update[T]) Conditions() []op.Condition {
	return u.conditions
}

// Sets returns every set clause accumulated so far.
func (u *Update[T]) Sets() []SetClause {
	return u.sets
}
