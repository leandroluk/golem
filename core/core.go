package core

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
	"unsafe"
)

//region Condition

type Condition struct {
	FieldName string
	Operator  *Operator
	Value     any
	Children  []*Condition
}

func (c *Condition) And(conditions ...*Condition) *Condition {
	return &Condition{
		Operator: &OpAnd,
		Children: append([]*Condition{c}, conditions...),
	}
}

func (c *Condition) Or(conditions ...*Condition) *Condition {
	return &Condition{
		Operator: &OpOr,
		Children: append([]*Condition{c}, conditions...),
	}
}

func (c *Condition) Not() *Condition {
	return &Condition{
		Operator: &OpNot,
		Children: []*Condition{c},
	}
}

func (c *Condition) Nil() *Condition {
	c.Operator = &OpNil
	c.Value = nil
	return c
}

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

//region Driver

type Sort struct {
	FieldName string
	Order     int // 1 = ASC, -1 = DESC
}

type Where struct {
	Condition   *Condition
	Limit       int
	Offset      int
	Sort        []Sort
	WithDeleted bool
	OnlyDeleted bool
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
	FindOne(ctx context.Context, schema *SchemaCore, options *Where) (any, error)
	FindMany(ctx context.Context, schema *SchemaCore, options *Where) (any, error)
	Update(ctx context.Context, schema *SchemaCore, condition *Condition, changes Changes) error
	Delete(ctx context.Context, schema *SchemaCore, condition *Condition) error
	Count(ctx context.Context, schema *SchemaCore, condition *Condition) (int64, error)
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
	mutex       sync.RWMutex
	handlerList map[Event][]EventHandler
}

var globalDispatcher = &EventDispatcher{
	handlerList: make(map[Event][]EventHandler),
}

func On(event Event, handler EventHandler) {
	globalDispatcher.mutex.Lock()
	defer globalDispatcher.mutex.Unlock()
	globalDispatcher.handlerList[event] = append(globalDispatcher.handlerList[event], handler)
}

func Emit(event Event, payload any) {
	globalDispatcher.mutex.RLock()
	defer globalDispatcher.mutex.RUnlock()
	if hs, ok := globalDispatcher.handlerList[event]; ok {
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
	Where  *Where
	Doc    *T
}

type FindManyPayload[T any] struct {
	Schema  *SchemaCore
	Where   *Where
	DocList []T
}

//endregion

//region helpers

func offsetOf[T any, F any](selector func(*T) *F) uintptr {
	var zero T
	base := uintptr(unsafe.Pointer(&zero))
	ptr := selector(&zero)
	return uintptr(unsafe.Pointer(ptr)) - base
}

func fieldNameFromSelectorFor[T any](selector any) string {
	if selector == nil {
		return ""
	}
	selectorValue := reflect.ValueOf(selector)
	if selectorValue.Kind() != reflect.Func {
		panic("selector must be a function")
	}

	// cria *T
	var zero T
	typ := reflect.TypeOf(zero)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	arg := reflect.New(typ) // *T

	// executa o selector e obtém o retorno
	out := selectorValue.Call([]reflect.Value{arg})
	if len(out) == 0 {
		panic("selector must return a pointer to a field")
	}
	ret := out[0]
	if ret.Kind() != reflect.Pointer {
		panic("selector must return a pointer to a field")
	}

	// calcula o offset do ponteiro retornado relativo à base de *T
	basePtr := arg.Pointer()
	fieldPtr := ret.Pointer()
	offset := uintptr(fieldPtr - basePtr)

	// encontra o campo cujo offset bate
	for _, sf := range reflect.VisibleFields(typ) {
		if sf.Offset == offset {
			return sf.Name // Go struct field name
		}
	}
	return "???"
}

func mapToStruct[T any](row map[string]any, out *T) error {
	value := reflect.ValueOf(out).Elem()
	for rowKey, rowValue := range row {
		field := value.FieldByNameFunc(func(name string) bool { return strings.EqualFold(name, rowKey) })
		if !field.IsValid() || !field.CanSet() {
			continue
		}

		if rowValue == nil {
			// Se campo é ponteiro, setar nil; caso contrário, ignorar
			if field.Kind() == reflect.Pointer {
				field.Set(reflect.Zero(field.Type()))
			}
			continue
		}

		rv := reflect.ValueOf(rowValue)

		// 1) tipo exatamente compatível
		if rv.Type().AssignableTo(field.Type()) {
			field.Set(rv)
			continue
		}

		// 2) valor → ponteiro (ex.: time.Time → *time.Time)
		if field.Kind() == reflect.Pointer && rv.Type().AssignableTo(field.Type().Elem()) {
			ptr := reflect.New(field.Type().Elem())
			ptr.Elem().Set(rv)
			field.Set(ptr)
			continue
		}

		// 3) ponteiro → valor (ex.: *time.Time → time.Time)
		if rv.Kind() == reflect.Pointer && !rv.IsNil() && rv.Type().Elem().AssignableTo(field.Type()) {
			field.Set(rv.Elem())
			continue
		}

		// 4) conversões
		if rv.Type().ConvertibleTo(field.Type()) {
			field.Set(rv.Convert(field.Type()))
			continue
		}
		if field.Kind() == reflect.Pointer && rv.Type().ConvertibleTo(field.Type().Elem()) {
			ptr := reflect.New(field.Type().Elem())
			ptr.Elem().Set(rv.Convert(field.Type().Elem()))
			field.Set(ptr)
			continue
		}
	}
	return nil
}

func foldConditionsAnd(conds ...*Condition) *Condition {
	switch len(conds) {
	case 0:
		return nil
	case 1:
		return conds[0]
	default:
		acc := conds[0]
		for i := 1; i < len(conds); i++ {
			acc = acc.And(conds[i])
		}
		return acc
	}
}

func StructValues(schema *SchemaCore, doc any) ([]any, []string) {
	value := reflect.ValueOf(doc)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	valueList := []any{}
	placeholderList := []string{}

	for index, field := range schema.Fields {
		fv := value.FieldByName(field.GoStructFieldName)

		var v any
		if fv.Kind() == reflect.Pointer {
			if fv.IsNil() {
				v = nil
			} else {
				v = fv.Elem().Interface()
			}
		} else {
			v = fv.Interface()
		}

		valueList = append(valueList, v)
		placeholderList = append(placeholderList, fmt.Sprintf("$%d", index+1))
	}

	return valueList, placeholderList
}

func Include[L any, F any](selector func(*L) *F) string {
	return fieldNameFromSelectorFor[L](selector)
}

func setTimeField(field reflect.Value, t time.Time) {
	if !field.IsValid() || !field.CanSet() {
		return
	}
	timeType := reflect.TypeOf(time.Time{})

	switch field.Kind() {
	case reflect.Struct:
		if field.Type() == timeType {
			field.Set(reflect.ValueOf(t))
		}
	case reflect.Pointer:
		if field.Type().Elem() == timeType {
			if field.IsNil() {
				ptr := reflect.New(timeType)
				ptr.Elem().Set(reflect.ValueOf(t))
				field.Set(ptr)
			} else {
				field.Elem().Set(reflect.ValueOf(t))
			}
		}
	}
}

//endregion

//region Hook's

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

//region Model - struct and common

type Model[T any] struct {
	schema *SchemaMeta[T]
	driver Driver
}

func NewModel[T any](schema *SchemaMeta[T], driver Driver) *Model[T] {
	return &Model[T]{schema: schema, driver: driver}
}

func (m *Model[T]) LoadRelation(ctx context.Context, doc *T, fieldPtrs ...any) error {
	value := reflect.ValueOf(doc).Elem()

	for _, ptr := range fieldPtrs {
		rv := reflect.ValueOf(ptr)
		if rv.Kind() != reflect.Pointer {
			return fmt.Errorf("LoadRelation: argumento deve ser ponteiro para campo")
		}

		// descobre o nome do campo em T que corresponde a esse ponteiro
		fieldName := ""
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			if field.Addr().Interface() == ptr {
				fieldName = value.Type().Field(i).Name
				break
			}
		}
		if fieldName == "" {
			return fmt.Errorf("LoadRelation: campo não encontrado para ponteiro %v", ptr)
		}

		if err := m.loadRelationList(ctx, doc, []string{fieldName}); err != nil {
			return err
		}
	}
	return nil
}

func (m *Model[T]) withSoftDelete(where *Where) *Where {
	if where == nil || m.schema.deletedAtField == nil {
		return where
	}
	eff := *where // shallow copy
	col := m.schema.deletedAtField.DatabaseColumnName

	if where.OnlyDeleted {
		eff.Condition = foldConditionsAnd(
			where.Condition,
			(&Condition{FieldName: col}).Nil().Not(),
		)
		return &eff
	}
	if !where.WithDeleted {
		eff.Condition = foldConditionsAnd(
			where.Condition,
			(&Condition{FieldName: col}).Nil(),
		)
	}
	return &eff
}

func (m *Model[T]) loadRelationList(ctx context.Context, doc *T, nameList []string) error {
	value := reflect.ValueOf(doc).Elem()

	for _, relationName := range nameList {
		relation := m.schema.findRelation(relationName)
		if relation == nil {
			continue
		}

		field := value.FieldByName(relation.FieldName)
		if !field.IsValid() || !field.CanSet() {
			continue
		}

		switch relation.Kind {
		case OneToOne:
			localVal := value.FieldByName(relation.LocalKey).Interface()
			qb := NewQuery(relation.RefSchema)
			qb.Filter(func(_ Filter[any]) []*Condition {
				return []*Condition{
					(&Condition{FieldName: relation.ForeignKey}).Eq(localVal),
				}
			})
			model := NewModel(relation.RefSchema, m.driver)
			result, err := model.findOneInternal(ctx, qb)
			if err != nil {
				return err
			}
			if result != nil {
				field.Set(reflect.ValueOf(result))
			}

		case OneToMany:
			localVal := value.FieldByName(relation.LocalKey).Interface()
			qb := NewQuery(relation.RefSchema)
			qb.Filter(func(_ Filter[any]) []*Condition {
				return []*Condition{
					(&Condition{FieldName: relation.ForeignKey}).Eq(localVal),
				}
			})
			model := NewModel(relation.RefSchema, m.driver)
			results, err := model.findManyInternal(ctx, qb)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(results))

		case ManyToMany:
			localVal := value.FieldByName(relation.LocalKey).Interface()

			// 1) pega linhas na join table onde JoinLocalKey = localVal
			joinQuery := &Where{
				Condition: (&Condition{FieldName: relation.JoinLocalKey}).Eq(localVal),
			}
			rawJoin, err := m.driver.FindMany(ctx, &SchemaCore{Collection: relation.JoinTable}, joinQuery)
			if err != nil {
				return err
			}
			joinRows, _ := rawJoin.([]map[string]any)

			// 2) extrai os IDs remotos
			foreignIDs := make([]any, 0, len(joinRows))
			for _, jr := range joinRows {
				foreignIDs = append(foreignIDs, jr[relation.JoinForeignKey])
			}

			// 3) busca entidades remotas por IN
			if len(foreignIDs) > 0 {
				qb := NewQuery(relation.RefSchema)
				qb.Filter(func(_ Filter[any]) []*Condition {
					return []*Condition{
						(&Condition{FieldName: relation.ForeignKey}).In(foreignIDs...),
					}
				})
				model := NewModel(relation.RefSchema, m.driver)
				results, err := model.findManyInternal(ctx, qb)
				if err != nil {
					return err
				}
				field.Set(reflect.ValueOf(results))
			}
		}
	}

	return nil
}

//endregion

//region Model - hook's runners

func (m *Model[T]) runPre(hook PreHook, doc *T) error {
	if fnList, ok := m.schema.PreHookList[hook]; ok {
		for _, fn := range fnList {
			if err := fn(doc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Model[T]) runPost(hook PostHook, doc *T) error {
	if fnList, ok := m.schema.PostHookList[hook]; ok {
		for _, fn := range fnList {
			if err := fn(doc); err != nil {
				return err
			}
		}
	}
	return nil
}

//endregion

//region Model - Create

func (m *Model[T]) Create(ctx context.Context, doc *T) error {
	now := time.Now()
	val := reflect.ValueOf(doc).Elem()

	if m.schema.createdAtField != nil {
		f := val.FieldByName(m.schema.createdAtField.GoStructFieldName)
		setTimeField(f, now)
	}
	if m.schema.updatedAtField != nil {
		f := val.FieldByName(m.schema.updatedAtField.GoStructFieldName)
		setTimeField(f, now)
	}

	if err := m.runPre(PreInsert, doc); err != nil {
		return err
	}
	if err := m.driver.Insert(ctx, &m.schema.SchemaCore, doc); err != nil {
		return err
	}
	if err := m.runPost(PostInsert, doc); err != nil {
		return err
	}
	Emit(EventInsert, InsertPayload[T]{Schema: &m.schema.SchemaCore, Doc: doc})
	return nil
}

//endregion

//region Model - FindOne

type FindOneQuery[T any] struct {
	model           *Model[T]
	query           *Query[T]
	includeNameList []string
}

func (m *Model[T]) FindOne(ctx context.Context, query *Query[T]) *FindOneQuery[T] {
	return &FindOneQuery[T]{model: m, query: query}
}

// selector deve retornar ponteiro para o campo (convertido para any).
// Ex.: .Include(func(u *User) any { return &u.RoleList })
func (q *FindOneQuery[T]) Include(selector func(*T) any) *FindOneQuery[T] {
	q.includeNameList = append(q.includeNameList, fieldNameFromSelectorFor[T](selector))
	return q
}

func (q *FindOneQuery[T]) Run(ctx context.Context) (*T, error) {
	return q.model.findOneInternal(ctx, q.query, q.includeNameList...)
}

func (m *Model[T]) findOneInternal(ctx context.Context, qb *Query[T], relationList ...string) (*T, error) {
	var zero T
	_ = m.runPre(PreFind, &zero)

	// cria cópia com soft delete aplicado
	where := m.withSoftDelete(qb.where)

	raw, err := m.driver.FindOne(ctx, &m.schema.SchemaCore, where)
	if err != nil || raw == nil {
		return nil, err
	}

	row, ok := raw.(map[string]any)
	if !ok {
		return nil, nil
	}

	value := new(T)
	if err := mapToStruct(row, value); err != nil {
		return nil, err
	}

	if err := m.loadRelationList(ctx, value, relationList); err != nil {
		return nil, err
	}

	_ = m.runPost(PostFind, value)
	Emit(EventFind, FindOnePayload[T]{Schema: &m.schema.SchemaCore, Where: where, Doc: value})
	return value, nil
}

//endregion

//region Model - FindMany

type FindManyQuery[T any] struct {
	model        *Model[T]
	qb           *Query[T]
	includeNames []string
}

func (m *Model[T]) FindMany(ctx context.Context, qb *Query[T]) *FindManyQuery[T] {
	return &FindManyQuery[T]{model: m, qb: qb}
}

func (q *FindManyQuery[T]) Include(selector func(*T) any) *FindManyQuery[T] {
	q.includeNames = append(q.includeNames, fieldNameFromSelectorFor[T](selector))
	return q
}

func (q *FindManyQuery[T]) Run(ctx context.Context) ([]T, error) {
	return q.model.findManyInternal(ctx, q.qb, q.includeNames...)
}

func (m *Model[T]) findManyInternal(ctx context.Context, qb *Query[T], relationList ...string) ([]T, error) {
	var zero T
	_ = m.runPre(PreFind, &zero)

	// cria cópia com soft delete aplicado
	where := m.withSoftDelete(qb.where)

	raw, err := m.driver.FindMany(ctx, &m.schema.SchemaCore, where)
	if err != nil || raw == nil {
		return nil, err
	}

	rows, ok := raw.([]map[string]any)
	if !ok {
		return nil, nil
	}

	var results []T
	for _, row := range rows {
		value := new(T)
		if err := mapToStruct(row, value); err != nil {
			return nil, err
		}
		if err := m.loadRelationList(ctx, value, relationList); err != nil {
			return nil, err
		}
		_ = m.runPost(PostFind, value)
		results = append(results, *value)
	}

	Emit(EventFind, FindManyPayload[T]{Schema: &m.schema.SchemaCore, Where: where, DocList: results})
	return results, nil
}

//endregion

//region Model - Update

func (m *Model[T]) Update(ctx context.Context, condition *Condition, changes Changes) error {
	if m.schema.updatedAtField != nil {
		changes[m.schema.updatedAtField.DatabaseColumnName] = time.Now()
	}

	if err := m.driver.Update(ctx, &m.schema.SchemaCore, condition, changes); err != nil {
		return err
	}
	Emit(EventUpdate, UpdatePayload{Schema: &m.schema.SchemaCore, Condition: condition, Changes: changes})
	return nil
}

//endregion

//region Model - Delete

func (m *Model[T]) Delete(ctx context.Context, condition *Condition) error {
	// soft delete se houver campo configurado
	if m.schema.deletedAtField != nil {
		changes := Changes{m.schema.deletedAtField.DatabaseColumnName: time.Now()}
		if err := m.driver.Update(ctx, &m.schema.SchemaCore, condition, changes); err != nil {
			return err
		}
		Emit(EventUpdate, UpdatePayload{Schema: &m.schema.SchemaCore, Condition: condition, Changes: changes})
		return nil
	}

	// fallback: delete real
	if err := m.driver.Delete(ctx, &m.schema.SchemaCore, condition); err != nil {
		return err
	}
	Emit(EventDelete, DeletePayload{Schema: &m.schema.SchemaCore, Condition: condition})
	return nil
}

//endregion

//region Model - Count

func (m *Model[T]) Count(ctx context.Context, qb *Query[T]) (int64, error) {
	// usa cópia com soft delete aplicado
	where := m.withSoftDelete(qb.where)
	return m.driver.Count(ctx, &m.schema.SchemaCore, where.Condition)
}

//endregion

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

//region Query

type Query[T any] struct {
	schema *SchemaMeta[T]
	where  *Where
}

func NewQuery[T any](schema *SchemaMeta[T]) *Query[T] {
	return &Query[T]{
		schema: schema,
		where:  &Where{},
	}
}

func (q *Query[T]) WithDeleted() *Query[T] {
	q.where.WithDeleted = true
	return q
}

func (q *Query[T]) OnlyDeleted() *Query[T] {
	q.where.OnlyDeleted = true
	return q
}

// Where type-safe no builder.
// Agora aceita tanto ponteiro direto para campo (&User.Name)
// quanto selector (func(*User) *string).
func (q *Query[T]) Where(fieldPtr any) *Condition {
	var goFieldName string

	switch f := fieldPtr.(type) {
	case func(*T) any:
		goFieldName = fieldNameFromSelectorFor[T](f)
	default:
		// detectar ponteiro para campo direto
		rt := reflect.TypeOf((*T)(nil)).Elem()
		rv := reflect.ValueOf(fieldPtr)
		if rv.Kind() != reflect.Ptr {
			panic("Where: argumento deve ser ponteiro para campo ou selector func(*T) *F")
		}
		// percorre campos do struct procurando offset
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

	// mapear para o nome da coluna/tags do banco
	dbCol := goFieldName
	for _, f := range q.schema.Fields {
		if f.GoStructFieldName == goFieldName {
			dbCol = f.DatabaseColumnName
			break
		}
	}

	return &Condition{FieldName: dbCol}
}

// Filter “funcional”: recebe um escopo (Filter[T]) e retorna uma lista de condições,
// que serão combinadas com AND por padrão.
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

// Escopo passado para a função do Filter. Expõe Where type-safe.
type Filter[T any] struct{ queryBuilder *Query[T] }

func (f Filter[T]) Where(fieldPtr any) *Condition {
	return f.queryBuilder.Where(fieldPtr)
}

func (q *Query[T]) OrderBy(field string, order int) *Query[T] {
	q.where.Sort = append(q.where.Sort, Sort{FieldName: field, Order: order})
	return q
}

func (q *Query[T]) Limit(limit int) *Query[T] {
	q.where.Limit = limit
	return q
}

func (q *Query[T]) Offset(offset int) *Query[T] {
	q.where.Offset = offset
	return q
}

//endregion

//region Schema - Field

type Field struct {
	GoStructFieldName  string
	DatabaseColumnName string
	Type               reflect.Type
	IsPrimaryKey       bool
	IsUnique           bool
	IsRequired         bool
	DefaultValue       string
	MemoryOffset       uintptr

	IsCreatedAt bool
	IsUpdatedAt bool
	IsDeletedAt bool
}

type FieldOption func(*Field)

func PrimaryKey() FieldOption {
	return func(f *Field) { f.IsPrimaryKey = true }
}

func Unique() FieldOption {
	return func(f *Field) { f.IsUnique = true }
}

func Required() FieldOption {
	return func(f *Field) { f.IsRequired = true }
}

func Default(value string) FieldOption {
	return func(f *Field) { f.DefaultValue = value }
}

func CreatedAt() FieldOption {
	return func(f *Field) { f.IsCreatedAt = true }
}

func UpdatedAt() FieldOption {
	return func(f *Field) { f.IsUpdatedAt = true }
}

func DeletedAt() FieldOption {
	return func(f *Field) { f.IsDeletedAt = true }
}

//endregion

//region Schema - Core and Meta

type SchemaCore struct {
	Database       string
	Collection     string
	Fields         []*Field
	fieldsByOffset map[uintptr]*Field
}

type RelationKind int

const (
	OneToOne   RelationKind = 1
	OneToMany  RelationKind = 2
	ManyToMany RelationKind = 3
)

type Relation[L any, F any, J any] struct {
	Kind           RelationKind
	Field          any            // func(*L) *<FieldType em L> (ex: *[]Role)
	RefSchema      *SchemaMeta[F] // schema da entidade remota
	LocalKey       any            // func(*L) *<KeyType em L>
	ForeignKey     any            // func(*F) *<KeyType em F>
	JoinTable      string         // nome da tabela/coleção pivot (ManyToMany)
	JoinLocalKey   any            // func(*J) *<KeyType em J> (ManyToMany)
	JoinForeignKey any            // func(*J) *<KeyType em J> (ManyToMany)
}

// Forma interna normalizada usada em runtime pelo Model.
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

type SchemaMeta[T any] struct {
	SchemaCore
	PreHookList  map[PreHook][]func(*T) error
	PostHookList map[PostHook][]func(*T) error
	RelationList []RelationInternal

	createdAtField *Field
	updatedAtField *Field
	deletedAtField *Field
}

func (s *SchemaMeta[T]) RegisterPreHook(hook PreHook, fn func(*T) error) {
	s.PreHookList[hook] = append(s.PreHookList[hook], fn)
}

func (s *SchemaMeta[T]) RegisterPostHook(hook PostHook, fn func(*T) error) {
	s.PostHookList[hook] = append(s.PostHookList[hook], fn)
}

// AddRelation resolve selectors → nomes de campo e registra na lista interna.
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

func (s *SchemaMeta[T]) findRelation(name string) *RelationInternal {
	for _, relation := range s.RelationList {
		if relation.FieldName == name {
			return &relation
		}
	}
	return nil
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

func TagKey[T any](key string) SchemaOption[T] {
	return func(schemaBuilder *SchemaBuilder[T]) { schemaBuilder.tagKey = key }
}

func Table[T any](name string) SchemaOption[T] {
	return func(schemaBuilder *SchemaBuilder[T]) { schemaBuilder.collection = name }
}

func Database[T any](name string) SchemaOption[T] {
	return func(schemaBuilder *SchemaBuilder[T]) { schemaBuilder.database = name }
}

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

	// aplica opções do builder (Table/Database/TagKey/OverrideField) — atenção:
	// OverrideField só terá efeito após criarmos os fields (abaixo),
	// então chamaremos as opções novamente depois de popular 'fieldsByOffset'.
	for _, option := range options {
		option(builder)
	}

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
			GoStructFieldName:  sf.Name,
			DatabaseColumnName: dbName,
			Type:               sf.Type,
			MemoryOffset:       sf.Offset,
		}
		builder.fields = append(builder.fields, field)
		builder.fieldsByOffset[sf.Offset] = field
	}

	// re-aplica opções para que OverrideField funcione (agora que os fields existem)
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

	// detectar campos especiais (CreatedAt/UpdatedAt/DeletedAt) uma única vez
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
