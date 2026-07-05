// Package dialecttest is a reusable, dialect-agnostic conformance test
// harness. Every golem adapter (driver/postgres today; driver/mysql,
// driver/sqlite, etc. per .specs/project/ROADMAP.md's M16-M21) proves itself
// conformant by calling Run with a working golem.Connector, a Schema
// (dialect-specific DDL for the harness's fixed logical tables), and a
// Capabilities declaration (which optional behaviors — currently just
// locking strengths/wait modes — the dialect actually supports).
//
// Internal-only: every adapter this harness targets lives inside this same
// module (see .specs/project/STATE.md AD-034/AD-035's reasoning), so there's
// no need for external packages to import it.
package dialecttest

import (
	"context"
	"testing"

	"github.com/leandroluk/golem"
)

// Schema supplies the dialect-correct DDL for the harness's fixed logical
// tables. Every statement should use CREATE TEMPORARY TABLE (or the
// dialect's closest equivalent) so no explicit teardown is required — see
// design.md's "Schema ownership" for why the harness itself never generates
// DDL. Table names are conf_widget/conf_deleted/conf_parent/
// conf_cascade_child/conf_setnull_child/conf_restrict_child (see
// entities.go) to avoid colliding with anything else that might run against
// the same test database.
type Schema struct {
	Widget  string // single-PK CRUD/Where/Select/OrderBy/Limit/Offset table
	Deleted string // same shape as Widget, plus a nullable soft-delete timestamp column
	Parent  string // cascade-FK/join/preload parent (single-PK)

	// Cascade needs 3 physically distinct child tables, one per OnDelete
	// mode, since the mode is fixed per entity.Entity[Child] mapping (via
	// the FK registry entity.New populates), not per-row. Joins/Preload
	// reuse CascadeChild — they never trigger a delete.
	CascadeChild  string // ON DELETE CASCADE
	SetNullChild  string // ON DELETE SET NULL
	RestrictChild string // ON DELETE RESTRICT
}

// LockCapabilities declares which of query.LockStrength/query.LockWait's
// exact value set (M14) the dialect actually supports. A false field makes
// Run's Locking subtest group skip (t.Skip, never silently omit) exactly
// that case.
type LockCapabilities struct {
	Update      bool // FOR UPDATE / equivalent
	NoKeyUpdate bool // FOR NO KEY UPDATE — Postgres-specific
	Share       bool // FOR SHARE / equivalent
	KeyShare    bool // FOR KEY SHARE — Postgres-specific
	NoWait      bool
	SkipLocked  bool
}

// Capabilities declares every optional behavior a dialect may or may not
// support. Currently just locking; grows as more optional (dialect-varying)
// behaviors get added to the harness.
type Capabilities struct {
	Locking LockCapabilities
}

// Run executes every conformance subtest group against a DataSource built
// from opts (e.g. postgres.New(func(o *postgres.Options) {...}) — whatever
// golem.Option the adapter's own constructor returns; opts is variadic
// purely to accept that single Option value, not to take several unrelated
// ones). Each group is registered as its own named t.Run subtest so
// failures/skips are independently reportable.
func Run(t *testing.T, schema Schema, caps Capabilities, opts ...golem.Option) {
	t.Helper()

	opts = append(opts, golem.DataSourceName(t.Name()))
	ds, err := golem.NewDataSource(opts...)
	if err != nil {
		t.Fatalf("dialecttest: NewDataSource: %v", err)
	}
	defer ds.Close()

	if err := ds.Connect(); err != nil {
		t.Fatalf("dialecttest: Connect: %v", err)
	}

	ctx := context.Background()

	// Every table is created once, up front — Widget/Parent/CascadeChild
	// are each read by more than one subtest group below, and CREATE
	// TEMPORARY TABLE isn't safe to run twice in the same session.
	for _, ddl := range []string{
		schema.Widget, schema.Deleted, schema.Parent,
		schema.CascadeChild, schema.SetNullChild, schema.RestrictChild,
	} {
		if _, err := ds.Exec(ctx, ddl); err != nil {
			t.Fatalf("dialecttest: schema setup: %v", err)
		}
	}

	t.Run("BindScanRoundTrip", func(t *testing.T) { runBindScanRoundTrip(t, ctx, ds, schema) })
	t.Run("CRUD", func(t *testing.T) { runCRUD(t, ctx, ds, schema) })
	t.Run("SoftDelete", func(t *testing.T) { runSoftDelete(t, ctx, ds, schema) })
	t.Run("Cascade", func(t *testing.T) { runCascade(t, ctx, ds, schema) })
	t.Run("Joins", func(t *testing.T) { runJoins(t, ctx, ds, schema) })
	t.Run("Preload", func(t *testing.T) { runPreload(t, ctx, ds, schema) })
	t.Run("Aggregates", func(t *testing.T) { runAggregates(t, ctx, ds, schema) })
	t.Run("Locking", func(t *testing.T) { runLocking(t, ctx, ds, schema, caps.Locking) })
	t.Run("ConflictDetection", func(t *testing.T) { runConflictDetection(t, ctx, ds, schema) })
}
