package entity

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/relation"
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

// ForeignKeyMeta records that FieldName's column references TargetTableName's
// primary key (TargetPrimaryKey, always exactly one column — composite-PK
// targets are rejected at declaration time, see Table.ForeignKey).
type ForeignKeyMeta struct {
	FieldName        string
	ColumnName       string
	TargetTableName  string
	TargetPrimaryKey string
	Options          *relation.ForeignKeyOptions
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

type hooks[T any] struct {
	BeforeCreate     func(context.Context, *T, golem.Conn) error
	AfterCreate      func(context.Context, *T, golem.Conn) error
	OnConflictCreate func(context.Context, *T, golem.Conn) error
	BeforeUpdate     func(context.Context, *T, golem.Conn) error
	AfterUpdate      func(context.Context, *T, golem.Conn) error
	OnConflictUpdate func(context.Context, *T, golem.Conn) error
	BeforeDelete     func(context.Context, *T, golem.Conn) error
	AfterDelete      func(context.Context, *T, golem.Conn) error
	OnConflictDelete func(context.Context, *T, golem.Conn) error
}

// Entity holds a T's schema metadata, built once via New and read via
// Describe(). It never generates DDL (golem never does — migrations are
// always an external concern).
type Entity[T any] struct {
	meta  EntityMeta
	hooks hooks[T]
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

// TriggerBeforeCreate runs the registered BeforeCreate hook if present.
func (e *Entity[T]) TriggerBeforeCreate(ctx context.Context, i *T, conn golem.Conn) error {
	if e.hooks.BeforeCreate != nil {
		return e.hooks.BeforeCreate(ctx, i, conn)
	}
	return nil
}

// TriggerAfterCreate runs the registered AfterCreate hook if present.
func (e *Entity[T]) TriggerAfterCreate(ctx context.Context, i *T, conn golem.Conn) error {
	if e.hooks.AfterCreate != nil {
		return e.hooks.AfterCreate(ctx, i, conn)
	}
	return nil
}

// TriggerOnConflictCreate runs the registered OnConflictCreate hook if present.
func (e *Entity[T]) TriggerOnConflictCreate(ctx context.Context, i *T, conn golem.Conn) error {
	if e.hooks.OnConflictCreate != nil {
		return e.hooks.OnConflictCreate(ctx, i, conn)
	}
	return nil
}

// TriggerBeforeUpdate runs the registered BeforeUpdate hook if present.
func (e *Entity[T]) TriggerBeforeUpdate(ctx context.Context, i *T, conn golem.Conn) error {
	if e.hooks.BeforeUpdate != nil {
		return e.hooks.BeforeUpdate(ctx, i, conn)
	}
	return nil
}

// TriggerAfterUpdate runs the registered AfterUpdate hook if present.
func (e *Entity[T]) TriggerAfterUpdate(ctx context.Context, i *T, conn golem.Conn) error {
	if e.hooks.AfterUpdate != nil {
		return e.hooks.AfterUpdate(ctx, i, conn)
	}
	return nil
}

// TriggerOnConflictUpdate runs the registered OnConflictUpdate hook if present.
func (e *Entity[T]) TriggerOnConflictUpdate(ctx context.Context, i *T, conn golem.Conn) error {
	if e.hooks.OnConflictUpdate != nil {
		return e.hooks.OnConflictUpdate(ctx, i, conn)
	}
	return nil
}

// TriggerBeforeDelete runs the registered BeforeDelete hook if present.
func (e *Entity[T]) TriggerBeforeDelete(ctx context.Context, i *T, conn golem.Conn) error {
	if e.hooks.BeforeDelete != nil {
		return e.hooks.BeforeDelete(ctx, i, conn)
	}
	return nil
}

// TriggerAfterDelete runs the registered AfterDelete hook if present.
func (e *Entity[T]) TriggerAfterDelete(ctx context.Context, i *T, conn golem.Conn) error {
	if e.hooks.AfterDelete != nil {
		return e.hooks.AfterDelete(ctx, i, conn)
	}
	return nil
}

// TriggerOnConflictDelete runs the registered OnConflictDelete hook if present.
func (e *Entity[T]) TriggerOnConflictDelete(ctx context.Context, i *T, conn golem.Conn) error {
	if e.hooks.OnConflictDelete != nil {
		return e.hooks.OnConflictDelete(ctx, i, conn)
	}
	return nil
}

