# SQL Server (MSSQL) Adapter (M18) — Design

**Spec**: `.specs/features/mssql-adapter/spec.md`
**Status**: Approved — every open question below confirmed empirically against a real SQL Server
2025 container (`mcr.microsoft.com/mssql/server:2025-latest`), not just documentation research.

## Research note

Every design decision below was validated with a real, throwaway `internal/mssqlprobetmp2` package
(deleted after use) run against a real SQL Server 2025 container, the same rigor M17's design got
against SQLite. `mcr.microsoft.com` was initially unreachable from the sandbox this design was
authored in (DNS resolves, but the CDN blob-storage backend resets the connection — no Docker Hub
mirror exists as a fallback); the user pulled the image on their own machine (network fix: enabling
Cloudflare One/WARP — see README.md's M18 note) and the probe ran there. Every numbered
"Needs Empirical Verification" item from the original draft is now a plain, confirmed fact — see
each section below for what the probe actually returned.

## Package

`driver/mssql` (`github.com/leandroluk/golem/driver/mssql`), same file layout as `driver/mysql`/
`driver/sqlite`: `mssql.go` (`Options`/`New`), `connector.go`, `dialect.go`, plus `_test.go`/
`_integration_test.go` pairs. Unlike `driver/sqlite`, this adapter genuinely needs a running server
— `conformance_integration_test.go` stays `//go:build integration` (SQL Server is not embeddable),
same reasoning `driver/postgres`/`driver/mysql` already established.

## Driver library

`github.com/microsoft/go-mssqldb` via `database/sql` — the official, actively maintained driver;
`denisenkom/go-mssqldb` (the original) now redirects to this fork. Registers the `"sqlserver"`
`database/sql` driver name.

**Placeholders**: confirmed empirically — the `"sqlserver"` driver name uses native `@p1`, `@p2`,
... `@pN` ordinal parameter placeholders (`db.Exec("... VALUES (@p1, @p2)", args...)` worked
directly in the probe), not `?`-style pre-processing. Same numbered-placeholder shape as
`driver/postgres`'s `$1`, `$2`, so `compilePredicate` needs an `argOffset *int` counter again
(dropped for M16/M17, comes back for M18).

`sqlIface` mirrors `driver/mysql`'s exact pattern (abstracts `*sql.DB`'s surface so
`DATA-DOG/go-sqlmock` can substitute for it in unit tests).

## Identifier quoting, placeholders, pagination

- **Identifiers**: square brackets (`[col]`, `[table].[col]`) — T-SQL's own quoting convention,
  distinct from every prior adapter's double-quote/backtick choice.
- **Placeholders**: `@p1`, `@p2`, ... (see above) — `compilePredicate` ported from
  `driver/postgres`'s numbered-placeholder shape, s/`$`/`@p`/.
- **Pagination**: `OFFSET y ROWS FETCH NEXT x ROWS ONLY` — **confirmed empirically that SQL Server
  requires an `ORDER BY` clause for this to be valid syntax at all**: the probe's
  `SELECT * FROM probe_parent OFFSET 0 ROWS FETCH NEXT 10 ROWS ONLY` (no `ORDER BY`) failed with
  `mssql: Invalid usage of the option NEXT in the FETCH statement`; adding `ORDER BY id` before
  `OFFSET`/`FETCH` made the identical query succeed. Decision (per spec.md's flagged edge case):
  when `s.Limit`/`s.Offset` is set and `s.OrderBy` is empty, `CompileSelect` injects `ORDER BY` on
  the entity's primary key column(s) (available via `stmt.Select` — confirm exact field name
  during implementation; if `stmt.Select` doesn't already carry primary-key info, this may need a
  small `internal/stmt` addition, which would itself be an AD-worthy core-schema finding, same
  class as M16's `PrimaryKey` addition). Chose "inject a stable default order" over "compile-time
  error" because every other adapter's `LIMIT`/`OFFSET` already had unspecified row order without
  an explicit `OrderBy` — matching that existing (weak) guarantee is less surprising than a new
  compile error only this adapter raises, and it's now confirmed *necessary* (not just nice to
  have) since the bare form is a hard SQL Server syntax error.

## Value conversion (`Bind`/`Scan`)

Per INSIGHT.md's type table: `BOOLEAN` → `BIT`, `UUID` → `UNIQUEIDENTIFIER`, `CHAR`/`VARCHAR`/
`TEXT`/`JSON` → `NCHAR`/`NVARCHAR`/`NVARCHAR(MAX)`, `BLOB` → `VARBINARY(MAX)`, `DATETIME` →
`DATETIME2`. `Bind`/`Scan` mirror `driver/postgres`'s simple-passthrough shape for every kind
(dead code in the real path, same as every adapter since AD-037 — unit-tested directly, not
exercised by `Insert`/`Update`/`Query`).

**`BIT` ↔ `bool`**: confirmed empirically — the probe's generic `Scan(&any)` against a `BIT`
column returned a native Go `bool` (`type=bool value=true`) directly, no conversion needed (SQL
Server's own actual boolean type, unlike MySQL's `TINYINT(1)` — no `assignFieldValue`-style
numeric→bool conversion needed here, same as `driver/postgres`).

**`DATETIME2`/`DATE`/`TIME` ↔ `time.Time`**: confirmed empirically — generic scan returned a
native `time.Time` (`type=time.Time value=2024-03-15 10:30:45 +0000 UTC`) directly, no DSN option
needed (unlike `driver/mysql`'s `ParseTime`/`driver/sqlite`'s `_time_format`) — matches
`driver/postgres`'s zero-config case.

**`UNIQUEIDENTIFIER` ↔ `UUID`**: both halves confirmed empirically.
- **Binding a plain Go `string`** (e.g. `"123e4567-e89b-12d3-a456-426614174000"`) directly as a
  parameter against a `UNIQUEIDENTIFIER` column **worked with no special handling** — SQL Server's
  implicit string→`uniqueidentifier` conversion applies to parameters, not just literals, so no
  `mssql.UniqueIdentifier` wrapper is needed for writes (matches every other adapter's plain-string
  UUID stance).
- **Reading back via generic `Scan(&any)` returns raw `[]byte` (16 bytes) in SQL Server's
  mixed-endian layout, NOT a formatted string.** Probe evidence: inserted UUID string
  `123e4567-e89b-12d3-a456-426614174000`, read back as
  `67 45 3e 12 9b e8 d3 12 a4 56 42 66 14 17 40 00`. Confirmed byte-group transform (comparing to
  the original hex groups `123e4567`-`e89b`-`12d3`-`a456`-`426614174000`): the first 3 groups (4
  bytes, 2 bytes, 2 bytes) are byte-reversed; the last 2 groups (2 bytes, 6 bytes) are unchanged.
  This is exactly the transform `mssql.UniqueIdentifier.Scan`/`.Value` implement internally —
  `collectRows`' `normalizeCell` (see below) needs this same reversal.

## Row scanning (`collectRows`)

Ported from `driver/mysql`'s/`driver/sqlite`'s shape (`database/sql`-generic, not `pgx`-based —
`go-mssqldb` is a plain `database/sql` driver). `normalizeCell`, keyed on `rows.ColumnTypes()`'s
`DatabaseTypeName()` (same mechanism as M16/M17), handles exactly one case: any column reported as
`"UNIQUEIDENTIFIER"` — reverse the first 3 byte groups (4/2/2 bytes) per SQL Server's mixed-endian
GUID layout (confirmed above), leave the last 2 groups (2/6 bytes) unchanged, then format as a
standard hyphenated UUID string.

## Insert / Update (single round-trip, `OUTPUT INSERTED.*`)

Mirrors `driver/postgres`'s/`driver/sqlite`'s shape — confirmed working via the probe
(`INSERT ... OUTPUT INSERTED.* VALUES (...)` run through `QueryRow`, scanned successfully):

```sql
INSERT INTO [table] ([col1],[col2]) OUTPUT INSERTED.* VALUES (@p1,@p2)
```

```sql
UPDATE [table] SET [col1]=@p1 OUTPUT INSERTED.* WHERE [col2]=@p2
```

Both single-round-trip via `QueryContext` + `collectRows` (not `ExecContext` — `OUTPUT` makes these
statements return a result set). `stmt.Insert/Update.PrimaryKey` (M16/AD-038) read but unused, same
as `driver/postgres`/`driver/sqlite`.

## Correction: `SaveOne`/`SaveMany` are NOT upsert — no `MERGE INTO` needed

An earlier draft of this design (and spec.md's P2 AC3) assumed `SaveOne`/`SaveMany` needed
`MERGE INTO`-based upsert semantics, inherited from INSIGHT.md's per-dialect "UPSERT" table entry
without cross-checking it against golem's actual implementation first (a real violation of the
Knowledge Verification Chain's own "check the codebase first" step). **Checked now**: grepping the
entire `driver/*` tree for `ON CONFLICT`/`ON DUPLICATE`/`MERGE` returns zero matches, and grepping
the whole repo for `upsert`/`Upsert`/`UPSERT` in non-test `.go` files also returns zero matches.
`repository.go`'s `SaveOne` (`repository/repository.go:543`) just builds a plain
`stmt.Update{Table, Sets, Where: <pk predicate>}` and calls `Dialect.Update` — the *exact same*
method `Repository[T].Update` already uses — then returns `golem.ErrNotFound` if zero rows matched
(the entity doesn't exist), rather than falling back to an insert. INSIGHT.md's UPSERT column is
reference material for a feature golem has never actually built, on any adapter, ever — not a
requirement M18 needs to satisfy. **`driver/mssql`'s `Update` (via `OUTPUT INSERTED.*`, see above)
is the only method `SaveOne`/`SaveMany` need — no separate `MERGE INTO` implementation, no new
`Dialect` method, and T7 (originally scoped to this) is dropped from tasks.md entirely.**

## Locking (table hints, not a trailing clause)

The one place this adapter's `CompileSelect` structurally differs from every prior one: the lock
hint attaches to the **table reference** (`FROM [table] WITH (...)`), not the end of the statement.
**All 4 hint combinations below were run inside a real transaction against the probe table and
compiled/executed without error:**

| `query.LockStrength`/`LockWait` | SQL Server table hint | Confirmed |
| --- | --- | --- |
| `Update` | `WITH (UPDLOCK, ROWLOCK)` | ✅ |
| `Share` | `WITH (HOLDLOCK, ROWLOCK)` | ✅ (compiles; semantic blocking behavior under real concurrent transactions not separately re-verified beyond M14's own established two-transaction test pattern — reasonable to trust given `HOLDLOCK` is the standard, widely-documented T-SQL idiom for this) |
| `NoKeyUpdate`, `KeyShare` | Unsupported — Postgres-specific concept, no SQL Server equivalent, same stance as `driver/mysql` | n/a |
| `NoWait` | Append `NOWAIT` inside the hint list: `WITH (UPDLOCK, ROWLOCK, NOWAIT)` | ✅ |
| `SkipLocked` | Append `READPAST` inside the hint list: `WITH (UPDLOCK, ROWLOCK, READPAST)` | ✅ |

`Capabilities.Locking` for `driver/mssql`: `Update`/`Share`/`NoWait`/`SkipLocked` `true`,
`NoKeyUpdate`/`KeyShare` `false`.

## Error mapping (`IsConflict`, `mapError`)

`go-mssqldb` exposes `mssql.Error` with a `Number int32` field — **confirmed via `errors.As` against
2 real constraint violations**:

- Unique violation: `Number=2627`, message `"Violation of UNIQUE KEY constraint '...'. Cannot
  insert duplicate key..."` → `golem.ErrDuplicateKey`. (`2601`, a narrower "duplicate key on a
  unique index" variant of the same class, is also mapped to `golem.ErrDuplicateKey` per standard
  SQL Server error documentation — not separately provoked in the probe, but same well-established
  code as `2627`.)
- FK violation: `Number=547`, message `"The INSERT statement conflicted with the FOREIGN KEY
  constraint..."` → `golem.ErrForeignKeyViolation`. Note: `547` covers *both* foreign key and
  `CHECK` constraints in real SQL Server; golem has no `CHECK` concept (AD-012/Out of Scope), so
  this is treated uniformly as `golem.ErrForeignKeyViolation` — document the caveat in the
  implementation's doc comment (same simplification `driver/mysql`/`driver/sqlite` didn't need but
  this adapter does).

`IsConflict` true for any of `2627`/`2601`/`547`.

## Transactions (`Begin`/`getExecutor`)

Identical shape to `driver/mysql`'s/`driver/sqlite`'s: `*sql.DB.BeginTx(ctx, nil)` → `*sql.Tx`,
same `getExecutor` type-assertion-on-`Underlying()` pattern.

## Connector / DSN

`go-mssqldb` DSN format: `sqlserver://user:pass@host:port?database=dbname` (confirmed via the
probe's working DSN, `sqlserver://sa:...@localhost:41433?database=master`) — URL-style,
`net/url`-parseable, unlike MySQL's bracket syntax. `resolveDSN` can likely reuse
`driver/postgres`'s `net/url`-based approach almost directly (discrete fields/DSN precedence rules
identical to every prior adapter). `Options` mirrors the established shape: `DSN`, `Host`, `Port`,
`User`, `Password`, `Database`, `Logging`, `Logger`.

## Testing

- **Unit** (`_test.go`, no build tag): `DATA-DOG/go-sqlmock`, mirroring the established Bind/Scan/
  Compile*/Insert/Update/mapError/Begin/getExecutor test list from `driver/mysql`/`driver/sqlite`.
  Target 100% (or documented exceptions, applying `internal/must`/`internal/testutil` where they
  genuinely apply, per AD-042/AD-043).
- **Integration** (`_integration_test.go`, `//go:build integration`): `connector_integration_test.go`
  + `conformance_integration_test.go` (SQL Server `Schema` DDL + `Capabilities`), calling
  `internal/dialecttest.Run`. Needs `mcr.microsoft.com/mssql/server` reachable (see README.md's M18
  note on network setups where the registry's CDN backend is blocked — Cloudflare WARP fixed it for
  one real case).
- **Docker**: `.docker/docker-compose.test.yml` gains an `mssql` service
  (`mcr.microsoft.com/mssql/server:2022-latest` — the probe used `2025-latest` for the quick check;
  confirm which version to pin for the actual CI/compose service), `ACCEPT_EULA=Y`, a strong
  `MSSQL_SA_PASSWORD`, own port. `Taskfile.yml`'s `test-integration` gains a
  `GOLEM_MSSQL_TEST_DSN`-equivalent env var.

## Confirmed via empirical probe (2026-07-06, SQL Server 2025 container)

Every item below was an open question in this design's first draft; all are now resolved facts,
not assumptions:

1. **`UNIQUEIDENTIFIER` generic-scan shape**: raw `[]byte`, mixed-endian (needs reversal) — see
   Value conversion section above for the exact byte-group transform.
2. **String→`uniqueidentifier` implicit binding**: works with a plain Go `string`, no wrapper type
   needed.
3. **`MERGE INTO ... OUTPUT INSERTED.*` exact syntax**: confirmed it compiles and works (trailing
   `;` mandatory) — moot for this milestone though, since golem's `SaveOne`/`SaveMany` turned out to
   never need upsert semantics in the first place (see the Correction section above); kept as a
   confirmed fact in case a future feature actually needs `MERGE`.
4. **Locking hint combinations**: all 4 (`UPDLOCK,ROWLOCK` / `HOLDLOCK,ROWLOCK` / `+NOWAIT` /
   `+READPAST`) compile and execute inside a real transaction with no syntax error.
5. **`mssql.Error`'s exact field**: `Number int32` (confirmed `2627` unique, `547` FK).
6. **`OFFSET/FETCH` without an explicit `ORDER BY`**: confirmed hard error
   (`mssql: Invalid usage of the option NEXT in the FETCH statement`); confirmed fixed by adding
   `ORDER BY` — validates the primary-key-injection strategy is necessary, not optional polish.
