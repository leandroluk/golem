package repository

import (
	"context"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/query"
)

func TestRepository_FindMany_ForUpdate_OutsideTx_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		q.ForUpdate()
	})
	if err == nil {
		t.Fatal("expected error for ForUpdate outside a transaction, got nil")
	}
	if len(d.selectCalls) != 0 {
		t.Errorf("expected 0 Select calls (rejected before compiling), got %d", len(d.selectCalls))
	}
}

func newFakeTxConn(t *testing.T, d *fakeDialect) golem.Conn {
	t.Helper()
	ds := newFakeConn(t, d)
	txConn, err := d.Begin(context.Background(), ds)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	d.beginCalls = 0
	return golem.NewTx(d, txConn)
}

func TestRepository_FindMany_ForUpdate_InsideTx_PassesLockClause(t *testing.T) {
	d := &fakeDialect{}
	tx := newFakeTxConn(t, d)
	repo := Get(tx, testSubjectEntity)

	_, err := repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		q.ForUpdate()
	})
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}
	if len(d.selectCalls) != 1 {
		t.Fatalf("expected 1 Select call, got %d", len(d.selectCalls))
	}
	lock := d.selectCalls[0].Lock
	if lock == nil || lock.Strength != string(query.LockForUpdate) || lock.Wait != "" {
		t.Errorf("unexpected Lock clause: %+v", lock)
	}
}

func TestRepository_FindMany_ForShare_WithNoWait_InsideTx(t *testing.T) {
	d := &fakeDialect{}
	tx := newFakeTxConn(t, d)
	repo := Get(tx, testSubjectEntity)

	_, err := repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		q.ForShare(query.LockWaitNoWait)
	})
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}
	lock := d.selectCalls[0].Lock
	if lock == nil || lock.Strength != string(query.LockForShare) || lock.Wait != string(query.LockWaitNoWait) {
		t.Errorf("unexpected Lock clause: %+v", lock)
	}
}

func TestRepository_FindMany_NoLock_LockClauseIsNil(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindMany(context.Background())
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}
	call := d.selectCalls[0]
	if call.Lock != nil {
		t.Errorf("expected nil Lock, got %+v", call.Lock)
	}
}

func TestRepository_FindOne_ForUpdate_OutsideTx_ReturnsError(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{{"id": int64(1), "name": "Ada", "email": "ada@x.com"}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindOne(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		q.ForUpdate()
	})
	if err == nil {
		t.Fatal("expected error for FindOne+ForUpdate outside a transaction, got nil")
	}
}
