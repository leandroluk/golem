# Predicate-Based Delete (M25) Specification

**Driving use case:** caller wants to delete a row knowing only its PK (or any other filterable
condition) — e.g. `userRepo.Delete(ctx, func(u *User, d *golem.Delete[User]) { d.Where(op.Eq(&u.ID, id)) })`
— without constructing/loading a full `*T` first.

## Problem Statement

`Repository[T].Delete(ctx, items ...*T)` (M3/M11) requires the caller to already hold a `*T`
instance. In practice the caller usually only has the PK value (from a request body, a URL param,
another query) — not a fully populated entity. Every other criteria-capable operation on
`Repository[T]` (`FindMany`, `FindOne`, `Update`, `Count`, `Exists`) already takes a
`func(t *T, x *query.X[T])` criteria callback instead of a pre-built instance. `Delete` is the one
write path still shaped like `SaveOne`/`SaveMany` (which legitimately need a full instance, since
they persist its non-PK field values) even though `Delete` never reads any non-PK field.

## Goals

- [ ] `Repository[T].Delete` accepts a `func(t *T, d *golem.Delete[T])` criteria callback (same shape
      as `Update`'s `func(t *T, u *golem.Update[T])`), not `items ...*T`
- [ ] Deleting by PK only requires `d.Where(op.Eq(&t.ID, id))` — no struct literal needed
- [ ] Every existing per-row guarantee is preserved for every row the predicate matches:
      `BeforeDelete`/`AfterDelete`/`OnConflictDelete` hooks fire with a real, fully-populated `*T`;
      `OnDelete` cascade (Cascade/SetNull/Restrict, M11) still runs per matched row; soft-delete
      (`DeleteDate`) still applies instead of a hard delete when the entity declares it
- [ ] Zero matching rows is not an error (same precedent as `Update`, AD-031)

## Out of Scope

| Feature                                              | Reason                                                                                                    |
| ----------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| A separate `DeleteMany`/bulk single-statement DELETE  | Would skip per-row hooks/cascade — a different, weaker guarantee than what `Delete` promises today; not requested |
| Changing `Restore`'s signature                        | `Restore` already only needs PK fields and this spec doesn't touch it; a future pass can reconsider it separately |
| Changing hook signatures                              | `BeforeDelete`/`AfterDelete`/`OnConflictDelete` keep `func(ctx, *T, golem.Conn) error` — only what feeds `*T` into them changes |

## User Stories

### P1: Delete by PK without an instance ⭐ MVP

**User Story**: As a caller who only has an entity's PK (e.g. from an HTTP route param), I want to
delete that row without first constructing or fetching a `*T`, so that deletion is as ergonomic as
`FindOne`/`Update`.

**Why P1**: This is the entire problem statement — the current API forces an unnecessary
instance/struct-literal step for the single most common delete shape (delete by ID).

**Acceptance Criteria**:

1. WHEN `repo.Delete(ctx, func(t *T, d *golem.Delete[T]) { d.Where(op.Eq(&t.ID, id)) })` is called
   AND exactly one row matches THEN system SHALL delete (or soft-delete, if `DeleteDate` is
   declared) that row and return it in the result slice.
2. WHEN the predicate matches zero rows THEN system SHALL return an empty slice and a nil error (not
   `golem.ErrNotFound` — mirrors `Update`'s "zero rows affected is not an error", AD-031).
3. WHEN the predicate matches N>1 rows THEN system SHALL process every matched row (hooks + cascade +
   delete/soft-delete each), returning all N in the result slice.
4. WHEN no `.Where(...)` is called at all THEN system SHALL match every row in the table (same
   "empty predicate = no filter" semantics `Update`/`FindMany` already have) — callers relying on
   today's "must pass a fully-keyed instance" safety net lose that implicit guard; documented as a
   deliberate, not accidental, consequence (see design.md's "unbounded predicate" note).

**Independent Test**: Insert 3 rows, call `Delete` with a predicate matching 1 of them, assert only
that row is gone and the other 2 remain.

---

### P1: Soft delete still applies per matched row ⭐ MVP

**User Story**: As a caller with an entity that declares `DeleteDate`, I want predicate-based
`Delete` to still soft-delete (not hard-delete) every matched row, exactly like today.

**Why P1**: Soft delete is a load-bearing guarantee (M12) — regressing it silently would be a data-loss-shaped bug.

**Acceptance Criteria**:

1. WHEN the entity declares `DeleteDate` AND the predicate matches 1+ rows THEN system SHALL `UPDATE`
   the `DeleteDate` column to now() for each matched row instead of issuing a `DELETE`.
2. WHEN the entity does NOT declare `DeleteDate` THEN system SHALL hard-delete each matched row, same
   as today.

**Independent Test**: Reuses the existing `TestRepository_Delete_SoftDelete`/`TestRepository_Delete_HardDelete`
fixtures, adapted to the new predicate call shape.

---

### P1: Hooks and cascade still run per matched row ⭐ MVP

**User Story**: As an entity author who registered `BeforeDelete`/`AfterDelete`/`OnConflictDelete`
hooks or an `OnDelete` cascade rule, I want predicate-based `Delete` to keep firing them exactly as
before, once per row actually deleted.

**Why P1**: These are the reason `Delete` can't become a single bulk `DELETE ... WHERE` statement —
losing them silently would be the worst kind of regression (correct-looking code, wrong behavior).

**Acceptance Criteria**:

1. WHEN `Delete`'s predicate matches N rows THEN system SHALL call `BeforeDelete` once per row, with a
   `*T` populated from that row's actual column values (not a zero-valued/PK-only stub).
2. WHEN a row's delete violates an `OnDeleteRestrict` FK rule THEN system SHALL stop processing (same
   fail-fast behavior as today) and return `golem.ErrForeignKeyViolation`.
3. WHEN a row's delete hits a conflict THEN system SHALL call `OnConflictDelete` for that row, same as
   today.
4. WHEN `AfterDelete` is registered THEN system SHALL call it once per successfully deleted row.

**Independent Test**: Reuses the existing `TestRepository_Delete_HooksAndErrors`,
`TestRepository_Delete_OnDelete*` test families, adapted to the new predicate call shape — same
assertions, different call site.

---

## Edge Cases

- WHEN the entity has no primary key declared at all THEN system SHALL return an error before
  running any SELECT/DELETE (same guard as today's `TestRepository_Delete_NoPK`, moved from
  per-item to a single upfront check).
- WHEN the underlying SELECT (matching the predicate) errors THEN system SHALL return that error
  wrapped, with no partial deletes attempted.
- WHEN `.WithDeleted()` is called on the `Delete` builder THEN system SHALL include already
  soft-deleted rows in the match set (consistent with every other `.WithDeleted()`-capable builder).

---

## Requirement Traceability

| Requirement ID | Story                                    | Phase  | Status  |
| --------------- | ----------------------------------------- | ------ | ------- |
| PBDEL-01        | P1: Delete by PK without an instance      | Design | Pending |
| PBDEL-02        | P1: Zero matches is not an error          | Design | Pending |
| PBDEL-03        | P1: Soft delete still applies             | Design | Pending |
| PBDEL-04        | P1: Hooks still run per matched row       | Design | Pending |
| PBDEL-05        | P1: Cascade (OnDelete) still runs         | Design | Pending |
| PBDEL-06        | Edge: no PK declared → upfront error      | Design | Pending |

**Coverage:** 6 total, 0 mapped to tasks yet, 6 unmapped ⚠️ (see tasks.md)

---

## Success Criteria

- [ ] `Repository[T].Delete`'s signature is `func(ctx, criteria func(t *T, d *golem.Delete[T])) ([]T, error)`
- [ ] Every adapter's conformance suite (`internal/dialecttest`, all 5 adapters:
      Postgres/MySQL/SQLite/MSSQL/Oracle) passes unmodified in intent (same guarantees, new call
      shape) — cascade/soft-delete/hook subtests all green against real databases
- [ ] `task coverage` stays at 100.0% total
- [ ] README.md + `docs/guides/repository.md` + all `.examples/*` updated to the new call shape, no
      stale `Delete(ctx, &entity)` examples left anywhere in the repo
