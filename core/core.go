package core

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

//
// ~=~=~= CLIENT ~=~=~=
//

type Client[T any] struct {
	driver  Driver[T]
	schemas []*Schema
	Dialect Dialect
}

func NewClient[T any](driver Driver[T]) *Client[T] {
	return &Client[T]{driver: driver, Dialect: driver.Dialect()}
}

func (c *Client[T]) Connect(ctx context.Context) error {
	return c.driver.Connect(ctx)
}
func (c *Client[T]) Close() error {
	return c.driver.Close()
}
func (c *Client[T]) Ping(ctx context.Context) error {
	return c.driver.Ping(ctx)
}
func (c *Client[T]) Schemas(schemas ...*Schema) *Client[T] {
	c.schemas = append(c.schemas, schemas...)
	return c
}
func (c *Client[T]) Transaction(ctx context.Context, fn func(tx Tx[T]) error) error {
	tx, err := c.driver.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
func (c *Client[T]) Exec(ctx context.Context, stmt T, args ...any) (Result, error) {
	return c.driver.Exec(ctx, stmt, args...)
}
func (c *Client[T]) Query(ctx context.Context, stmt T, args ...any) (Rows, error) {
	return c.driver.Query(ctx, stmt, args...)
}

//
// ~=~=~= DRIVER ~=~=~=
//

type Tx[T any] interface {
	Exec(ctx context.Context, stmt T, args ...any) (Result, error)
	Query(ctx context.Context, stmt T, args ...any) (Rows, error)
	Commit() error
	Rollback() error
}

type Dialect interface {
	ToValue(field *FieldMeta, v any) any
	FromValue(f *FieldMeta, raw any) (any, error)
}

type Driver[T any] interface {
	Connect(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error
	Exec(ctx context.Context, stmt T, args ...any) (Result, error)
	Query(ctx context.Context, stmt T, args ...any) (Rows, error)
	Begin(ctx context.Context) (Tx[T], error)
	Dialect() Dialect
}

type Result interface {
	RowsAffected() (int64, error)
}

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
	Columns() []string
}

//
// ~=~=~= QUERY ~=~=~=
//

type OperatorToken string

const (
	EqO      OperatorToken = "op.eq"
	NeqO     OperatorToken = "op.neq"
	LikeO    OperatorToken = "op.like"
	NLikeO   OperatorToken = "op.nlike"
	GtO      OperatorToken = "op.gt"
	NGtO     OperatorToken = "op.ngt"
	GteO     OperatorToken = "op.gte"
	NGteO    OperatorToken = "op.ngte"
	LtO      OperatorToken = "op.lt"
	NLtO     OperatorToken = "op.nlt"
	LteO     OperatorToken = "op.lte"
	NLteO    OperatorToken = "op.nlte"
	InO      OperatorToken = "op.in"
	NinO     OperatorToken = "op.nin"
	NullO    OperatorToken = "op.null"
	NotNullO OperatorToken = "op.notnull"
	LimitO   OperatorToken = "op.limit"
	SkipO    OperatorToken = "op.skip"
	SetO     OperatorToken = "op.set"
	AndO     OperatorToken = "op.and"
	OrO      OperatorToken = "op.or"
	OrderO   OperatorToken = "op.order"
	JoinO    OperatorToken = "op.join"
)

type Condition struct {
	Field any
	Op    OperatorToken
	Args  []any
}

type Query struct {
	Conds   []Condition
	schema  *Schema
	addrIdx map[uintptr]*FieldMeta
}

func NewQuery(s *Schema, root any) *Query {
	q := &Query{schema: s}
	if root != nil {
		q.addrIdx = s.AddrIndex(root)
	}
	return q
}

func (q *Query) normalizeField(field any) any {
	// 1) ponteiro de campo &root.Campo
	if q.addrIdx != nil {
		if rv := reflect.ValueOf(field); rv.Kind() == reflect.Pointer {
			if fm, ok := q.addrIdx[rv.Pointer()]; ok {
				return fm.Index
			}
		}
	}
	// 2) string → coluna, depois nome Go
	if s, ok := field.(string); ok && q.schema != nil {
		if fm := q.schema.FieldByColumn(s); fm != nil {
			return fm.Index
		}
		if fm := q.schema.FieldByName(s); fm != nil {
			return fm.Index
		}
		return s // literal/expr
	}
	// 3) []int já normalizado
	return field
}

// básicos
func (q *Query) Eq(field any, value any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: EqO, Args: []any{value}})
	return q
}
func (q *Query) Neq(field any, value any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: NeqO, Args: []any{value}})
	return q
}
func (q *Query) Like(field any, pat string) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: LikeO, Args: []any{pat}})
	return q
}
func (q *Query) NotLike(field any, pat string) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: NLikeO, Args: []any{pat}})
	return q
}

// comparadores
func (q *Query) Gt(field any, v any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: GtO, Args: []any{v}})
	return q
}
func (q *Query) NotGt(field any, v any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: NGtO, Args: []any{v}})
	return q
}
func (q *Query) Gte(field any, v any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: GteO, Args: []any{v}})
	return q
}
func (q *Query) NotGte(field any, v any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: NGteO, Args: []any{v}})
	return q
}
func (q *Query) Lt(field any, v any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: LtO, Args: []any{v}})
	return q
}
func (q *Query) NotLt(field any, v any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: NLtO, Args: []any{v}})
	return q
}
func (q *Query) Lte(field any, v any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: LteO, Args: []any{v}})
	return q
}
func (q *Query) NotLte(field any, v any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: NLteO, Args: []any{v}})
	return q
}

// conjuntos
func (q *Query) In(field any, values ...any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: InO, Args: values})
	return q
}
func (q *Query) NotIn(field any, values ...any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: NinO, Args: values})
	return q
}

// nulls
func (q *Query) IsNull(field any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: NullO})
	return q
}
func (q *Query) IsNotNull(field any) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: NotNullO})
	return q
}

// paginação
func (q *Query) Limit(n int) *Query {
	q.Conds = append(q.Conds, Condition{Op: LimitO, Args: []any{n}})
	return q
}
func (q *Query) Skip(n int) *Query {
	q.Conds = append(q.Conds, Condition{Op: SkipO, Args: []any{n}})
	return q
}

// lógicos
func (q *Query) And(subs ...*Query) *Query {
	q.Conds = append(q.Conds, Condition{Op: AndO, Args: []any{subs}})
	return q
}
func (q *Query) Or(subs ...*Query) *Query {
	q.Conds = append(q.Conds, Condition{Op: OrO, Args: []any{subs}})
	return q
}

// ordenação
type OrderDirection string

const (
	Asc  OrderDirection = "ASC"
	Desc OrderDirection = "DESC"
)

func (q *Query) OrderBy(field any, dir OrderDirection) *Query {
	q.Conds = append(q.Conds, Condition{Field: q.normalizeField(field), Op: OrderO, Args: []any{dir}})
	return q
}

// joins
type JoinType string

const (
	Inner JoinType = "INNER"
	Left  JoinType = "LEFT"
	Right JoinType = "RIGHT"
)

func (q *Query) Join(joinType JoinType, schema *Schema, on func(left, right *Schema, cond *Query)) *Query {
	jq := &Query{}
	on(nil, schema, jq)
	q.Conds = append(q.Conds, Condition{Op: JoinO, Args: []any{joinType, schema, jq}})
	return q
}

//
// ================== UPDATE ==================
//

type Update struct {
	Sets    []Condition
	schema  *Schema
	addrIdx map[uintptr]*FieldMeta
}

func NewUpdate(s *Schema, root any) *Update {
	u := &Update{schema: s}
	if root != nil {
		u.addrIdx = s.AddrIndex(root)
	}
	return u
}

func (u *Update) normalizeField(field any) any {
	if u.addrIdx != nil {
		if rv := reflect.ValueOf(field); rv.Kind() == reflect.Pointer {
			if fm, ok := u.addrIdx[rv.Pointer()]; ok {
				return fm.Index
			}
		}
	}
	if s, ok := field.(string); ok && u.schema != nil {
		if fm := u.schema.FieldByColumn(s); fm != nil {
			return fm.Index
		}
		if fm := u.schema.FieldByName(s); fm != nil {
			return fm.Index
		}
		return s
	}
	return field
}

func (u *Update) Set(field any, value any) *Update {
	u.Sets = append(u.Sets, Condition{Field: u.normalizeField(field), Op: SetO, Args: []any{value}})
	return u
}

//
// ================== RESULT DECORATORS ==================
//

type JsonResult struct{}

func AsJson(rows Rows) ([]json.RawMessage, error) {
	defer rows.Close()

	cols := rows.Columns()
	n := len(cols)
	if n == 0 {
		return nil, nil
	}

	var result []json.RawMessage
	for rows.Next() {
		values := make([]any, n)
		dest := make([]any, n)
		for i := range dest {
			dest[i] = &values[i]
		}

		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}

		// monta objeto coluna->valor; normaliza []byte -> string
		obj := make(map[string]any, n)
		for i, name := range cols {
			v := values[i]
			if b, ok := v.([]byte); ok {
				obj[name] = string(b)
			} else {
				obj[name] = v
			}
		}

		b, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, nil
}

type TypeResult[T any] struct{}

func AsType[T any](rows Rows) ([]T, error) {
	defer rows.Close()

	out := []T{}
	rt := reflect.TypeOf((*T)(nil)).Elem()

	for rows.Next() {
		var t T
		v := reflect.ValueOf(&t).Elem()

		dest := make([]any, rt.NumField())
		for i := 0; i < rt.NumField(); i++ {
			fv := v.Field(i)
			if fv.CanAddr() {
				dest[i] = fv.Addr().Interface()
			} else {
				var dummy any
				dest[i] = &dummy
			}
		}

		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

//
// ================== SCHEMA ==================
//

type FieldToken string

const (
	UndefO     FieldToken = "core.undef"
	DefaultO   FieldToken = "core.default"
	NullableO  FieldToken = "core.nullable"
	UniqueO    FieldToken = "core.unique"
	GetterO    FieldToken = "core.getter"
	SetterO    FieldToken = "core.setter"
	EnumO      FieldToken = "core.enum"
	ReferenceO FieldToken = "core.reference"
	PrimaryO   FieldToken = "core.primary"
	CreatedAtO FieldToken = "core.created_at"
	UpdatedAtO FieldToken = "core.updated_at"
	DeletedAtO FieldToken = "core.deleted_at"
	HookO      FieldToken = "core.hook"
)

type FieldOption struct {
	Token FieldToken
	Value any
}

func f(t FieldToken, v any) FieldOption { return FieldOption{Token: t, Value: v} }

func Default(value any) FieldOption            { return f(DefaultO, value) }
func Nullable() FieldOption                    { return f(NullableO, true) }
func Unique() FieldOption                      { return f(UniqueO, true) }
func Getter[T any](fn func(v T) T) FieldOption { return f(GetterO, fn) }
func Setter[T any](fn func(v T) T) FieldOption { return f(SetterO, fn) }
func Enum(values ...any) FieldOption           { return f(EnumO, values) }
func Reference[T any]() FieldOption            { return f(ReferenceO, new(T)) }
func Primary() FieldOption                     { return f(PrimaryO, true) }
func CreatedAt() FieldOption                   { return f(CreatedAtO, true) }
func UpdatedAt() FieldOption                   { return f(UpdatedAtO, true) }
func DeletedAt() FieldOption                   { return f(DeletedAtO, true) }

type FieldMeta struct {
	Name   string
	Column string
	Index  []int
	GoType reflect.Type
	Meta   map[FieldToken]any
}

type Schema struct {
	Name       string
	SchemaName string
	TableName  string
	TagName    string
	Fields     []*FieldMeta

	// índices rápidos
	byName   map[string]*FieldMeta
	byColumn map[string]*FieldMeta
	byIndex  map[string]*FieldMeta

	_root reflect.Value
	Hooks map[string]func(any) error
}

func NewSchema[T any](builder func(*T, *Schema) *Schema) *Schema {
	var instance T
	rt := reflect.TypeOf((*T)(nil)).Elem()
	s := &Schema{
		Name:     rt.Name(),
		TagName:  "db",
		_root:    reflect.ValueOf(&instance).Elem(),
		Hooks:    make(map[string]func(any) error),
		byName:   make(map[string]*FieldMeta),
		byColumn: make(map[string]*FieldMeta),
		byIndex:  make(map[string]*FieldMeta),
	}
	return builder(&instance, s)
}

func (s *Schema) WithSchema(schema string) *Schema {
	s.SchemaName = schema
	return s
}
func (s *Schema) WithTable(table string) *Schema {
	s.TableName = table
	return s
}
func (s *Schema) WithTag(tag string) *Schema {
	s.TagName = tag
	return s
}
func (s *Schema) FindPrimaryKey() *FieldMeta {
	for _, f := range s.Fields {
		if v, ok := f.Meta[PrimaryO]; ok {
			if b, ok2 := v.(bool); ok2 && b {
				return f
			}
		}
	}
	return nil
}
func (s *Schema) FieldByName(name string) *FieldMeta {
	return s.byName[name]
}
func (s *Schema) FieldByColumn(col string) *FieldMeta {
	return s.byColumn[col]
}
func (s *Schema) FieldByIndex(index []int) *FieldMeta {
	return s.byIndex[KeyIndex(index)]
}
func (s *Schema) AddrIndex(root any) map[uintptr]*FieldMeta {
	rv := reflect.ValueOf(root)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}
	out := make(map[uintptr]*FieldMeta, len(s.Fields))
	for _, fm := range s.Fields {
		fv := rv.FieldByIndex(fm.Index)
		if fv.CanAddr() {
			out[fv.Addr().Pointer()] = fm
		}
	}
	return out
}
func (s *Schema) Field(fieldPtr any, opts ...FieldOption) *Schema {
	name, column, index := s.ResolveField(fieldPtr)
	meta := make(map[FieldToken]any, len(opts)+1)
	for _, o := range opts {
		meta[o.Token] = o.Value
	}

	rt := reflect.TypeOf(fieldPtr).Elem()
	meta[UndefO] = reflect.Zero(rt).Interface()

	fm := &FieldMeta{
		Name:   name,
		Column: column,
		Index:  index,
		GoType: rt,
		Meta:   meta,
	}
	s.Fields = append(s.Fields, fm)

	s.byName[name] = fm
	if column != "" {
		s.byColumn[column] = fm
	}
	s.byIndex[KeyIndex(index)] = fm
	return s
}
func (s *Schema) Hook(name string, fn func(any) error) *Schema {
	s.Hooks[name] = fn
	return s
}
func (s *Schema) ResolveField(fieldPtr any) (name string, column string, index []int) {
	pv := reflect.ValueOf(fieldPtr)
	if pv.Kind() != reflect.Pointer {
		return "field", "field", nil
	}
	target := pv.Pointer()
	root := s._root
	rt := root.Type()

	for i := 0; i < root.NumField(); i++ {
		fv := root.Field(i)
		if fv.CanAddr() && fv.Addr().Pointer() == target {
			sf := rt.Field(i)
			name = sf.Name
			tag := sf.Tag.Get(s.TagName)
			if tag != "" && tag != "-" {
				tag = strings.Split(tag, ",")[0]
			} else {
				tag = strings.ToLower(name)
			}
			return name, tag, sf.Index
		}
	}
	return "", "", nil
}

func KeyIndex(idx []int) string {
	if len(idx) == 0 {
		return ""
	}
	var b strings.Builder
	for i, n := range idx {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(n))
	}
	return b.String()
}

//
// ================== CACHE ==================
//

type FieldCache struct {
	Idx    int
	Column string
	Getter func(any) any
	Setter func(any) any
}

var fieldCacheMap sync.Map // map[reflect.Type][]FieldCache

func BuildFieldCache(s *Schema, t reflect.Type) []FieldCache {
	if v, ok := fieldCacheMap.Load(t); ok {
		return v.([]FieldCache)
	}
	out := []FieldCache{}
	for i, f := range s.Fields {
		fm := FieldCache{Idx: i, Column: f.Column}
		if fn, ok := f.Meta[GetterO]; ok {
			if g, ok2 := fn.(func(any) any); ok2 {
				fm.Getter = g
			}
		}
		if fn, ok := f.Meta[SetterO]; ok {
			if st, ok2 := fn.(func(any) any); ok2 {
				fm.Setter = st
			}
		}
		out = append(out, fm)
	}
	fieldCacheMap.Store(t, out)
	return out
}
