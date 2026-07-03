package entity

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/leandroluk/golem"
)

// resolveField returns the struct field name that fieldPtr points into,
// given zero (a pointer to the same zero-value T instance the caller's
// builder callback was given). Resolution is by memory OFFSET, not by type
// or declaration order — this is what lets two fields of the same Go type
// (e.g. two int64 fields) be told apart correctly.
func resolveField(zero any, fieldPtr any) (string, error) {
	base := reflect.ValueOf(zero).Elem() // zero is *T; .Elem() must be addressable
	baseAddr := base.UnsafeAddr()
	fieldAddr := reflect.ValueOf(fieldPtr).Pointer() // fieldPtr is *FieldType
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
	FieldName string
	Name      string
	Type      golem.ColumnType
}

// ForeignKeyMeta records that FieldName's column references another
// entity's primary key. It does not store the target itself (see
// Builder.ForeignKey's doc comment for why), since Builder is not generic
// over the target entity's type parameter — this pass's repository CRUD
// operations don't need to walk FK metadata to a target, only know that a
// column is a plain column like any other.
type ForeignKeyMeta struct {
	FieldName string
}

// EntityMeta holds a T's fully resolved schema metadata.
type EntityMeta struct {
	TableName   string
	SchemaName  string
	Columns     []ColumnMeta
	PrimaryKey  []string // COLUMN names, in declared order
	ForeignKeys []ForeignKeyMeta
}

// Entity holds a T's schema metadata, built once via New and read via
// Describe(). It never generates DDL (golem never does — migrations are
// always an external concern).
type Entity[T any] struct {
	meta EntityMeta
}

// New builds an Entity[T] by running fn against a zero-value *T and a
// Builder that resolves every field-pointer argument by memory offset
// against that same zero value.
func New[T any](fn func(t *T, b *Builder)) *Entity[T] {
	var zero T
	e := &Entity[T]{}
	// default table name = lowercased struct name, overridable via b.TableName(...)
	e.meta.TableName = strings.ToLower(reflect.TypeOf(zero).Name())
	b := newBuilder(&zero, e)
	fn(&zero, b)
	b.finalize()
	return e
}

// Describe returns the entity's fully resolved schema metadata.
func (e *Entity[T]) Describe() EntityMeta {
	return e.meta
}
