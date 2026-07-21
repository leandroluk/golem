package core

import "testing"

func TestColumnType_Accessors(t *testing.T) {
	c := ColumnType{kind: "decimal", precision: 10, scale: 2, length: 255}
	if c.Kind() != "decimal" {
		t.Errorf("expected decimal, got %s", c.Kind())
	}
	if c.Precision() != 10 {
		t.Errorf("expected 10, got %d", c.Precision())
	}
	if c.Scale() != 2 {
		t.Errorf("expected 2, got %d", c.Scale())
	}
	if c.Length() != 255 {
		t.Errorf("expected 255, got %d", c.Length())
	}
}
