// Package stmt defines AST nodes representing SQL statements and logical
// query predicates. It is an internal package used by repository and dialect
// adapters, not exposed to end users.
package stmt

import "database/sql/driver"

// Predicate represents a node in a WHERE condition tree.
type Predicate interface {
	isPredicate()
}

// Comparison represents a field-to-value or column-to-value comparison.
type Comparison struct {
	Column string
	Op     string // "eq", "gt", "gte", "lt", "lte", "like", "in", "is_null"
	Value  any    // driver.Value or []driver.Value for "in"
}

func (Comparison) isPredicate() { return }

// Logical represents a logical combination (AND, OR) of nested predicates.
type Logical struct {
	Op         string // "and", "or"
	Predicates []Predicate
}

func (Logical) isPredicate() { return }

// Not represents a logical negation of a nested predicate.
type Not struct {
	Predicate Predicate
}

func (Not) isPredicate() { return }

// AggregateComparison represents an aggregate-expression-to-value comparison
// (e.g. `SUM("amount")>$1`), used in HAVING clauses. Unlike Comparison, Column
// is the source column an aggregate function applies to, not a bare
// identifier — Func names which aggregate wraps it ("sum", "avg", "min",
// "max", "count", "count_all"; "count_all" ignores Column, always COUNT(*)).
type AggregateComparison struct {
	Func   string
	Column string
	Op     string // "eq", "gt", "gte", "lt", "lte"
	Value  any
}

func (AggregateComparison) isPredicate() { return }

// OrderElement represents a single column ordering clause.
type OrderElement struct {
	Column string
	Desc   bool
}

// OnCondition represents column = column equality in JOIN ON clause.
type OnCondition struct {
	LeftCol  string
	RightCol string
}

// Join represents a single JOIN clause in SELECT statement.
type Join struct {
	Type  string // "inner", "left", "right", "full"
	Table string
	On    []OnCondition
	Where Predicate
}

// Projection represents one SELECT-list expression in an aggregate query.
// Func "" means a bare column (a GROUP BY column being carried through to
// the result); any other value ("sum", "avg", "min", "max", "count",
// "count_all") wraps Column in that aggregate function ("count_all" ignores
// Column, always COUNT(*)). Alias is always present — it's both the SQL
// column alias and the row-map key the caller scans results by.
type Projection struct {
	Column string
	Func   string
	Alias  string
}

// LockClause represents a SELECT ... FOR {UPDATE|NO KEY UPDATE|SHARE|KEY
// SHARE} [NOWAIT|SKIP LOCKED] row-locking clause.
type LockClause struct {
	Strength string // "update", "no_key_update", "share", "key_share"
	Wait     string // "", "nowait", "skip_locked"
}

// Select represents a SELECT statement plan.
type Select struct {
	Table       string
	Columns     []string     // Empty means "*"
	Projections []Projection // When non-empty, used instead of Columns/Count (aggregate queries)
	Where       Predicate
	GroupBy     []string
	Having      Predicate
	OrderBy     []OrderElement
	Limit       *int
	Offset      *int
	Count       bool // If true, projects COUNT(*) instead of Columns
	Joins       []Join
	Lock        *LockClause // nil means no row locking requested

	// PrimaryKey lists the entity's primary key column names, in declared
	// order. Every dialect so far ignores this (LIMIT/OFFSET pagination
	// needs no ORDER BY to be valid SQL for them). driver/mssql needs it:
	// SQL Server's OFFSET/FETCH pagination is a hard syntax error without
	// an ORDER BY clause, so when a caller pages without specifying one,
	// that dialect injects ORDER BY on PrimaryKey instead of letting SQL
	// Server's own cryptic error surface. Only populated by
	// Repository[T].FindMany (the one caller that supports pagination
	// against a real entity table) — repository.Aggregate's own
	// stmt.Select construction leaves this empty, since an aggregate
	// query's result columns don't include the source entity's primary
	// key at all.
	PrimaryKey []string
}

// Delete represents a DELETE statement plan.
type Delete struct {
	Table string
	Where Predicate
}

// Insert represents an INSERT statement plan.
type Insert struct {
	Table   string
	Columns []string
	Values  []driver.Value

	// PrimaryKey lists the entity's primary key column names, in declared
	// order. Dialects with a single-round-trip RETURNING (Postgres) can
	// ignore this entirely. Dialects without one (MySQL) need it to build a
	// follow-up SELECT after the INSERT: for each PrimaryKey column already
	// present in Columns (composite/explicit PK), reuse that bound value;
	// for the one PrimaryKey column NOT in Columns (the common case — a
	// single auto-increment PK, omitted because Repository[T].Insert skips
	// zero-value fields), use the driver's last-insert-id equivalent.
	PrimaryKey []string
}

// UpdateClause represents a single SET column = value clause.
type UpdateClause struct {
	Column string
	Value  driver.Value
}

// Update represents an UPDATE statement plan.
type Update struct {
	Table string
	Sets  []UpdateClause
	Where Predicate

	// PrimaryKey lists the entity's primary key column names. Dialects with
	// a single-round-trip RETURNING (Postgres) can ignore this entirely.
	// Dialects without one (MySQL) need it because Where's matching rows
	// can no longer be re-selected by Where verbatim after Sets has been
	// applied, if Sets happens to modify a column Where also filters on
	// (e.g. `Set(&t.Category, "hardware")` after `Where(op.Eq(&t.Category,
	// "tools"))`) — re-running Where post-update would then match zero
	// rows. Dialects without RETURNING should instead capture PrimaryKey's
	// values (via a SELECT using Where) before running the UPDATE, then
	// read the updated rows back by primary key.
	PrimaryKey []string
}
