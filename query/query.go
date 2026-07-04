package query

import "github.com/leandroluk/golem/op"

// Query is received by a FindMany/FindOne criteria callback.
// Built internally by repository — never constructed directly by end users.
type Query[T any] struct {
	conditions   []op.Condition
	selectFields []any
	orderBy      []op.Order
	limit        *int
	offset       *int
	withDeleted  bool
	joins        []JoinData
	lockStrength LockStrength
	lockWait     LockWait
}

// New builds an empty Query[T]. Used internally by repository.
func New[T any]() *Query[T] {
	return &Query[T]{}
}

// Where adds conditions (AND semantics), accumulating across multiple calls.
func (q *Query[T]) Where(conditions ...op.Condition) *Query[T] {
	q.conditions = append(q.conditions, conditions...)
	return q
}

// Select specifies which struct fields should be projected in the query.
func (q *Query[T]) Select(fieldPtrs ...any) *Query[T] {
	q.selectFields = append(q.selectFields, fieldPtrs...)
	return q
}

// OrderBy adds field sorting descriptors.
func (q *Query[T]) OrderBy(orders ...op.Order) *Query[T] {
	q.orderBy = append(q.orderBy, orders...)
	return q
}

// Limit constrains the maximum number of rows returned.
func (q *Query[T]) Limit(limit int) *Query[T] {
	q.limit = &limit
	return q
}

// Offset skips the first offset rows.
func (q *Query[T]) Offset(offset int) *Query[T] {
	q.offset = &offset
	return q
}

// WithDeleted disables the default soft-delete filter (WHERE deleted_at IS NULL).
func (q *Query[T]) WithDeleted() *Query[T] {
	q.withDeleted = true
	return q
}

// Conditions returns every condition accumulated so far.
func (q *Query[T]) Conditions() []op.Condition {
	return q.conditions
}

// SelectFields returns the select projection field pointers.
func (q *Query[T]) SelectFields() []any {
	return q.selectFields
}

// OrderByFields returns the sorting descriptors.
func (q *Query[T]) OrderByFields() []op.Order {
	return q.orderBy
}

// GetLimit returns the limit value, or nil if not set.
func (q *Query[T]) GetLimit() *int {
	return q.limit
}

// GetOffset returns the offset value, or nil if not set.
func (q *Query[T]) GetOffset() *int {
	return q.offset
}

// IsWithDeleted returns true if soft-deletes should be ignored.
func (q *Query[T]) IsWithDeleted() bool {
	return q.withDeleted
}

// SetClause represents a single "set this column to this value" clause.
type SetClause struct {
	FieldPtr any
	Value    any
}

// Update is received by an Update criteria callback.
type Update[T any] struct {
	conditions  []op.Condition
	sets        []SetClause
	withDeleted bool
}

// NewUpdate builds an empty Update[T]. Used internally by repository.
func NewUpdate[T any]() *Update[T] {
	return &Update[T]{}
}

// Where adds conditions (AND semantics), accumulating across multiple calls.
func (u *Update[T]) Where(conditions ...op.Condition) *Update[T] {
	u.conditions = append(u.conditions, conditions...)
	return u
}

// Set adds a set clause, accumulating across multiple calls.
func (u *Update[T]) Set(fieldPtr any, value any) *Update[T] {
	u.sets = append(u.sets, SetClause{FieldPtr: fieldPtr, Value: value})
	return u
}

// WithDeleted disables the default soft-delete filter.
func (u *Update[T]) WithDeleted() *Update[T] {
	u.withDeleted = true
	return u
}

// Conditions returns every condition accumulated so far.
func (u *Update[T]) Conditions() []op.Condition {
	return u.conditions
}

// Sets returns every set clause accumulated so far.
func (u *Update[T]) Sets() []SetClause {
	return u.sets
}

// IsWithDeleted returns true if soft-deletes should be ignored.
func (u *Update[T]) IsWithDeleted() bool {
	return u.withDeleted
}

// Count is received by a Count criteria callback.
type Count[T any] struct {
	conditions  []op.Condition
	withDeleted bool
}

// NewCount builds an empty Count[T]. Used internally by repository.
func NewCount[T any]() *Count[T] {
	return &Count[T]{}
}

// Where adds conditions (AND semantics), accumulating across multiple calls.
func (c *Count[T]) Where(conditions ...op.Condition) *Count[T] {
	c.conditions = append(c.conditions, conditions...)
	return c
}

// WithDeleted disables the default soft-delete filter (WHERE deleted_at IS NULL).
func (c *Count[T]) WithDeleted() *Count[T] {
	c.withDeleted = true
	return c
}

// Conditions returns every condition accumulated so far.
func (c *Count[T]) Conditions() []op.Condition {
	return c.conditions
}

// IsWithDeleted returns true if soft-deletes should be ignored.
func (c *Count[T]) IsWithDeleted() bool {
	return c.withDeleted
}

// JoinOn represents column-to-column equality in join criteria.
type JoinOn struct {
	LeftField  any
	RightField any
}

// JoinData holds resolved join metadata to avoid circular imports.
type JoinData struct {
	Type            string
	TableName       string
	Zero            any
	OnFieldPairs    []JoinOn
	Conditions      []op.Condition
	WithDeleted     bool
	DeleteDateField string
	FieldToColumn   map[string]string
}

// Join is received by a join criteria callback.
type Join[T any] struct {
	ons         []JoinOn
	conditions  []op.Condition
	withDeleted bool
}

// NewJoin creates a new Join builder.
func NewJoin[T any]() *Join[T] {
	return &Join[T]{}
}

// On defines a column = column mapping for the join ON clause.
func (j *Join[T]) On(leftField any, rightField any) *Join[T] {
	j.ons = append(j.ons, JoinOn{LeftField: leftField, RightField: rightField})
	return j
}

// Where defines column = value filters for the joined entity query.
func (j *Join[T]) Where(conditions ...op.Condition) *Join[T] {
	j.conditions = append(j.conditions, conditions...)
	return j
}

// WithDeleted bypasses the soft-delete filter on the joined entity query.
func (j *Join[T]) WithDeleted() *Join[T] {
	j.withDeleted = true
	return j
}

// Ons returns every column-to-column ON pair accumulated so far.
func (j *Join[T]) Ons() []JoinOn {
	return j.ons
}

// Conditions returns every column-to-value filter accumulated so far.
func (j *Join[T]) Conditions() []op.Condition {
	return j.conditions
}

// IsWithDeleted reports whether WithDeleted was called.
func (j *Join[T]) IsWithDeleted() bool {
	return j.withDeleted
}

// AddJoinData registers a resolved join descriptor on the parent query.
// Called by the join package (Inner/Left/Right/Full), not directly by end
// users.
func (q *Query[T]) AddJoinData(jd JoinData) {
	q.joins = append(q.joins, jd)
}

// Joins returns every join descriptor registered so far.
func (q *Query[T]) Joins() []JoinData {
	return q.joins
}
