// Package golem is a type-safe, TypeORM-inspired ORM for Go, built with
// generics and field-pointer references instead of struct tags or code
// generation. Entities are declared with plain structs; every mapping —
// columns, keys, relations, hooks, query criteria — is expressed via
// pointers to struct fields (e.g. &t.Name), resolved by memory offset and
// checked by the Go compiler, never by reflection over string tags.
//
// This file is the ONLY public door into the module: everything below is a
// type alias or a var-aliased/wrapped function pointing at the real
// implementation living under internal/<concept>. All real logic,
// encapsulation, and test coverage live in internal/*; this file
// intentionally carries none of its own (same convention gonest.dev/gonest
// uses, see its own AD-004).
//
// The one deliberate exception: driver/* (driver/postgres, driver/mysql,
// driver/mssql, driver/sqlite, driver/oracle) are NOT re-exported here and
// stay their own public packages. Each carries a heavy, exclusive
// third-party dependency (pgx, go-sql-driver/mysql, go-mssqldb,
// modernc.org/sqlite, go-ora) — folding all 5 into this single file would
// force every consumer to compile all 5 in, even if they only use one.
// Import the driver package you actually need alongside this one.
package golem

import (
	"context"

	"github.com/leandroluk/golem/internal/core"
	"github.com/leandroluk/golem/internal/entity"
	"github.com/leandroluk/golem/internal/join"
	"github.com/leandroluk/golem/internal/op"
	"github.com/leandroluk/golem/internal/query"
	"github.com/leandroluk/golem/internal/relation"
	"github.com/leandroluk/golem/internal/repository"
)

// ---------------------------------------------------------------------------
// internal/core -- Conn/DataSource, Dialect, Parser, ColumnType, Logger, errors
// ---------------------------------------------------------------------------

type Conn = core.Conn
type Tx = core.Tx
type TxConn = core.TxConn
type Result = core.Result
type DataSource = core.DataSource
type Option = core.Option
type Dialect = core.Dialect
type Connector = core.Connector
type ColumnType = core.ColumnType
type Parser = core.Parser
type Logger = core.Logger
type LogLevel = core.LogLevel

const (
	LogLevelDebug = core.LogLevelDebug
	LogLevelInfo  = core.LogLevelInfo
	LogLevelWarn  = core.LogLevelWarn
	LogLevelError = core.LogLevelError
)

var (
	NewDataSource     = core.NewDataSource
	MustNewDataSource = core.MustNewDataSource
	GetDataSource     = core.GetDataSource
	NewTx             = core.NewTx
	WithConnector     = core.WithConnector
	DataSourceName    = core.DataSourceName
	CustomParser      = core.CustomParser
	DefaultParser     = core.DefaultParser
	DefaultLogger     = core.DefaultLogger

	BOOLEAN  = core.BOOLEAN
	SMALLINT = core.SMALLINT
	INTEGER  = core.INTEGER
	BIGINT   = core.BIGINT
	DECIMAL  = core.DECIMAL
	FLOAT    = core.FLOAT
	CHAR     = core.CHAR
	VARCHAR  = core.VARCHAR
	TEXT     = core.TEXT
	DATE     = core.DATE
	DATETIME = core.DATETIME
	TIME     = core.TIME
	BLOB     = core.BLOB
	UUID     = core.UUID
	JSON     = core.JSON

	ErrNotFound            = core.ErrNotFound
	ErrDuplicateKey        = core.ErrDuplicateKey
	ErrForeignKeyViolation = core.ErrForeignKeyViolation
	ErrDataSourceNotFound  = core.ErrDataSourceNotFound
)

// ---------------------------------------------------------------------------
// internal/entity -- schema declaration
// ---------------------------------------------------------------------------

type Entity[T any] = entity.Entity[T]
type Table = entity.Table
type Column = entity.Column
type ColumnMeta = entity.ColumnMeta
type ForeignKeyMeta = entity.ForeignKeyMeta
type IndexMeta = entity.IndexMeta
type EntityMeta = entity.EntityMeta
type Index = entity.Index
type FKRegistration = entity.FKRegistration
type HookBuilder[T any] = entity.HookBuilder[T]

var (
	ResolveField           = entity.ResolveField
	ForeignKeysReferencing = entity.ForeignKeysReferencing
)

// NewTable declares an entity's schema. fn resolves every field-pointer
// argument by memory offset against the same zero-value T instance.
func NewTable[T any](fn func(t *T, b *Table)) *Entity[T] { return entity.New(fn) }

// AddHook registers lifecycle hooks (BeforeCreate/AfterCreate/... ) against e.
func AddHook[T any](e *Entity[T]) *HookBuilder[T] { return entity.AddHook(e) }

// ---------------------------------------------------------------------------
// internal/repository -- CRUD layer
// ---------------------------------------------------------------------------

type Repository[T any] = repository.Repository[T]

// NewRepository builds a Repository[T] bound to conn and e.
func NewRepository[T any](conn Conn, e *Entity[T]) *Repository[T] {
	return repository.Get(conn, e)
}

// RunAggregate executes an aggregate query built by fn against r.
func RunAggregate[T, R any](ctx context.Context, r *Repository[T], fn func(t *T, res *R, a *Aggregate[T, R])) ([]R, error) {
	return repository.Aggregate(ctx, r, fn)
}

// Preload batch-loads a related entity J for items, grouped by foreign key.
func Preload[T, J any](ctx context.Context, r *Repository[T], items []T, target *Entity[J], criteria ...func(*J, *Query[J])) (map[any][]J, error) {
	return repository.Preload(ctx, r, items, target, criteria...)
}

// ---------------------------------------------------------------------------
// internal/relation -- foreign key options
// ---------------------------------------------------------------------------

type OnDeleteAction = relation.OnDeleteAction
type ForeignKeyOptions = relation.ForeignKeyOptions

const (
	OnDeleteCascade  = relation.OnDeleteCascade
	OnDeleteSetNull  = relation.OnDeleteSetNull
	OnDeleteRestrict = relation.OnDeleteRestrict
	OnDeleteDefault  = relation.OnDeleteDefault
	OnDeleteNoAction = relation.OnDeleteNoAction
)

var NewForeignKeyOptions = relation.NewForeignKeyOptions

// ---------------------------------------------------------------------------
// internal/op -- query condition/order builders
// ---------------------------------------------------------------------------

type Condition = op.Condition
type Order = op.Order

var (
	Eq   = op.Eq
	Gt   = op.Gt
	Gte  = op.Gte
	Lt   = op.Lt
	Lte  = op.Lte
	In   = op.In
	Like = op.Like
	Or   = op.Or
	Not  = op.Not
	Asc  = op.Asc
	Desc = op.Desc
)

// ---------------------------------------------------------------------------
// internal/query -- query/update/count/join builders
// ---------------------------------------------------------------------------

type AggMapping = query.AggMapping
type Aggregate[T, R any] = query.Aggregate[T, R]
type LockStrength = query.LockStrength
type LockWait = query.LockWait
type Query[T any] = query.Query[T]
type SetClause = query.SetClause
type Update[T any] = query.Update[T]
type Count[T any] = query.Count[T]
type JoinOn = query.JoinOn
type JoinData = query.JoinData
type Join[T any] = query.Join[T]

const (
	LockForUpdate      = query.LockForUpdate
	LockForNoKeyUpdate = query.LockForNoKeyUpdate
	LockForShare       = query.LockForShare
	LockForKeyShare    = query.LockForKeyShare

	LockWaitBlock      = query.LockWaitBlock
	LockWaitNoWait     = query.LockWaitNoWait
	LockWaitSkipLocked = query.LockWaitSkipLocked
)

func NewAggregate[T, R any]() *Aggregate[T, R] { return query.NewAggregate[T, R]() }
func NewQuery[T any]() *Query[T]               { return query.New[T]() }
func NewUpdate[T any]() *Update[T]             { return query.NewUpdate[T]() }
func NewCount[T any]() *Count[T]               { return query.NewCount[T]() }
func NewJoin[T any]() *Join[T]                 { return query.NewJoin[T]() }

// ---------------------------------------------------------------------------
// internal/join -- Inner/Left/Right/Full join registration
// ---------------------------------------------------------------------------

func JoinInner[T, J any](q *Query[T], target *Entity[J], fn func(j *J, q1 *Join[J])) {
	join.Inner(q, target, fn)
}

func JoinLeft[T, J any](q *Query[T], target *Entity[J], fn func(j *J, q1 *Join[J])) {
	join.Left(q, target, fn)
}

func JoinRight[T, J any](q *Query[T], target *Entity[J], fn func(j *J, q1 *Join[J])) {
	join.Right(q, target, fn)
}

func JoinFull[T, J any](q *Query[T], target *Entity[J], fn func(j *J, q1 *Join[J])) {
	join.Full(q, target, fn)
}
