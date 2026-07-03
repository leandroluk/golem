# Preload / Eager Loading (M12) Specification

## Status: ✅ DONE

---

## Goals

- [x] `repository.Preload[T, J any](ctx, r *Repository[T], items []T, target *entity.Entity[J], criteria ...func(*J, *query.Query[J])) (map[any][]J, error)` — fetches every `J` row related to `items`, grouped by the join key, never attached back onto `items` (AD-001/AD-024: no navigational collection field)
- [x] The join column is discovered automatically from the FK registry M11 built (`entity.ForeignKeysReferencing`) — no manual `.On(...)` needed, works whether `target` is the FK-declaring side (loading a parent's children) or the referenced side (loading a child's parent)
- [x] `criteria` reuses the exact same `func(*J, *query.Query[J])` shape as `FindMany`'s criteria (Where/OrderBy/Limit/Offset/WithDeleted), combined (AND) with the join filter `Preload` builds internally
- [x] `Preload` honors the target entity's own soft-delete configuration (excludes soft-deleted rows by default, same as every other `Where`-capable read path)
- [x] `ForeignKeyOptions.Eager(true)` (accepted/stored since M11) is explicitly documented as NOT auto-wired into `FindMany`/`FindOne` in this pass — see design.md for why, and Todos in STATE.md

## Acceptance Criteria

1. WHEN `Preload(ctx, userRepo, users, PostEntity)` is called (Post declares `ForeignKey(&t.OwnerUserID, UserEntity)`) THEN it SHALL return `map[any][]Post]` keyed by each `User.ID`, containing that user's posts.
2. WHEN `Preload(ctx, postRepo, posts, UserEntity)` is called (same FK, reverse direction) THEN it SHALL return `map[any][]User]` keyed by each `Post.OwnerUserID` value, containing the referenced `User` row.
3. WHEN `items` is empty THEN `Preload` SHALL return an empty map and SHALL NOT issue any query.
4. WHEN no `ForeignKey` is registered between `T`'s entity and `target` (in either direction) THEN `Preload` SHALL return an error.
5. WHEN the entity `items` belong to (`T`'s) has a composite primary key THEN `Preload` SHALL return an error (mirrors M11's `ForeignKey` composite-PK-target restriction).
6. WHEN multiple `items` share the same join-key value THEN the IN-filter's key list SHALL be deduplicated (not sent as duplicate placeholders).
7. WHEN `criteria` adds a `Where`/`OrderBy`/`Limit`/`Offset` THEN it SHALL be applied to the underlying query exactly as `FindMany` would apply it, ANDed with the join filter.
8. WHEN `target`'s entity has a `DeleteDate` field and `criteria` doesn't call `.WithDeleted()` THEN soft-deleted rows SHALL be excluded.

## Success Criteria

- [x] `go test ./repository/...` — green, every new function (`Preload`, `resolvePreloadJoin`) at 100% statement coverage
- [x] `go build ./...` — no errors
- [x] `.examples/postgres-minimal-blog`: `TestBlogExample_Preload_LoadsPostsPerUser` (real Postgres, `task test-integration`) verifies loading multiple users' posts in one call
