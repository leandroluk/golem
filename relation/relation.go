// Package relation declares the option types used by entity.Table.ForeignKey
// to configure a relation's referential-action behavior.
//
// This is a deliberately small surface: OnDelete is the only option, because
// it's the only one with a real runtime meaning given golem's architecture.
// Everything TypeORM-style relations normally also offer was considered and
// cut (see .specs/project/STATE.md AD-032 for the full reasoning):
//
//   - Cascade / Persistence / OrphanedRowAction: these cascade writes onto
//     an in-memory *attached* related object (post.owner = newUser;
//     post.save()) — golem entities never carry a navigational collection
//     field for a related entity (AD-001/AD-024), so there's nothing to
//     cascade from.
//   - CreateForeignKeyConstraints / Deferrable: these describe DDL-time
//     constraint behavior, and golem never generates DDL (AD-012) — not now,
//     and (per AD-032's discussion) not planned to.
//   - Eager / Lazy: golem's eager-loading story is repository.Preload,
//     called explicitly (AD-028) — there's no automatic-loading hook for
//     these flags to plug into, and no plan to add one (a real Go-generics
//     wall, not a missing feature).
//   - OnUpdate: would control what happens to referencing rows when the
//     referenced row's primary key changes, mirroring OnDelete — but no
//     golem operation mutates a primary key value, so there's nothing to
//     trigger it from.
package relation

// OnDeleteAction controls what happens to rows referencing the deleted row
// through this ForeignKey. Repository[T].Delete applies it for real.
type OnDeleteAction string

const (
	// OnDeleteDefault means "do nothing at the golem level" — if the
	// database schema itself has a real ON DELETE constraint (created
	// outside golem, e.g. via Liquibase), that constraint's own behavior
	// applies; otherwise referencing rows are left untouched.
	OnDeleteDefault  OnDeleteAction = ""
	OnDeleteRestrict OnDeleteAction = "restrict"
	OnDeleteSetNull  OnDeleteAction = "set-null"
	OnDeleteCascade  OnDeleteAction = "cascade"
	OnDeleteNoAction OnDeleteAction = "no-action"
)

// ForeignKeyOptions is the options builder passed as entity.Table.
// ForeignKey's optional 3rd argument.
type ForeignKeyOptions struct {
	onDelete OnDeleteAction
}

// NewForeignKeyOptions builds a ForeignKeyOptions with OnDelete unset
// (OnDeleteDefault).
func NewForeignKeyOptions() *ForeignKeyOptions {
	return &ForeignKeyOptions{}
}

// OnDelete sets what happens to referencing rows when the row this
// ForeignKey points at is deleted. Real runtime behavior — see
// Repository[T].Delete.
func (o *ForeignKeyOptions) OnDelete(a OnDeleteAction) *ForeignKeyOptions {
	o.onDelete = a
	return o
}

// ResolvedOnDelete returns the configured OnDelete action.
func (o *ForeignKeyOptions) ResolvedOnDelete() OnDeleteAction { return o.onDelete }
