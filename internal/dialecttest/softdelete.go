package dialecttest

import (
	"context"
	"testing"

	"github.com/leandroluk/golem"
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
	if err != nil {
		t.Fatalf("InsertMany: %v", err)
	}

	if err := repo.Delete(ctx, &rows[0]); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Errorf("Count after Delete = %d, want 1 (soft-deleted row filtered by default)", count)
	}

	countWithDeleted, err := repo.Count(ctx, func(d *Deleted, c *query.Count[Deleted]) {
		c.WithDeleted()
	})
	if err != nil {
		t.Fatalf("Count with WithDeleted: %v", err)
	}
	if countWithDeleted != 2 {
		t.Errorf("Count with WithDeleted = %d, want 2", countWithDeleted)
	}

	found, err := repo.FindOne(ctx, func(d *Deleted, q *query.Query[Deleted]) {
		q.Where(op.Eq(&d.ID, rows[0].ID))
		q.WithDeleted()
	})
	if err != nil {
		t.Fatalf("FindOne with WithDeleted: %v", err)
	}
	if found.DeletedAt == nil {
		t.Error("expected DeletedAt to be set after Delete")
	}

	if err := repo.Restore(ctx, &rows[0]); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	countAfterRestore, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count after Restore: %v", err)
	}
	if countAfterRestore != 2 {
		t.Errorf("Count after Restore = %d, want 2", countAfterRestore)
	}
}
