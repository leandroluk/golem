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

// runSoftDelete proves Delete/Restore on an entity with DeleteDate soft-
// delete instead of removing the row, and that default filtering /
// .WithDeleted() behave against a real database (M12's guarantee).
func runSoftDelete(t *testing.T, ctx context.Context, ds *golem.DataSource, _ Schema) {
	repo := repository.Get(ds, deletedEntity)

	rows, err := repo.InsertMany(ctx,
		&Deleted{Name: "soft-1"},
		&Deleted{Name: "soft-2"},
	)
	testutil.FatalIfError(t, err, "InsertMany")

	err = repo.Delete(ctx, &rows[0])
	testutil.FatalIfError(t, err, "Delete")

	count, err := repo.Count(ctx)
	testutil.FatalIfError(t, err, "Count")
	testutil.ErrorIf(t, count != 1, "Count after Delete = %d, want 1 (soft-deleted row filtered by default)", count)

	countWithDeleted, err := repo.Count(ctx, func(d *Deleted, c *query.Count[Deleted]) {
		c.WithDeleted()
	})
	testutil.FatalIfError(t, err, "Count with WithDeleted")
	testutil.ErrorIf(t, countWithDeleted != 2, "Count with WithDeleted = %d, want 2", countWithDeleted)

	found, err := repo.FindOne(ctx, func(d *Deleted, q *query.Query[Deleted]) {
		q.Where(op.Eq(&d.ID, rows[0].ID))
		q.WithDeleted()
	})
	testutil.FatalIfError(t, err, "FindOne with WithDeleted")
	testutil.ErrorIf(t, found.DeletedAt == nil, "expected DeletedAt to be set after Delete")

	err = repo.Restore(ctx, &rows[0])
	testutil.FatalIfError(t, err, "Restore")

	countAfterRestore, err := repo.Count(ctx)
	testutil.FatalIfError(t, err, "Count after Restore")
	testutil.ErrorIf(t, countAfterRestore != 2, "Count after Restore = %d, want 2", countAfterRestore)
}
