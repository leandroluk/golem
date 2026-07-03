package postgres

import (
	"testing"
	"time"

	"github.com/leandroluk/golem"
)

var _ golem.Dialect = (*dialect)(nil)

// -----------------------------------------------------------------------
// Bind
// -----------------------------------------------------------------------

func TestBind_Boolean_BoolValue(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.BOOLEAN(), true)
	if err != nil {
		t.Fatalf("Bind(BOOLEAN, true) error = %v", err)
	}
	if got != true {
		t.Fatalf("Bind(BOOLEAN, true) = %v, want true", got)
	}
}

func TestBind_SMALLINT_Int64(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.SMALLINT(), int64(7))
	if err != nil {
		t.Fatalf("Bind(SMALLINT, 7) error = %v", err)
	}
	if got != int64(7) {
		t.Fatalf("Bind(SMALLINT, 7) = %v, want 7", got)
	}
}

func TestBind_INTEGER_Int(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.INTEGER(), int(42))
	if err != nil {
		t.Fatalf("Bind(INTEGER, 42) error = %v", err)
	}
	if got != int64(42) {
		t.Fatalf("Bind(INTEGER, 42) = %v, want int64(42)", got)
	}
}

func TestBind_BIGINT_Int64(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.BIGINT(), int64(99))
	if err != nil {
		t.Fatalf("Bind(BIGINT, 99) error = %v", err)
	}
	if got != int64(99) {
		t.Fatalf("Bind(BIGINT, 99) = %v, want int64(99)", got)
	}
}

func TestBind_DECIMAL_Float64(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.DECIMAL(10, 2), float64(3.14))
	if err != nil {
		t.Fatalf("Bind(DECIMAL, 3.14) error = %v", err)
	}
	if got != float64(3.14) {
		t.Fatalf("Bind(DECIMAL, 3.14) = %v, want 3.14", got)
	}
}

func TestBind_FLOAT_Float32(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.FLOAT(53), float32(1.5))
	if err != nil {
		t.Fatalf("Bind(FLOAT, 1.5) error = %v", err)
	}
	if got != float64(float32(1.5)) {
		t.Fatalf("Bind(FLOAT, 1.5) = %v, want float64(1.5)", got)
	}
}

func TestBind_CHAR_String(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.CHAR(10), "abc")
	if err != nil {
		t.Fatalf("Bind(CHAR, abc) error = %v", err)
	}
	if got != "abc" {
		t.Fatalf("Bind(CHAR, abc) = %v, want abc", got)
	}
}

func TestBind_VARCHAR_String(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.VARCHAR(50), "hello")
	if err != nil {
		t.Fatalf("Bind(VARCHAR, hello) error = %v", err)
	}
	if got != "hello" {
		t.Fatalf("Bind(VARCHAR, hello) = %v, want hello", got)
	}
}

func TestBind_TEXT_String(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.TEXT(), "world")
	if err != nil {
		t.Fatalf("Bind(TEXT, world) error = %v", err)
	}
	if got != "world" {
		t.Fatalf("Bind(TEXT, world) = %v, want world", got)
	}
}

func TestBind_DATE_Time(t *testing.T) {
	d := dialect{}
	now := time.Now()
	got, err := d.Bind(golem.DATE(), now)
	if err != nil {
		t.Fatalf("Bind(DATE, time.Now()) error = %v", err)
	}
	if got != now {
		t.Fatalf("Bind(DATE) = %v, want %v", got, now)
	}
}

func TestBind_DATETIME_Time(t *testing.T) {
	d := dialect{}
	now := time.Now()
	got, err := d.Bind(golem.DATETIME(), now)
	if err != nil {
		t.Fatalf("Bind(DATETIME, time.Now()) error = %v", err)
	}
	if got != now {
		t.Fatalf("Bind(DATETIME) = %v, want %v", got, now)
	}
}

func TestBind_TIME_Time(t *testing.T) {
	d := dialect{}
	now := time.Now()
	got, err := d.Bind(golem.TIME(), now)
	if err != nil {
		t.Fatalf("Bind(TIME, time.Now()) error = %v", err)
	}
	if got != now {
		t.Fatalf("Bind(TIME) = %v, want %v", got, now)
	}
}

func TestBind_BLOB_Bytes(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.BLOB(), []byte{0x01, 0x02})
	if err != nil {
		t.Fatalf("Bind(BLOB, bytes) error = %v", err)
	}
	b, ok := got.([]byte)
	if !ok || len(b) != 2 || b[0] != 0x01 {
		t.Fatalf("Bind(BLOB) = %v, want [0x01 0x02]", got)
	}
}

func TestBind_UUID_String(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.UUID(), "550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("Bind(UUID, string) error = %v", err)
	}
	if got != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("Bind(UUID) = %v, want original string", got)
	}
}

func TestBind_JSON_String(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.JSON(), `{"key":"val"}`)
	if err != nil {
		t.Fatalf("Bind(JSON, string) error = %v", err)
	}
	if got != `{"key":"val"}` {
		t.Fatalf("Bind(JSON) = %v, want original string", got)
	}
}

func TestBind_UnknownKind_ReturnsError(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.ColumnType{}, "anything")
	if err == nil {
		t.Fatal("expected error for unrecognized kind, got nil")
	}
}

// -----------------------------------------------------------------------
// Scan
// -----------------------------------------------------------------------

func TestScan_Boolean(t *testing.T) {
	d := dialect{}
	var dest bool
	if err := d.Scan(golem.BOOLEAN(), true, &dest); err != nil {
		t.Fatalf("Scan(BOOLEAN) error = %v", err)
	}
	if !dest {
		t.Fatal("Scan(BOOLEAN) dest = false, want true")
	}
}

func TestScan_BIGINT(t *testing.T) {
	d := dialect{}
	var dest int64
	if err := d.Scan(golem.BIGINT(), int64(42), &dest); err != nil {
		t.Fatalf("Scan(BIGINT) error = %v", err)
	}
	if dest != 42 {
		t.Fatalf("Scan(BIGINT) dest = %d, want 42", dest)
	}
}

func TestScan_DECIMAL(t *testing.T) {
	d := dialect{}
	var dest float64
	if err := d.Scan(golem.DECIMAL(10, 2), float64(9.99), &dest); err != nil {
		t.Fatalf("Scan(DECIMAL) error = %v", err)
	}
	if dest != 9.99 {
		t.Fatalf("Scan(DECIMAL) dest = %f, want 9.99", dest)
	}
}

func TestScan_VARCHAR(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.VARCHAR(50), "hello", &dest); err != nil {
		t.Fatalf("Scan(VARCHAR) error = %v", err)
	}
	if dest != "hello" {
		t.Fatalf("Scan(VARCHAR) dest = %q, want hello", dest)
	}
}

func TestScan_DATETIME(t *testing.T) {
	d := dialect{}
	now := time.Now().Truncate(time.Second)
	var dest time.Time
	if err := d.Scan(golem.DATETIME(), now, &dest); err != nil {
		t.Fatalf("Scan(DATETIME) error = %v", err)
	}
	if !dest.Equal(now) {
		t.Fatalf("Scan(DATETIME) dest = %v, want %v", dest, now)
	}
}

func TestScan_BLOB(t *testing.T) {
	d := dialect{}
	var dest []byte
	if err := d.Scan(golem.BLOB(), []byte{0xDE, 0xAD}, &dest); err != nil {
		t.Fatalf("Scan(BLOB) error = %v", err)
	}
	if len(dest) != 2 || dest[0] != 0xDE {
		t.Fatalf("Scan(BLOB) dest = %v, want [0xDE 0xAD]", dest)
	}
}

func TestScan_UUID(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.UUID(), "550e8400-e29b-41d4-a716-446655440000", &dest); err != nil {
		t.Fatalf("Scan(UUID) error = %v", err)
	}
	if dest != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("Scan(UUID) dest = %q", dest)
	}
}

func TestScan_NilRaw_IsNoOp(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.TEXT(), nil, &dest); err != nil {
		t.Fatalf("Scan(TEXT, nil) error = %v, want nil", err)
	}
	if dest != "" {
		t.Fatalf("Scan(TEXT, nil) dest = %q, want empty (no-op)", dest)
	}
}

func TestScan_UnknownKind_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest string
	err := d.Scan(golem.ColumnType{}, "raw", &dest)
	if err == nil {
		t.Fatal("expected error for unrecognized kind, got nil")
	}
}

func TestScan_WrongDestType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest int // wrong: BIGINT expects *int64
	err := d.Scan(golem.BIGINT(), int64(1), &dest)
	if err == nil {
		t.Fatal("expected error for wrong dest type, got nil")
	}
}

