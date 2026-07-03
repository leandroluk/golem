package op

import "testing"

func TestEq_ReturnsConditionWithSameFieldPtrAndValue(t *testing.T) {
	someVar := 0
	cond := Eq(&someVar, 5)

	if cond.FieldPtr != any(&someVar) {
		t.Fatalf("FieldPtr = %p, want same pointer as %p", cond.FieldPtr, &someVar)
	}
	if cond.Value != 5 {
		t.Fatalf("Value = %v, want %v", cond.Value, 5)
	}
}

func TestEq_FieldPtrIdentity_DereferencedValueMatches(t *testing.T) {
	someVar := 42
	cond := Eq(&someVar, someVar)

	ptr, ok := cond.FieldPtr.(*int)
	if !ok {
		t.Fatalf("FieldPtr type = %T, want *int", cond.FieldPtr)
	}
	if ptr != &someVar {
		t.Fatalf("FieldPtr pointer identity mismatch: got %p, want %p", ptr, &someVar)
	}
	if *ptr != 42 {
		t.Fatalf("*FieldPtr = %d, want %d", *ptr, 42)
	}
}
