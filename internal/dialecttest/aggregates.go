package dialecttest

import (
	"context"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
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

	if _, err := repo.InsertMany(ctx,
		&Widget{Name: "agg-1", Category: "agg-cat-a", Score: 10},
		&Widget{Name: "agg-2", Category: "agg-cat-a", Score: 20},
		&Widget{Name: "agg-3", Category: "agg-cat-b", Score: 5},
	); err != nil {
		t.Fatalf("InsertMany: %v", err)
	}

	results, err := repository.Aggregate(ctx, repo, func(w *Widget, res *aggregateResult, a *query.Aggregate[Widget, aggregateResult]) {
		a.GroupBy(&w.Category, &res.Category)
		a.Sum(&w.Score, &res.Total)
		a.CountAll(&res.Count)
		a.Where(op.Eq(&w.Category, "agg-cat-a"))
		a.Having(op.Gt(&res.Count, int64(1)))
	})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 group matching Having, got %d", len(results))
	}
	if results[0].Category != "agg-cat-a" {
		t.Errorf("Category = %q, want agg-cat-a", results[0].Category)
	}
	if results[0].Total != 30 {
		t.Errorf("Total = %v, want 30", results[0].Total)
	}
	if results[0].Count != 2 {
		t.Errorf("Count = %d, want 2", results[0].Count)
	}
}
