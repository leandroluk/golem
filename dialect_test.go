package golem

import (
	"database/sql/driver"
	"testing"
)

type fakeDialect struct{}

func (fakeDialect) Bind(t ColumnType, value any) (driver.Value, error) { return value, nil }
func (fakeDialect) Scan(t ColumnType, raw any, dest any) error         { return nil }

var _ Dialect = (*fakeDialect)(nil)

func TestFakeDialect_Bind(t *testing.T) {
	d := fakeDialect{}
	got, err := d.Bind(ColumnType{}, "value")
	if err != nil {
		t.Fatalf("Bind returned error: %v", err)
	}
	if got != "value" {
		t.Fatalf("Bind = %v, want %v", got, "value")
	}
}

func TestFakeDialect_Scan(t *testing.T) {
	d := fakeDialect{}
	var dest any
	if err := d.Scan(ColumnType{}, "raw", &dest); err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
}
