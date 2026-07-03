package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
	"github.com/leandroluk/golem/internal/stmt"
	"github.com/leandroluk/golem/relation"
)

// -----------------------------------------------------------------------
// Cascade test entities. Each test uses a distinct table name (the FK
// registry, entity.ForeignKeysReferencing, is a package-level global) so
// registrations from one test never leak into another's assertions.
// -----------------------------------------------------------------------

type cascadeParent struct {
	ID int64
}

type cascadeChild struct {
	ID       int64
	ParentID int64
}

func newCascadeParentEntity(table string) *entity.Entity[cascadeParent] {
	return entity.New(func(p *cascadeParent, b *entity.Table) {
		b.TableName(table)
		b.Col(&p.ID, golem.BIGINT())
		b.PrimaryKey(&p.ID)
	})
}

func newCascadeChildEntity(table string, parent *entity.Entity[cascadeParent], opts *relation.ForeignKeyOptions) *entity.Entity[cascadeChild] {
	return entity.New(func(c *cascadeChild, b *entity.Table) {
		b.TableName(table)
		b.Col(&c.ID, golem.BIGINT())
		b.Col(&c.ParentID, golem.BIGINT()).Name("parent_id")
		b.PrimaryKey(&c.ID)
		if opts != nil {
			b.ForeignKey(&c.ParentID, parent, opts)
		} else {
			b.ForeignKey(&c.ParentID, parent)
		}
	})
}

func TestRepository_Delete_OnDeleteCascade_DeletesChildRows(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_delete")
	newCascadeChildEntity("cascade_child_delete", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 5}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if d.beginCalls != 1 {
		t.Errorf("beginCalls = %d, want 1 (cascade-actionable FK must open a transaction)", d.beginCalls)
	}
	if len(d.deleteCalls) != 2 {
		t.Fatalf("expected 2 CompileDelete calls (child cascade + parent itself), got %d", len(d.deleteCalls))
	}
	childCall := d.deleteCalls[0]
	if childCall.Table != "cascade_child_delete" {
		t.Errorf("first delete table = %q, want cascade_child_delete", childCall.Table)
	}
	childComp, ok := childCall.Where.(stmt.Comparison)
	if !ok || childComp.Column != "parent_id" || childComp.Op != "eq" || childComp.Value != int64(5) {
		t.Errorf("unexpected cascade delete Where: %+v", childCall.Where)
	}
	parentCall := d.deleteCalls[1]
	if parentCall.Table != "cascade_parent_delete" {
		t.Errorf("second delete table = %q, want cascade_parent_delete", parentCall.Table)
	}
}

func TestRepository_Delete_OnDeleteSetNull_NullsChildColumn(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_setnull")
	newCascadeChildEntity("cascade_child_setnull", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteSetNull))

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 9}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(d.updateCalls) != 1 {
		t.Fatalf("expected 1 Update call (SET NULL on child), got %d", len(d.updateCalls))
	}
	call := d.updateCalls[0]
	if call.Table != "cascade_child_setnull" {
		t.Errorf("update table = %q, want cascade_child_setnull", call.Table)
	}
	if len(call.Sets) != 1 || call.Sets[0].Column != "parent_id" || call.Sets[0].Value != nil {
		t.Errorf("expected Set parent_id = nil, got %+v", call.Sets)
	}
	if len(d.deleteCalls) != 1 {
		t.Fatalf("expected 1 CompileDelete call (parent itself only), got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_OnDeleteRestrict_BlocksWhenChildrenExist(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_restrict_block")
	newCascadeChildEntity("cascade_child_restrict_block", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))

	d := &fakeDialect{
		selectResult: []map[string]any{{"count": int64(3)}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 1})
	if !errors.Is(err, golem.ErrForeignKeyViolation) {
		t.Fatalf("Delete: expected errors.Is(err, golem.ErrForeignKeyViolation), got %v", err)
	}
	if len(d.deleteCalls) != 0 {
		t.Errorf("expected 0 CompileDelete calls (blocked before parent delete), got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_OnDeleteRestrict_AllowsWhenNoChildren(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_restrict_allow")
	newCascadeChildEntity("cascade_child_restrict_allow", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))

	d := &fakeDialect{
		selectResult: []map[string]any{{"count": int64(0)}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 2}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(d.deleteCalls) != 1 {
		t.Errorf("expected 1 CompileDelete call (parent itself), got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_OnDeleteDefault_NoCascadeSideEffects(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_default")
	newCascadeChildEntity("cascade_child_default", parent, nil) // default options: OnDelete unset

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 3}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if d.beginCalls != 0 {
		t.Errorf("beginCalls = %d, want 0 (OnDeleteDefault is not cascade-actionable, no transaction should open)", d.beginCalls)
	}
	if len(d.deleteCalls) != 1 {
		t.Errorf("expected 1 CompileDelete call (parent itself only, no cascade), got %d", len(d.deleteCalls))
	}
	if len(d.updateCalls) != 0 {
		t.Errorf("expected 0 Update calls, got %d", len(d.updateCalls))
	}
}

func TestRepository_Delete_NoIncomingForeignKeys_DoesNotOpenTransaction(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_no_fk")

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 4}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if d.beginCalls != 0 {
		t.Errorf("beginCalls = %d, want 0 (no entity references this table)", d.beginCalls)
	}
}

func TestRepository_Delete_AlreadyInsideTx_ReusesIt(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_already_tx")
	newCascadeChildEntity("cascade_child_already_tx", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	d := &fakeDialect{}
	ds := newFakeConn(t, d)

	txConn, err := d.Begin(context.Background(), ds)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	d.beginCalls = 0 // reset: only count Begin calls made *by Delete itself* below
	tx := golem.NewTx(d, txConn)

	repo := Get(tx, parent)
	if err := repo.Delete(context.Background(), &cascadeParent{ID: 6}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if d.beginCalls != 0 {
		t.Errorf("beginCalls = %d, want 0 (Delete must reuse the existing Tx, not open a nested one)", d.beginCalls)
	}
	if len(d.deleteCalls) != 2 {
		t.Fatalf("expected 2 CompileDelete calls (child cascade + parent), got %d", len(d.deleteCalls))
	}
}

// -----------------------------------------------------------------------
// Soft-delete-aware child variant, for restrict/cascade branches that read
// ChildDeleteDateColumn.
// -----------------------------------------------------------------------

type cascadeChildSoftDelete struct {
	ID        int64
	ParentID  int64
	DeletedAt *time.Time
}

func newCascadeChildSoftDeleteEntity(table string, parent *entity.Entity[cascadeParent], opts *relation.ForeignKeyOptions) *entity.Entity[cascadeChildSoftDelete] {
	return entity.New(func(c *cascadeChildSoftDelete, b *entity.Table) {
		b.TableName(table)
		b.Col(&c.ID, golem.BIGINT())
		b.Col(&c.ParentID, golem.BIGINT()).Name("parent_id")
		b.Col(&c.DeletedAt, golem.DATETIME()).Name("deleted_at")
		b.PrimaryKey(&c.ID)
		b.DeleteDate(&c.DeletedAt)
		b.ForeignKey(&c.ParentID, parent, opts)
	})
}

func TestRepository_Delete_OnDeleteRestrict_SoftDeleteAwareChild_AddsIsNullFilter(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_restrict_sd")
	newCascadeChildSoftDeleteEntity("cascade_child_restrict_sd", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))

	d := &fakeDialect{
		selectResult: []map[string]any{{"count": int64(0)}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 11}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(d.selectCalls) != 1 {
		t.Fatalf("expected 1 restrict-check Select call, got %d", len(d.selectCalls))
	}
	logical, ok := d.selectCalls[0].Where.(stmt.Logical)
	if !ok || logical.Op != "and" || len(logical.Predicates) != 2 {
		t.Fatalf("expected logical AND (fk eq + deleted_at IS NULL), got %+v", d.selectCalls[0].Where)
	}
	comp1, ok1 := logical.Predicates[1].(stmt.Comparison)
	if !ok1 || comp1.Column != "deleted_at" || comp1.Op != "is_null" {
		t.Errorf("expected deleted_at IS NULL as second predicate, got %+v", logical.Predicates[1])
	}
}

func TestRepository_Delete_OnDeleteCascade_SoftDeleteAwareChild_UpdatesInsteadOfDeleting(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_cascade_sd")
	newCascadeChildSoftDeleteEntity("cascade_child_cascade_sd", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 12}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(d.updateCalls) != 1 {
		t.Fatalf("expected 1 Update call (soft-delete cascade), got %d", len(d.updateCalls))
	}
	call := d.updateCalls[0]
	if call.Table != "cascade_child_cascade_sd" {
		t.Errorf("update table = %q, want cascade_child_cascade_sd", call.Table)
	}
	if len(call.Sets) != 1 || call.Sets[0].Column != "deleted_at" || call.Sets[0].Value == nil {
		t.Errorf("expected Set deleted_at = time.Now(), got %+v", call.Sets)
	}
	// Only the parent's own hard delete should hit CompileDelete — the
	// child cascade went through Update (soft-delete), not Delete.
	if len(d.deleteCalls) != 1 {
		t.Errorf("expected 1 CompileDelete call (parent only), got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_OnDeleteRestrict_EmptyRows_Continues(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_restrict_empty")
	newCascadeChildEntity("cascade_child_restrict_empty", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))

	d := &fakeDialect{
		selectResult: nil, // zero rows returned at all (not even a {"count":0} row)
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 13}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(d.deleteCalls) != 1 {
		t.Errorf("expected 1 CompileDelete call (parent itself), got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_OnDeleteRestrict_CompileSelectError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_restrict_compileerr")
	newCascadeChildEntity("cascade_child_restrict_compileerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))

	wantErr := errors.New("boom: compile select")
	d := &fakeDialect{selectErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 14})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Delete_OnDeleteRestrict_QueryError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_restrict_queryerr")
	newCascadeChildEntity("cascade_child_restrict_queryerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))

	wantErr := errors.New("boom: query")
	d := &fakeDialect{queryErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 15})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Delete_OnDeleteCascade_CompileDeleteError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_cascade_compileerr")
	newCascadeChildEntity("cascade_child_cascade_compileerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	wantErr := errors.New("boom: compile delete")
	d := &fakeDialect{deleteCompErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 16})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Delete_OnDeleteCascade_ExecError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_cascade_execerr")
	newCascadeChildEntity("cascade_child_cascade_execerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	wantErr := errors.New("boom: exec")
	d := &fakeDialect{execErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 17})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Delete_OnDeleteCascade_SoftDeleteUpdateError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_cascade_sd_updateerr")
	newCascadeChildSoftDeleteEntity("cascade_child_cascade_sd_updateerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	wantErr := errors.New("boom: update")
	d := &fakeDialect{updateErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 18})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Delete_OnDeleteSetNull_UpdateError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_setnull_updateerr")
	newCascadeChildEntity("cascade_child_setnull_updateerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteSetNull))

	wantErr := errors.New("boom: update")
	d := &fakeDialect{updateErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 19})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Delete_BeginCascadeTxError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_beginerr")
	newCascadeChildEntity("cascade_child_beginerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	wantErr := errors.New("boom: begin")
	d := &fakeDialect{beginErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 20})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
	if len(d.deleteCalls) != 0 {
		t.Errorf("expected 0 CompileDelete calls (never got past begin), got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_CommitCascadeTxError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_commiterr")
	newCascadeChildEntity("cascade_child_commiterr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	wantErr := errors.New("boom: commit")
	d := &fakeDialect{commitErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 21})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestCountValue(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int64
	}{
		{"int64", int64(5), 5},
		{"int32", int32(6), 6},
		{"int", int(7), 7},
		{"unrecognized type", "not a number", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := countValue(tc.in); got != tc.want {
				t.Errorf("countValue(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
