package dialecttest

import (
	"context"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
)

// runLocking proves M14's pessimistic locking against a real database, one
// subtest per lock strength/wait mode, each skipped (not failed, not
// silently omitted) when caps says the dialect doesn't support it. The
// outside-a-transaction error guard always runs — it's dialect-independent
// (Repository[T] checks conn's Go type, not the database).
func runLocking(t *testing.T, ctx context.Context, ds *golem.DataSource, _ Schema, caps LockCapabilities) {
	repo := repository.Get(ds, widgetEntity)

	w, err := repo.Insert(ctx, &Widget{Name: "lock-subject"})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	t.Run("OutsideTransaction_ReturnsError", func(t *testing.T) {
		_, err := repo.FindOne(ctx, func(wg *Widget, q *query.Query[Widget]) {
			q.Where(op.Eq(&wg.ID, w.ID))
			q.ForUpdate()
		})
		if err == nil {
			t.Fatal("expected an error locking outside a transaction, got nil")
		}
	})

	lockedFind := func(t *testing.T, apply func(q *query.Query[Widget])) {
		t.Helper()
		err := ds.Transaction(ctx, func(tx golem.Tx) error {
			txRepo := repository.Get(tx, widgetEntity)
			_, err := txRepo.FindOne(ctx, func(wg *Widget, q *query.Query[Widget]) {
				q.Where(op.Eq(&wg.ID, w.ID))
				apply(q)
			})
			return err
		})
		if err != nil {
			t.Fatalf("locked FindOne inside a transaction: %v", err)
		}
	}

	t.Run("ForUpdate", func(t *testing.T) {
		if !caps.Update {
			t.Skip("dialect does not support FOR UPDATE")
		}
		lockedFind(t, func(q *query.Query[Widget]) { q.ForUpdate() })
	})

	t.Run("ForNoKeyUpdate", func(t *testing.T) {
		if !caps.NoKeyUpdate {
			t.Skip("dialect does not support FOR NO KEY UPDATE")
		}
		lockedFind(t, func(q *query.Query[Widget]) { q.ForNoKeyUpdate() })
	})

	t.Run("ForShare", func(t *testing.T) {
		if !caps.Share {
			t.Skip("dialect does not support FOR SHARE")
		}
		lockedFind(t, func(q *query.Query[Widget]) { q.ForShare() })
	})

	t.Run("ForKeyShare", func(t *testing.T) {
		if !caps.KeyShare {
			t.Skip("dialect does not support FOR KEY SHARE")
		}
		lockedFind(t, func(q *query.Query[Widget]) { q.ForKeyShare() })
	})

	t.Run("NoWait", func(t *testing.T) {
		if !caps.Update || !caps.NoWait {
			t.Skip("dialect does not support FOR UPDATE NOWAIT")
		}
		lockedFind(t, func(q *query.Query[Widget]) { q.ForUpdate(query.LockWaitNoWait) })
	})

	t.Run("SkipLocked", func(t *testing.T) {
		if !caps.Update || !caps.SkipLocked {
			t.Skip("dialect does not support FOR UPDATE SKIP LOCKED")
		}
		lockedFind(t, func(q *query.Query[Widget]) { q.ForUpdate(query.LockWaitSkipLocked) })
	})
}
