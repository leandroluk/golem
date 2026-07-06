package dialecttest

import (
	"context"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/testutil"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
)

// runPreload proves repository.Preload (M12) against a real database.
func runPreload(t *testing.T, ctx context.Context, ds *golem.DataSource, _ Schema) {
	parentRepo := repository.Get(ds, parentEntity)
	childRepo := repository.Get(ds, cascadeChildEntity)

	parent, err := parentRepo.Insert(ctx, &Parent{Name: "preload-parent"})
	testutil.FatalIfError(t, err, "Insert parent")
	_, err = childRepo.InsertMany(ctx,
		&Child{ParentID: &parent.ID, Name: "preload-child-1"},
		&Child{ParentID: &parent.ID, Name: "preload-child-2"},
	)
	testutil.FatalIfError(t, err, "InsertMany child")

	parents, err := parentRepo.FindMany(ctx, func(p *Parent, q *query.Query[Parent]) {
		q.Where(op.Eq(&p.ID, parent.ID))
	})
	testutil.FatalIfError(t, err, "FindMany parents")

	childrenByParentID, err := repository.Preload(ctx, parentRepo, parents, cascadeChildEntity)
	testutil.FatalIfError(t, err, "Preload")
	testutil.ErrorIf(t, len(childrenByParentID[parent.ID]) != 2, "Preload: expected 2 children for parent %d, got %d", parent.ID, len(childrenByParentID[parent.ID]))
}
