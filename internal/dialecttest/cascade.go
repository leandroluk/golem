package dialecttest

import (
	"context"
	"errors"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/testutil"
	"github.com/leandroluk/golem/internal/op"
	"github.com/leandroluk/golem/internal/query"
	"github.com/leandroluk/golem/internal/repository"
)

// runCascade proves ForeignKeyOptions.OnDelete's 3 modes (M11) against a
// real database: Cascade actually deletes referencing rows, SetNull clears
// the referencing column, Restrict blocks the parent delete.
func runCascade(t *testing.T, ctx context.Context, ds *golem.DataSource, _ Schema) {
	t.Run("Cascade", func(t *testing.T) {
		parentRepo := repository.Get(ds, parentEntity)
		childRepo := repository.Get(ds, cascadeChildEntity)

		parent, err := parentRepo.Insert(ctx, &Parent{Name: "cascade-parent"})
		testutil.FatalIfError(t, err, "Insert parent")
		child, err := childRepo.Insert(ctx, &Child{ParentID: &parent.ID, Name: "cascade-child"})
		testutil.FatalIfError(t, err, "Insert child")

		err = parentRepo.Delete(ctx, &parent)
		testutil.FatalIfError(t, err, "Delete parent")

		exists, err := childRepo.Exists(ctx, func(c *Child, q *query.Count[Child]) {
			q.Where(op.Eq(&c.ID, child.ID))
		})
		testutil.FatalIfError(t, err, "Exists child")
		testutil.ErrorIf(t, exists, "expected child row to be cascade-deleted along with its parent")
	})

	t.Run("SetNull", func(t *testing.T) {
		parentRepo := repository.Get(ds, parentEntity)
		childRepo := repository.Get(ds, setNullChildEntity)

		parent, err := parentRepo.Insert(ctx, &Parent{Name: "setnull-parent"})
		testutil.FatalIfError(t, err, "Insert parent")
		child, err := childRepo.Insert(ctx, &Child{ParentID: &parent.ID, Name: "setnull-child"})
		testutil.FatalIfError(t, err, "Insert child")

		err = parentRepo.Delete(ctx, &parent)
		testutil.FatalIfError(t, err, "Delete parent")

		found, err := childRepo.FindOne(ctx, func(c *Child, q *query.Query[Child]) {
			q.Where(op.Eq(&c.ID, child.ID))
		})
		testutil.FatalIfError(t, err, "FindOne child")
		testutil.ErrorIf(t, found.ParentID != nil, "ParentID is not nil after parent delete")
	})

	t.Run("Restrict", func(t *testing.T) {
		parentRepo := repository.Get(ds, parentEntity)
		childRepo := repository.Get(ds, restrictChildEntity)

		parent, err := parentRepo.Insert(ctx, &Parent{Name: "restrict-parent"})
		testutil.FatalIfError(t, err, "Insert parent")
		_, err = childRepo.Insert(ctx, &Child{ParentID: &parent.ID, Name: "restrict-child"})
		testutil.FatalIfError(t, err, "Insert child")

		err = parentRepo.Delete(ctx, &parent)
		testutil.FatalIf(t, !errors.Is(err, golem.ErrForeignKeyViolation), "Delete parent with referencing child: err = %v, want errors.Is(err, golem.ErrForeignKeyViolation)", err)

		exists, err := parentRepo.Exists(ctx, func(p *Parent, q *query.Count[Parent]) {
			q.Where(op.Eq(&p.ID, parent.ID))
		})
		testutil.FatalIfError(t, err, "Exists parent")
		testutil.ErrorIf(t, !exists, "expected parent to still exist — blocked delete must not remove it")
	})
}
