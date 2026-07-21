// Package entity declares entity schema metadata — table/column/key/
// relation/hook mapping — from plain Go structs, with zero struct tags and
// zero code generation.
//
// entity.New builds an *Entity[T] by running a callback against a zero-
// value *T and a *Table: every Table/Column method takes a field pointer
// (e.g. &t.Name) and resolves it to the underlying struct field by memory
// offset (ResolveField), not by type or declaration order — this is the
// mechanism that lets two fields of the same Go type be told apart
// correctly, and lets golem avoid struct tags entirely.
//
// The resolved metadata is read back via Entity[T].Describe() (an
// EntityMeta value), which is what the query, repository, and driver
// packages actually consume — they never touch Entity[T]'s internals
// directly. FKRegistration/ForeignKeysReferencing is a package-level
// registry, populated as a side effect of declaring a ForeignKey, that
// lets a parent-side operation (Repository[T].Delete's cascade logic)
// discover which entities reference it — the declaration direction (child
// declares the FK pointing at its parent) is the opposite of the direction
// cascade needs to query in.
package entity
