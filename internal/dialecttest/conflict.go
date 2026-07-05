package dialecttest

import (
	"context"
	"errors"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/repository"
)

// runConflictDetection proves golem.ErrDuplicateKey/golem.ErrForeignKeyViolation
// (M10) surface via errors.Is against a real database.
func runConflictDetection(t *testing.T, ctx context.Context, ds *golem.DataSource, _ Schema) {
	t.Run("ErrDuplicateKey", func(t *testing.T) {
		repo := repository.Get(ds, parentEntity)
		if _, err := repo.Insert(ctx, &Parent{Name: "conflict-unique-name"}); err != nil {
			t.Fatalf("first Insert: %v", err)
		}
		_, err := repo.Insert(ctx, &Parent{Name: "conflict-unique-name"})
		if !errors.Is(err, golem.ErrDuplicateKey) {
			t.Fatalf("second Insert with duplicate Name: err = %v, want errors.Is(err, golem.ErrDuplicateKey)", err)
		}
	})

	t.Run("ErrForeignKeyViolation", func(t *testing.T) {
		repo := repository.Get(ds, restrictChildEntity)
		badParentID := int64(-1)
		_, err := repo.Insert(ctx, &Child{ParentID: &badParentID, Name: "conflict-bad-fk"})
		if !errors.Is(err, golem.ErrForeignKeyViolation) {
			t.Fatalf("Insert with non-existent ParentID: err = %v, want errors.Is(err, golem.ErrForeignKeyViolation)", err)
		}
	})
}
