package dialecttest

import (
	"context"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/testutil"
	"github.com/leandroluk/golem/internal/join"
	"github.com/leandroluk/golem/internal/op"
	"github.com/leandroluk/golem/internal/query"
	"github.com/leandroluk/golem/internal/repository"
)

// runJoins proves join.Inner (M6) against a real database, reusing
// Parent/CascadeChild (no delete happens here, so any of the 3 cascade
// child tables would work — CascadeChild is the arbitrary pick).
func runJoins(t *testing.T, ctx context.Context, ds *golem.DataSource, _ Schema) {
	parentRepo := repository.Get(ds, parentEntity)
	childRepo := repository.Get(ds, cascadeChildEntity)

	parent, err := parentRepo.Insert(ctx, &Parent{Name: "join-parent"})
	testutil.FatalIfError(t, err, "Insert parent")
	_, err = childRepo.Insert(ctx, &Child{ParentID: &parent.ID, Name: "join-child"})
	testutil.FatalIfError(t, err, "Insert child")

	parents, err := parentRepo.FindMany(ctx, func(p *Parent, q0 *query.Query[Parent]) {
		join.Inner(q0, cascadeChildEntity, func(c *Child, q1 *query.Join[Child]) {
			q1.On(&c.ParentID, &p.ID)
			q1.Where(op.Eq(&c.Name, "join-child"))
		})
		q0.Where(op.Eq(&p.ID, parent.ID))
	})
	testutil.FatalIfError(t, err, "FindMany with join.Inner")
	testutil.ErrorIf(t, len(parents) != 1, "expected 1 parent matched via inner join, got %d", len(parents))
}
