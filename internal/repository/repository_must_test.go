package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/leandroluk/golem/internal/op"
	"github.com/leandroluk/golem/internal/query"
)

var assertErr = errors.New("must test: boom")

// -----------------------------------------------------------------------
// Must* variants: each wraps its non-Must counterpart 1:1 (must.Value /
// must.Exec). These tests only need to prove the delegation — success
// returns the same value, failure panics with the same error — not
// re-exercise the wrapped method's own behavior (already covered above).
// -----------------------------------------------------------------------

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic, got none")
		}
	}()
	fn()
}

func TestRepository_MustInsert(t *testing.T) {
	d := &fakeDialect{insertResult: map[string]any{"id": int64(1), "name": "Ada"}}
	repo := Get(newFakeConn(t, d), testSubjectEntity)
	got := repo.MustInsert(context.Background(), &testSubject{Name: "Ada"})
	if got.ID != 1 {
		t.Errorf("got = %+v", got)
	}

	errD := &fakeDialect{}
	errRepo := Get(newFakeConn(t, errD), valuerErrSubjectEntity)
	mustPanic(t, func() {
		errRepo.MustInsert(context.Background(), &valuerErrSubject{Field: fakeValuerErrField{x: 1}})
	})
}

func TestRepository_MustInsertMany(t *testing.T) {
	d := &fakeDialect{insertResult: map[string]any{"id": int64(1), "name": "Ada"}}
	repo := Get(newFakeConn(t, d), testSubjectEntity)
	got := repo.MustInsertMany(context.Background(), &testSubject{Name: "Ada"})
	if len(got) != 1 {
		t.Errorf("got = %+v", got)
	}

	errD := &fakeDialect{}
	errRepo := Get(newFakeConn(t, errD), valuerErrSubjectEntity)
	mustPanic(t, func() {
		errRepo.MustInsertMany(context.Background(), &valuerErrSubject{Field: fakeValuerErrField{x: 1}})
	})
}

func TestRepository_MustFindMany(t *testing.T) {
	d := &fakeDialect{selectResult: []map[string]any{{"id": int64(1), "name": "Ada"}}}
	repo := Get(newFakeConn(t, d), testSubjectEntity)
	got := repo.MustFindMany(context.Background())
	if len(got) != 1 {
		t.Errorf("got = %+v", got)
	}

	errD := &fakeDialect{selectErr: assertErr}
	errRepo := Get(newFakeConn(t, errD), testSubjectEntity)
	mustPanic(t, func() {
		errRepo.MustFindMany(context.Background())
	})
}

func TestRepository_MustFindOne(t *testing.T) {
	d := &fakeDialect{selectResult: []map[string]any{{"id": int64(1), "name": "Ada"}}}
	repo := Get(newFakeConn(t, d), testSubjectEntity)
	got := repo.MustFindOne(context.Background())
	if got == nil || got.ID != 1 {
		t.Errorf("got = %+v", got)
	}

	errD := &fakeDialect{selectErr: assertErr}
	errRepo := Get(newFakeConn(t, errD), testSubjectEntity)
	mustPanic(t, func() {
		errRepo.MustFindOne(context.Background())
	})
}

func TestRepository_MustSaveOne(t *testing.T) {
	d := &fakeDialect{updateResult: []map[string]any{{"id": int64(7), "name": "Grace", "email": ""}}}
	repo := Get(newFakeConn(t, d), testSubjectEntity)
	got := repo.MustSaveOne(context.Background(), &testSubject{ID: 7, Name: "Grace"})
	if got == nil || got.ID != 7 {
		t.Errorf("got = %+v", got)
	}

	errD := &fakeDialect{}
	errRepo := Get(newFakeConn(t, errD), valuerErrSubjectEntity)
	mustPanic(t, func() {
		errRepo.MustSaveOne(context.Background(), &valuerErrSubject{Field: fakeValuerErrField{x: 1}})
	})
}

func TestRepository_MustSaveMany(t *testing.T) {
	d := &fakeDialect{updateResult: []map[string]any{{"id": int64(7), "name": "Grace", "email": ""}}}
	repo := Get(newFakeConn(t, d), testSubjectEntity)
	got := repo.MustSaveMany(context.Background(), &testSubject{ID: 7, Name: "Grace"})
	if len(got) != 1 {
		t.Errorf("got = %+v", got)
	}

	errD := &fakeDialect{}
	errRepo := Get(newFakeConn(t, errD), valuerErrSubjectEntity)
	mustPanic(t, func() {
		errRepo.MustSaveMany(context.Background(), &valuerErrSubject{Field: fakeValuerErrField{x: 1}})
	})
}

func TestRepository_MustUpdate(t *testing.T) {
	d := &fakeDialect{updateResult: []map[string]any{{"id": int64(7), "name": "Grace", "email": "new@example.com"}}}
	repo := Get(newFakeConn(t, d), testSubjectEntity)
	got := repo.MustUpdate(context.Background(), func(t *testSubject, u *query.Update[testSubject]) {
		u.Where(op.Eq(&t.ID, int64(7)))
		u.Set(&t.Email, "new@example.com")
	})
	if len(got) != 1 {
		t.Errorf("got = %+v", got)
	}

	errD := &fakeDialect{updateErr: assertErr}
	errRepo := Get(newFakeConn(t, errD), testSubjectEntity)
	mustPanic(t, func() {
		errRepo.MustUpdate(context.Background(), func(t *testSubject, u *query.Update[testSubject]) {
			u.Where(op.Eq(&t.ID, int64(7)))
			u.Set(&t.Email, "new@example.com")
		})
	})
}

func TestRepository_MustDelete(t *testing.T) {
	d := &fakeDialect{selectResult: []map[string]any{{"id": int64(100), "name": "To Be Deleted"}}}
	repo := Get(newFakeConn(t, d), testSubjectEntity)
	got := repo.MustDelete(context.Background(), func(t *testSubject, del *query.Delete[testSubject]) {
		del.Where(op.Eq(&t.ID, int64(100)))
	})
	if len(got) != 1 {
		t.Errorf("got = %+v", got)
	}

	errD := &fakeDialect{selectErr: assertErr}
	errRepo := Get(newFakeConn(t, errD), testSubjectEntity)
	mustPanic(t, func() {
		errRepo.MustDelete(context.Background(), func(t *testSubject, del *query.Delete[testSubject]) {
			del.Where(op.Eq(&t.ID, int64(100)))
		})
	})
}

func TestRepository_MustRestore(t *testing.T) {
	d := &fakeDialect{updateResult: []map[string]any{{"id": int64(300)}}}
	repo := Get(newFakeConn(t, d), softDeleteSubjectEntity)
	repo.MustRestore(context.Background(), &softDeleteSubject{ID: 300})

	errRepo := Get(newFakeConn(t, &fakeDialect{}), testSubjectEntity)
	mustPanic(t, func() {
		errRepo.MustRestore(context.Background(), &testSubject{ID: 400})
	})
}

func TestRepository_MustCount(t *testing.T) {
	d := &fakeDialect{selectResult: []map[string]any{{"count": int64(42)}}}
	repo := Get(newFakeConn(t, d), testSubjectEntity)
	if got := repo.MustCount(context.Background()); got != 42 {
		t.Errorf("got = %d", got)
	}

	errD := &fakeDialect{selectErr: assertErr}
	errRepo := Get(newFakeConn(t, errD), testSubjectEntity)
	mustPanic(t, func() {
		errRepo.MustCount(context.Background())
	})
}

func TestRepository_MustExists(t *testing.T) {
	d := &fakeDialect{selectResult: []map[string]any{{"count": int64(1)}}}
	repo := Get(newFakeConn(t, d), testSubjectEntity)
	if got := repo.MustExists(context.Background()); !got {
		t.Errorf("got = %v", got)
	}

	errD := &fakeDialect{selectErr: assertErr}
	errRepo := Get(newFakeConn(t, errD), testSubjectEntity)
	mustPanic(t, func() {
		errRepo.MustExists(context.Background())
	})
}

func TestRepository_MustExec(t *testing.T) {
	d := &fakeDialect{selectResult: []map[string]any{{"id": int64(10), "name": "Subject A"}}}
	repo := Get(newFakeConn(t, d), testSubjectEntity)
	got := repo.MustExec(context.Background(), "SELECT * FROM testsubject WHERE id = $1", 10)
	if len(got) != 1 || got[0].ID != 10 {
		t.Errorf("got = %+v", got)
	}

	errD := &fakeDialect{execErr: assertErr}
	errRepo := Get(newFakeConn(t, errD), testSubjectEntity)
	mustPanic(t, func() {
		errRepo.MustExec(context.Background(), "SELECT * FROM testsubject")
	})
}
