// Package repository provides Repository[T], a thin CRUD layer bound to one
// entity's metadata (entity.EntityMeta) and a golem.Conn. It builds no SQL
// itself — that's the golem.Dialect's job (Insert/FindByID) — repository
// only shuttles Go struct field values to/from the driver.Value/map[string]any
// shapes Dialect deals in, via reflect.
package repository

import (
	"context"
	"database/sql/driver"
	"fmt"
	"reflect"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
)

// Repository[T] is bound to one entity's metadata and a connection.
type Repository[T any] struct {
	conn golem.Conn
	meta entity.EntityMeta
}

// Get builds a Repository[T] bound to conn and e. T is inferred from e's
// type (*entity.Entity[T]).
func Get[T any](conn golem.Conn, e *entity.Entity[T]) *Repository[T] {
	return &Repository[T]{conn: conn, meta: e.Describe()}
}

// Insert inserts one new T, returning it with every column (including any
// DB-populated PK) filled from the RETURNING row. A column whose field holds
// its Go zero value is omitted from the INSERT entirely, so a DB-side
// default (e.g. a BIGSERIAL primary key, or any other column default) can
// apply — callers that want to insert a literal zero value for a column
// can't currently express that (out of scope for this pass: no builder API
// exists yet to distinguish "unset" from "explicitly zero").
func (r *Repository[T]) Insert(ctx context.Context, i *T) (T, error) {
	var zero T
	v := reflect.ValueOf(i).Elem()

	columns := make([]string, 0, len(r.meta.Columns))
	values := make([]driver.Value, 0, len(r.meta.Columns))
	for _, col := range r.meta.Columns {
		fieldVal := v.FieldByName(col.FieldName)
		if fieldVal.IsZero() {
			continue
		}
		columns = append(columns, col.Name)
		values = append(values, fieldVal.Interface())
	}

	row, err := r.conn.Dialect().Insert(ctx, r.conn, r.meta.TableName, columns, values)
	if err != nil {
		return zero, fmt.Errorf("repository: insert: %w", err)
	}

	return r.scanRow(row)
}

// InsertMany inserts each item via sequential Insert calls, preserving order.
func (r *Repository[T]) InsertMany(ctx context.Context, items ...*T) ([]T, error) {
	results := make([]T, 0, len(items))
	for _, item := range items {
		result, err := r.Insert(ctx, item)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

// FindByID fetches one T by its single-column primary key. Returns
// golem.ErrNotFound when no row matches. Returns a plain descriptive error
// (not ErrNotFound) if the entity has a composite primary key — that case is
// out of scope for this pass.
func (r *Repository[T]) FindByID(ctx context.Context, id any) (T, error) {
	var zero T
	if len(r.meta.PrimaryKey) != 1 {
		return zero, fmt.Errorf("repository: FindByID requires a single-column primary key, %s has %d", r.meta.TableName, len(r.meta.PrimaryKey))
	}

	row, found, err := r.conn.Dialect().FindByID(ctx, r.conn, r.meta.TableName, r.meta.PrimaryKey[0], id)
	if err != nil {
		return zero, fmt.Errorf("repository: find by id: %w", err)
	}
	if !found {
		return zero, golem.ErrNotFound
	}

	return r.scanRow(row)
}

// scanRow builds a new T and, for each declared column, writes the matching
// entry from row (keyed by column NAME) onto the field named by FieldName.
func (r *Repository[T]) scanRow(row map[string]any) (T, error) {
	var result T
	v := reflect.ValueOf(&result).Elem()

	for _, col := range r.meta.Columns {
		raw, ok := row[col.Name]
		if !ok {
			continue // column not present in the returned row — leave field zero
		}
		field := v.FieldByName(col.FieldName)
		if !field.IsValid() || !field.CanSet() {
			continue
		}
		rawVal := reflect.ValueOf(raw)
		if !rawVal.IsValid() {
			continue // raw was nil
		}
		if rawVal.Type() == field.Type() {
			field.Set(rawVal)
		} else if rawVal.Type().ConvertibleTo(field.Type()) {
			field.Set(rawVal.Convert(field.Type()))
		} else {
			return result, fmt.Errorf("repository: column %q (Go value %v, type %s) is not convertible to field %s (%s)", col.Name, raw, rawVal.Type(), col.FieldName, field.Type())
		}
	}
	return result, nil
}
