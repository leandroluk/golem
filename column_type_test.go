package golem

import "testing"

func TestColumnType_EqualityBySameKind(t *testing.T) {
	a := ColumnType{kind: "x"}
	b := ColumnType{kind: "x"}

	if a != b {
		t.Fatalf("expected ColumnType{kind: %q} == ColumnType{kind: %q}, got not equal", a.kind, b.kind)
	}
}

func TestColumnType_ZeroValueHasEmptyKind(t *testing.T) {
	var z ColumnType

	if z.kind != "" {
		t.Fatalf("expected zero-value ColumnType.kind == \"\", got %q", z.kind)
	}
}
