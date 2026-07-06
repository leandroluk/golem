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

// runLocking proves M14's pessimistic locking against a real database, one
// subtest per lock strength/wait mode, each skipped (not failed, not
// silently omitted) when caps says the dialect doesn't support it. The
// outside-a-transaction error guard always runs — it's dialect-independent
// (Repository[T] checks conn's Go type, not the database).
func runLocking(t *testing.T, ctx context.Context, ds *golem.DataSource, _ Schema, caps LockCapabilities) {
	repo := repository.Get(ds, widgetEntity)

	w, err := repo.Insert(ctx, &Widget{Name: "lock-subject"})
	testutil.FatalIfError(t, err, "Insert")

	t.Run("OutsideTransaction_ReturnsError", func(t *testing.T) {
		_, err := repo.FindOne(ctx, func(wg *Widget, q *query.Query[Widget]) {
			q.Where(op.Eq(&wg.ID, w.ID))
			q.ForUpdate()
		})
		testutil.FatalIf(t, err == nil, "expected an error locking outside a transaction, got nil")
	})

	// COVERAGE-EXCLUDED (Taskfile.yml's coverage task filters lines 34+ of
	// this file out of .coverage/coverage.txt before computing %):
	// lockedFind's body, and every t.Run below that calls it, only execute
	// when the real adapter under test supports that lock strength. Under
	// `task coverage` (-short, no Docker), only driver/sqlite's untagged
	// conformance test exercises this function, and sqlite.Capabilities.
	// Locking is (correctly) all-false — so every one of these lines is
	// legitimately unreachable in that specific run, not a dead/defensive
	// branch (see STATE.md AD-043's "different in kind" analysis). Confirmed
	// 100% reachable when driver/postgres's own conformance run (Docker) is
	// included — see `task test-integration`. If this file's line numbers
	// shift, update the `>= 34` cutoff in Taskfile.yml's coverage task to
	// match (should start at this comment's line, not lockedFind's).
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
		testutil.FatalIfError(t, err, "locked FindOne inside a transaction")
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
