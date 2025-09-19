// Package core provides the fundamental building blocks of the golem ORM.
// This file defines the fluent query builder, which allows type-safe and
// expressive construction of queries.
package core

import (
	"reflect"
	"unsafe"
)

// Query represents a fluent query builder for an entity of type T.
//
// It allows chaining of filtering, ordering, pagination, and soft-delete
// options in a type-safe manner.
//
// Example:
//
//	users, _ := userModel.
//		NewQuery(userSchema).
//		Filter(func(q core.Filter[User]) []*core.Condition {
//			return []*core.Condition{
//				q.Where(&User.Email).Like("%gmail.com"),
//				q.Where(&User.Active).Eq(true),
//			}
//		}).
//		OrderBy("created_at", -1).
//		Limit(10).
//		Offset(0)
type Query[T any] struct {
	schema *SchemaMeta[T]
	where  *Where
}

// NewQuery creates a new Query instance for the given schema.
func NewQuery[T any](schema *SchemaMeta[T]) *Query[T] {
	return &Query[T]{
		schema: schema,
		where:  &Where{},
	}
}

// WithDeleted includes soft-deleted rows in the query results.
func (q *Query[T]) WithDeleted() *Query[T] {
	q.where.WithDeleted = true
	return q
}

// OnlyDeleted restricts the query results to soft-deleted rows only.
func (q *Query[T]) OnlyDeleted() *Query[T] {
	q.where.OnlyDeleted = true
	return q
}

// Where adds a new condition to the query in a type-safe manner.
//
// It supports both:
//   - Direct pointers to fields (e.g., &User.Name)
//   - Selector functions (e.g., func(*User) *string { return &u.Name })
//
// The condition is returned so that an operator can be applied immediately.
//
// Example:
//
//	q.Where(&User.Age).Gt(18)
func (q *Query[T]) Where(fieldPtr any) *Condition {
	var goFieldName string

	switch f := fieldPtr.(type) {
	case func(*T) any:
		goFieldName = fieldNameFromSelectorFor[T](f)
	default:
		// detect direct pointer to field
		rt := reflect.TypeOf((*T)(nil)).Elem()
		rv := reflect.ValueOf(fieldPtr)
		if rv.Kind() != reflect.Ptr {
			panic("Where: argument must be a pointer to field or selector func(*T) *F")
		}
		// traverse struct fields to find offset
		ptr := rv.Pointer()
		var zero T
		base := uintptr(unsafe.Pointer(&zero))
		offset := uintptr(ptr) - base
		for _, sf := range reflect.VisibleFields(rt) {
			if sf.Offset == offset {
				goFieldName = sf.Name
				break
			}
		}
	}

	// map to database column name if defined
	dbCol := goFieldName
	for _, f := range q.schema.Fields {
		if f.StructFieldName == goFieldName {
			dbCol = f.DatabaseColumnName
			break
		}
	}

	return &Condition{FieldName: dbCol}
}

// Filter builds a set of conditions using a functional style.
//
// The provided function receives a Filter[T] scope that exposes a type-safe
// Where method. The returned conditions are combined with AND by default.
//
// Example:
//
//	q.Filter(func(f core.Filter[User]) []*core.Condition {
//		return []*core.Condition{
//			f.Where(&User.Age).Gt(18),
//			f.Where(&User.Active).Eq(true),
//		}
//	})
func (q *Query[T]) Filter(build func(Filter[T]) []*Condition) *Query[T] {
	if build == nil {
		q.where.Condition = nil
		return q
	}
	scope := Filter[T]{queryBuilder: q}
	conds := build(scope)
	q.where.Condition = foldConditionsAnd(conds...)
	return q
}

// Filter provides the scope passed to the Filter function.
// It exposes a type-safe Where method bound to the parent query.
type Filter[T any] struct{ queryBuilder *Query[T] }

// Where delegates to the parent query’s Where method.
// This is used inside functional filters for cleaner syntax.
func (f Filter[T]) Where(fieldPtr any) *Condition {
	return f.queryBuilder.Where(fieldPtr)
}

// OrderBy adds an ordering rule to the query.
//
// Field is the column/field name, and order is 1 (ASC) or -1 (DESC).
func (q *Query[T]) OrderBy(field string, order int) *Query[T] {
	q.where.Sort = append(q.where.Sort, Sort{FieldName: field, Order: order})
	return q
}

// Limit sets the maximum number of results to return.
func (q *Query[T]) Limit(limit int) *Query[T] {
	q.where.Limit = limit
	return q
}

// Offset sets the number of rows to skip before starting to return results.
func (q *Query[T]) Offset(offset int) *Query[T] {
	q.where.Offset = offset
	return q
}
