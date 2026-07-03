package golem

import "testing"

func TestBIGINT(t *testing.T) {
	got := BIGINT()

	if got.kind != "bigint" {
		t.Fatalf("expected kind == %q, got %q", "bigint", got.kind)
	}
}

func TestVARCHAR(t *testing.T) {
	got := VARCHAR(50)

	if got.kind != "varchar" {
		t.Fatalf("expected kind == %q, got %q", "varchar", got.kind)
	}

	if got.length != 50 {
		t.Fatalf("expected length == 50, got %d", got.length)
	}
}

func TestTEXT(t *testing.T) {
	got := TEXT()

	if got.kind != "text" {
		t.Fatalf("expected kind == %q, got %q", "text", got.kind)
	}
}
