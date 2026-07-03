package postgres

import (
	"testing"

	golem "github.com/leandroluk/golem"
)

var _ golem.Dialect = (*dialect)(nil)

func TestDialect_Bind_ReturnsDescriptiveError(t *testing.T) {
	d := dialect{}

	value, err := d.Bind(golem.ColumnType{}, "anything")

	if err == nil {
		t.Fatal("expected non-nil error, got nil")
	}
	if value != nil {
		t.Fatalf("expected nil driver.Value, got %v", value)
	}
}

func TestDialect_Scan_ReturnsDescriptiveError(t *testing.T) {
	d := dialect{}
	var dest string

	err := d.Scan(golem.ColumnType{}, "raw", &dest)

	if err == nil {
		t.Fatal("expected non-nil error, got nil")
	}
}
