package dialecttest

import (
	"context"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
)

// runPreload proves repository.Preload (M12) against a real database.
func runPreload(t *testing.T, ctx context.Context, ds *golem.DataSource, _ Schema) {
	parentRepo := repository.Get(ds, parentEntity)
	childRepo := repository.Get(ds, cascadeChildEntity)

	parent, err := parentRepo.Insert(ctx, &Parent{Name: "preload-parent"})
	if err != nil {
		t.Fatalf("Insert parent: %v", err)
	}
	if _, err := childRepo.InsertMany(ctx,
		&Child{ParentID: &parent.ID, Name: "preload-child-1"},
		&Child{ParentID: &parent.ID, Name: "preload-child-2"},
	); err != nil {
		t.Fatalf("InsertMany child: %v", err)
	}

	parents, err := parentRepo.FindMany(ctx, func(p *Parent, q *query.Query[Parent]) {
		q.Where(op.Eq(&p.ID, parent.ID))
	})
	if err != nil {
		t.Fatalf("FindMany parents: %v", err)
	}

	childrenByParentID, err := repository.Preload(ctx, parentRepo, parents, cascadeChildEntity)
	if err != nil {
		t.Fatalf("Preload: %v", err)
	}
	if len(childrenByParentID[parent.ID]) != 2 {
		t.Errorf("Preload: expected 2 children for parent %d, got %d", parent.ID, len(childrenByParentID[parent.ID]))
	}
}
