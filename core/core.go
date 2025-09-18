package core

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"unsafe"
)

//region Operator

type Operator string

const (
	opAnd Operator = "AND"
	opOr  Operator = "OR"
	opNot Operator = "NOT"

	opNil  Operator = "NIL"
	opEq   Operator = "EQ"
	opGt   Operator = "GT"
	opGte  Operator = "GTE"
	opLt   Operator = "LT"
	opLte  Operator = "LTE"
	opLike Operator = "LIKE"
	opIn   Operator = "IN"
)

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

//endregion

//region Hooks

type PreHook string
type PostHook string

const (
	PreInsert PreHook = "pre:insert"
	PreUpdate PreHook = "pre:update"
	PreDelete PreHook = "pre:delete"
	PreFind   PreHook = "pre:find"

	PostInsert PostHook = "post:insert"
	PostUpdate PostHook = "post:update"
	PostDelete PostHook = "post:delete"
	PostFind   PostHook = "post:find"
)

//endregion

//region Condition

type Condition struct {
	FieldName string
	Operator  *Operator
	Value     any
	Children  []*Condition
}

func (c *Condition) And(conditions ...*Condition) *Condition {
	return &Condition{Operator: &OpAnd, Children: append([]*Condition{c}, conditions...)}
}

func (c *Condition) Or(conditions ...*Condition) *Condition {
	return &Condition{Operator: &OpOr, Children: append([]*Condition{c}, conditions...)}
}

func (c *Condition) Not() *Condition {
	return &Condition{Operator: &OpNot, Children: []*Condition{c}}
}

func (c *Condition) Nil() *Condition { c.Operator = &OpNil; c.Value = nil; return c }
func (c *Condition) Eq(v any) *Condition {
	c.Operator = &OpEq
	c.Value = v
	return c
}
func (c *Condition) Gt(v any) *Condition {
	c.Operator = &OpGt
	c.Value = v
	return c
}
func (c *Condition) Gte(v any) *Condition {
	c.Operator = &OpGte
	c.Value = v
	return c
}
func (c *Condition) Lt(v any) *Condition {
	c.Operator = &OpLt
	c.Value = v
	return c
}
func (c *Condition) Lte(v any) *Condition {
	c.Operator = &OpLte
	c.Value = v
	return c
}
func (c *Condition) Like(v any) *Condition {
	c.Operator = &OpLike
	c.Value = v
	return c
}
func (c *Condition) In(values ...any) *Condition {
	c.Operator = &OpIn
	c.Value = values
	return c
}

//endregion

//region Schema - Fields

type Field struct {
	GoStructFieldName  string
	DatabaseColumnName string
	Type               reflect.Type
	IsPrimaryKey       bool
	IsUnique           bool
	IsRequired         bool
	DefaultValue       string
	MemoryOffset       uintptr
}

type FieldOption func(*Field)

func PrimaryKey() FieldOption { return func(f *Field) { f.IsPrimaryKey = true } }
func Unique() FieldOption     { return func(f *Field) { f.IsUnique = true } }
func Required() FieldOption   { return func(f *Field) { f.IsRequired = true } }
func Default(value string) FieldOption {
	return func(f *Field) { f.DefaultValue = value }
}

//endregion

//region Schema - Core and Meta

type SchemaCore struct {
	Database       string
	Collection     string
	Fields         []*Field
	fieldsByOffset map[uintptr]*Field
}

type SchemaMeta[T any] struct {
	SchemaCore
	PreHooks  map[PreHook][]func(*T) error
	PostHooks map[PostHook][]func(*T) error
}

func (s *SchemaMeta[T]) RegisterPreHook(hook PreHook, fn func(*T) error) {
	s.PreHooks[hook] = append(s.PreHooks[hook], fn)
}
func (s *SchemaMeta[T]) RegisterPostHook(hook PostHook, fn func(*T) error) {
	s.PostHooks[hook] = append(s.PostHooks[hook], fn)
}

//endregion

//region Schema - Builder

type SchemaBuilder[T any] struct {
	database       string
	collection     string
	tagKey         string
	structType     reflect.Type
	fields         []*Field
	fieldsByOffset map[uintptr]*Field
}

type SchemaOption[T any] func(*SchemaBuilder[T])

func TagKey[T any](key string) SchemaOption[T] { return func(b *SchemaBuilder[T]) { b.tagKey = key } }
func Table[T any](name string) SchemaOption[T] {
	return func(b *SchemaBuilder[T]) { b.collection = name }
}
func Database[T any](name string) SchemaOption[T] {
	return func(b *SchemaBuilder[T]) { b.database = name }
}
func OverrideField[T any, F any](selector func(*T) *F, opts ...FieldOption) SchemaOption[T] {
	return func(b *SchemaBuilder[T]) {
		offset := offsetOf(selector)
		if f, ok := b.fieldsByOffset[offset]; ok {
			for _, opt := range opts {
				opt(f)
			}
		} else {
			panic("core: OverrideField — field not found by selector")
		}
	}
}

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

	for _, option := range options {
		option(builder)
	}

	for _, sf := range reflect.VisibleFields(structType) {
		dbName := ""
		if builder.tagKey != "" {
			dbName = sf.Tag.Get(builder.tagKey)
		}
		if dbName == "" {
			dbName = sf.Name
		}

		field := &Field{
			GoStructFieldName:  sf.Name,
			DatabaseColumnName: dbName,
			Type:               sf.Type,
			MemoryOffset:       sf.Offset,
		}
		builder.fields = append(builder.fields, field)
		builder.fieldsByOffset[sf.Offset] = field
	}

	return &SchemaMeta[T]{
		SchemaCore: SchemaCore{
			Database:       builder.database,
			Collection:     builder.collection,
			Fields:         builder.fields,
			fieldsByOffset: builder.fieldsByOffset,
		},
		PreHooks:  make(map[PreHook][]func(*T) error),
		PostHooks: make(map[PostHook][]func(*T) error),
	}
}

//endregion

//region Helpers

func offsetOf[T any, F any](selector func(*T) *F) uintptr {
	var zero T
	base := uintptr(unsafe.Pointer(&zero))
	ptr := selector(&zero)
	return uintptr(unsafe.Pointer(ptr)) - base
}

func StructValues(schema *SchemaCore, doc any) ([]any, []string) {
	v := reflect.ValueOf(doc)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	vals := []any{}
	placeholders := []string{}

	for i, f := range schema.Fields {
		field := v.FieldByName(f.GoStructFieldName)
		vals = append(vals, field.Interface())
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}

	return vals, placeholders
}

//endregion

//region Where

func Where[T any, F any](schema *SchemaMeta[T], selector func(*T) *F) *Condition {
	offset := offsetOf(selector)
	if field := schema.fieldsByOffset[offset]; field != nil {
		return &Condition{FieldName: field.DatabaseColumnName}
	}
	return &Condition{FieldName: "???"}
}

//endregion

//region transactionKey

type transactionKey struct{}

func WithTransaction(ctx context.Context, tx Transaction) context.Context {
	return context.WithValue(ctx, transactionKey{}, tx)
}

func TransactionFrom(ctx context.Context) Transaction {
	if v, ok := ctx.Value(transactionKey{}).(Transaction); ok {
		return v
	}
	return nil
}

//endregion

//region Driver

type Sort struct {
	FieldName string
	Order     int // 1 = ASC, -1 = DESC
}

type Query struct {
	Condition *Condition
	Limit     int
	Offset    int
	Sort      []Sort
}

type Changes map[string]any

type Transaction interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type Driver interface {
	Connect(ctx context.Context) error
	Ping(ctx context.Context) error
	Close(ctx context.Context) error
	Transaction(ctx context.Context) (Transaction, error)
	Insert(ctx context.Context, schema *SchemaCore, documents ...any) error
	FindOne(ctx context.Context, schema *SchemaCore, options *Query) (any, error)
	FindMany(ctx context.Context, schema *SchemaCore, options *Query) (any, error)
	Update(ctx context.Context, schema *SchemaCore, condition *Condition, changes Changes) error
	Delete(ctx context.Context, schema *SchemaCore, condition *Condition) error
	Count(ctx context.Context, schema *SchemaCore, condition *Condition) (int64, error)
}

//endregion

//region Model

type Model[T any] struct {
	schema *SchemaMeta[T]
	driver Driver
}

func NewModel[T any](schema *SchemaMeta[T], driver Driver) *Model[T] {
	return &Model[T]{schema: schema, driver: driver}
}

func (m *Model[T]) runPre(h PreHook, doc *T) error {
	if fns, ok := m.schema.PreHooks[h]; ok {
		for _, fn := range fns {
			if err := fn(doc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Model[T]) runPost(h PostHook, doc *T) error {
	if fns, ok := m.schema.PostHooks[h]; ok {
		for _, fn := range fns {
			if err := fn(doc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Model[T]) Create(ctx context.Context, document *T) error {
	if err := m.runPre(PreInsert, document); err != nil {
		return err
	}
	if err := m.driver.Insert(ctx, &m.schema.SchemaCore, document); err != nil {
		return err
	}
	if err := m.runPost(PostInsert, document); err != nil {
		return err
	}
	Emit(EventInsert, InsertPayload[T]{Schema: &m.schema.SchemaCore, Doc: document})
	return nil
}

func (m *Model[T]) FindOne(ctx context.Context, options *Query) (*T, error) {
	var z T
	_ = m.runPre(PreFind, &z)

	raw, err := m.driver.FindOne(ctx, &m.schema.SchemaCore, options)
	if err != nil || raw == nil {
		return nil, err
	}
	if v, ok := raw.(*T); ok {
		_ = m.runPost(PostFind, v)
		Emit(EventFind, FindOnePayload[T]{Schema: &m.schema.SchemaCore, Query: options, Doc: v})
		return v, nil
	}
	return nil, nil
}

func (m *Model[T]) FindMany(ctx context.Context, options *Query) ([]T, error) {
	var z T
	_ = m.runPre(PreFind, &z)

	raw, err := m.driver.FindMany(ctx, &m.schema.SchemaCore, options)
	if err != nil || raw == nil {
		return nil, err
	}
	if v, ok := raw.([]T); ok {
		for i := range v {
			_ = m.runPost(PostFind, &v[i])
		}
		Emit(EventFind, FindManyPayload[T]{Schema: &m.schema.SchemaCore, Query: options, Docs: v})
		return v, nil
	}
	return nil, nil
}

func (m *Model[T]) Update(ctx context.Context, condition *Condition, changes Changes) error {
	if err := m.driver.Update(ctx, &m.schema.SchemaCore, condition, changes); err != nil {
		return err
	}

	Emit(EventUpdate, UpdatePayload{Schema: &m.schema.SchemaCore, Condition: condition, Changes: changes})
	return nil
}

func (m *Model[T]) Delete(ctx context.Context, condition *Condition) error {
	if err := m.driver.Delete(ctx, &m.schema.SchemaCore, condition); err != nil {
		return err
	}
	Emit(EventDelete, DeletePayload{Schema: &m.schema.SchemaCore, Condition: condition})
	return nil
}

func (m *Model[T]) Count(ctx context.Context, condition *Condition) (int64, error) {
	return m.driver.Count(ctx, &m.schema.SchemaCore, condition)
}

//endregion

//region Event

type Event string

const (
	EventInsert Event = "insert"
	EventUpdate Event = "update"
	EventDelete Event = "delete"
	EventFind   Event = "find"
)

type EventHandler func(payload any)

type EventDispatcher struct {
	mu       sync.RWMutex
	handlers map[Event][]EventHandler
}

var globalDispatcher = &EventDispatcher{
	handlers: make(map[Event][]EventHandler),
}

func On(event Event, handler EventHandler) {
	globalDispatcher.mu.Lock()
	defer globalDispatcher.mu.Unlock()
	globalDispatcher.handlers[event] = append(globalDispatcher.handlers[event], handler)
}

func Emit(event Event, payload any) {
	globalDispatcher.mu.RLock()
	defer globalDispatcher.mu.RUnlock()
	if hs, ok := globalDispatcher.handlers[event]; ok {
		for _, h := range hs {
			go h(payload)
		}
	}
}

type InsertPayload[T any] struct {
	Schema *SchemaCore
	Doc    *T
}

type UpdatePayload struct {
	Schema    *SchemaCore
	Condition *Condition
	Changes   Changes
}

type DeletePayload struct {
	Schema    *SchemaCore
	Condition *Condition
}

type FindOnePayload[T any] struct {
	Schema *SchemaCore
	Query  *Query
	Doc    *T
}

type FindManyPayload[T any] struct {
	Schema *SchemaCore
	Query  *Query
	Docs   []T
}

//endregion
