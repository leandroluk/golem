# Cross-Dialect Conformance Suite (M15) — Design

## Package

`internal/dialecttest` (`github.com/leandroluk/golem/internal/dialecttest`). Internal-only per
user decision — every current and planned adapter (M16-M21) lives inside this same module, so
`internal/` visibility is sufficient; revisit only if a genuinely out-of-tree adapter shows up.

Building `internal/stmt` already established the naming convention (no underscore) this package
follows.

## Prior art already in this repo

`.examples/postgres/main_integration_test.go` is today's only real integration proof
of CRUD/joins/cascade/aggregate/locking/preload against a live Postgres. It already solved two
problems the harness reuses as-is:

1. **Ephemeral schema via `CREATE TEMPORARY TABLE`** (see `TestBlogExample_DeleteCountAndExists`'s
   `TempPost`/`TempPostEntity`) — no dependency on `.docker/postgres.sql`'s pre-provisioned tables,
   no manual cleanup (temp tables drop with the session), no collision risk with anything else
   running against the same test database.
2. **Per-test unique `DataSourceName(t.Name())`** (AD-035) — required now that `NewDataSource`
   errors on a duplicate name.

The harness's design is this pattern, generalized and given a stable Go API instead of copy-paste.

## Entrypoint

```go
package dialecttest

// Run executes every conformance case as Go subtests against a DataSource built from opts (e.g.
// postgres.New(func(o *postgres.Options) {...}) — postgres.New returns a golem.Option, not a
// golem.Connector directly, since that's the only exported way to build one; opts is variadic
// purely to accept that single Option value). schema supplies the dialect-correct DDL for the
// harness's fixed logical tables (see "Schema ownership" below); caps declares which optional
// capabilities (locking strengths/wait modes) the dialect actually supports, so the harness can
// skip exactly what it can't test instead of failing or omitting silently.
func Run(t *testing.T, schema Schema, caps Capabilities, opts ...golem.Option)
```

Called from each adapter's own `_integration_test.go` (own build tag, own Docker service), e.g.
`driver/postgres/conformance_integration_test.go`:

```go
func TestPostgres_Conformance(t *testing.T) {
	dialecttest.Run(t, postgresConformanceSchema, postgresCapabilities,
		postgres.New(func(o *postgres.Options) { o.DSN = testDSN() }))
}
```

## Schema ownership (the core design call)

The harness defines a **fixed logical schema** in Go (entities + their `entity.New` mappings,
100% dialect-agnostic, written once, reused by every adapter) — but does **not** generate DDL
text for it. Each adapter supplies its own `CREATE TEMPORARY TABLE` statements matching that exact
logical shape, in its own dialect's syntax.

Rejected the alternative (harness auto-generates DDL from `golem.ColumnType`) because that
reintroduces schema-generation logic golem's core deliberately never has (AD-012) — the
conformance suite is dev/test infrastructure, but "does golem ever turn `ColumnType` into DDL" is
a bright line worth keeping bright even here. Requiring each adapter to hand-write ~5 short
`CREATE TEMPORARY TABLE` statements once is a small, honest cost; it also doubles as the first
real proof an adapter's basic DDL syntax (temp tables, `SERIAL`/`AUTO_INCREMENT`/`IDENTITY`
equivalent, etc.) works at all.

```go
type Schema struct {
	// Every entry is run once, up front, before any subtest group (several
	// tables — Widget, Parent, CascadeChild — are read by more than one
	// group, and CREATE TEMPORARY TABLE isn't safe to run twice in the same
	// session). Table names are already prefixed (see below); statements
	// must use CREATE TEMPORARY TABLE (or the dialect's closest equivalent)
	// so no explicit teardown is needed.
	Widget  string // single-PK CRUD/Where/Select/OrderBy/Limit/Offset table
	Deleted string // same shape as Widget, plus a nullable soft-delete timestamp column
	Parent  string // cascade-FK/join/preload parent (single-PK)

	// Cascade needs 3 physically distinct child tables, one per OnDelete
	// mode — the mode is fixed per entity.Entity[Child] mapping (via the
	// FK registry entity.New populates at init time), not per-row, so
	// testing all 3 modes means 3 separate entity declarations. Matches
	// repository/repository_test.go's existing cascade_child_delete /
	// cascade_child_setnull / cascade_child_restrict pattern exactly.
	CascadeChild  string // ON DELETE CASCADE
	SetNullChild  string // ON DELETE SET NULL
	RestrictChild string // ON DELETE RESTRICT

	// Joins/Preload don't trigger deletes, so they reuse CascadeChild's
	// table/entity — no separate table needed for them.
}
```

Postgres's `Schema` value (defined in `driver/postgres`'s new conformance test file) is the first
implementation and the harness's own de facto acceptance test for the `Schema` shape itself.

## Table naming

Every table name the harness's entities declare is prefixed `conf_` (`conf_widget`,
`conf_deleted`, `conf_parent`, `conf_cascade_child`, `conf_setnull_child`, `conf_restrict_child`)
so a conformance run can never collide with
`.docker/postgres.sql`'s `users`/`post`/`category`/`post_to_category` tables when both run in the
same CI job against the same test database — matches the existing collision-avoidance reasoning
already applied to `DataSourceName` (AD-035).

## Capabilities

```go
type Capabilities struct {
	// Locking mirrors query.LockStrength/query.LockWait's exact value set (M14) so a dialect
	// declares support 1:1 against what query.Query[T] actually exposes, not an
	// invented parallel vocabulary.
	Locking LockCapabilities
}

type LockCapabilities struct {
	Update       bool // FOR UPDATE / equivalent
	NoKeyUpdate  bool // FOR NO KEY UPDATE — Postgres-specific; MySQL/others likely false
	Share        bool // FOR SHARE / equivalent
	KeyShare     bool // FOR KEY SHARE — Postgres-specific; MySQL/others likely false
	NoWait       bool
	SkipLocked   bool
}
```

`driver/postgres`'s `Capabilities` value sets every field `true` (it supports everything M14
built). A future MySQL adapter sets `NoKeyUpdate`/`KeyShare` to `false`; the harness's locking
subtest group checks each field before running the matching case and calls `t.Skip(...)` (never
silently omits, satisfying CONF-05) when a capability is `false`. A future Snowflake adapter (M21)
sets every `Locking` field `false`, skipping the entire locking group while every other group
still runs.

## Subtest groups (`t.Run` names, one group per Goal in spec.md)

| Group               | What it proves                                                                                                                        | Reuses                      |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------- | --------------------------- |
| `BindScanRoundTrip` | every `golem.ColumnType` kind round-trips through `Insert`+`FindOne` unchanged                                                        | —                           |
| `CRUD`              | `Insert`/`InsertMany`/`SaveOne`/`SaveMany`/`Update`/`FindOne`/`FindMany`/`Count`/`Exists`/`Delete` (hard delete, `Widget`)            | —                           |
| `SoftDelete`        | `Delete`/`Restore` on `Deleted`, default filtering + `.WithDeleted()`                                                                 | M12's soft-delete guarantee |
| `Cascade`           | `OnDeleteCascade`/`OnDeleteSetNull`/`OnDeleteRestrict` between `Parent`/`Child`                                                       | M11                         |
| `Joins`             | `join.Inner` (at minimum; `Left`/`Right`/`Full` if the dialect's `Capabilities` doesn't gate them out later) between `Parent`/`Child` | M6                          |
| `Preload`           | `repository.Preload` between `Parent`/`Child`                                                                                         | M12                         |
| `Aggregates`        | `repository.Aggregate` (`GroupBy`/`Sum`/`Count`/`CountAll`/`Having`) over `Widget`                                                    | M13                         |
| `Locking`           | each `Capabilities.Locking`-gated case, plus the outside-a-`Tx` error guard                                                           | M14                         |
| `ConflictDetection` | `golem.ErrDuplicateKey`/`golem.ErrForeignKeyViolation` surface via `errors.Is`                                                        | M10                         |

Each group is an unexported function (`runCRUD(t *testing.T, ds *golem.DataSource, schema Schema)`,
etc.) called from `Run` via `t.Run(name, func(t *testing.T) { ... })` — keeps `Run` itself a short
dispatch table, each group independently readable/skippable.

## What `driver/postgres` changes to adopt this

- New `driver/postgres/conformance_integration_test.go` (own `//go:build integration` tag, matching
  `connector_integration_test.go`'s existing convention): defines `postgresConformanceSchema`
  (the four `CREATE TEMPORARY TABLE` statements) and `postgresCapabilities` (all fields `true`),
  then calls `dialecttest.Run`.
- No changes to `.examples/postgres` — it stays as a hand-written, narrative example
  (its purpose is documentation-by-demonstration, not conformance proof), and no changes to
  `driver/postgres/connector_integration_test.go` (adapter-specific connection plumbing stays
  separate per spec.md's Out of Scope).
- `Taskfile.yml`'s `test-integration` already runs `go test -tags=integration ./...`, which picks
  up the new file automatically — no Taskfile change needed (closes CONF-06 for free).

## Open risk carried into Tasks

pgxpool may hand different queries to different backend connections; per-adapter temp-table
visibility across a connection pool is unproven at the harness level (though already relied on by
`.examples`'s `TestBlogExample_DeleteCountAndExists`, which passes today). If this turns out to be
flaky under the harness's larger subtest count, the fallback is a single dedicated connection
lease for the whole `Run` call — deferred until proven necessary, not designed preemptively.
