package op

import (
	"reflect"
	"testing"
)

func TestEq_ReturnsConditionWithSameFieldPtrAndValue(t *testing.T) {
	someVar := 0
	cond := Eq(&someVar, 5)

	if cond.FieldPtr != any(&someVar) {
		t.Fatalf("FieldPtr = %p, want same pointer as %p", cond.FieldPtr, &someVar)
	}
	if cond.Value != 5 {
		t.Fatalf("Value = %v, want %v", cond.Value, 5)
	}
	if cond.Op != "eq" {
		t.Fatalf("Op = %q, want %q", cond.Op, "eq")
	}
}

func TestOperators(t *testing.T) {
	var v int
	tests := []struct {
		name string
		cond Condition
		want string
	}{
		{"Gt", Gt(&v, 10), "gt"},
		{"Gte", Gte(&v, 10), "gte"},
		{"Lt", Lt(&v, 10), "lt"},
		{"Lte", Lte(&v, 10), "lte"},
		{"Like", Like(&v, "%test%"), "like"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cond.FieldPtr != &v {
				t.Errorf("expected FieldPtr to match &v")
			}
			if tt.cond.Op != tt.want {
				t.Errorf("Op = %q, want %q", tt.cond.Op, tt.want)
			}
		})
	}
}

func TestIn_ReturnsConditionWithSliceValue(t *testing.T) {
	var v int
	cond := In(&v, 1, 2, 3)

	if cond.Op != "in" {
		t.Fatalf("Op = %q, want %q", cond.Op, "in")
	}
	wantSlice := []any{1, 2, 3}
	if !reflect.DeepEqual(cond.Value, wantSlice) {
		t.Fatalf("Value = %v, want %v", cond.Value, wantSlice)
	}
}

func TestOr_Composition(t *testing.T) {
	var a, b int
	cond := Or(Eq(&a, 1), Eq(&b, 2))

	if cond.Op != "or" {
		t.Fatalf("Op = %q, want %q", cond.Op, "or")
	}
	if len(cond.SubConditions) != 2 {
		t.Fatalf("SubConditions len = %d, want 2", len(cond.SubConditions))
	}
}

func TestNot_Composition(t *testing.T) {
	var a int
	cond := Not(Eq(&a, 1))

	if cond.Op != "not" {
		t.Fatalf("Op = %q, want %q", cond.Op, "not")
	}
	if len(cond.SubConditions) != 1 {
		t.Fatalf("SubConditions len = %d, want 1", len(cond.SubConditions))
	}
}

func TestSorting(t *testing.T) {
	var a int
	asc := Asc(&a)
	desc := Desc(&a)

	if asc.FieldPtr != &a || asc.Desc {
		t.Errorf("Asc Order mismatch: %+v", asc)
	}
	if desc.FieldPtr != &a || !desc.Desc {
		t.Errorf("Desc Order mismatch: %+v", desc)
	}
}

