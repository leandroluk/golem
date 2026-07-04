package query

import "github.com/leandroluk/golem/op"

// AggMapping pairs a source field pointer (into T's zero) with a
// destination field pointer (into R's zero) for one GROUP BY column or
// aggregate expression.
type AggMapping struct {
	SourceFieldPtr any
	DestFieldPtr   any
}

// Aggregate is received by a repository.Aggregate criteria callback. R is
// an arbitrary plain struct (not an entity.Entity) whose fields receive
// GROUP BY columns and aggregate results — resolved by field-pointer
// offset, same mechanism as everywhere else in golem, no struct tags.
type Aggregate[T, R any] struct {
	groupBy     []AggMapping
	sums        []AggMapping
	avgs        []AggMapping
	counts      []AggMapping
	countAlls   []any // DestFieldPtr only — COUNT(*) has no source column
	conditions  []op.Condition
	having      []op.Condition
	orderBy     []op.Order
	limit       *int
	offset      *int
	withDeleted bool
}

// NewAggregate builds an empty Aggregate[T, R]. Used internally by
// repository.Aggregate.
func NewAggregate[T, R any]() *Aggregate[T, R] {
	return &Aggregate[T, R]{}
}

// GroupBy adds a GROUP BY column: sourceFieldPtr (a T field) becomes a
// grouping key, its value carried through into destFieldPtr (an R field).
func (a *Aggregate[T, R]) GroupBy(sourceFieldPtr any, destFieldPtr any) *Aggregate[T, R] {
	a.groupBy = append(a.groupBy, AggMapping{SourceFieldPtr: sourceFieldPtr, DestFieldPtr: destFieldPtr})
	return a
}

// Sum adds a SUM(sourceFieldPtr) expression, written into destFieldPtr.
func (a *Aggregate[T, R]) Sum(sourceFieldPtr any, destFieldPtr any) *Aggregate[T, R] {
	a.sums = append(a.sums, AggMapping{SourceFieldPtr: sourceFieldPtr, DestFieldPtr: destFieldPtr})
	return a
}

// Avg adds an AVG(sourceFieldPtr) expression, written into destFieldPtr.
func (a *Aggregate[T, R]) Avg(sourceFieldPtr any, destFieldPtr any) *Aggregate[T, R] {
	a.avgs = append(a.avgs, AggMapping{SourceFieldPtr: sourceFieldPtr, DestFieldPtr: destFieldPtr})
	return a
}

// Count adds a COUNT(sourceFieldPtr) expression (counts non-NULL values),
// written into destFieldPtr.
func (a *Aggregate[T, R]) Count(sourceFieldPtr any, destFieldPtr any) *Aggregate[T, R] {
	a.counts = append(a.counts, AggMapping{SourceFieldPtr: sourceFieldPtr, DestFieldPtr: destFieldPtr})
	return a
}

// CountAll adds a COUNT(*) expression, written into destFieldPtr.
func (a *Aggregate[T, R]) CountAll(destFieldPtr any) *Aggregate[T, R] {
	a.countAlls = append(a.countAlls, destFieldPtr)
	return a
}

// Where adds pre-aggregation filter conditions (AND semantics), resolved
// against T — filters rows before GROUP BY runs, same as FindMany's Where.
func (a *Aggregate[T, R]) Where(conditions ...op.Condition) *Aggregate[T, R] {
	a.conditions = append(a.conditions, conditions...)
	return a
}

// Having adds post-aggregation filter conditions (AND semantics), resolved
// against R — each condition's FieldPtr must be an R field previously
// registered via Sum/Avg/Count/CountAll (not a GroupBy field; HAVING
// filters on aggregate results).
func (a *Aggregate[T, R]) Having(conditions ...op.Condition) *Aggregate[T, R] {
	a.having = append(a.having, conditions...)
	return a
}

// OrderBy adds sort orders (accumulating, applied in call order), resolved
// against R — may reference either a GroupBy or an aggregate field.
func (a *Aggregate[T, R]) OrderBy(orders ...op.Order) *Aggregate[T, R] {
	a.orderBy = append(a.orderBy, orders...)
	return a
}

// Limit constrains the maximum number of grouped rows returned.
func (a *Aggregate[T, R]) Limit(n int) *Aggregate[T, R] {
	a.limit = &n
	return a
}

// Offset skips the first n grouped rows.
func (a *Aggregate[T, R]) Offset(n int) *Aggregate[T, R] {
	a.offset = &n
	return a
}

// WithDeleted disables the default soft-delete filter on T, if T has one.
func (a *Aggregate[T, R]) WithDeleted() *Aggregate[T, R] {
	a.withDeleted = true
	return a
}

// GroupByMappings returns every GroupBy source/dest field pointer pair
// accumulated so far.
func (a *Aggregate[T, R]) GroupByMappings() []AggMapping { return a.groupBy }

// SumMappings returns every Sum source/dest field pointer pair accumulated
// so far.
func (a *Aggregate[T, R]) SumMappings() []AggMapping { return a.sums }

// AvgMappings returns every Avg source/dest field pointer pair accumulated
// so far.
func (a *Aggregate[T, R]) AvgMappings() []AggMapping { return a.avgs }

// CountMappings returns every Count source/dest field pointer pair
// accumulated so far.
func (a *Aggregate[T, R]) CountMappings() []AggMapping { return a.counts }

// CountAllFields returns every CountAll dest field pointer accumulated so
// far.
func (a *Aggregate[T, R]) CountAllFields() []any { return a.countAlls }

// Conditions returns every pre-aggregation Where condition accumulated so
// far.
func (a *Aggregate[T, R]) Conditions() []op.Condition { return a.conditions }

// HavingConditions returns every post-aggregation Having condition
// accumulated so far.
func (a *Aggregate[T, R]) HavingConditions() []op.Condition { return a.having }

// OrderByFields returns every sort order accumulated so far.
func (a *Aggregate[T, R]) OrderByFields() []op.Order { return a.orderBy }

// GetLimit returns the requested row limit, or nil if unset.
func (a *Aggregate[T, R]) GetLimit() *int { return a.limit }

// GetOffset returns the requested row offset, or nil if unset.
func (a *Aggregate[T, R]) GetOffset() *int { return a.offset }

// IsWithDeleted reports whether WithDeleted was called.
func (a *Aggregate[T, R]) IsWithDeleted() bool { return a.withDeleted }
