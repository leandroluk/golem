# Predicate-Based Delete (M25) Tasks

**Design**: `.specs/features/predicate-based-delete/design.md`
**Status**: Done — T1-T9 complete, `task coverage` 100.0%, README/docs updated.

---

## Execution Plan

### Phase 1: Foundation (Sequential)

```
T1 → T2
```

### Phase 2: Call-site migration (Parallel OK, after T2)

```
       ┌→ T3 [P] ─┐
T2 ────┼→ T4 [P] ─┼──→ T7
       └→ T5 [P] ─┘
       T6 [P] ────┘
```

### Phase 3: Docs + real-server confirmation (Sequential, after T7)

```
T7 → T8 → T9
```

---

## Task Breakdown

### T1: Add `query.Delete[T]` builder + `golem.Delete[T]` alias

**What**: New `query.Delete[T]` type (`Where`/`WithDeleted`, mirrors `query.Count[T]`'s shape
exactly — no `Set`), plus `golem.Delete[T]` type alias in `golem.go`.
**Where**: `internal/query/query.go` (add type + `NewDelete[T]`), `golem.go` (add alias)
**Depends on**: None
**Reuses**: `query.Count[T]` (repository.go:142-160) as the structural template
**Requirement**: PBDEL-01

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `query.Delete[T]` has `Where(...op.Condition) *Delete[T]`, `WithDeleted() *Delete[T]`,
      `Conditions() []op.Condition`, `IsWithDeleted() bool`
- [ ] `query.NewDelete[T]() *Delete[T]` returns an empty builder
- [ ] `golem.go` exports `type Delete[T any] = query.Delete[T]`
- [ ] Gate check passes: `task gate-quick`
- [ ] Test count: existing suite count + new unit tests for `query.Delete[T]`'s 4 methods, all pass

**Tests**: unit
**Gate**: quick

---

### T2: Rewrite `Repository[T].Delete` to predicate-based (SELECT-then-loop)

**What**: Replace `Delete(ctx, items ...*T) error` with
`Delete(ctx, criteria func(t *T, d *query.Delete[T])) ([]T, error)`, per design.md's Data Flow
section: build `wherePred`, upfront PK guard, `CompileSelect`+`Query` to get matching rows, then
reuse the existing per-row loop (hooks → cascade → soft/hard delete → commit) fed from the raw row
maps instead of caller-supplied `*T`s.
**Where**: `internal/repository/repository.go:746` (modify `Delete`; `applyDeleteCascades`,
`beginCascadeTx`, `cascadeActionable` stay untouched)
**Depends on**: T1
**Reuses**: `r.buildWherePredicate` (repository.go:132), `r.applySoftDeleteFilter` (repository.go:156),
`r.scanRow` (repository.go:192), the `CompileSelect`+`Query` pattern already used by
`applyDeleteCascades` (repository.go:665-673), the per-row hook/cascade/delete body currently at
repository.go:754-841 (moved, not rewritten)
**Requirement**: PBDEL-01, PBDEL-02, PBDEL-03, PBDEL-04, PBDEL-05, PBDEL-06

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] New signature compiles: `func (r *Repository[T]) Delete(ctx context.Context, criteria func(t *T, d *query.Delete[T])) ([]T, error)`
- [ ] Empty match set returns `(nil, nil)`, not an error (PBDEL-02)
- [ ] No PK declared on the entity returns an error before any SELECT runs (PBDEL-06)
- [ ] Soft-delete entities get an `UPDATE ... SET <deleteDateCol> = now()` per matched row; hard-delete
      entities get a `DELETE` per matched row (PBDEL-03)
- [ ] `BeforeDelete`/`AfterDelete`/`OnConflictDelete` fire once per matched row with a fully-scanned
      `*T` (PBDEL-04)
- [ ] `OnDeleteCascade`/`OnDeleteSetNull`/`OnDeleteRestrict` still apply per matched row (PBDEL-05)
- [ ] Gate check passes: `task gate-quick`
- [ ] Test count: unit tests in `internal/repository/repository_test.go` are updated in THIS task for
      every `TestRepository_Delete_*`/`TestRepository_Restore_*`-adjacent test that calls the old
      `Delete(ctx, &instance)` shape (≈25 tests per design.md's Breaking Change section) — new call
      shape (`func(t *T, d *query.Delete[T]) { d.Where(op.Eq(&t.ID, instance.ID)) }`), same assertions
      per scenario. No test deleted without an equivalent replacement.

**Tests**: unit
**Gate**: quick

**Commit**: `feat(repository)!: Delete takes a predicate callback instead of *T instances`

---

### T3: Migrate `internal/dialecttest` conformance harness call sites [P]

**What**: Update every `repo.Delete(ctx, &x)`-shaped call inside the shared cross-adapter conformance
suite (`dialecttest.go`, `softdelete.go`, cascade-related files) to the new predicate shape.
**Where**: `internal/dialecttest/*.go`
**Depends on**: T2
**Reuses**: T2's new signature; existing conformance fixtures/schema (no schema changes needed)
**Requirement**: PBDEL-01, PBDEL-03, PBDEL-04, PBDEL-05

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Every `Delete` call in `internal/dialecttest` uses the new predicate shape
- [ ] Gate check passes: `task gate-quick`
- [ ] Test count: same conformance subtest count as before this task, all compiling and passing under
      `-short` (real-server runs happen in T9)

**Tests**: unit
**Gate**: quick

---

### T4: Migrate `driver/*` adapter-level tests that reference `Delete` [P]

**What**: Grep-confirm and fix any `Delete` call sites inside `driver/postgres`, `driver/mysql`,
`driver/sqlite`, `driver/mssql`, `driver/oracle` test files that aren't already covered by T3 (most
`Delete` usage lives in `internal/dialecttest`/`internal/repository`, but each driver package's own
`dialect_test.go` files reference `CompileDelete`, not `Repository[T].Delete` — this task's actual
job is confirming that and fixing anything found, not a large rewrite).
**Where**: `driver/{postgres,mysql,sqlite,mssql,oracle}/*_test.go`
**Depends on**: T2
**Reuses**: T2's new signature
**Requirement**: PBDEL-01

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `grep -rn "\.Delete(ctx" driver/` reviewed; every hit either already targets `CompileDelete`
      (untouched, out of scope) or is migrated to the new predicate shape
- [ ] Gate check passes: `task gate-quick`
- [ ] Test count: no regression in any `driver/*` package's unit test count

**Tests**: unit
**Gate**: quick

---

### T5: Migrate `.examples/*` narrative code + integration tests [P]

**What**: Update every `repo.Delete(ctx, &x)` call in `.examples/{postgres,mysql,mssql,oracle,sqlite}`
— both `main.go` narrative usage and `main_integration_test.go` (`TestBlogExample_DeleteCountAndExists`,
`TestBlogExample_CascadeDeleteUser_DeletesTheirPosts`, and any others found via grep) — to the new
predicate shape.
**Where**: `.examples/{postgres,mysql,mssql,oracle,sqlite}/{main.go,main_integration_test.go}`
**Depends on**: T2
**Reuses**: T2's new signature; existing example entities/schema (no schema changes needed)
**Requirement**: PBDEL-01, PBDEL-03, PBDEL-05

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Every `.Delete(ctx, ...)` call across all 5 `.examples/*` uses the new predicate shape
- [ ] Gate check passes: `task gate-quick` (integration assertions themselves confirmed in T9, real
      servers not required for this task's own gate)
- [ ] Test count: no test deleted; same assertion count per test, translated to the new call shape

**Tests**: unit
**Gate**: quick

---

### T6: Repository core-crud spec/design docs cross-reference update [P]

**What**: `.specs/features/repository-core-crud/{spec.md,design.md}` (M3's own docs) mention
`Delete(ctx, entities ...*T) error` as a historical decision record — add a pointer to this
milestone (`.specs/features/predicate-based-delete/`) so a future reader isn't misled by the stale
signature, without rewriting M3's own historical record (those docs are point-in-time, per the
project's own convention — see `relations/spec.md`'s similar "historical record" note in
ROADMAP.md's M11 section).
**Where**: `.specs/features/repository-core-crud/spec.md`, `.specs/features/repository-core-crud/design.md`
**Depends on**: None (pure docs, no code dependency)
**Reuses**: The exact "historical record, see AD-0XX" annotation pattern ROADMAP.md already uses for
M11 (`relations` section)
**Requirement**: N/A (housekeeping, not a spec requirement)

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Both files have a one-line note pointing to `predicate-based-delete/` for the current signature
- [ ] No other content in either file changed

**Tests**: none
**Gate**: build

---

### T7: README.md + docs/guides/repository.md update

**What**: Update the `Delete` reference table row and both narrative examples (README.md lines
~752/787/928 per design.md's Breaking Change section) plus `docs/guides/repository.md`'s matching
reference row to the new predicate signature and call shape.
**Where**: `README.md`, `docs/guides/repository.md`
**Depends on**: T3, T4, T5, T6
**Reuses**: The exact narrative style already used for `Update`'s README example (predicate callback
with `.Where(op.Eq(...))`)
**Requirement**: PBDEL-01

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `grep -n "Delete(ctx" README.md docs/guides/repository.md` shows only the new predicate shape
- [ ] Reference table row reads `Delete(ctx, criteria func(t *T, d *golem.Delete[T])) ([]T, error)`
- [ ] Gate check passes: `task gate-quick`

**Tests**: none
**Gate**: quick

---

### T8: Full unit gate + coverage confirmation

**What**: Run the full unit gate and `task coverage` across the whole module, fix any coverage gap
the rewritten `Delete`/new `query.Delete[T]` introduced (e.g. an error branch in the new SELECT step
not yet hit by a unit test).
**Where**: whole repo (no new files expected — fixes land in whichever file has the gap)
**Depends on**: T7
**Reuses**: existing `internal/must`/`internal/testutil` patterns if a genuinely unreachable branch
needs the same treatment M22/AD-053 already established
**Requirement**: Success Criteria — "`task coverage` stays at 100.0% total"

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `task gate-quick` green
- [ ] `task coverage` reports 100.0% total, no new exception
- [ ] Test count: full suite count recorded (baseline for T9's real-server run to match)

**Tests**: unit
**Gate**: quick

---

### T9: Real-server confirmation across all 5 adapters

**What**: Run the full integration suite (all 5 real databases) to confirm the new `Delete` behaves
identically to the old one against real servers — cascade, soft-delete, hooks, conflict detection —
per M15-M19's established discipline of never trusting mocks alone for adapter-facing behavior.
**Where**: whole repo (test execution only; fixes for anything found land back in T2/T3/T4/T5 files)
**Depends on**: T8
**Reuses**: `task test-integration` (already wires all 5 `docker-compose.test.yml` services)
**Requirement**: Success Criteria — "Every adapter's conformance suite... passes... against real
databases"

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `task test-integration` green for all 5 adapters (`TestPostgres_Conformance`,
      `TestMySQL_Conformance`, `TestSQLite_Conformance`, `TestMSSQL_Conformance`,
      `TestOracle_Conformance`) plus all 5 `.examples/*` integration suites
- [ ] Any real-server-only bug found (same category as AD-037/AD-038/AD-045/AD-046 — mocks routinely
      miss adapter-specific quirks) is fixed and re-verified before marking this task done
- [ ] Gate check passes: `task gate-full`

**Tests**: integration
**Gate**: full

---

## Parallel Execution Map

```
Phase 1 (Sequential):
  T1 ──→ T2

Phase 2 (Parallel, after T2):
    ├── T3 [P]  (internal/dialecttest)
    ├── T4 [P]  (driver/* test sweep)
    ├── T5 [P]  (.examples/*)
    └── T6 [P]  (M3 docs cross-reference — no dependency at all, can start anytime)

Phase 3 (Sequential, after T3/T4/T5/T6):
  T7 ──→ T8 ──→ T9
```

**Parallelism constraint:** T3/T4/T5/T6 touch disjoint file sets (`internal/dialecttest/*` vs.
`driver/*/*_test.go` vs. `.examples/*` vs. `.specs/features/repository-core-crud/*`) — no shared
mutable state, safe to run as 4 concurrent sub-agents. T6 has no code dependency at all and could run
in Phase 1 too; kept in Phase 2 only for scheduling convenience (all doc/call-site cleanup lands
together before README).

---

## Task Granularity Check

| Task                                                  | Scope                          | Status      |
| ------------------------------------------------------ | ------------------------------- | ----------- |
| T1: `query.Delete[T]` + alias                          | 1 type + 1 alias, 2 files       | ✅ Granular |
| T2: Rewrite `Repository[T].Delete`                     | 1 method, 1 file (+ its tests)  | ✅ Granular |
| T3: `internal/dialecttest` call sites                  | 1 package, mechanical migration | ✅ Granular |
| T4: `driver/*` test sweep                               | grep + fix, bounded scope        | ✅ Granular |
| T5: `.examples/*` call sites                            | 1 package family, mechanical    | ✅ Granular |
| T6: M3 docs cross-reference                             | 2 files, 1 line each            | ✅ Granular |
| T7: README + guide docs                                 | 2 files                         | ✅ Granular |
| T8: Coverage confirmation                                | whole-repo gate run             | ✅ Granular |
| T9: Real-server confirmation                             | whole-repo integration run      | ✅ Granular |

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows                  | Status  |
| ---- | ----------------------- | -------------------------------- | ------- |
| T1   | None                     | (start of Phase 1)               | ✅ Match |
| T2   | T1                       | T1 → T2                          | ✅ Match |
| T3   | T2                       | T2 → T3 [P]                      | ✅ Match |
| T4   | T2                       | T2 → T4 [P]                      | ✅ Match |
| T5   | T2                       | T2 → T5 [P]                      | ✅ Match |
| T6   | None                     | Shown in Phase 2 for scheduling, no arrow required | ✅ Match (explicitly noted as no-dependency) |
| T7   | T3, T4, T5, T6           | T3/T4/T5/T6 → T7                 | ✅ Match |
| T8   | T7                       | T7 → T8                          | ✅ Match |
| T9   | T8                       | T8 → T9                          | ✅ Match |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified                          | Matrix Requires | Task Says | Status |
| ---- | ------------------------------------------------------ | ---------------- | --------- | ------ |
| T1   | `internal/query` (pure Go, no I/O)                      | unit              | unit      | ✅ OK  |
| T2   | `internal/repository` (pure Go, no I/O)                 | unit              | unit      | ✅ OK  |
| T3   | `internal/dialecttest` (shared harness, no direct I/O)  | unit              | unit      | ✅ OK  |
| T4   | `driver/*` unit test files                              | unit              | unit      | ✅ OK  |
| T5   | `.examples/*` (compiles under `-short`, no live DB)     | unit              | unit      | ✅ OK  |
| T6   | `.specs/*` docs only                                    | none              | none      | ✅ OK  |
| T7   | `README.md`/`docs/*` only                               | none              | none      | ✅ OK  |
| T8   | whole repo, no new logic                                | unit              | unit      | ✅ OK  |
| T9   | whole repo, real DB connections                          | integration       | integration | ✅ OK |

**No violations.** T9 is the only integration-gated task and is correctly left un-parallelized (`Parallel-Safe: No` per TESTING.md's own note that all integration tests share one Docker Compose run).

---

## Requirement Traceability (from spec.md)

| Requirement ID | Covered by      |
| --------------- | ---------------- |
| PBDEL-01         | T1, T2, T3, T4, T5, T7 |
| PBDEL-02         | T2                |
| PBDEL-03         | T2, T3, T5        |
| PBDEL-04         | T2                |
| PBDEL-05         | T2, T3, T5        |
| PBDEL-06         | T2                |

**Coverage:** 6 total, 6 mapped to tasks, 0 unmapped.
