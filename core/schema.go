// Package core provides the fundamental building blocks of the golem ORM.
// This file defines the schema system, which maps Go structs to database
// collections/tables, describes fields and relations, and supports schema building.
package core

import "reflect"

// Field represents a struct field mapped to a database column.
//
// It contains metadata such as the Go field name, database column name,
// type information, constraints (primary key, unique, required), default value,
// and special markers for timestamp fields (createdAt, updatedAt, deletedAt).
type Field struct {
	StructFieldName    string       // Name of the field in the Go struct
	DatabaseColumnName string       // Name of the column in the database
	Type               reflect.Type // Go type of the field
	IsPrimaryKey       bool         // Whether this field is a primary key
	IsUnique           bool         // Whether this field is unique
	IsRequired         bool         // Whether this field is required
	DefaultValue       string       // Default value (if any)
	MemoryOffset       uintptr      // Memory offset within the struct

	// Special timestamp markers
	IsCreatedAt bool
	IsUpdatedAt bool
	IsDeletedAt bool
}

// FieldOption is a function used to configure a Field.
type FieldOption func(*Field)

// PrimaryKey marks the field as a primary key.
func PrimaryKey() FieldOption {
	return func(f *Field) { f.IsPrimaryKey = true }
}

// Unique marks the field as unique.
func Unique() FieldOption {
	return func(f *Field) { f.IsUnique = true }
}

// Required marks the field as required (non-nullable).
func Required() FieldOption {
	return func(f *Field) { f.IsRequired = true }
}

// Default sets a default value for the field.
func Default(value string) FieldOption {
	return func(f *Field) { f.DefaultValue = value }
}

// CreatedAt marks the field as the createdAt timestamp.
func CreatedAt() FieldOption {
	return func(f *Field) { f.IsCreatedAt = true }
}

// UpdatedAt marks the field as the updatedAt timestamp.
func UpdatedAt() FieldOption {
	return func(f *Field) { f.IsUpdatedAt = true }
}

// DeletedAt marks the field as the deletedAt timestamp (for soft deletes).
func DeletedAt() FieldOption {
	return func(f *Field) { f.IsDeletedAt = true }
}

// SchemaCore contains the minimal schema information required at runtime.
//
// It includes the database name, collection/table name, fields, and a
// map of fields indexed by their memory offsets.
type SchemaCore struct {
	Database       string
	Collection     string
	Fields         []*Field
	fieldsByOffset map[uintptr]*Field
}

// RelationKind defines the type of relationship between entities.
type RelationKind int

const (
	OneToOne   RelationKind = 1
	OneToMany  RelationKind = 2
	ManyToMany RelationKind = 3
)

// Relation describes a relationship between two schemas in a generic form.
//
// L = Local type, F = Foreign type, J = Join type (for many-to-many).
type Relation[L any, F any, J any] struct {
	Kind           RelationKind
	Field          any            // func(*L) *<FieldType in L> (e.g. *[]Role)
	RefSchema      *SchemaMeta[F] // Schema of the foreign entity
	LocalKey       any            // func(*L) *<KeyType in L>
	ForeignKey     any            // func(*F) *<KeyType in F>
	JoinTable      string         // Join table/collection name (for many-to-many)
	JoinLocalKey   any            // func(*J) *<KeyType in J> (many-to-many)
	JoinForeignKey any            // func(*J) *<KeyType in J> (many-to-many)
}

// RelationInternal is the normalized runtime representation of a relation.
//
// Unlike Relation, it stores resolved field names instead of selector functions.
type RelationInternal struct {
	Kind           RelationKind
	FieldName      string
	RefSchema      *SchemaMeta[any]
	LocalKey       string
	ForeignKey     string
	JoinTable      string
	JoinLocalKey   string
	JoinForeignKey string
}

// SchemaMeta extends SchemaCore with runtime metadata.
//
// It contains registered hooks, relations, and cached references to special
// fields (createdAt, updatedAt, deletedAt).
type SchemaMeta[T any] struct {
	SchemaCore
	PreHookList  map[PreHook][]func(*T) error
	PostHookList map[PostHook][]func(*T) error
	RelationList []RelationInternal

	createdAtField *Field
	updatedAtField *Field
	deletedAtField *Field
}

// RegisterPreHook registers a pre-operation hook for the schema.
func (s *SchemaMeta[T]) RegisterPreHook(hook PreHook, fn func(*T) error) {
	s.PreHookList[hook] = append(s.PreHookList[hook], fn)
}

// RegisterPostHook registers a post-operation hook for the schema.
func (s *SchemaMeta[T]) RegisterPostHook(hook PostHook, fn func(*T) error) {
	s.PostHookList[hook] = append(s.PostHookList[hook], fn)
}

// AddRelation resolves selectors into field names and adds the relation
// to the schema's internal list.
func AddRelation[L any, F any, J any](schema *SchemaMeta[L], r Relation[L, F, J]) {
	internal := RelationInternal{
		Kind:           r.Kind,
		FieldName:      fieldNameFromSelectorFor[L](r.Field),
		RefSchema:      any(r.RefSchema).(*SchemaMeta[any]),
		LocalKey:       fieldNameFromSelectorFor[L](r.LocalKey),
		ForeignKey:     fieldNameFromSelectorFor[F](r.ForeignKey),
		JoinTable:      r.JoinTable,
		JoinLocalKey:   fieldNameFromSelectorFor[J](r.JoinLocalKey),
		JoinForeignKey: fieldNameFromSelectorFor[J](r.JoinForeignKey),
	}
	schema.RelationList = append(schema.RelationList, internal)
}

// findRelation finds a registered relation by field name.
func (s *SchemaMeta[T]) findRelation(name string) *RelationInternal {
	for _, relation := range s.RelationList {
		if relation.FieldName == name {
			return &relation
		}
	}
	return nil
}

// SchemaBuilder is used to construct a schema definition from a Go struct.
//
// It collects field metadata using reflection and applies customization
// through SchemaOptions.
type SchemaBuilder[T any] struct {
	database       string
	collection     string
	tagKey         string
	structType     reflect.Type
	fields         []*Field
	fieldsByOffset map[uintptr]*Field
}

// SchemaOption represents a function that customizes the schema builder.
type SchemaOption[T any] func(*SchemaBuilder[T])

// TagKey sets the struct tag key to use for database column mapping.
func TagKey[T any](key string) SchemaOption[T] {
	return func(schemaBuilder *SchemaBuilder[T]) { schemaBuilder.tagKey = key }
}

// Table sets the database collection/table name for the schema.
func Table[T any](name string) SchemaOption[T] {
	return func(schemaBuilder *SchemaBuilder[T]) { schemaBuilder.collection = name }
}

// Database sets the database name for the schema.
func Database[T any](name string) SchemaOption[T] {
	return func(schemaBuilder *SchemaBuilder[T]) { schemaBuilder.database = name }
}

// OverrideField allows modifying the metadata of a specific field
// (e.g., making it required, unique, primary key, etc.).
func OverrideField[T any, F any](selector func(*T) *F, opts ...FieldOption) SchemaOption[T] {
	return func(schemaBuilder *SchemaBuilder[T]) {
		offset := offsetOf(selector)
		if field, ok := schemaBuilder.fieldsByOffset[offset]; ok {
			for _, opt := range opts {
				opt(field)
			}
		} else {
			panic("core: OverrideField — field not found by selector")
		}
	}
}

// Schema builds a SchemaMeta[T] by reflecting on struct fields
// and applying the given SchemaOptions.
//
// It detects special timestamp fields (createdAt, updatedAt, deletedAt)
// based on FieldOptions applied via OverrideField.
func Schema[T any](options ...SchemaOption[T]) *SchemaMeta[T] {
	var zero T
	structType := reflect.TypeOf(zero)
	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}

	builder := &SchemaBuilder[T]{
		structType:     structType,
		fieldsByOffset: make(map[uintptr]*Field),
	}

	// Apply options before building fields (Table/Database/TagKey/etc.)
	for _, option := range options {
		option(builder)
	}

	// Reflect fields from struct type
	for _, sf := range reflect.VisibleFields(structType) {
		dbName := ""
		if builder.tagKey != "" {
			dbName = sf.Tag.Get(builder.tagKey)
		} else {
			dbName = sf.Tag.Get("db")
		}
		if dbName == "" {
			dbName = sf.Name
		}

		field := &Field{
			StructFieldName:    sf.Name,
			DatabaseColumnName: dbName,
			Type:               sf.Type,
			MemoryOffset:       sf.Offset,
		}
		builder.fields = append(builder.fields, field)
		builder.fieldsByOffset[sf.Offset] = field
	}

	// Re-apply options so that OverrideField can work after fields exist
	for _, option := range options {
		option(builder)
	}

	meta := &SchemaMeta[T]{
		SchemaCore: SchemaCore{
			Database:       builder.database,
			Collection:     builder.collection,
			Fields:         builder.fields,
			fieldsByOffset: builder.fieldsByOffset,
		},
		PreHookList:  make(map[PreHook][]func(*T) error),
		PostHookList: make(map[PostHook][]func(*T) error),
	}

	// Detect special fields once
	for _, f := range builder.fields {
		if f.IsCreatedAt {
			meta.createdAtField = f
		}
		if f.IsUpdatedAt {
			meta.updatedAtField = f
		}
		if f.IsDeletedAt {
			meta.deletedAtField = f
		}
	}

	return meta
}
