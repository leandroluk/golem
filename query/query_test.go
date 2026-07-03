package query

import (
	"testing"

	"github.com/leandroluk/golem/op"
)

type someType struct {
	A int
	B int
}

func TestNew_StartsWithEmptyConditions(t *testing.T) {
	q := New[someType]()

	if q == nil {
		t.Fatalf("New() = nil, want non-nil *Query[someType]")
	}
	if len(q.Conditions()) != 0 {
		t.Fatalf("Conditions() len = %d, want 0", len(q.Conditions()))
	}
}

func TestQuery_Where_AccumulatesAcrossMultipleCalls(t *testing.T) {
	var a, b int
	q := New[someType]()

	q.Where(op.Eq(&a, 1))
	q.Where(op.Eq(&b, 2))

	if len(q.Conditions()) != 2 {
		t.Fatalf("Conditions() len = %d, want 2", len(q.Conditions()))
	}
}

func TestQuery_Where_AccumulatesMultipleConditionsInOneCall(t *testing.T) {
	var a, b int
	q := New[someType]()

	q.Where(op.Eq(&a, 1), op.Eq(&b, 2))

	if len(q.Conditions()) != 2 {
		t.Fatalf("Conditions() len = %d, want 2", len(q.Conditions()))
	}
}

func TestQuery_Where_AccumulatesAcrossMixedCalls(t *testing.T) {
	var a, b, c int
	q := New[someType]()

	q.Where(op.Eq(&a, 1), op.Eq(&b, 2))
	q.Where(op.Eq(&c, 3))

	if len(q.Conditions()) != 3 {
		t.Fatalf("Conditions() len = %d, want 3", len(q.Conditions()))
	}
}

func TestNewUpdate_StartsWithEmptyConditionsAndSets(t *testing.T) {
	u := NewUpdate[someType]()

	if u == nil {
		t.Fatalf("NewUpdate() = nil, want non-nil *Update[someType]")
	}
	if len(u.Conditions()) != 0 {
		t.Fatalf("Conditions() len = %d, want 0", len(u.Conditions()))
	}
	if len(u.Sets()) != 0 {
		t.Fatalf("Sets() len = %d, want 0", len(u.Sets()))
	}
}

func TestUpdate_Set_AccumulatesAcrossMultipleCalls(t *testing.T) {
	var a, b int
	u := NewUpdate[someType]()

	u.Set(&a, 1)
	u.Set(&b, 2)

	if len(u.Sets()) != 2 {
		t.Fatalf("Sets() len = %d, want 2", len(u.Sets()))
	}
}

func TestUpdate_Where_AccumulatesAcrossMultipleCalls(t *testing.T) {
	var a, b int
	u := NewUpdate[someType]()

	u.Where(op.Eq(&a, 1))
	u.Where(op.Eq(&b, 2))

	if len(u.Conditions()) != 2 {
		t.Fatalf("Conditions() len = %d, want 2", len(u.Conditions()))
	}
}

func TestUpdate_Where_AccumulatesMultipleConditionsInOneCall(t *testing.T) {
	var a, b int
	u := NewUpdate[someType]()

	u.Where(op.Eq(&a, 1), op.Eq(&b, 2))

	if len(u.Conditions()) != 2 {
		t.Fatalf("Conditions() len = %d, want 2", len(u.Conditions()))
	}
}
