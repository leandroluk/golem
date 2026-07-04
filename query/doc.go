// Package query provides the criteria accumulator types passed into
// Repository[T] read/write methods' callbacks — Query[T] (FindMany/
// FindOne), Update[T] (Update), Count[T] (Count/Exists), Join[T] (joins,
// via the join package), and Aggregate[T, R] (repository.Aggregate).
//
// None of these build SQL themselves, and none of them resolve field
// pointers to column names — they're pure data accumulators (Where/Set/
// GroupBy/... just append to an internal slice). Field-pointer resolution
// and SQL generation both happen one layer up, in repository, which is the
// only package with both the entity metadata (via the entity package) and
// the active golem.Dialect needed to do either. This keeps query itself
// free of any dependency on entity or a specific dialect — it only ever
// deals in field pointers (any) and op.Condition/op.Order values.
//
// Every type here is built via its New*/NewUpdate/NewCount/NewJoin/
// NewAggregate constructor by repository (and, for joins, the join
// package) and handed to the caller's criteria callback — end users only
// ever receive one as a callback parameter, never construct one directly.
package query
