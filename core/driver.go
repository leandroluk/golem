// Package core provides the fundamental building blocks of the golem ORM.
// It defines abstractions for queries, models, schema handling, and drivers.
package core

import "context"

// Sort represents an ordering rule used in queries.
//
// FieldName specifies which column/field to sort by.
// Order determines the direction: 1 for ascending (ASC), -1 for descending (DESC).
type Sort struct {
	FieldName string
	Order     int // 1 = ASC, -1 = DESC
}

// Where encapsulates filtering and pagination options for queries.
//
// It contains:
//   - Condition: the root filter condition (composed of one or more *Condition).
//   - Limit: maximum number of results to return.
//   - Offset: number of rows to skip.
//   - Sort: list of Sort rules to apply.
//   - WithDeleted: whether to include soft-deleted rows.
//   - OnlyDeleted: whether to return only soft-deleted rows.
type Where struct {
	Condition   *Condition
	Limit       int
	Offset      int
	Sort        []Sort
	WithDeleted bool
	OnlyDeleted bool
}

// Changes represents a set of field updates, mapping column names to new values.
// It is typically used in Update operations.
type Changes map[string]any

// Transaction defines the contract for database transaction management.
//
// Implementations must provide atomic commit and rollback semantics.
type Transaction interface {
	// Commit finalizes the transaction and makes all changes permanent.
	Commit(ctx context.Context) error
	// Rollback reverts the transaction, discarding all changes.
	Rollback(ctx context.Context) error
}

// Driver defines the contract for database backends supported by the ORM.
//
// Each driver (e.g., PostgresDriver, MongoDriver) must implement this interface
// to handle basic CRUD operations, transactions, and connectivity.
type Driver interface {
	// Connect establishes a new connection or validates connectivity.
	Connect(ctx context.Context) error
	// Ping checks if the underlying database is reachable.
	Ping(ctx context.Context) error
	// Close terminates the connection and releases resources.
	Close(ctx context.Context) error

	// Transaction starts a new database transaction.
	Transaction(ctx context.Context) (Transaction, error)

	// Insert persists one or more documents/entities in the database.
	Insert(ctx context.Context, schema *SchemaCore, documents ...any) error
	// FindOne retrieves a single document/entity matching the given options.
	FindOne(ctx context.Context, schema *SchemaCore, options *Where) (any, error)
	// FindMany retrieves multiple documents/entities matching the given options.
	FindMany(ctx context.Context, schema *SchemaCore, options *Where) (any, error)
	// Update modifies existing documents/entities matching the condition.
	Update(ctx context.Context, schema *SchemaCore, condition *Condition, changes Changes) error
	// Delete removes documents/entities matching the condition.
	// If soft-delete is enabled, this may update a deletedAt column instead.
	Delete(ctx context.Context, schema *SchemaCore, condition *Condition) error
	// Count returns the number of documents/entities matching the condition.
	Count(ctx context.Context, schema *SchemaCore, condition *Condition) (int64, error)
}
