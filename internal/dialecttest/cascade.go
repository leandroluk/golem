package dialecttest

import (
	"context"
	"errors"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
)

// runCascade proves ForeignKeyOptions.OnDelete's 3 modes (M11) against a
// real database: Cascade actually deletes referencing rows, SetNull clears
// the referencing column, Restrict blocks the parent delete.
func runCascade(t *testing.T, ctx context.Context, ds *golem.DataSource, _ Schema) {
	t.Run("Cascade", func(t *testing.T) {
		parentRepo := repository.Get(ds, parentEntity)
		childRepo := repository.Get(ds, cascadeChildEntity)

		parent, err := parentRepo.Insert(ctx, &Parent{Name: "cascade-parent"})
		if err != nil {
			t.Fatalf("Insert parent: %v", err)
		}
		child, err := childRepo.Insert(ctx, &Child{ParentID: parent.ID, Name: "cascade-child"})
		if err != nil {
			t.Fatalf("Insert child: %v", err)
		}

		if err := parentRepo.Delete(ctx, &parent); err != nil {
			t.Fatalf("Delete parent: %v", err)
		}

		exists, err := childRepo.Exists(ctx, func(c *Child, q *query.Count[Child]) {
			q.Where(op.Eq(&c.ID, child.ID))
		})
		if err != nil {
			t.Fatalf("Exists child: %v", err)
		}
		if exists {
			t.Error("expected child row to be cascade-deleted along with its parent")
		}
	})

	t.Run("SetNull", func(t *testing.T) {
		parentRepo := repository.Get(ds, parentEntity)
		childRepo := repository.Get(ds, setNullChildEntity)

		parent, err := parentRepo.Insert(ctx, &Parent{Name: "setnull-parent"})
		if err != nil {
			t.Fatalf("Insert parent: %v", err)
		}
		child, err := childRepo.Insert(ctx, &Child{ParentID: parent.ID, Name: "setnull-child"})
		if err != nil {
			t.Fatalf("Insert child: %v", err)
		}

		if err := parentRepo.Delete(ctx, &parent); err != nil {
			t.Fatalf("Delete parent: %v", err)
		}

		// Query the raw column instead of scanning into Child.ParentID
		// (int64, not nullable) — the point here is proving the database
		// column itself became NULL, not round-tripping it through Go.
		res, err := ds.Exec(ctx, "SELECT parent_id FROM conf_setnull_child WHERE id = $1", child.ID)
		if err != nil {
			t.Fatalf("raw select: %v", err)
		}
		if !res.Next() {
			t.Fatal("expected 1 row")
		}
		row, err := res.Scan()
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if row["parent_id"] != nil {
			t.Errorf("parent_id = %v, want NULL after parent delete", row["parent_id"])
		}
	})

	t.Run("Restrict", func(t *testing.T) {
		parentRepo := repository.Get(ds, parentEntity)
		childRepo := repository.Get(ds, restrictChildEntity)

		parent, err := parentRepo.Insert(ctx, &Parent{Name: "restrict-parent"})
		if err != nil {
			t.Fatalf("Insert parent: %v", err)
		}
		if _, err := childRepo.Insert(ctx, &Child{ParentID: parent.ID, Name: "restrict-child"}); err != nil {
			t.Fatalf("Insert child: %v", err)
		}

		err = parentRepo.Delete(ctx, &parent)
		if !errors.Is(err, golem.ErrForeignKeyViolation) {
			t.Fatalf("Delete parent with referencing child: err = %v, want errors.Is(err, golem.ErrForeignKeyViolation)", err)
		}

		exists, err := parentRepo.Exists(ctx, func(p *Parent, q *query.Count[Parent]) {
			q.Where(op.Eq(&p.ID, parent.ID))
		})
		if err != nil {
			t.Fatalf("Exists parent: %v", err)
		}
		if !exists {
			t.Error("expected parent to still exist — blocked delete must not remove it")
		}
	})
}
