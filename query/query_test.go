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

func TestQuery_ChainableMethods(t *testing.T) {
	q := New[someType]()

	var a, b int
	ret := q.Where(op.Eq(&a, 1)).
		Select(&a).
		OrderBy(op.Asc(&b)).
		Limit(10).
		Offset(20).
		WithDeleted()

	if ret != q {
		t.Fatalf("chaining returned different pointer")
	}

	if len(q.Conditions()) != 1 {
		t.Errorf("expected 1 condition, got %d", len(q.Conditions()))
	}
	if len(q.SelectFields()) != 1 || q.SelectFields()[0] != &a {
		t.Errorf("SelectFields mismatch: %v", q.SelectFields())
	}
	if len(q.OrderByFields()) != 1 || q.OrderByFields()[0].FieldPtr != &b || q.OrderByFields()[0].Desc {
		t.Errorf("OrderByFields mismatch")
	}
	if q.GetLimit() == nil || *q.GetLimit() != 10 {
		t.Errorf("Limit mismatch: %v", q.GetLimit())
	}
	if q.GetOffset() == nil || *q.GetOffset() != 20 {
		t.Errorf("Offset mismatch: %v", q.GetOffset())
	}
	if !q.IsWithDeleted() {
		t.Errorf("expected IsWithDeleted to be true")
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

func TestUpdate_Chainable(t *testing.T) {
	u := NewUpdate[someType]()
	var a int

	ret := u.Where(op.Eq(&a, 1)).Set(&a, 2).WithDeleted()

	if ret != u {
		t.Fatalf("chaining returned different pointer")
	}
	if len(u.Conditions()) != 1 {
		t.Errorf("expected 1 condition")
	}
	if len(u.Sets()) != 1 {
		t.Errorf("expected 1 set")
	}
	if !u.IsWithDeleted() {
		t.Errorf("expected IsWithDeleted to be true")
	}
}

func TestNewCount_StartsWithEmptyConditions(t *testing.T) {
	c := NewCount[someType]()

	if c == nil {
		t.Fatalf("NewCount() = nil, want non-nil *Count[someType]")
	}
	if len(c.Conditions()) != 0 {
		t.Fatalf("Conditions() len = %d, want 0", len(c.Conditions()))
	}
}

func TestCount_Chainable(t *testing.T) {
	c := NewCount[someType]()
	var a int

	ret := c.Where(op.Eq(&a, 1)).WithDeleted()

	if ret != c {
		t.Fatalf("chaining returned different pointer")
	}
	if len(c.Conditions()) != 1 {
		t.Errorf("expected 1 condition")
	}
	if !c.IsWithDeleted() {
		t.Errorf("expected IsWithDeleted to be true")
	}
}

