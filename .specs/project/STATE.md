# State

**Last Updated:** 2026-07-03
**Current Work:** M1-M12 concluídos com sucesso (ver histórico abaixo). M12 (Preload/Eager Loading, AD-028) entregou `repository.Preload[T, J]` — descobre a coluna de junção sozinho via o registro de FK do M11, funciona nas duas direções, nunca anexa dado de volta no struct (mantém AD-001/AD-024). `Eager(true)` fica só metadata (auto-wiring bateu num limite real de generics do Go, não foi só preguiça — ver AD-028). M13 (Aggregations) é o próximo milestone.

---

## Recent Decisions (Last 60 days)

### AD-028: M12 shipped — `repository.Preload`, no `Eager` auto-wiring (a real Go-generics wall, not a choice) (2026-07-03)

**Decision:** `repository.Preload[T, J any](ctx, r *Repository[T], items []T, target *entity.Entity[J], criteria ...) (map[any][]J, error)` — a free function (mirrors `join.Inner`'s two-type-param shape), reusing M11's FK registry to auto-discover the join column in either direction (works whether `target` declares the FK or `T`'s own entity does). `ForeignKeyOptions.Eager(true)` (accepted since M11) stays metadata-only — NOT wired to automatically run inside `FindMany`/`FindOne`.
**Reason:** Auto-wiring `Eager` hit a genuine wall, not a preference: `FindMany`'s signature (`([]T, error)`) is fixed and used everywhere; the related type varies per FK (2+ `Eager` FKs on one entity could point at 2+ different types), so there's no way to return typed preloaded data through that signature without either (a) attaching to `T` (ruled out, AD-001/AD-024) or (b) a stateful side-channel method (rejected — not idiomatic, racy under concurrent reuse of one `Repository[T]`).
**Trade-off:** Less "automatic-feeling" than TypeORM's `eager: true`. Accepted: two explicit lines (`FindMany` then `Preload`) beats hidden magic, matches this project's existing bias.
**Impact:** `repository/preload.go` (new). Every new function at 100% coverage (AD-026). `.examples/postgres-minimal-blog` gets `TestBlogExample_Preload_LoadsPostsPerUser`. Full reasoning: `.specs/features/preload-eager-loading/design.md`.

### AD-027: M11 shipped — only `OnDelete` gets real cascade behavior, via a global FK registry (2026-07-03)

**Decision:** `entity.Table.ForeignKey` now takes a 3rd variadic `...*relation.ForeignKeyOptions`. Of the 9 documented options, only `OnDelete` (`Cascade`/`SetNull`/`Restrict`) changes `Repository[T]` behavior — implemented via a new package-level registry (`entity.ForeignKeysReferencing`, populated as a side effect of `entity.New` running `ForeignKey`, keyed by the TARGET/parent table since the declaration direction is child→parent, the opposite of what cascade needs to query) consulted by `Repository[T].Delete`, which wraps cascade+delete in an implicit transaction (reusing an existing `Tx` if already inside one) only when 1+ registered FK is cascade-actionable.
**Reason:** This was flagged to the user mid-design (see conversation) before implementation: `Cascade*`/`Persistence`/`OrphanedRowAction` (TypeORM semantics) all require an attached in-memory related object on the owning struct to cascade a write from/to — which AD-001/AD-024 already ruled out (no navigational collection field). `CreateForeignKeyConstraints`/`Deferrable` describe DDL-time behavior golem never emits (AD-012). `OnDelete`/`OnUpdate` are the only two that operate purely on "who references this row," needing no attached object — and only `OnDelete` had an operation (`Delete`) to hang real behavior off of; no operation currently mutates a PK value, so `OnUpdate` has nothing to trigger from yet.
**Trade-off:** 5 of 9 options (`Cascade*`, `Persistence`, `OrphanedRowAction`, `CreateForeignKeyConstraints`, `Deferrable`) are accepted/stored but functionally inert — a caller setting them gets no error but also no behavior, which could surprise someone coming from TypeORM. Mitigated by loud documentation (package doc in `relation/relation.go`, README.md inline comments, spec.md/design.md) rather than silently rejecting them, so the API shape stays a faithful superset of the original design and nothing needs to change if a future milestone finds a way to make more of them real (e.g. once M12's Preload exists, `Eager` gets wired up for real, no API break).
**Impact:** Also fixed a pre-existing bug found while implementing this: `ForeignKey`'s `target` argument was NEVER type-asserted or read before M11 — `ForeignKeyMeta` only stored `FieldName`. Now stores `ColumnName`/`TargetTableName`/`TargetPrimaryKey`/`Options` too, and panics if `target` doesn't implement `Describe() EntityMeta` or has a composite PK (composite-PK FK targets unsupported — `ForeignKey` only resolves one `fieldPtr`). New packages/functions from this milestone (`relation`, `entity.FKRegistration`+registry, `repository`'s `cascadeActionable`/`beginCascadeTx`/`applyDeleteCascades`/`countValue`) are all at 100% statement coverage per AD-026. Full design: `.specs/features/relations/design.md`.

### AD-026: 100% test coverage is a standing requirement going forward (2026-07-03)

**Decision:** Every package should reach and hold 100% statement coverage, not just "the gate is green." Triggered by discovering `join` package sat at 0% direct coverage (only exercised indirectly through `repository`'s tests and the example) while a real fan-out bug lived in it undetected.
**Reason:** User explicitly requested this after seeing the join-package coverage gap correlate with a real bug. "Gate is green" was previously read as sufficient; it wasn't, because a 0%-covered package can still be *exercised* by other packages' tests without any assertion actually pinning its behavior down.
**Trade-off:** More test-writing overhead per feature going forward; some code (e.g. thin type-erasure/registry glue) may need light refactoring to become directly testable rather than only reachable transitively.
**Impact:** Immediate: `join` package gets direct unit tests (tracked below). Going forward: every new package/feature (M11-M14) must ship with direct tests reaching 100% statement coverage in its own package, checked via `go test ./... -cover`, not inferred from other packages' tests passing.

### AD-025: M11-M14 execution order — Relations → Preload → Aggregations → Locking (2026-07-03)

**Decision:** The four items promoted from ROADMAP.md's "Future Considerations" to real milestones (`ForeignKeyOptions`+Cascade, Preload/Eager loading, Aggregations, Pessimistic locking) are built in that order.
**Reason:** User chose this order explicitly. Relations first because `Preload`/`Eager` conceptually build on top of knowing a relation's cascade/eager settings; Aggregations and Locking are both query-builder-only additions with no cross-feature dependency, left for last as the lower-risk/smaller items.
**Trade-off:** None — straightforward sequencing decision, no technical constraint forces a different order.
**Impact:** ROADMAP.md's M11-M14 sections are in this order. Each gets its own `.specs/features/*/spec.md` (+ design.md where real ambiguity exists) before implementation, same convention as M1-M10.

### AD-024: `Preload`/`With` stays a separate query-level API, not a struct field — AD-001 upheld (2026-07-03)

**Decision:** When M12 (Preload/Eager Loading) is built, related rows load into a separate structure (e.g. `map[ParentID][]Child`, or a typed paired-result returned alongside `T`), never by adding a navigational-collection field (`Posts []Post`) onto the entity struct itself. `ForeignKeyOptions.Eager(true)` (M11) means "run the same Preload/With mechanism automatically when the query doesn't call it explicitly" — it does not imply or require a struct field to populate.
**Reason:** User was asked directly ("keep AD-001, separate API" vs. "revoke AD-001, add a relation field to the struct") and deferred to whichever is most coherent. Keeping AD-001 avoids a breaking change to every already-declared entity struct and preserves the project's core "plain struct, zero magic" positioning — a struct field would need either a pointer/slice added to every entity that might ever be eager-loaded (viral, breaks existing structs) or reflection-based dynamic field injection (far more magic than the project has ever done elsewhere).
**Trade-off:** Ergonomics are a step below TypeORM's `user.posts` direct property access — callers get a side-table/paired-result instead of just reading a field off the returned `T`. Accepted: matches AD-001's original trade-off acceptance almost verbatim.
**Impact:** M12's design.md must pin down the exact `Preload`/`With` return shape before implementation (not yet decided — this AD only fixes *where it doesn't go*, not the final API surface).

### AD-023: Duplicate/FK-violation sentinels wrap the driver error, not replace it (2026-07-03)

**Decision:** `driver/postgres/dialect.go`'s `mapError(err error) error` checks `errors.As(err, &pgconn.PgError)`; on SQLSTATE `23505`/`23503` it returns `fmt.Errorf("%w: %w", golem.ErrDuplicateKey /* or ErrForeignKeyViolation */, err)` (Go 1.20+ multi-`%w`) instead of returning a bare sentinel. Applied at every point `Insert`/`Update`/`Query`/`Exec`/`ExecRaw` in the Postgres dialect surface a driver error.
**Reason:** Callers need both `errors.Is(err, golem.ErrDuplicateKey)` (branch on kind) AND `errors.As(err, &pgErr)` (inspect the real `*pgconn.PgError` — constraint name, detail, etc.) without losing either. A bare sentinel would satisfy the first but break the second.
**Trade-off:** None — this is strictly additive over the plain driver error other than message text.
**Impact:** `errors.go` gains `ErrDuplicateKey`/`ErrForeignKeyViolation`. `IsConflict` (used by the hook system, AD-007/M7) is unrelated and unchanged — it stays a broad "any class-23 violation" check for deciding whether to run `OnConflictCreate`/`OnConflictUpdate`/`OnConflictDelete` hooks, independent of the new typed-sentinel wrapping.

### AD-022: `FindByID` removido — substituído por `FindOne` + `op.Eq` (2026-07-03)

**Decision:** `Repository[T].FindByID` foi removido do package `repository`. Busca por PK passa a ser expressa como `repo.FindOne(ctx, func(t *T, q *query.Query[T]) { q.Where(op.Eq(&t.ID, id)) })`.
**Reason:** O usuário apontou que `FindOne` já cobre o caso de uso de `FindByID` sem necessidade de um método dedicado — e sem necessidade de adicionar `FindByID` à interface `golem.Dialect` (que nunca foi implementado e quebrava o build).
**Trade-off:** Sintaxe um pouco mais verbosa para o caso de busca por PK vs. um método curto e conveniente. Aceito: é consistente com o princípio do projeto de não crescer a API sem justificativa.
**Impact:** `repository.go` sem `FindByID`; `examples/postgres-minimal-blog/main.go` e `main_integration_test.go` atualizados para usar `FindOne`; `errors.go` atualizado; `golem.Dialect` não precisou de novo método.

### AD-021: M2/M3 scoped to `postgres-minimal-blog`, not the full ROADMAP.md bullet list (2026-07-03)

**Decision:** Implementing M2 (Schema Declaration) and M3 (Repository Core CRUD) now, but only the subset `examples/postgres-minimal-blog` (User 1→N Post, Post N↔N Category via PostToCategory) actually exercises: `entity.New`/`Builder` (`TableName`/`SchemaName`/`PrimaryKey`/`Col`/`ForeignKey` two-arg form), `golem.BIGINT()`/`VARCHAR`/`TEXT`, `Repository[T].Insert`/`InsertMany`/`FindByID` (single-column PK only).
**Reason:** User explicitly triggered M2/M3 implementation as a consequence of wanting this example built, not as a request to finish 100% of ROADMAP.md's M2/M3 feature lists in one pass.
**Trade-off:** `Unique`, `Index`, `CreateDate`/`UpdateDate`/`DeleteDate` (+ soft-delete filtering), `entity.Column.Default`/`.DefaultFunc`, full `relation.ForeignKeyOptions`, `SaveOne`/`SaveMany`/`UpdateOne`/`UpdateMany`/`Delete`/`Restore`/`Count`/`Exists`/`FindMany`/`FindOne`, and composite-PK `FindByID` are all deferred — see Todos below.
**Impact:** `.specs/features/schema-declaration/spec.md` and `.specs/features/repository-core-crud/spec.md` both document this scoping explicitly as a SPEC_DEVIATION from ROADMAP.md. ROADMAP.md's M2/M3 sections should be treated as "partially done" until a later continuation picks up the deferred items.

### AD-020: No `internal/stmt` AST yet — direct SQL building in `Dialect.Insert`/`FindByID` (2026-07-03)

**Decision:** Instead of building AD-016's anticipated `internal/stmt` AST + fully asymmetric `Compile*`/execute `Dialect` contract, this pass adds two simple, concrete `Dialect` methods (`Insert(ctx, conn, table, columns, values)`, `FindByID(ctx, conn, table, pkColumn, id)`) that build parameterized SQL directly.
**Reason:** The query builder is deferred in this pass, and the driving example only needs simple insert/find by ID.
**Trade-off:** When M4 (query builder) lands, `internal/stmt` will need to be built for real, and `Dialect.Insert`/`FindByID` either get subsumed into that contract or coexist as a simple-case fast path — a real (if contained) rework, accepted now to avoid guessing the AST's shape before there's a second real caller.
**Impact:** `Dialect` interface in `dialect.go` grows two methods beyond the AD-016-anticipated `Bind`/`Scan`/`CompileSelect`/`CompileDelete`/`Insert`/`Update` shape — the ones added here are simpler than AD-016's eventual `Insert`/`Update(ctx, conn, *stmt.Insert/*stmt.Update)` signatures (no `stmt.*` types exist yet). Full details in `repository-core-crud/design.md`.

### AD-019: `golem.Conn` grows `Dialect() Dialect` (2026-07-03, the FOUND-11/18-anticipated growth)

**Decision:** `Conn` interface (`conn.go`, M1) gains one exported method: `Dialect() Dialect`. `*DataSource` implements it by returning its stored `dialect` field.
**Reason:** `repository` (a separate package from `golem`) has no other way to reach the active `Dialect` given a `golem.Conn` value — M1's `spec.md` (FOUND-11) explicitly anticipated exactly this: "no speculative methods... grown incrementally as M3/M9 land."
**Trade-off:** None — this is the planned, not-speculative, growth path already documented in M1.
**Impact:** `conn.go` and `datasource.go` both modified (additive). `Conn`'s sealing (`isConn()`, unexported) is unaffected — external packages still cannot implement `Conn` themselves, they can only call `Dialect()` on a `Conn` value they were handed.

### AD-018: Test Postgres remapped to host port 55432 (2026-07-03)

**Decision:** `docker-compose.test.yml`'s Postgres service binds to host port `55432` (container-internal port stays `5432`), not the default `5432`.
**Reason:** Discovered during M1/T9 that the dev machine already runs an unrelated Postgres instance on host port 5432 (`postgres://postgres:postgres@localhost:5432/postgres`). Binding the test container to the same host port would either fail outright or, worse, silently interact with an unrelated database.
**Trade-off:** `GOLEM_TEST_DSN`'s default (in `Taskfile.yml.yml`) and any local override must use `:55432`, not `:5432` — a minor deviation from the "just use the standard port" default, but avoids ever touching a database the test suite doesn't own.
**Impact:** `Taskfile.yml.yml`'s `GOLEM_TEST_DSN` default and `driver/postgres/connector_integration_test.go`'s fallback DSN both use `:55432`. Anyone running `task test-integration` on a machine where 5432 is free is unaffected either way.

### AD-000: Implement standalone in `golem`, not inside `gox/orm` (2026-07-03)

**Decision:** The design drafted at `gox/orm/.specs` (as a subpath of the `gox` monorepo) is implemented in the standalone `golem` repo instead. Root package import path becomes `github.com/leandroluk/golem` (the module root *is* the core ORM package — no `orm` subpackage). All `orm.*` symbol references in the design (`orm.NewDataSource`, `orm.Dialect`, `orm.Conn`, `orm.ColumnType`, ...) become `golem.*`. Subpackages keep their original names (`entity`, `query`, `op`, `join`, `repository`, `relation`, `driver/postgres`), just rooted under `github.com/leandroluk/golem/...` instead of `github.com/leandroluk/gox/orm/...`.
**Reason:** User decided to build this as its own dedicated repo rather than as a `gox` monorepo member; `golem` already existed as an empty repo (README/LICENSE/CI scaffolding, `go.mod` with `pgx/v5` already vendored) intended for exactly this.
**Trade-off:** None functionally — this is a pure rename/relocation. All AD-001 through AD-017 below (carried over from `gox/orm/.specs/project/STATE.md`) still apply as-is; only import paths and the `orm.` → `golem.` prefix changed.
**Impact:** Every code example in README.md and every acceptance criterion in `.specs/features/*/spec.md` uses `golem.` instead of `orm.`. The Postgres driver dependency (`github.com/jackc/pgx/v5`) is already present in `go.mod`.

### AD-001: No `ManyToMany`/`JoinTable` relation type (2026-07-02)

**Decision:** Many-to-many relationships are modeled as a plain entity (the junction table) with two `ForeignKey`s — no dedicated relation type or `@JoinTable`-style API.
**Reason:** A junction table is exactly what the database does under the hood; hiding it behind a parallel API (like TypeORM's `@JoinTable`/`@JoinColumn`) just adds a second way to describe the same thing.
**Trade-off:** No automatic navigational collection (`question.Categories []Category` populated for free). Loading that becomes a query-level concern later (see Future Considerations in ROADMAP.md — `Preload`/`With`).
**Impact:** `entity.Table` never needs a `ManyToMany` method. `ForeignKey` covers every relation shape (one-to-one, one-to-many/many-to-one, and many-to-many via the junction entity).

### AD-002: Cyclic entity references resolved via thunk, not two-phase declaration (2026-07-02)

**Decision:** When two entities reference each other (mutual `ForeignKey`/relation), pass `func() *entity.Entity[T] { return X }` instead of `X` directly to break the Go package-init cycle.
**Reason:** The alternative (`entity.New[T]()` then a separate `.Define(func(...){...})` call) avoids the cycle too, but forces every entity to be declared in two places, which was rejected as less readable.
**Trade-off:** Slightly unusual syntax (thunk) for the rare cyclic case; unnecessary for the common non-cyclic case (pass the entity var directly, e.g. `ForeignKey(&t.OwnerUserID, UserEntity)`).
**Impact:** Only needed when entity A's builder closure references entity B and vice versa. In practice this became mostly moot after AD-001 (junction-entity pattern has no cycles: the junction references both sides, neither side references the junction back).

### AD-003: No generic `Manager` — only `Repository[T]` (2026-07-02)

**Decision:** Dropped the TypeORM-style `dataSource.manager` (entity-agnostic manager). Every CRUD/query operation goes through `Repository[T]`, always bound to one entity type.
**Reason:** A parallel "generic manager vs typed repository" API doubles the surface for no real benefit — `repository.Get(conn, Entity)` is cheap to call per entity type.
**Trade-off:** Saving multiple different entity types "in one call" (e.g. `manager.save(a, b, c)` across types) now takes one `repository.Get(...).Insert*` call per type instead of one combined call. Accepted as a rare case.
**Impact:** `Transaction` moved from `Manager` to `DataSource` directly (`dataSource.Transaction(ctx, func(tx golem.Tx) error {...})`).

### AD-004: `golem.Conn` unifies `*DataSource` and `golem.Tx` (2026-07-02)

**Decision:** `repository.Get(conn golem.Conn, e *entity.Entity[T])` and `golem.Conn.Exec` accept either `*golem.DataSource` or `golem.Tx` — both implement the same interface.
**Reason:** Without this, every call site (including inside hooks and inside `Transaction` callbacks) would need to pass both the datasource AND the tx separately, which was flagged as repetitive.
**Trade-off:** None significant — this is a pure simplification.
**Impact:** Hooks only need `conn golem.Conn` (not a wrapper `entity.Context` type, see AD-007). Inside `dataSource.Transaction`, `repository.Get(tx, Entity)` replaces `repository.Get(dataSource, Entity, tx)`.

### AD-005: Three distinct write concepts — `Insert*` / `Save*` / `Update*` (2026-07-02)

**Decision:** `Insert`/`InsertMany` = brand-new entity (no runtime instance yet). `SaveOne`/`SaveMany` = re-persist an instance you already have in runtime (e.g. from a prior `Insert`/`FindByID`). `UpdateOne`/`UpdateMany` = criteria-based `Where`+`Set` update with no instance at all.
**Reason:** These are genuinely different operations with different inputs (new data vs. existing instance vs. no instance/just criteria); collapsing them into one `Save`/`Update` was ambiguous about what each call needed.
**Trade-off:** More methods on `Repository[T]` (8 write methods instead of 2-3), but each name states its precondition unambiguously.
**Impact:** Naming rule going forward: suffix `One` always returns a single `T`, suffix `Many` always returns `[]T`. `Insert`/`FindByID` are the two exceptions that omit `One` because the base name is already unambiguous.

### AD-006: Fluent hook builder, no per-hook wrapper types (2026-07-02)

**Decision:** `entity.AddHook(Entity).BeforeCreate(fn).AfterCreate(fn)...` instead of `entity.AddHook(Entity, entity.BeforeCreateHook(fn))`.
**Reason:** Matches the builder-chain style already used elsewhere (`ForeignKeyOptions`, `JoinTableOptions` in early drafts); removes the need for a dedicated exported type (`BeforeCreateHook`, `AfterCreateHook`, ...) per hook slot.
**Trade-off:** None identified.
**Impact:** Registering the same slot twice on the same entity must panic (naming the slot) — noted as a requirement for M7, not yet a design detail (error message format TBD).

### AD-007: Hooks take `(ctx, *T, golem.Conn)` directly — no `entity.Context` wrapper (2026-07-02)

**Decision:** Hook signature is `func(ctx context.Context, i *T, conn golem.Conn) error`. An earlier `entity.Context` interface (wrapping `Context()`/`Tx()`/later `DataSource()`) was designed and then removed.
**Reason:** Once `golem.Conn` existed (AD-004), the wrapper added no value — `conn` alone is enough to build a `repository.Get(conn, OtherEntity)` inside a hook, and `ctx` is idiomatically a separate, explicit parameter (matches every other method in the API, e.g. `repo.Save(ctx, ...)`).
**Trade-off:** None — straightforward simplification.
**Impact:** One less exported type in `entity` package.

### AD-008: `Unique`/`Index` live on `Builder` (table scope), not chained on `Col` (2026-07-02)

**Decision:** `b.Unique(&t.Email)` and `b.Index(&t.OwnerUserID)` are `entity.Table` methods (like `PrimaryKey`), not `.Unique()`/`.Index()` chained off `Col(...)`.
**Reason:** Both can be composite (span more than one column), so they can't be scoped to a single `Col` call.
**Trade-off:** None.
**Impact:** `entity.Column` (the `Col` chain) only has single-column concerns: `.Name`, `.Nullable`, `.Default`, `.DefaultFunc`.

### AD-009: `CreateDate`/`UpdateDate` always auto-fill — no `.AutoNow()` toggle (2026-07-02)

**Decision:** Removed `.AutoNow()`. Marking a field via `CreateDate`/`UpdateDate` always means "fill this with the operation's timestamp automatically."
**Reason:** There's no realistic case where you'd mark a field as "created at" and NOT want it auto-filled — the toggle was dead weight, always called together with the marker in every example.
**Trade-off:** None.
**Impact:** `b.CreateDate(&t.CreatedAt)` / `b.UpdateDate(&t.UpdatedAt)`, full stop.

### AD-010: ~~Column types live on the dialect package (`postgres.BIGINT()`, etc.)~~ — SUPERSEDED by AD-015 (2026-07-02, superseded 2026-07-03)

**Decision:** No separate generic `conector`/`connector` types package — `postgres.BIGINT()`, `postgres.VARCHAR(50)`, etc. live directly on the adapter.
**Reason:** Early drafts referenced an undefined `conector.*` package inconsistently; consolidating on the adapter package removed the dangling import and the "which package has the types" ambiguity.
**Trade-off:** Column type constructors aren't portable across dialects without a rewrite if a second adapter (MySQL, etc.) is added later — accepted, revisit only if/when a second adapter is actually built.
**Impact:** Every entity example imports `postgres "github.com/leandroluk/golem/driver/postgres"` for both connection options AND column types.
**Why superseded:** user decided upfront (before M2 shipped any code) to support multiple dialects (MySQL, MSSQL, Oracle) properly instead of waiting for "if/when a second adapter is actually built" — see AD-015.

### AD-011: `query.Count[T]`, `query.Join[T]`, `query.Update[T]` are distinct from `query.Query[T]` (2026-07-02)

**Decision:** Each write/read shape gets its own criteria type instead of reusing `query.Query[T]` (which has `Select`/`Where`/`OrderBy`/`Limit`/`Offset`) for everything.
**Reason:** `Count`/`Exists` only need `Where`. `Update` needs `Where`+`Set`. `Join` needs `On` (column-to-column) plus its own `Where` (column-to-value) — reusing `op.Eq` for both column-to-column and column-to-value comparisons would be ambiguous, so `Join` got a dedicated `.On(fieldPtr, fieldPtr)`.
**Trade-off:** More types to document, but each is minimal and matches exactly what that operation needs.
**Impact:** `op.*` (e.g. `op.Eq`, `op.Gte`) is always "column vs. literal value"; `.On(...)` is always "column vs. column." Never conflate the two.

### AD-012: Soft delete filters by default; Migrations explicitly out of scope (2026-07-02)

**Decision:** Any entity with `DeleteDate` declared gets automatic `WHERE deleted_at IS NULL`-style filtering on every `Where`-capable builder (`Query`/`Count`/`Update`/`Join`), opt-out via `.WithDeleted()`. Separately, schema migrations/synchronization were declared out of scope for `golem` entirely.
**Reason:** Matches standard ORM soft-delete behavior (TypeORM does the same). Migrations were cut because the user already uses Liquibase externally and successfully — building a competing migration system is large, separate scope with no clear benefit over an existing mature tool.
**Trade-off:** None for soft-delete. For migrations: `golem` entities are runtime mapping only, never a DDL source of truth — anyone using this module needs a separate schema-management tool.
**Impact:** `Delete()`/`Restore()` on `Repository[T]` exist specifically to pair with `DeleteDate`. No `Synchronize()`/migration-runner will ever be built into this module.

### AD-013: `golem.Conn.Exec` returns `golem.Result`, not `[]map[string]any` (2026-07-02)

**Decision:** Raw SQL execution (`Exec`) returns a `golem.Result` interface (`Next() bool`, `Scan() (map[string]any, error)`, `RowsAffected() (int64, error)`), not a plain `[]map[string]any`.
**Reason:** Different statements return fundamentally different things — a `SELECT` has rows to iterate, a plain `UPDATE` (no `RETURNING`) only has an affected-row count, and `UPDATE ... RETURNING` has both. A single `[]map[string]any` return can't represent the "just a count, no rows" case cleanly.
**Trade-off:** One more type to learn (`golem.Result`) vs. a bare slice, but it mirrors the already-familiar `database/sql.Rows` iteration pattern.
**Impact:** `Repository[T].Exec` is unaffected — it always has a known destination type `T`, so it returns `([]T, error)` directly, no `Result` involved.

### AD-014: `FindMany`/`FindOne` naming, `One`/`Many` suffix convention (2026-07-02)

**Decision:** Settled on `FindMany` (not `Find` or `FindAll`) paired with `FindOne`. General rule going forward: any method ending in `One` returns a single `T`; any method ending in `Many` returns `[]T`.
**Reason:** `FindAll`/`SaveMany` naming was inconsistent early on (mixed with a bare `Find`); `Many` was chosen over `All` because these methods operate on exactly the entities/criteria you passed, not literally "the whole table" (calling it `All` when it might return one row would be misleading).
**Trade-off:** None.
**Impact:** Applies to `InsertMany`, `SaveOne`/`SaveMany`, `UpdateOne`/`UpdateMany`, `FindOne`/`FindMany`. `Insert`, `FindByID`, `Delete`, `Restore`, `Count`, `Exists` are the accepted exceptions (base name already unambiguous about cardinality).

### AD-015: Dialect-agnostic `golem.ColumnType` + `golem.Dialect` contract (2026-07-03, supersedes AD-010)

**Decision:** Column type constructors (`golem.BIGINT()`, `golem.VARCHAR(50)`, `golem.TEXT()`, `golem.BOOLEAN()`, `golem.TIMESTAMPTZ()`, `golem.UUID()`, `golem.JSON()`, etc.) live in the core `golem` package, not on the adapter. Each adapter (`postgres`, and future `mysql`/`mssql`/`oracle`) implements a `golem.Dialect` interface (`Bind(t ColumnType, value any) (driver.Value, error)`, `Scan(t ColumnType, raw any, dest any) error`) that knows how to marshal/unmarshal each semantic type for that specific driver.
**Reason:** User wants real multi-database support (MySQL, MSSQL, Oracle) to be architecturally possible from the start, not deferred until "a second adapter is actually built" (AD-010's original stance). Since `golem` never generates DDL (migrations are out of scope, AD-012), the column type was never really about schema — it's about telling the adapter how to bind/scan values for types `database/sql` can't infer alone (UUID, JSON/JSONB, arrays, ENUMs, etc.), and that lookup can be keyed by a dialect-agnostic semantic type just as well as a dialect-specific one.
**Trade-off:** Adding a new adapter is still real work (SQL generation differs a lot per dialect — placeholders, pagination, upsert, `RETURNING` vs `OUTPUT` vs `RETURNING INTO`), but the type system itself and every entity declaration are portable across adapters without changes. MySQL/SQLite are "moderate effort" adapters (closer to ANSI SQL); MSSQL/Oracle are "high effort" (syntax diverges more).
**Impact:** Every `Col(fieldPtr, type)` call in `entity.Table` now takes `golem.ColumnType` instead of an adapter-specific type. All README examples updated (`postgres.BIGINT()` → `golem.BIGINT()`, etc. — the `postgres` import is still needed for `postgres.New`/`postgres.Options`, just not for column types anymore). A rejected alternative was a separate `golem/type` package — impossible, `type` is a Go reserved keyword and can't be a package name.

**Considered follow-up (not yet decided):** exact initial `golem.ColumnType` set beyond what README examples use (`BIGINT`, `INT`, `VARCHAR`, `TEXT`, `BOOLEAN`, `TIMESTAMPTZ`, `UUID`, `JSON`) — grows on demand (YAGNI), needed before M2 implementation starts.

### AD-016: `internal/stmt` AST + asymmetric `Dialect` statement contract (2026-07-03, extends AD-015)

**Decision:** `golem.Dialect` grows beyond `Bind`/`Scan` to also cover statement generation, via a shared internal AST (`golem/internal/stmt`: `stmt.Select`, `stmt.Insert`, `stmt.Update`, `stmt.Delete`) that `query.Query[T]`/`query.Update[T]`/`query.Count[T]`/`query.Join[T]`/`Repository[T]` build internally and hand to the active `Dialect`. The contract is **asymmetric by statement kind**, not 4 symmetric `Compile*` methods:
  - `CompileSelect(s *stmt.Select) (sql string, args []any, err error)` — pure compile, always 1 round-trip (used by `FindMany`/`FindOne`/`FindByID`/`Count`/`Exists`/joins).
  - `CompileDelete(s *stmt.Delete) (sql string, args []any, err error)` — pure compile, always 1 round-trip, no row data needed (`Delete`/`Restore` only return `error`, never `T`).
  - `Insert(ctx context.Context, conn golem.Conn, s *stmt.Insert) (map[string]any, error)` — **execute**, not compile. The adapter decides internally how many round-trips it needs.
  - `Update(ctx context.Context, conn golem.Conn, s *stmt.Update) ([]map[string]any, error)` — same reasoning as `Insert`.
**Reason:** `RETURNING` (or equivalent) is not universal. Postgres/Oracle/MSSQL have a native one-round-trip way to get the affected row back (`RETURNING`, `RETURNING INTO`, `OUTPUT`). MySQL does not — getting the full row back after an `INSERT`/`UPDATE` (required because `Insert`/`InsertMany`/`SaveOne`/`SaveMany`/`UpdateOne`/`UpdateMany` all return `T`/`[]T` per AD-005) needs a second `SELECT` there. A pure `Compile → (sql, args)` contract can't express "maybe 2 queries," so `Insert`/`Update` must be full `Execute`-style methods where the adapter owns the round-trip strategy. `Select`/`Delete` never need this (no dialect lacks a working `SELECT`, and `Delete`/`Restore` don't return row data at all), so they stay pure-compile.
**Trade-off:** `Dialect` is no longer 4 symmetric methods — slightly more surface to document, but each method's shape now matches what it actually needs to do per dialect instead of forcing every dialect through the same (Postgres-shaped) assumption.
**Impact:** `stmt.*` types are internal (`golem/internal/stmt`), not part of the public API — end users never construct them directly, only `query.*`/`Repository[T]` (via `golem`) and adapters touch them. Side effect: since `Dialect`'s method signatures reference internal types, only adapters built inside the `golem` module tree can implement it — acceptable, since that was already true in practice (nobody outside the module was expected to write a conforming adapter anyway). `stmt.*` grows incrementally per milestone (M1: minimal `Select`/`Insert`/`Update`/`Delete` skeletons with table ref + PK-equality `Where` only — PK-equality is AND of 1+ column=value checks from day one, since composite PKs already exist, e.g. `QuestionToCategory`; M4: full arbitrary predicate tree; M5: `Set` clause on `stmt.Update` for criteria-only updates; M6: `Join` on `stmt.Select`), not fully speced upfront.

### AD-017: `op.Not(condition)` composes over any condition — no dedicated negated variants (2026-07-03)

**Decision:** There is no `op.NotIn`, `op.NotEq`, `op.NotLike`, etc. Negation is always `op.Not(op.In(...))`, `op.Not(op.Eq(...))`, `op.Not(op.Like(...))` — one composable `Not` wrapping any other condition (including `Or(...)`).
**Reason:** `NOT (x IN (list))` and `x NOT IN (list)` are semantically identical in SQL (same for other operators) — enumerating a negated variant for every comparison operator doubles the `op` package surface for zero functional gain.
**Trade-off:** None — this is a pure surface reduction, not a capability trade-off.
**Impact:** `op` package only needs positive comparison operators (`Eq`, `Gt`, `Gte`, `Lt`, `Lte`, `In`, `Like`, ...) plus `Or`/`Not` for composition. The exact positive operator list is still open (see Todos).

---

## Active Blockers

None.

---

## Lessons Learned

### L-003: `.examples/` never actually runs under `go test ./...` — a real bug sat undetected (2026-07-03)

**Context:** While validating M10, ran `.examples/postgres-minimal-blog`'s full integration suite directly (explicit path) instead of via `task test-integration`, and `TestBlogExample_FullFlow` failed: a `join.Inner` query returned 2 rows for 1 user (fan-out from 2 matching posts, no dedup) — a pre-existing bug, unrelated to M10.
**Problem:** Go's `./...` pattern silently skips dot-prefixed directories (`.examples`, `.docker`, `.specs`, etc.) in `go build`/`go test`/`go vet`. `task gate-quick`/`task gate-full`/`task test-integration` all invoke `go test ... ./...`, so `.examples/postgres-minimal-blog`'s tests have *never* run as part of any gate — every milestone from M4 onward was declared "done" partly on the strength of an example test suite that silently never executed.
**Prevents:** Don't trust `task gate-full`/`test-integration` alone to have covered `.examples/` without checking — the `...` wildcard skips dot-prefixed dirs even when spelled out explicitly (`./.examples/...` also matches nothing; confirmed by testing it directly). **Fixed:** `Taskfile.yml`'s `test-integration` now has a second `go test -tags=integration ./.examples/postgres-minimal-blog` step using the exact (non-wildcard) package path, and `repository.FindMany` now dedupes 1:N join fan-out by parent PK (see Todos) — `TestBlogExample_FullFlow` passes for real now.

### L-002: Filenames in this repo are snake_case; the "unauthorized rename" I fought was actually the user's own convention (2026-07-03, corrected same day)

**Context:** During M2/M3 execution, files kept turning up renamed to snake_case (`columntype.go`→`column_type.go`, `datasource.go`→`data_source.go`) while sub-agents worked in the shared working tree. I assumed this was sub-agent hallucination (models sometimes "correct" perceived naming inconsistencies) and reverted it twice, and recorded a now-corrected lesson blaming sub-agents for it.
**Correction:** The user clarified they were the one renaming files themselves (for consistency) while work was in progress, and separately moved `testdata/` + `docker-compose.test.yml` into a new `.docker/` directory. Neither was agent misbehavior.
**Convention going forward:** This repo uses **snake_case filenames** (`column_type.go`, `data_source.go`, `data_source_test.go`, etc.) — NOT the no-underscore style (`columntype.go`) I initially assumed and defended in sub-agent prompts. Docker/test infra lives under `.docker/` (`.docker/docker-compose.test.yml`, `.docker/testdata/schema.sql`), tracked in git (removed from `.gitignore`).
**Prevents:** Don't assume a repo-wide naming convention from a handful of early files without checking for concurrent user edits; when files change unexpectedly mid-session, ask before reverting rather than fighting it twice. The underlying practice from the original (wrong) lesson still stands, though: run `git status --short` after every sub-agent task before staging, to catch anything genuinely unintended (scope creep, stray scratch files) — just don't assume renames are automatically agent-caused.

### L-001: Direct README iteration worked better than upfront brainstorming for this feature (2026-07-02)

**Context:** Started with the `superpowers:brainstorming` flow, but the user redirected almost immediately to editing the design README directly and iterating example-by-example.
**Problem:** A formal spec-first pass would have required guessing at API shapes (`Manager` vs `Repository`, `ManyToMany` vs junction entity, `Save` vs `Insert`/`Update` split) that only became clear by writing concrete code examples and having the user react to them.
**Solution:** Let the README's worked examples be the spec. Each iteration surfaced one concrete design question (naming, cyclic refs, error handling) that was resolved before moving to the next example.
**Prevents:** Don't force a heavyweight spec process when the user is already iterating on a living example document — formalize into `.specs/` once the shape has stabilized (this is what M1-M10 in ROADMAP.md now capture).

---

## Deferred Ideas

- [ ] `Preload`/`With` query helper for eager-loading related rows (e.g. `question.Categories`) without a dedicated relation type — Captured during: many-to-many design (AD-001)
- [ ] Aggregations (`GroupBy`/`Sum`/`Avg`/`Having`) on `query.Query[T]` — Captured during: Query Builder design
- [ ] Pessimistic locking (`SELECT ... FOR UPDATE`) — Captured during: "what's missing for a full ORM" review
- [ ] Named type `entity.Ref[T] = func() *entity.Entity[T]` for the cyclic-reference thunk (AD-002), purely cosmetic — Captured during: many-to-many design, before AD-001 made most cycles moot
- [ ] Configurable transaction isolation level on `dataSource.Transaction(ctx, fn)` (currently always the driver/DB default — no option to request e.g. `SERIALIZABLE`/`REPEATABLE READ`) — Captured during: "will this scale to other databases" review (M8 territory)
- [ ] Oracle identifier length constraint (historically 30 bytes pre-12.2, 128 from 12.2+) — no validation/truncation planned yet; whoever builds the Oracle adapter needs to decide whether to validate table/column names at entity-registration time or let the driver error surface as-is — Captured during: "will this scale to other databases" review (Oracle-adapter territory, not blocking Postgres/M1-M10)

---

## Todos

- [x] ~~Decide the exact panic message format for duplicate hook slot registration~~ — DONE
- [x] ~~Design the exact `op.*` positive comparison operator set~~ — DONE
- [x] ~~Design the initial `golem.ColumnType` set~~ — DONE
- [x] ~~M3 continuation: Delete/Restore, Count/Exists~~ — DONE
- [x] ~~Build `internal/stmt` AST for real~~ — DONE
- [x] ~~Fix `join.Inner`/`FindMany` fan-out: a 1:N join duplicates the parent row once per matched child row instead of deduplicating by PK~~ — DONE (`repository.FindMany` dedupes by `r.meta.PrimaryKey` via `pkRowKey`, gated on `len(q.Joins()) > 0` so no-join queries are unaffected)
- [x] ~~Make `.examples/postgres-minimal-blog`'s integration tests actually run under `task test-integration`~~ — DONE (Taskfile.yml's `test-integration` now runs `go test -tags=integration ./.examples/postgres-minimal-blog` as a separate explicit-path step, since go's `...` wildcard skips dot-prefixed dirs even when given explicitly)
- [ ] `OnUpdate` cascade (relation.OnUpdateCascade/SetNull/Restrict) is accepted/stored (AD-027) but not wired to any Repository[T] operation — no operation currently mutates a PK value to trigger it from. Revisit if/when a real use case for mutable PKs shows up.
- [ ] `Eager(true)` auto-wiring into `FindMany`/`FindOne` (AD-028) — not a bug, a real Go-generics limitation (see design.md). Revisit only if a design is found that doesn't hit it.
- [ ] Legacy per-package coverage gaps below 100% (AD-026 applies going forward, not retroactively): `golem` root 52.1%, `driver/postgres` 54.3%, `entity` 75.3% (mostly untested M7 hook Trigger* methods, 0%), `query` 75.0%, `repository` 76.0% (`SaveMany`/`UpdateMany` at 0%). Not blocking M13+, but should be backfilled eventually.


