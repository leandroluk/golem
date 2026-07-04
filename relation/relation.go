// Package relation declares the option types used by entity.Table.ForeignKey
// to configure a relation's referential-action and persistence behavior.
//
// golem never generates DDL (migrations are always an external concern, see
// STATE.md AD-012) and entities never carry a navigational collection field
// for a related entity (STATE.md AD-001/AD-024 — plain structs only). Given
// that, only a subset of these options change runtime behavior:
//
//   - OnDelete / OnUpdate: real. Repository[T].Delete consults the global FK
//     registry (entity.ForeignKeysReferencing) and applies the configured
//     action against every entity whose ForeignKey points at the one being
//     deleted.
//   - Cascade* / Persistence / OrphanedRowAction: accepted and stored on the
//     entity's metadata, but have no runtime effect — they exist in TypeORM
//     to cascade writes onto an in-memory *attached* related object
//     (post.owner = newUser; post.save()), which has no equivalent here
//     since there is no attached-object field to cascade from.
//   - CreateForeignKeyConstraints / Deferrable: accepted and stored, but
//     informational only — they describe DDL-time constraint behavior, and
//     golem never creates DDL.
//   - Eager: accepted and stored; wired up to actually run automatic
//     preloading once M12 (Preload/Eager Loading) lands. Lazy is the
//     default absent Eager and has no separate runtime meaning of its own.
package relation

// CascadeOption is one of the granular cascade-persistence flags. Accepted
// and stored on ForeignKeyOptions for documentation/metadata purposes only
// (see package doc) — no Repository[T] method currently reads it.
type CascadeOption string

const (
	CascadeInsert     CascadeOption = "insert"      // cascade an insert of the related object
	CascadeUpdate     CascadeOption = "update"      // cascade an update of the related object
	CascadeRemove     CascadeOption = "remove"      // cascade a hard delete of the related object
	CascadeSoftRemove CascadeOption = "soft-remove" // cascade a soft delete of the related object
	CascadeRecover    CascadeOption = "recover"     // cascade a soft-delete restore of the related object
	CascadeAll        CascadeOption = "all"         // shorthand for all 5 flags above at once
)

// OnDeleteAction controls what happens to rows referencing the deleted row
// through this ForeignKey. Real runtime behavior — see Repository[T].Delete.
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

// OnUpdateAction mirrors OnDeleteAction for changes to the referenced row's
// primary key value. Accepted and stored; not yet wired to a Repository[T]
// method (primary keys are effectively immutable in every shipped example —
// see .specs/features/relations/design.md).
type OnUpdateAction string

const (
	// OnUpdateDefault means "do nothing at the golem level" — same
	// reasoning as OnDeleteDefault, mirrored for the update side.
	OnUpdateDefault  OnUpdateAction = ""
	OnUpdateRestrict OnUpdateAction = "restrict"
	OnUpdateSetNull  OnUpdateAction = "set-null"
	OnUpdateCascade  OnUpdateAction = "cascade"
	OnUpdateNoAction OnUpdateAction = "no-action"
)

// DeferrableMode describes DB constraint-timing behavior. Accepted and
// stored, informational only — golem never creates the DB constraint this
// would apply to (see package doc).
type DeferrableMode string

const (
	DeferrableDefault   DeferrableMode = ""          // no opinion — whatever the DB constraint (if any) defaults to
	DeferrableDeferred  DeferrableMode = "deferred"  // check at transaction commit, not immediately
	DeferrableImmediate DeferrableMode = "immediate" // check immediately (the usual DB default)
)

// OrphanedRowActionMode describes what happens to a previously related child
// row that's no longer present when a parent's in-memory collection is
// re-saved. Accepted and stored, no runtime effect — golem has no
// navigational collection field to detect "no longer present" from (see
// package doc).
type OrphanedRowActionMode string

const (
	OrphanedRowActionNullify    OrphanedRowActionMode = "nullify"     // clear the relation's FK column
	OrphanedRowActionDelete     OrphanedRowActionMode = "delete"      // hard-delete the orphaned row
	OrphanedRowActionSoftDelete OrphanedRowActionMode = "soft-delete" // soft-delete the orphaned row
	OrphanedRowActionDisable    OrphanedRowActionMode = "disable"     // leave the orphaned row untouched
)

// ForeignKeyOptions is the fluent options builder passed as entity.Table.
// ForeignKey's optional 3rd argument. Matches README.md's documented chain.
type ForeignKeyOptions struct {
	cascade                     map[CascadeOption]bool
	onDelete                    OnDeleteAction
	onUpdate                    OnUpdateAction
	deferrable                  DeferrableMode
	createForeignKeyConstraints bool
	lazy                        bool
	eager                       bool
	persistence                 bool
	orphanedRowAction           OrphanedRowActionMode
}

// NewForeignKeyOptions builds a ForeignKeyOptions with the same defaults
// README.md documents: CreateForeignKeyConstraints and Persistence both
// default to true, everything else defaults to its zero/"do nothing" value.
func NewForeignKeyOptions() *ForeignKeyOptions {
	return &ForeignKeyOptions{
		createForeignKeyConstraints: true,
		persistence:                 true,
	}
}

// Cascade marks one or more cascade-persistence flags. Cascade(CascadeAll)
// sets every flag; calling Cascade multiple times accumulates (does not
// replace) the flag set.
func (o *ForeignKeyOptions) Cascade(opts ...CascadeOption) *ForeignKeyOptions {
	if o.cascade == nil {
		o.cascade = make(map[CascadeOption]bool, len(opts))
	}
	for _, opt := range opts {
		if opt == CascadeAll {
			o.cascade[CascadeInsert] = true
			o.cascade[CascadeUpdate] = true
			o.cascade[CascadeRemove] = true
			o.cascade[CascadeSoftRemove] = true
			o.cascade[CascadeRecover] = true
			continue
		}
		o.cascade[opt] = true
	}
	return o
}

// OnDelete sets what happens to referencing rows when the row this
// ForeignKey points at is deleted. Real runtime behavior — see
// Repository[T].Delete.
func (o *ForeignKeyOptions) OnDelete(a OnDeleteAction) *ForeignKeyOptions {
	o.onDelete = a
	return o
}

// OnUpdate sets what happens to referencing rows when the row this
// ForeignKey points at has its primary key changed. Accepted and stored;
// not yet wired to any Repository[T] method (see package doc).
func (o *ForeignKeyOptions) OnUpdate(a OnUpdateAction) *ForeignKeyOptions {
	o.onUpdate = a
	return o
}

// Deferrable sets the DB constraint-timing hint. Accepted and stored,
// informational only (see package doc).
func (o *ForeignKeyOptions) Deferrable(m DeferrableMode) *ForeignKeyOptions {
	o.deferrable = m
	return o
}

// CreateForeignKeyConstraints sets whether a DB-level FK constraint would
// be created for this relation. Accepted and stored, informational only —
// golem never creates DDL (see package doc).
func (o *ForeignKeyOptions) CreateForeignKeyConstraints(b bool) *ForeignKeyOptions {
	o.createForeignKeyConstraints = b
	return o
}

// Lazy marks the relation as lazily loaded. Accepted and stored, no
// separate runtime meaning of its own — it's the default absent Eager (see
// package doc).
func (o *ForeignKeyOptions) Lazy(b bool) *ForeignKeyOptions {
	o.lazy = b
	return o
}

// Eager marks the relation for automatic loading. Accepted and stored;
// wiring it to actually run repository.Preload automatically is M12+
// territory, not done in this pass (see package doc).
func (o *ForeignKeyOptions) Eager(b bool) *ForeignKeyOptions {
	o.eager = b
	return o
}

// Persistence sets whether writes to this relation are reflected in the
// database. Accepted and stored, no runtime effect (see package doc).
func (o *ForeignKeyOptions) Persistence(b bool) *ForeignKeyOptions {
	o.persistence = b
	return o
}

// OrphanedRowAction sets what happens to a previously related row that's
// no longer present when a parent's in-memory collection is re-saved.
// Accepted and stored, no runtime effect (see package doc).
func (o *ForeignKeyOptions) OrphanedRowAction(m OrphanedRowActionMode) *ForeignKeyOptions {
	o.orphanedRowAction = m
	return o
}

// HasCascade reports whether opt (a single granular flag, not CascadeAll)
// was set, directly or via Cascade(CascadeAll).
func (o *ForeignKeyOptions) HasCascade(opt CascadeOption) bool {
	return o.cascade[opt]
}

// ResolvedOnDelete returns the configured OnDelete action.
func (o *ForeignKeyOptions) ResolvedOnDelete() OnDeleteAction { return o.onDelete }

// ResolvedOnUpdate returns the configured OnUpdate action.
func (o *ForeignKeyOptions) ResolvedOnUpdate() OnUpdateAction { return o.onUpdate }

// ResolvedDeferrable returns the configured Deferrable mode.
func (o *ForeignKeyOptions) ResolvedDeferrable() DeferrableMode { return o.deferrable }

// ResolvedCreateForeignKeyConstraints returns the configured
// CreateForeignKeyConstraints flag.
func (o *ForeignKeyOptions) ResolvedCreateForeignKeyConstraints() bool {
	return o.createForeignKeyConstraints
}

// ResolvedLazy returns the configured Lazy flag.
func (o *ForeignKeyOptions) ResolvedLazy() bool { return o.lazy }

// ResolvedEager returns the configured Eager flag.
func (o *ForeignKeyOptions) ResolvedEager() bool { return o.eager }

// ResolvedPersistence returns the configured Persistence flag.
func (o *ForeignKeyOptions) ResolvedPersistence() bool { return o.persistence }

// ResolvedOrphanedRowAction returns the configured OrphanedRowAction mode.
func (o *ForeignKeyOptions) ResolvedOrphanedRowAction() OrphanedRowActionMode {
	return o.orphanedRowAction
}
