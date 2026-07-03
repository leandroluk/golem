package golem

import "testing"

func TestBOOLEAN(t *testing.T) {
	got := BOOLEAN()
	if got.kind != "boolean" {
		t.Fatalf("expected kind == %q, got %q", "boolean", got.kind)
	}
}

func TestSMALLINT(t *testing.T) {
	got := SMALLINT()
	if got.kind != "smallint" {
		t.Fatalf("expected kind == %q, got %q", "smallint", got.kind)
	}
}

func TestINTEGER(t *testing.T) {
	got := INTEGER()
	if got.kind != "integer" {
		t.Fatalf("expected kind == %q, got %q", "integer", got.kind)
	}
}

func TestBIGINT(t *testing.T) {
	got := BIGINT()
	if got.kind != "bigint" {
		t.Fatalf("expected kind == %q, got %q", "bigint", got.kind)
	}
}

func TestDECIMAL(t *testing.T) {
	got := DECIMAL(10, 2)
	if got.kind != "decimal" {
		t.Fatalf("expected kind == %q, got %q", "decimal", got.kind)
	}
	if got.precision != 10 {
		t.Fatalf("expected precision == 10, got %d", got.precision)
	}
	if got.scale != 2 {
		t.Fatalf("expected scale == 2, got %d", got.scale)
	}
}

func TestFLOAT(t *testing.T) {
	got := FLOAT(53)
	if got.kind != "float" {
		t.Fatalf("expected kind == %q, got %q", "float", got.kind)
	}
	if got.precision != 53 {
		t.Fatalf("expected precision == 53, got %d", got.precision)
	}
}

func TestCHAR(t *testing.T) {
	got := CHAR(10)
	if got.kind != "char" {
		t.Fatalf("expected kind == %q, got %q", "char", got.kind)
	}
	if got.length != 10 {
		t.Fatalf("expected length == 10, got %d", got.length)
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

func TestDATE(t *testing.T) {
	got := DATE()
	if got.kind != "date" {
		t.Fatalf("expected kind == %q, got %q", "date", got.kind)
	}
}

func TestDATETIME(t *testing.T) {
	got := DATETIME()
	if got.kind != "datetime" {
		t.Fatalf("expected kind == %q, got %q", "datetime", got.kind)
	}
}

func TestTIME(t *testing.T) {
	got := TIME()
	if got.kind != "time" {
		t.Fatalf("expected kind == %q, got %q", "time", got.kind)
	}
}

func TestBLOB(t *testing.T) {
	got := BLOB()
	if got.kind != "blob" {
		t.Fatalf("expected kind == %q, got %q", "blob", got.kind)
	}
}

func TestUUID(t *testing.T) {
	got := UUID()
	if got.kind != "uuid" {
		t.Fatalf("expected kind == %q, got %q", "uuid", got.kind)
	}
}

func TestJSON(t *testing.T) {
	got := JSON()
	if got.kind != "json" {
		t.Fatalf("expected kind == %q, got %q", "json", got.kind)
	}
}

