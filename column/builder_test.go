package column

import "testing"

func TestBuilder_Name_SetsResolvedName(t *testing.T) {
	b := (&Builder{}).Name("foo")

	if got := b.ResolvedName(); got != "foo" {
		t.Fatalf("ResolvedName() = %q, want %q", got, "foo")
	}
}

func TestBuilder_ZeroValue_ResolvedNameIsEmpty(t *testing.T) {
	b := Builder{}

	if got := b.ResolvedName(); got != "" {
		t.Fatalf("ResolvedName() = %q, want empty string", got)
	}
}

func TestBuilder_Name_ChainingReturnsSamePointer(t *testing.T) {
	b := &Builder{}
	ret := b.Name("bar")

	if ret != b {
		t.Fatalf("Name() returned pointer %p, want same pointer as receiver %p", ret, b)
	}
	if got := ret.ResolvedName(); got != "bar" {
		t.Fatalf("ResolvedName() = %q, want %q", got, "bar")
	}
}
