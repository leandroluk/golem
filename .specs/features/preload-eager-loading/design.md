# Preload / Eager Loading (M12) Design

**Spec**: `.specs/features/preload-eager-loading/spec.md`
**Status**: Approved

---

## Why a free function returning a map, not a `FindMany` option

AD-024 (decided before M11 started) already ruled out attaching preloaded rows onto a struct field. What was still open going into M12 was the exact API shape. Three were considered:

1. **A struct field** (`user.Posts []Post`) — ruled out by AD-024/AD-001 already.
2. **An out-param inside the `FindMany` criteria callback** (`preload.With(q, PostEntity, &postsByUserID, func(p *Post, pl *query.Preload[Post]) {...})`) — technically workable (closures over the outer criteria callback's parent-zero variable, mirroring how `join.Inner` resolves cross-entity field pointers), but adds a genuinely new Go idiom (mutate-via-pointer-passed-into-a-registration-call) not used anywhere else in this codebase, for a benefit (single round-trip callback registration) that doesn't actually matter — Preload always needs `items` (the already-fetched parent rows) to know which keys to filter by, so it can't run *during* `FindMany` anyway; it's fundamentally a second, sequential query.
3. **A free function taking already-fetched `items`, returning a plain map** — chosen. `func Preload[T, J any](ctx, r *Repository[T], items []T, target *entity.Entity[J], criteria ...) (map[any][]J, error)`. Matches Go's existing `join.Inner[T, J any](...)` precedent (free function, two type parameters, since a method on `Repository[T]` can't introduce a new type parameter `J`). No new builder type needed — reuses `query.Query[J]`, the exact same criteria shape `FindMany` already uses.

---

## Auto-discovering the join column (reusing M11's registry)

Rather than requiring the caller to specify `.On(fieldPtr, fieldPtr)` (which `join.Inner` needs, since a join can be on ANY column pair, not necessarily a declared FK), `Preload` requires a `ForeignKey` to already be registered between the two entities and finds it via `entity.ForeignKeysReferencing` (M11) — since preloading a relationship implies that relationship is already a real declared FK, this is a reasonable restriction that removes an entire parameter from the call.

`resolvePreloadJoin(parentMeta, childMeta entity.EntityMeta) (fkCol, parentKeyCol string, childSide bool, err error)` checks both directions:

- **`childSide = true`** ("load a parent's children", e.g. `Preload(ctx, userRepo, users, PostEntity)`): looks up `entity.ForeignKeysReferencing(parentMeta.TableName)` (who points at `T`?) and finds `target`'s table among the results. `fkCol` = the child's own FK column (`post.owner_user_id`) — always present on every fetched row. `parentKeyCol` = `parentMeta`'s PK (`user.id`) — used to pull the IN-filter's key list off `items`.
- **`childSide = false`** ("load a child's parent", e.g. `Preload(ctx, postRepo, posts, UserEntity)`): looks up `entity.ForeignKeysReferencing(childMeta.TableName)` (who points at `target`?) and finds `parentMeta`'s table among the results (i.e. `T` itself holds the FK). `fkCol` = `target`'s PK (`user.id`) — present on every fetched row (it's what's being selected). `parentKeyCol` = `T`'s own FK column (`post.owner_user_id`) — used to pull the IN-filter's key list off `items`.

In both directions, **`fkCol` is always the column present on the FETCHED (target/`J`) rows** — so it's both the IN-filter's target column AND the column used to group the returned rows into the result map (`result[row[fkCol]] = append(..., item)`). This symmetry is what let one function handle both directions without a bigger branch — the only asymmetry is which side's PK vs. FK-column supplies the IN-filter's key *values* (`parentKeyCol`, read off `items` via reflection).

`items` being empty short-circuits before any of this runs (no query, empty map) — this also means a caller can safely call `Preload` on a `FindMany` result without checking `len(...) == 0` first.

---

## Why `Eager(true)` isn't auto-wired into `FindMany` in M12

ROADMAP.md's original M12 target said `Eager(true)` should "trigger the same loading automatically when no explicit `Preload`/`With` call is present." Attempting this ran into a real wall: `FindMany`'s signature is `(ctx, criteria...) ([]T, error)` — fixed, used everywhere, can't change without breaking every existing caller. Auto-triggering `Preload` for every `Eager` FK on `T` would need to return that preloaded data through *some* channel, but:

- It can't go back onto `T` (AD-001/AD-024, same reason `Preload` itself doesn't).
- The related type varies per FK (`Eager` could be set on 2+ different FKs pointing at 2+ different entity types) — Go generics can't express "return `map[any][]J1]` and `map[any][]J2]` and ... for a statically-unknown N of distinct types" from one non-generic-per-call method.
- A stateful side-channel (`repo.LastEagerLoaded()`) was considered and rejected — not idiomatic Go, and racy/confusing under concurrent use of the same `Repository[T]` value.

**Decision:** `Eager` stays accepted/stored metadata only (as it was after M11). Callers wanting automatic-feeling eager loading call `repository.Preload` explicitly right after `FindMany`/`FindOne` — two lines, no magic, matches this project's general bias against hidden behavior. Revisit only if a compelling ergonomic win is found that doesn't hit the generics wall above.
