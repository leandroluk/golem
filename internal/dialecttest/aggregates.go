package dialecttest

import (
	"context"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/testutil"
	"github.com/leandroluk/golem/internal/op"
	"github.com/leandroluk/golem/internal/query"
	"github.com/leandroluk/golem/internal/repository"
)

// aggregateResult is Aggregate's destination shape — a plain struct,
// resolved by field pointer against Widget, per M13's design.
type aggregateResult struct {
	Category string
	Total    float64
	Count    int64
}

// runAggregates proves repository.Aggregate (M13) against a real database.
func runAggregates(t *testing.T, ctx context.Context, ds *golem.DataSource, _ Schema) {
	repo := repository.Get(ds, widgetEntity)

	_, err := repo.InsertMany(ctx,
		&Widget{Name: "agg-1", Category: "agg-cat-a", Score: 10},
		&Widget{Name: "agg-2", Category: "agg-cat-a", Score: 20},
		&Widget{Name: "agg-3", Category: "agg-cat-b", Score: 5},
	)
	testutil.FatalIfError(t, err, "InsertMany")

	results, err := repository.Aggregate(ctx, repo, func(w *Widget, res *aggregateResult, a *query.Aggregate[Widget, aggregateResult]) {
		a.GroupBy(&w.Category, &res.Category)
		a.Sum(&w.Score, &res.Total)
		a.CountAll(&res.Count)
		a.Where(op.Eq(&w.Category, "agg-cat-a"))
		a.Having(op.Gt(&res.Count, int64(1)))
	})
	testutil.FatalIfError(t, err, "Aggregate")
	testutil.FatalIf(t, len(results) != 1, "expected 1 group matching Having, got %d", len(results))
	testutil.ErrorIf(t, results[0].Category != "agg-cat-a", "Category = %q, want agg-cat-a", results[0].Category)
	testutil.ErrorIf(t, results[0].Total != 30, "Total = %v, want 30", results[0].Total)
	testutil.ErrorIf(t, results[0].Count != 2, "Count = %d, want 2", results[0].Count)
}
