package repository

import (
	"context"
	"errors"
	"testing"

	golem "github.com/leandroluk/golem/internal/core"
	"github.com/leandroluk/golem/internal/entity"
	"github.com/leandroluk/golem/internal/stmt"
	"github.com/leandroluk/golem/internal/op"
	"github.com/leandroluk/golem/internal/query"
)

type preloadUser struct {
	ID int64
}

type preloadPost struct {
	ID      int64
	OwnerID int64
	Title   string
}

func newPreloadUserEntity(table string) *entity.Entity[preloadUser] {
	return entity.New(func(u *preloadUser, b *entity.Table) {
		b.TableName(table)
		b.Col(&u.ID, golem.BIGINT())
		b.PrimaryKey(&u.ID)
	})
}

func newPreloadPostEntity(table string, user *entity.Entity[preloadUser]) *entity.Entity[preloadPost] {
	return entity.New(func(p *preloadPost, b *entity.Table) {
		b.TableName(table)
		b.Col(&p.ID, golem.BIGINT())
		b.Col(&p.OwnerID, golem.BIGINT()).Name("owner_id")
		b.Col(&p.Title, golem.VARCHAR(50))
		b.PrimaryKey(&p.ID)
		b.ForeignKey(&p.OwnerID, user)
	})
}

func TestPreload_LoadsChildrenGroupedByParentPK(t *testing.T) {
	userEntity := newPreloadUserEntity("preload_users_1")
	postEntity := newPreloadPostEntity("preload_posts_1", userEntity)

	d := &fakeDialect{
		selectResult: []map[string]any{
			{"id": int64(10), "owner_id": int64(1), "title": "A"},
			{"id": int64(11), "owner_id": int64(1), "title": "B"},
			{"id": int64(12), "owner_id": int64(2), "title": "C"},
		},
	}
	conn := newFakeConn(t, d)
	userRepo := Get(conn, userEntity)

	users := []preloadUser{{ID: 1}, {ID: 2}}
	byUser, err := Preload(context.Background(), userRepo, users, postEntity)
	if err != nil {
		t.Fatalf("Preload: %v", err)
	}
	if len(byUser[int64(1)]) != 2 {
		t.Errorf("byUser[1] = %d posts, want 2", len(byUser[int64(1)]))
	}
	if len(byUser[int64(2)]) != 1 {
		t.Errorf("byUser[2] = %d posts, want 1", len(byUser[int64(2)]))
	}

	if len(d.selectCalls) != 1 {
		t.Fatalf("expected 1 Select call, got %d", len(d.selectCalls))
	}
	call := d.selectCalls[0]
	if call.Table != "preload_posts_1" {
		t.Errorf("table = %q, want preload_posts_1", call.Table)
	}
}

func TestPreload_LoadsParentGroupedByChildFK(t *testing.T) {
	userEntity := newPreloadUserEntity("preload_users_2")
	postEntity := newPreloadPostEntity("preload_posts_2", userEntity)

	d := &fakeDialect{
		selectResult: []map[string]any{
			{"id": int64(1)},
			{"id": int64(2)},
		},
	}
	conn := newFakeConn(t, d)
	postRepo := Get(conn, postEntity)

	posts := []preloadPost{
		{ID: 100, OwnerID: 1},
		{ID: 101, OwnerID: 2},
	}
	byPost, err := Preload(context.Background(), postRepo, posts, userEntity)
	if err != nil {
		t.Fatalf("Preload: %v", err)
	}
	if len(byPost[int64(1)]) != 1 || byPost[int64(1)][0].ID != 1 {
		t.Errorf("byPost[1] = %+v, want [User{ID:1}]", byPost[int64(1)])
	}
	if len(byPost[int64(2)]) != 1 || byPost[int64(2)][0].ID != 2 {
		t.Errorf("byPost[2] = %+v, want [User{ID:2}]", byPost[int64(2)])
	}

	call := d.selectCalls[0]
	if call.Table != "preload_users_2" {
		t.Errorf("table = %q, want preload_users_2", call.Table)
	}
}

func TestPreload_EmptyItems_ReturnsEmptyMapNoQuery(t *testing.T) {
	userEntity := newPreloadUserEntity("preload_users_3")
	postEntity := newPreloadPostEntity("preload_posts_3", userEntity)

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	userRepo := Get(conn, userEntity)

	byUser, err := Preload(context.Background(), userRepo, []preloadUser{}, postEntity)
	if err != nil {
		t.Fatalf("Preload: %v", err)
	}
	if len(byUser) != 0 {
		t.Errorf("expected empty map, got %+v", byUser)
	}
	if len(d.selectCalls) != 0 {
		t.Errorf("expected 0 Select calls for empty items, got %d", len(d.selectCalls))
	}
}

type preloadUnrelated struct {
	ID int64
}

func TestPreload_NoForeignKeyRegistered_ReturnsError(t *testing.T) {
	userEntity := newPreloadUserEntity("preload_users_4")
	unrelatedEntity := entity.New(func(u *preloadUnrelated, b *entity.Table) {
		b.TableName("preload_unrelated_4")
		b.Col(&u.ID, golem.BIGINT())
		b.PrimaryKey(&u.ID)
	})

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	userRepo := Get(conn, userEntity)

	_, err := Preload(context.Background(), userRepo, []preloadUser{{ID: 1}}, unrelatedEntity)
	if err == nil {
		t.Fatal("expected error for unrelated entities, got nil")
	}
}

type preloadCompositeParent struct {
	A int64
	B int64
}

func TestPreload_CompositePKParent_ReturnsError(t *testing.T) {
	compositeEntity := entity.New(func(p *preloadCompositeParent, b *entity.Table) {
		b.TableName("preload_composite_5")
		b.Col(&p.A, golem.BIGINT())
		b.Col(&p.B, golem.BIGINT())
		b.PrimaryKey(&p.A, &p.B)
	})
	userEntity := newPreloadUserEntity("preload_users_5")

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, compositeEntity)

	_, err := Preload(context.Background(), repo, []preloadCompositeParent{{A: 1, B: 2}}, userEntity)
	if err == nil {
		t.Fatal("expected error for composite-PK parent, got nil")
	}
}

func TestPreload_WithCriteria_AppliesExtraFilter(t *testing.T) {
	userEntity := newPreloadUserEntity("preload_users_6")
	postEntity := newPreloadPostEntity("preload_posts_6", userEntity)

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	userRepo := Get(conn, userEntity)

	_, err := Preload(context.Background(), userRepo, []preloadUser{{ID: 1}}, postEntity,
		func(p *preloadPost, q *query.Query[preloadPost]) {
			q.Where(op.Eq(&p.Title, "A"))
			q.Limit(5)
		},
	)
	if err != nil {
		t.Fatalf("Preload: %v", err)
	}

	call := d.selectCalls[0]
	logical, ok := call.Where.(stmt.Logical)
	if !ok || logical.Op != "and" || len(logical.Predicates) != 2 {
		t.Fatalf("expected logical AND (IN filter + criteria), got %+v", call.Where)
	}
	comp1, ok1 := logical.Predicates[1].(stmt.Comparison)
	if !ok1 || comp1.Column != "title" || comp1.Op != "eq" || comp1.Value != "A" {
		t.Errorf("expected criteria predicate title=A, got %+v", logical.Predicates[1])
	}
	if call.Limit == nil || *call.Limit != 5 {
		t.Errorf("expected Limit=5, got %v", call.Limit)
	}
}

func TestPreload_SoftDeleteChild_AppliesFilter(t *testing.T) {
	userEntity := newPreloadUserEntity("preload_users_7")

	type preloadSoftPost struct {
		ID        int64
		OwnerID   int64
		DeletedAt *int64
	}
	postEntity := entity.New(func(p *preloadSoftPost, b *entity.Table) {
		b.TableName("preload_posts_7")
		b.Col(&p.ID, golem.BIGINT())
		b.Col(&p.OwnerID, golem.BIGINT()).Name("owner_id")
		b.Col(&p.DeletedAt, golem.DATETIME()).Name("deleted_at")
		b.PrimaryKey(&p.ID)
		b.DeleteDate(&p.DeletedAt)
		b.ForeignKey(&p.OwnerID, userEntity)
	})

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	userRepo := Get(conn, userEntity)

	_, err := Preload(context.Background(), userRepo, []preloadUser{{ID: 1}}, postEntity)
	if err != nil {
		t.Fatalf("Preload: %v", err)
	}
}

func TestPreload_DedupesParentKeys(t *testing.T) {
	userEntity := newPreloadUserEntity("preload_users_8")
	postEntity := newPreloadPostEntity("preload_posts_8", userEntity)

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	userRepo := Get(conn, userEntity)

	users := []preloadUser{{ID: 1}, {ID: 1}, {ID: 2}}
	_, err := Preload(context.Background(), userRepo, users, postEntity)
	if err != nil {
		t.Fatalf("Preload: %v", err)
	}
	call := d.selectCalls[0]
	comp, ok := call.Where.(stmt.Comparison)
	if !ok || comp.Op != "in" {
		t.Fatalf("expected a single IN comparison (no criteria to AND with), got %+v", call.Where)
	}
	keys, ok := comp.Value.([]any)
	if !ok || len(keys) != 2 {
		t.Errorf("expected 2 deduped keys, got %+v", comp.Value)
	}
}

func TestPreload_CriteriaBadFieldPtr_ReturnsError(t *testing.T) {
	userEntity := newPreloadUserEntity("preload_users_12")
	postEntity := newPreloadPostEntity("preload_posts_12", userEntity)

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	userRepo := Get(conn, userEntity)

	var foreign int64
	_, err := Preload(context.Background(), userRepo, []preloadUser{{ID: 1}}, postEntity,
		func(p *preloadPost, q *query.Query[preloadPost]) {
			q.Where(op.Eq(&foreign, "x"))
		},
	)
	if err == nil {
		t.Fatal("expected error for a criteria field pointer that doesn't belong to the target entity, got nil")
	}
}

func TestPreload_ScanRowError_Propagates(t *testing.T) {
	userEntity := newPreloadUserEntity("preload_users_13")
	postEntity := newPreloadPostEntity("preload_posts_13", userEntity)

	d := &fakeDialect{
		selectResult: []map[string]any{
			{"id": int64(1), "owner_id": int64(1), "title": struct{}{}}, // unconvertible to string
		},
	}
	conn := newFakeConn(t, d)
	userRepo := Get(conn, userEntity)

	_, err := Preload(context.Background(), userRepo, []preloadUser{{ID: 1}}, postEntity)
	if err == nil {
		t.Fatal("expected scanRow error for unconvertible column value, got nil")
	}
}

func TestPreload_CompileSelectError_Propagates(t *testing.T) {
	userEntity := newPreloadUserEntity("preload_users_9")
	postEntity := newPreloadPostEntity("preload_posts_9", userEntity)

	wantErr := errors.New("boom: compile")
	d := &fakeDialect{selectErr: wantErr}
	conn := newFakeConn(t, d)
	userRepo := Get(conn, userEntity)

	_, err := Preload(context.Background(), userRepo, []preloadUser{{ID: 1}}, postEntity)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

func TestPreload_QueryError_Propagates(t *testing.T) {
	userEntity := newPreloadUserEntity("preload_users_10")
	postEntity := newPreloadPostEntity("preload_posts_10", userEntity)

	wantErr := errors.New("boom: query")
	d := &fakeDialect{queryErr: wantErr}
	conn := newFakeConn(t, d)
	userRepo := Get(conn, userEntity)

	_, err := Preload(context.Background(), userRepo, []preloadUser{{ID: 1}}, postEntity)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

func TestPreload_OrderBy_AppliesOrdering(t *testing.T) {
	userEntity := newPreloadUserEntity("preload_users_11")
	postEntity := newPreloadPostEntity("preload_posts_11", userEntity)

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	userRepo := Get(conn, userEntity)

	_, err := Preload(context.Background(), userRepo, []preloadUser{{ID: 1}}, postEntity,
		func(p *preloadPost, q *query.Query[preloadPost]) {
			q.OrderBy(op.Desc(&p.Title))
		},
	)
	if err != nil {
		t.Fatalf("Preload: %v", err)
	}
	call := d.selectCalls[0]
	if len(call.OrderBy) != 1 || call.OrderBy[0].Column != "title" || !call.OrderBy[0].Desc {
		t.Errorf("unexpected OrderBy: %+v", call.OrderBy)
	}
}
