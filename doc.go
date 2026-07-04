// Package golem is a type-safe, TypeORM-inspired ORM for Go, built with
// generics and field-pointer references instead of struct tags or code
// generation. Entities are declared with plain structs (see the entity
// package); every mapping — columns, keys, relations, hooks, query
// criteria — is expressed via pointers to struct fields (e.g. &t.Name),
// resolved by memory offset and checked by the Go compiler, never by
// reflection over string tags.
//
// This root package holds the pieces every adapter and every other golem
// subpackage depends on: DataSource/Conn (connection lifecycle), Dialect
// (the contract adapters implement), ColumnType (dialect-agnostic column
// type constructors), the sentinel errors, and the default Logger.
// Everything else — entity declaration, query building, joins, relations,
// preloading, aggregation, and the repository CRUD layer — lives in its
// own subpackage (entity, query, op, join, relation, repository), and a
// concrete database adapter lives under driver/ (e.g. driver/postgres).
package golem
