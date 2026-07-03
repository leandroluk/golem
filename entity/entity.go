package entity

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/leandroluk/golem"
)

// ResolveField returns the struct field name that fieldPtr points into,
// given zero (a pointer to the same zero-value T instance the caller's
// builder callback was given). Resolution is by memory OFFSET, not by type
// or declaration order — this is what lets two fields of the same Go type
// (e.g. two int64 fields) be told apart correctly.
func ResolveField(zero any, fieldPtr any) (string, error) {
	base := reflect.ValueOf(zero).Elem()
	baseAddr := base.UnsafeAddr()
	fieldAddr := reflect.ValueOf(fieldPtr).Pointer()
	offset := fieldAddr - baseAddr

	t := base.Type()
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Offset == offset {
			return t.Field(i).Name, nil
		}
	}
	return "", fmt.Errorf("entity: field pointer does not belong to %s", t.Name())
}

// ColumnMeta describes a single declared column.
type ColumnMeta struct {
	FieldName   string
	Name        string
	Type        golem.ColumnType
	Nullable    bool
	Default     any
	HasDefault  bool
	DefaultFunc func() (any, error)
}

// ForeignKeyMeta records that FieldName's column references another entity's
// primary key.
type ForeignKeyMeta struct {
	FieldName string
}

// IndexMeta describes a named index over one or more columns.
type IndexMeta struct {
	Columns []string // column names (not field names), in declared order
	Name    string   // "" if not overridden
	Unique  bool
}

// EntityMeta holds a T's fully resolved schema metadata.
type EntityMeta struct {
	TableName       string
	SchemaName      string
	Columns         []ColumnMeta
	PrimaryKey      []string // column names, in declared order
	ForeignKeys     []ForeignKeyMeta
	Uniques         [][]string // each inner slice is a unique constraint (column names)
	Indexes         []IndexMeta
	CreateDateField string // field name of the create-timestamp column, or ""
	UpdateDateField string // field name of the update-timestamp column, or ""
	DeleteDateField string // field name of the soft-delete timestamp column, or ""
}

// Entity holds a T's schema metadata, built once via New and read via
// Describe(). It never generates DDL (golem never does — migrations are
// always an external concern).
type Entity[T any] struct {
	meta EntityMeta
}

// New builds an Entity[T] by running fn against a zero-value *T and a
// Table that resolves every field-pointer argument by memory offset
// against that same zero value.
func New[T any](fn func(t *T, b *Table)) *Entity[T] {
	var zero T
	e := &Entity[T]{}
	e.meta.TableName = strings.ToLower(reflect.TypeOf(zero).Name())
	b := newTable(&zero, e)
	fn(&zero, b)
	b.finalize()
	return e
}

// Describe returns the entity's fully resolved schema metadata.
func (e *Entity[T]) Describe() EntityMeta {
	return e.meta
}

