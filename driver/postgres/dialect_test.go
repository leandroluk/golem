package postgres

import (
	"context"
	"database/sql/driver"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/stmt"
	"github.com/pashagolub/pgxmock/v4"
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

func TestBind_Boolean_IntZeroAndNonZero(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.BOOLEAN(), 0)
	if err != nil {
		t.Fatalf("Bind(BOOLEAN, 0) error = %v", err)
	}
	if got != false {
		t.Fatalf("Bind(BOOLEAN, 0) = %v, want false", got)
	}
	got, err = d.Bind(golem.BOOLEAN(), 1)
	if err != nil {
		t.Fatalf("Bind(BOOLEAN, 1) error = %v", err)
	}
	if got != true {
		t.Fatalf("Bind(BOOLEAN, 1) = %v, want true", got)
	}
}

func TestBind_Boolean_UnsupportedType_ReturnsError(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.BOOLEAN(), "not a bool")
	if err == nil {
		t.Fatal("expected error for unsupported boolean bind type, got nil")
	}
}

func TestBind_BIGINT_Int8Int16Int32(t *testing.T) {
	d := dialect{}
	if got, err := d.Bind(golem.BIGINT(), int8(1)); err != nil || got != int64(1) {
		t.Fatalf("Bind(BIGINT, int8(1)) = (%v, %v), want (1, nil)", got, err)
	}
	if got, err := d.Bind(golem.BIGINT(), int16(2)); err != nil || got != int64(2) {
		t.Fatalf("Bind(BIGINT, int16(2)) = (%v, %v), want (2, nil)", got, err)
	}
	if got, err := d.Bind(golem.BIGINT(), int32(3)); err != nil || got != int64(3) {
		t.Fatalf("Bind(BIGINT, int32(3)) = (%v, %v), want (3, nil)", got, err)
	}
}

func TestBind_BIGINT_UnsupportedType_ReturnsError(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.BIGINT(), "not a number")
	if err == nil {
		t.Fatal("expected error for unsupported bigint bind type, got nil")
	}
}

func TestBind_DECIMAL_UnsupportedType_ReturnsError(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.DECIMAL(10, 2), "not a float")
	if err == nil {
		t.Fatal("expected error for unsupported decimal bind type, got nil")
	}
}

func TestBind_VARCHAR_BytesInput(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.VARCHAR(50), []byte("hello"))
	if err != nil {
		t.Fatalf("Bind(VARCHAR, []byte) error = %v", err)
	}
	if got != "hello" {
		t.Fatalf("Bind(VARCHAR, []byte) = %v, want hello", got)
	}
}

func TestBind_VARCHAR_UnsupportedType_ReturnsError(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.VARCHAR(50), 123)
	if err == nil {
		t.Fatal("expected error for unsupported varchar bind type, got nil")
	}
}

func TestBind_DATETIME_PointerNil(t *testing.T) {
	d := dialect{}
	var nilTime *time.Time
	got, err := d.Bind(golem.DATETIME(), nilTime)
	if err != nil {
		t.Fatalf("Bind(DATETIME, nil *time.Time) error = %v", err)
	}
	if got != nil {
		t.Fatalf("Bind(DATETIME, nil *time.Time) = %v, want nil", got)
	}
}

func TestBind_DATETIME_PointerNonNil(t *testing.T) {
	d := dialect{}
	now := time.Now()
	got, err := d.Bind(golem.DATETIME(), &now)
	if err != nil {
		t.Fatalf("Bind(DATETIME, *time.Time) error = %v", err)
	}
	if got != now {
		t.Fatalf("Bind(DATETIME, *time.Time) = %v, want %v", got, now)
	}
}

func TestBind_DATETIME_UnsupportedType_ReturnsError(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.DATETIME(), "not a time")
	if err == nil {
		t.Fatal("expected error for unsupported datetime bind type, got nil")
	}
}

func TestBind_BLOB_StringInput(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.BLOB(), "hello")
	if err != nil {
		t.Fatalf("Bind(BLOB, string) error = %v", err)
	}
	b, ok := got.([]byte)
	if !ok || string(b) != "hello" {
		t.Fatalf("Bind(BLOB, string) = %v, want []byte(hello)", got)
	}
}

func TestBind_BLOB_UnsupportedType_ReturnsError(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.BLOB(), 123)
	if err == nil {
		t.Fatal("expected error for unsupported blob bind type, got nil")
	}
}

func TestBind_UUID_ByteArrayInput(t *testing.T) {
	d := dialect{}
	var arr [16]byte
	copy(arr[:], []byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x00})
	got, err := d.Bind(golem.UUID(), arr)
	if err != nil {
		t.Fatalf("Bind(UUID, [16]byte) error = %v", err)
	}
	if got != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("Bind(UUID, [16]byte) = %v, want 550e8400-e29b-41d4-a716-446655440000", got)
	}
}

func TestBind_UUID_UnsupportedType_ReturnsError(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.UUID(), 123)
	if err == nil {
		t.Fatal("expected error for unsupported uuid bind type, got nil")
	}
}

func TestBind_JSON_BytesInput(t *testing.T) {
	d := dialect{}
	got, err := d.Bind(golem.JSON(), []byte(`{"key":"val"}`))
	if err != nil {
		t.Fatalf("Bind(JSON, []byte) error = %v", err)
	}
	if got != `{"key":"val"}` {
		t.Fatalf("Bind(JSON, []byte) = %v, want original string", got)
	}
}

func TestBind_JSON_UnsupportedType_ReturnsError(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.JSON(), 123)
	if err == nil {
		t.Fatal("expected error for unsupported json bind type, got nil")
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

func TestScan_Boolean_WrongDestType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest string
	err := d.Scan(golem.BOOLEAN(), true, &dest)
	if err == nil {
		t.Fatal("expected error for wrong dest type, got nil")
	}
}

func TestScan_Boolean_Int64Raw(t *testing.T) {
	d := dialect{}
	var dest bool
	if err := d.Scan(golem.BOOLEAN(), int64(1), &dest); err != nil {
		t.Fatalf("Scan(BOOLEAN, int64) error = %v", err)
	}
	if !dest {
		t.Fatal("Scan(BOOLEAN, int64(1)) dest = false, want true")
	}
}

func TestScan_Boolean_UnsupportedRawType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest bool
	err := d.Scan(golem.BOOLEAN(), "not a bool", &dest)
	if err == nil {
		t.Fatal("expected error for unsupported raw type, got nil")
	}
}

func TestScan_BIGINT_WrongDestType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest string
	err := d.Scan(golem.BIGINT(), int64(1), &dest)
	if err == nil {
		t.Fatal("expected error for wrong dest type, got nil")
	}
}

func TestScan_BIGINT_Int32AndInt16Raw(t *testing.T) {
	d := dialect{}
	var dest int64
	if err := d.Scan(golem.BIGINT(), int32(7), &dest); err != nil {
		t.Fatalf("Scan(BIGINT, int32) error = %v", err)
	}
	if dest != 7 {
		t.Fatalf("Scan(BIGINT, int32(7)) dest = %d, want 7", dest)
	}
	if err := d.Scan(golem.BIGINT(), int16(8), &dest); err != nil {
		t.Fatalf("Scan(BIGINT, int16) error = %v", err)
	}
	if dest != 8 {
		t.Fatalf("Scan(BIGINT, int16(8)) dest = %d, want 8", dest)
	}
}

func TestScan_BIGINT_UnsupportedRawType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest int64
	err := d.Scan(golem.BIGINT(), "not a number", &dest)
	if err == nil {
		t.Fatal("expected error for unsupported raw type, got nil")
	}
}

func TestScan_DECIMAL_WrongDestType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest string
	err := d.Scan(golem.DECIMAL(10, 2), float64(1), &dest)
	if err == nil {
		t.Fatal("expected error for wrong dest type, got nil")
	}
}

func TestScan_DECIMAL_Float32Raw(t *testing.T) {
	d := dialect{}
	var dest float64
	if err := d.Scan(golem.DECIMAL(10, 2), float32(1.5), &dest); err != nil {
		t.Fatalf("Scan(DECIMAL, float32) error = %v", err)
	}
	if dest != float64(float32(1.5)) {
		t.Fatalf("Scan(DECIMAL, float32(1.5)) dest = %f, want %f", dest, float64(float32(1.5)))
	}
}

func TestScan_DECIMAL_UnsupportedRawType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest float64
	err := d.Scan(golem.DECIMAL(10, 2), "not a float", &dest)
	if err == nil {
		t.Fatal("expected error for unsupported raw type, got nil")
	}
}

func TestScan_VARCHAR_WrongDestType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest int
	err := d.Scan(golem.VARCHAR(50), "hello", &dest)
	if err == nil {
		t.Fatal("expected error for wrong dest type, got nil")
	}
}

func TestScan_VARCHAR_BytesRaw(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.VARCHAR(50), []byte("hello"), &dest); err != nil {
		t.Fatalf("Scan(VARCHAR, []byte) error = %v", err)
	}
	if dest != "hello" {
		t.Fatalf("Scan(VARCHAR, []byte) dest = %q, want hello", dest)
	}
}

func TestScan_VARCHAR_UnsupportedRawType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest string
	err := d.Scan(golem.VARCHAR(50), 123, &dest)
	if err == nil {
		t.Fatal("expected error for unsupported raw type, got nil")
	}
}

func TestScan_DATETIME_WrongDestType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest string
	err := d.Scan(golem.DATETIME(), time.Now(), &dest)
	if err == nil {
		t.Fatal("expected error for wrong dest type, got nil")
	}
}

func TestScan_DATETIME_UnsupportedRawType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest time.Time
	err := d.Scan(golem.DATETIME(), "not a time", &dest)
	if err == nil {
		t.Fatal("expected error for unsupported raw type, got nil")
	}
}

func TestScan_BLOB_WrongDestType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest string
	err := d.Scan(golem.BLOB(), []byte{0x01}, &dest)
	if err == nil {
		t.Fatal("expected error for wrong dest type, got nil")
	}
}

func TestScan_BLOB_StringRaw(t *testing.T) {
	d := dialect{}
	var dest []byte
	if err := d.Scan(golem.BLOB(), "hello", &dest); err != nil {
		t.Fatalf("Scan(BLOB, string) error = %v", err)
	}
	if string(dest) != "hello" {
		t.Fatalf("Scan(BLOB, string) dest = %q, want hello", dest)
	}
}

func TestScan_BLOB_UnsupportedRawType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest []byte
	err := d.Scan(golem.BLOB(), 123, &dest)
	if err == nil {
		t.Fatal("expected error for unsupported raw type, got nil")
	}
}

func TestScan_UUID_WrongDestType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest int
	err := d.Scan(golem.UUID(), "550e8400-e29b-41d4-a716-446655440000", &dest)
	if err == nil {
		t.Fatal("expected error for wrong dest type, got nil")
	}
}

func TestScan_UUID_ByteArrayRaw(t *testing.T) {
	d := dialect{}
	var arr [16]byte
	copy(arr[:], []byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x00})
	var dest string
	if err := d.Scan(golem.UUID(), arr, &dest); err != nil {
		t.Fatalf("Scan(UUID, [16]byte) error = %v", err)
	}
	if dest != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("Scan(UUID, [16]byte) dest = %q, want 550e8400-e29b-41d4-a716-446655440000", dest)
	}
}

func TestScan_UUID_UnsupportedRawType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest string
	err := d.Scan(golem.UUID(), 123, &dest)
	if err == nil {
		t.Fatal("expected error for unsupported raw type, got nil")
	}
}

func TestScan_JSON_WrongDestType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest int
	err := d.Scan(golem.JSON(), `{"key":"val"}`, &dest)
	if err == nil {
		t.Fatal("expected error for wrong dest type, got nil")
	}
}

func TestScan_JSON_StringRaw(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.JSON(), `{"key":"val"}`, &dest); err != nil {
		t.Fatalf("Scan(JSON, string) error = %v", err)
	}
	if dest != `{"key":"val"}` {
		t.Fatalf("Scan(JSON, string) dest = %q, want original", dest)
	}
}

func TestScan_JSON_BytesRaw(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.JSON(), []byte(`{"key":"val"}`), &dest); err != nil {
		t.Fatalf("Scan(JSON, []byte) error = %v", err)
	}
	if dest != `{"key":"val"}` {
		t.Fatalf("Scan(JSON, []byte) dest = %q, want original", dest)
	}
}

func TestScan_JSON_UnsupportedRawType_ReturnsError(t *testing.T) {
	d := dialect{}
	var dest string
	err := d.Scan(golem.JSON(), 123, &dest)
	if err == nil {
		t.Fatal("expected error for unsupported raw type, got nil")
	}
}

// -----------------------------------------------------------------------
// CompileSelect / CompileDelete (moved here from dialect_sql_test.go)
// -----------------------------------------------------------------------

func TestCompileSelect_Minimal(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSQL := `SELECT * FROM "users"`
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}
	if len(args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(args))
	}
}

func TestCompileSelect_Complex(t *testing.T) {
	d := dialect{}
	limit := 10
	offset := 20
	s := &stmt.Select{
		Table:   "users",
		Columns: []string{"id", "name"},
		Where: stmt.Logical{
			Op: "and",
			Predicates: []stmt.Predicate{
				stmt.Comparison{Column: "age", Op: "gte", Value: 18},
				stmt.Not{
					Predicate: stmt.Comparison{Column: "status", Op: "eq", Value: "pending"},
				},
				stmt.Logical{
					Op: "or",
					Predicates: []stmt.Predicate{
						stmt.Comparison{Column: "role", Op: "eq", Value: "admin"},
						stmt.Comparison{Column: "role", Op: "eq", Value: "moderator"},
					},
				},
				stmt.Comparison{Column: "deleted_at", Op: "is_null"},
				stmt.Comparison{Column: "tags", Op: "in", Value: []string{"vip", "staff"}},
			},
		},
		OrderBy: []stmt.OrderElement{
			{Column: "name", Desc: false},
			{Column: "id", Desc: true},
		},
		Limit:  &limit,
		Offset: &offset,
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSQL := `SELECT "id", "name" FROM "users" WHERE ("age">=$1 AND NOT ("status"=$2) AND ("role"=$3 OR "role"=$4) AND "deleted_at" IS NULL AND "tags" IN ($5,$6)) ORDER BY "name" ASC, "id" DESC LIMIT $7 OFFSET $8`
	if sql != wantSQL {
		t.Fatalf("sql =\n%q\nwant:\n%q", sql, wantSQL)
	}

	wantArgs := []any{18, "pending", "admin", "moderator", "vip", "staff", 10, 20}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
}

func TestCompileSelect_EmptyInClause(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
		Where: stmt.Comparison{Column: "id", Op: "in", Value: []int{}},
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSQL := `SELECT * FROM "users" WHERE FALSE`
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}
	if len(args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(args))
	}
}

func TestCompileDelete(t *testing.T) {
	d := dialect{}
	s := &stmt.Delete{
		Table: "users",
		Where: stmt.Comparison{Column: "id", Op: "eq", Value: 5},
	}

	sql, args, err := d.CompileDelete(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSQL := `DELETE FROM "users" WHERE "id"=$1`
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}

	wantArgs := []any{5}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
}

func TestCompileSelect_Count(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
		Count: true,
		Where: stmt.Comparison{Column: "active", Op: "eq", Value: true},
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSQL := `SELECT COUNT(*) FROM "users" WHERE "active"=$1`
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}

	wantArgs := []any{true}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
}

func TestCompileSelect_WhereCompileError_Propagates(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
		Where: stmt.Comparison{Column: "id", Op: "bogus", Value: 1},
	}
	_, _, err := d.CompileSelect(s)
	if err == nil {
		t.Fatal("expected error from an unsupported Where operator, got nil")
	}
}

func TestCompileSelect_JoinWhereCompileError_Propagates(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
		Joins: []stmt.Join{
			{
				Type:  "inner",
				Table: "posts",
				On:    []stmt.OnCondition{{LeftCol: "posts.owner_id", RightCol: "users.id"}},
				Where: stmt.Comparison{Column: "posts.id", Op: "bogus", Value: 1},
			},
		},
	}
	_, _, err := d.CompileSelect(s)
	if err == nil {
		t.Fatal("expected error from an unsupported join Where operator, got nil")
	}
}

func TestCompileDelete_WhereCompileError_Propagates(t *testing.T) {
	d := dialect{}
	s := &stmt.Delete{
		Table: "users",
		Where: stmt.Comparison{Column: "id", Op: "bogus", Value: 1},
	}
	_, _, err := d.CompileDelete(s)
	if err == nil {
		t.Fatal("expected error from an unsupported Where operator, got nil")
	}
}

func TestCompileSelect_WithJoins(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table:   "users",
		Columns: []string{"users.id", "posts.title"},
		Joins: []stmt.Join{
			{
				Type:  "inner",
				Table: "posts",
				On: []stmt.OnCondition{
					{LeftCol: "posts.owner_id", RightCol: "users.id"},
				},
				Where: stmt.Comparison{Column: "posts.published", Op: "eq", Value: true},
			},
		},
		Where: stmt.Comparison{Column: "users.active", Op: "eq", Value: true},
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSQL := `SELECT "users"."id", "posts"."title" FROM "users" INNER JOIN "posts" ON "posts"."owner_id" = "users"."id" AND "posts"."published"=$1 WHERE "users"."active"=$2`
	if sql != wantSQL {
		t.Fatalf("sql =\n%q\nwant:\n%q", sql, wantSQL)
	}

	wantArgs := []any{true, true}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
}

// -----------------------------------------------------------------------
// Aggregate projections / HAVING (moved here from aggregate_sql_test.go)
// -----------------------------------------------------------------------

func TestCompileSelect_Aggregate_GroupBySumAvgCount(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "orders",
		Projections: []stmt.Projection{
			{Column: "category", Func: "", Alias: "agg_0"},
			{Column: "amount", Func: "sum", Alias: "agg_1"},
			{Column: "amount", Func: "avg", Alias: "agg_2"},
			{Column: "id", Func: "count", Alias: "agg_3"},
			{Func: "count_all", Alias: "agg_4"},
		},
		GroupBy: []string{"category"},
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT "category" AS "agg_0", CAST(SUM("amount") AS DOUBLE PRECISION) AS "agg_1", CAST(AVG("amount") AS DOUBLE PRECISION) AS "agg_2", COUNT("id") AS "agg_3", COUNT(*) AS "agg_4" FROM "orders" GROUP BY "category"`
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
	if len(args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(args))
	}
}

func TestCompileSelect_Aggregate_WithHaving(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "orders",
		Projections: []stmt.Projection{
			{Column: "category", Alias: "agg_0"},
			{Column: "amount", Func: "sum", Alias: "agg_1"},
		},
		GroupBy: []string{"category"},
		Having:  stmt.AggregateComparison{Func: "sum", Column: "amount", Op: "gt", Value: 1000},
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT "category" AS "agg_0", CAST(SUM("amount") AS DOUBLE PRECISION) AS "agg_1" FROM "orders" GROUP BY "category" HAVING CAST(SUM("amount") AS DOUBLE PRECISION)>$1`
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
	if len(args) != 1 || args[0] != 1000 {
		t.Fatalf("args = %v, want [1000]", args)
	}
}

func TestCompileSelect_Aggregate_WithWhereAndOrderBy(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "orders",
		Projections: []stmt.Projection{
			{Column: "category", Alias: "agg_0"},
			{Column: "amount", Func: "sum", Alias: "agg_1"},
		},
		Where:   stmt.Comparison{Column: "status", Op: "eq", Value: "paid"},
		GroupBy: []string{"category"},
		OrderBy: []stmt.OrderElement{{Column: "agg_1", Desc: true}},
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT "category" AS "agg_0", CAST(SUM("amount") AS DOUBLE PRECISION) AS "agg_1" FROM "orders" WHERE "status"=$1 GROUP BY "category" ORDER BY "agg_1" DESC`
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
	if len(args) != 1 || args[0] != "paid" {
		t.Fatalf("args = %v, want [paid]", args)
	}
}

func TestCompileSelect_Aggregate_HavingError_Propagates(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "orders",
		Projections: []stmt.Projection{
			{Column: "amount", Func: "sum", Alias: "agg_0"},
		},
		Having: stmt.AggregateComparison{Func: "sum", Column: "amount", Op: "like", Value: 1},
	}

	_, _, err := d.CompileSelect(s)
	if err == nil {
		t.Fatal("expected error from an unsupported HAVING operator, got nil")
	}
}

func TestAggregateSQLFunc_UnrecognizedFunc_FallsBackToBareColumn(t *testing.T) {
	got := aggregateSQLFunc("bogus", "amount")
	want := `"amount"`
	if got != want {
		t.Errorf("aggregateSQLFunc(bogus) = %q, want %q", got, want)
	}
}

func TestAggregateComparison_UnsupportedOp_ReturnsError(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(stmt.AggregateComparison{Func: "sum", Column: "amount", Op: "like", Value: 1}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected error for unsupported having comparison operator, got nil")
	}
}

func TestAggregateComparison_AllOps(t *testing.T) {
	cases := []struct {
		op   string
		want string
	}{
		{"eq", `CAST(SUM("amount") AS DOUBLE PRECISION)=$1`},
		{"gt", `CAST(SUM("amount") AS DOUBLE PRECISION)>$1`},
		{"gte", `CAST(SUM("amount") AS DOUBLE PRECISION)>=$1`},
		{"lt", `CAST(SUM("amount") AS DOUBLE PRECISION)<$1`},
		{"lte", `CAST(SUM("amount") AS DOUBLE PRECISION)<=$1`},
	}
	for _, tc := range cases {
		t.Run(tc.op, func(t *testing.T) {
			argOffset := 1
			var args []any
			sql, err := compilePredicate(stmt.AggregateComparison{Func: "sum", Column: "amount", Op: tc.op, Value: 5}, &argOffset, &args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.want {
				t.Errorf("sql = %q, want %q", sql, tc.want)
			}
		})
	}
}

// -----------------------------------------------------------------------
// Row locking (moved here from lock_sql_test.go)
// -----------------------------------------------------------------------

func TestCompileSelect_Lock_ForUpdate(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
		Lock:  &stmt.LockClause{Strength: "update"},
	}
	sql, _, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT * FROM "users" FOR UPDATE`
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
}

func TestCompileSelect_Lock_AllStrengths(t *testing.T) {
	cases := []struct {
		strength string
		want     string
	}{
		{"update", "FOR UPDATE"},
		{"no_key_update", "FOR NO KEY UPDATE"},
		{"share", "FOR SHARE"},
		{"key_share", "FOR KEY SHARE"},
	}
	for _, tc := range cases {
		t.Run(tc.strength, func(t *testing.T) {
			d := dialect{}
			s := &stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: tc.strength}}
			sql, _, err := d.CompileSelect(s)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := `SELECT * FROM "users" ` + tc.want
			if sql != want {
				t.Errorf("sql = %q, want %q", sql, want)
			}
		})
	}
}

func TestCompileSelect_Lock_WaitPolicies(t *testing.T) {
	cases := []struct {
		wait string
		want string
	}{
		{"", `SELECT * FROM "users" FOR UPDATE`},
		{"nowait", `SELECT * FROM "users" FOR UPDATE NOWAIT`},
		{"skip_locked", `SELECT * FROM "users" FOR UPDATE SKIP LOCKED`},
	}
	for _, tc := range cases {
		t.Run(tc.wait, func(t *testing.T) {
			d := dialect{}
			s := &stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: "update", Wait: tc.wait}}
			sql, _, err := d.CompileSelect(s)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.want {
				t.Errorf("sql = %q, want %q", sql, tc.want)
			}
		})
	}
}

func TestCompileSelect_Lock_UnsupportedStrength_ReturnsError(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: "bogus"}}
	_, _, err := d.CompileSelect(s)
	if err == nil {
		t.Fatal("expected error for unsupported lock strength, got nil")
	}
}

func TestCompileSelect_Lock_UnsupportedWait_ReturnsError(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: "update", Wait: "bogus"}}
	_, _, err := d.CompileSelect(s)
	if err == nil {
		t.Fatal("expected error for unsupported lock wait policy, got nil")
	}
}

func TestCompileSelect_Lock_WithOrderByLimitOffset_AppearsLast(t *testing.T) {
	d := dialect{}
	limit := 10
	s := &stmt.Select{
		Table:   "users",
		OrderBy: []stmt.OrderElement{{Column: "id", Desc: false}},
		Limit:   &limit,
		Lock:    &stmt.LockClause{Strength: "update"},
	}
	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT * FROM "users" ORDER BY "id" ASC LIMIT $1 FOR UPDATE`
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
	if len(args) != 1 || args[0] != 10 {
		t.Fatalf("args = %v, want [10]", args)
	}
}

// -----------------------------------------------------------------------
// mapError (moved here from errors_test.go)
// -----------------------------------------------------------------------

func TestMapError_Nil_ReturnsNil(t *testing.T) {
	if err := mapError(nil); err != nil {
		t.Fatalf("mapError(nil) = %v, want nil", err)
	}
}

func TestMapError_UniqueViolation_WrapsErrDuplicateKey(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}

	got := mapError(pgErr)

	if !errors.Is(got, golem.ErrDuplicateKey) {
		t.Fatalf("mapError(23505) = %v, want wrapped golem.ErrDuplicateKey", got)
	}
	var asPgErr *pgconn.PgError
	if !errors.As(got, &asPgErr) {
		t.Fatalf("mapError(23505) = %v, want original *pgconn.PgError reachable via errors.As", got)
	}
	if asPgErr.Code != "23505" {
		t.Fatalf("unwrapped PgError.Code = %q, want 23505", asPgErr.Code)
	}
}

func TestMapError_ForeignKeyViolation_WrapsErrForeignKeyViolation(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23503", Message: "violates foreign key constraint"}

	got := mapError(pgErr)

	if !errors.Is(got, golem.ErrForeignKeyViolation) {
		t.Fatalf("mapError(23503) = %v, want wrapped golem.ErrForeignKeyViolation", got)
	}
	var asPgErr *pgconn.PgError
	if !errors.As(got, &asPgErr) {
		t.Fatalf("mapError(23503) = %v, want original *pgconn.PgError reachable via errors.As", got)
	}
}

func TestMapError_UnmappedPgCode_PassesThroughUnchanged(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "42601", Message: "syntax error"}

	got := mapError(pgErr)

	if got != error(pgErr) {
		t.Fatalf("mapError(42601) = %v, want unchanged original error", got)
	}
	if errors.Is(got, golem.ErrDuplicateKey) || errors.Is(got, golem.ErrForeignKeyViolation) {
		t.Fatalf("mapError(42601) should not match any sentinel, got %v", got)
	}
}

func TestMapError_NonPgError_PassesThroughUnchanged(t *testing.T) {
	plain := errors.New("connection refused")

	got := mapError(plain)

	if got != plain {
		t.Fatalf("mapError(plain) = %v, want unchanged original error", got)
	}
}

// -----------------------------------------------------------------------
// Execute-path dialect methods, mocked via pgxmock (moved here from
// postgres_test.go)
// -----------------------------------------------------------------------

func TestDialect_IsConflict(t *testing.T) {
	d := &dialect{}
	if d.IsConflict(errors.New("some error")) {
		t.Fatal("expected false")
	}
	pgErr := &pgconn.PgError{Code: "23505"}
	if !d.IsConflict(pgErr) {
		t.Fatal("expected true")
	}
}

func TestDialect_Query(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery("SELECT 1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(1))

	rows, err := d.Query(context.Background(), nil, "SELECT 1", nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(rows) != 1 || rows[0]["id"] != 1 {
		t.Fatalf("expected row id 1, got %v", rows)
	}
}

func TestDialect_Query_Error(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery("SELECT 1").WillReturnError(errors.New("query error"))

	_, err := d.Query(context.Background(), nil, "SELECT 1", nil)
	if err == nil || err.Error() != "query error" {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestDialect_Exec(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectExec("DELETE FROM users").WillReturnResult(pgxmock.NewResult("DELETE", 2))

	affected, err := d.Exec(context.Background(), nil, "DELETE FROM users", nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if affected != 2 {
		t.Fatalf("expected 2 affected, got %d", affected)
	}
}

func TestDialect_ExecRaw(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery("INSERT INTO users").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(1))

	rows, affected, err := d.ExecRaw(context.Background(), nil, "INSERT INTO users", nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(rows) != 1 || rows[0]["id"] != 1 {
		t.Fatalf("expected row id 1, got %v", rows)
	}
	if affected != 0 {
		t.Fatalf("expected 0 affected, got %d", affected)
	}
}

func TestDialect_Begin(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectBegin()

	tx, err := d.Begin(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if tx == nil {
		t.Fatal("expected tx, got nil")
	}
}

func TestDialect_TxConn(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectBegin()
	mock.ExpectCommit()

	tx, _ := d.Begin(context.Background(), nil)
	err := tx.Commit(context.Background())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDialect_TxConn_Rollback(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, _ := d.Begin(context.Background(), nil)
	err := tx.Rollback(context.Background())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDialect_Begin_Error(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectBegin().WillReturnError(errors.New("begin error"))

	_, err := d.Begin(context.Background(), nil)
	if err == nil || err.Error() != "postgres: begin: begin error" {
		t.Fatalf("expected begin error, got %v", err)
	}
}

func TestDialect_GetExecutor_UsesTxWhenPresent(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectBegin()
	pgxTx, err := mock.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	tx := golem.NewTx(d, &pgTx{tx: pgxTx, d: d})

	mock.ExpectQuery("SELECT 1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(1))

	rows, err := d.Query(context.Background(), tx, "SELECT 1", nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}

func TestDialect_Insert_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery(`INSERT INTO "users"`).WithArgs("Ada").WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).AddRow(1, "Ada"))

	row, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table:   "users",
		Columns: []string{"name"},
		Values:  []driver.Value{"Ada"},
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if row["id"] != int32(1) && row["id"] != 1 {
		t.Errorf("expected id=1, got %v", row["id"])
	}
}

func TestDialect_Insert_QueryError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery(`INSERT INTO "users"`).WithArgs("Ada").WillReturnError(errors.New("insert failed"))

	_, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table:   "users",
		Columns: []string{"name"},
		Values:  []driver.Value{"Ada"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDialect_Insert_CollectRowError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	// Zero rows returned -> pgx.CollectOneRow errors (ErrNoRows).
	mock.ExpectQuery(`INSERT INTO "users"`).WithArgs("Ada").WillReturnRows(pgxmock.NewRows([]string{"id"}))

	_, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table:   "users",
		Columns: []string{"name"},
		Values:  []driver.Value{"Ada"},
	})
	if err == nil {
		t.Fatal("expected error for zero returned rows, got nil")
	}
}

func TestDialect_Update_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery(`UPDATE "users"`).WithArgs("Ada2", 1).WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).AddRow(1, "Ada2"))

	rows, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "Ada2"}},
		Where: stmt.Comparison{Column: "id", Op: "eq", Value: 1},
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}

func TestDialect_Update_NoWhere(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery(`UPDATE "users"`).WithArgs("same").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))

	rows, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "same"}},
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestDialect_Update_WhereCompileError(t *testing.T) {
	d := &dialect{}
	_, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
		Where: stmt.AggregateComparison{Func: "sum", Column: "amount", Op: "like", Value: 1},
	})
	if err == nil {
		t.Fatal("expected error from an unsupported Where predicate, got nil")
	}
}

func TestDialect_Update_QueryError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery(`UPDATE "users"`).WithArgs("x").WillReturnError(errors.New("update failed"))

	_, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDialect_Update_CollectRowsError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery(`UPDATE "users"`).WithArgs("x").WillReturnRows(pgxmock.NewRows([]string{"id"}).RowError(0, errors.New("row scan error")).AddRow(1))

	_, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDialect_ExecRaw_ExecError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery("INSERT INTO users").WillReturnError(errors.New("exec raw error"))

	_, _, err := d.ExecRaw(context.Background(), nil, "INSERT INTO users", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDialect_ExecRaw_CollectRowsError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery("INSERT INTO users").WillReturnRows(pgxmock.NewRows([]string{"id"}).RowError(0, errors.New("row scan error")).AddRow(1))

	_, _, err := d.ExecRaw(context.Background(), nil, "INSERT INTO users", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDialect_Exec_Error(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectExec("DELETE FROM users").WillReturnError(errors.New("exec error"))

	_, err := d.Exec(context.Background(), nil, "DELETE FROM users", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCompilePredicate_Nil_ReturnsEmpty(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(nil, &argOffset, &args)
	if err != nil || sql != "" {
		t.Fatalf("compilePredicate(nil) = (%q, %v), want (\"\", nil)", sql, err)
	}
}

// unknownPredicate embeds stmt.Comparison purely to inherit its unexported
// isPredicate() method (the only way to satisfy stmt.Predicate from outside
// internal/stmt) while remaining a distinct concrete type. compilePredicate's
// type-switch has no case for it, so this is the only way to reach the
// switch's default branch from outside internal/stmt.
type unknownPredicate struct {
	stmt.Comparison
}

func TestCompilePredicate_UnknownType_ReturnsError(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(unknownPredicate{}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected error for unrecognized predicate type")
	}
}

func TestCompilePredicate_Comparison_AllOps(t *testing.T) {
	cases := []struct {
		op   string
		want string
	}{
		{"eq", `"age"=$1`},
		{"gt", `"age">$1`},
		{"gte", `"age">=$1`},
		{"lt", `"age"<$1`},
		{"lte", `"age"<=$1`},
		{"like", `"age" LIKE $1`},
	}
	for _, tc := range cases {
		t.Run(tc.op, func(t *testing.T) {
			argOffset := 1
			var args []any
			sql, err := compilePredicate(stmt.Comparison{Column: "age", Op: tc.op, Value: 18}, &argOffset, &args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.want {
				t.Errorf("sql = %q, want %q", sql, tc.want)
			}
		})
	}
}

func TestCompilePredicate_Comparison_In_NonSliceValue(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Comparison{Column: "id", Op: "in", Value: 5}, &argOffset, &args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `"id" IN ($1)` {
		t.Fatalf("sql = %q, want %q", sql, `"id" IN ($1)`)
	}
}

func TestCompilePredicate_Comparison_UnsupportedOp_ReturnsError(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(stmt.Comparison{Column: "id", Op: "bogus", Value: 1}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected error for unsupported comparison operator, got nil")
	}
}

func TestCompilePredicate_Logical_EmptyPredicates_ReturnsEmpty(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Logical{Op: "and", Predicates: nil}, &argOffset, &args)
	if err != nil || sql != "" {
		t.Fatalf("compilePredicate(empty Logical) = (%q, %v), want (\"\", nil)", sql, err)
	}
}

func TestCompilePredicate_Logical_AllNestedEmpty_ReturnsEmpty(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Logical{
		Op:         "and",
		Predicates: []stmt.Predicate{stmt.Logical{Op: "and", Predicates: nil}},
	}, &argOffset, &args)
	if err != nil || sql != "" {
		t.Fatalf("compilePredicate(nested empty Logical) = (%q, %v), want (\"\", nil)", sql, err)
	}
}

func TestCompilePredicate_Logical_PropagatesNestedError(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(stmt.Logical{
		Op:         "and",
		Predicates: []stmt.Predicate{stmt.Comparison{Column: "id", Op: "bogus", Value: 1}},
	}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected propagated error from nested predicate, got nil")
	}
}

func TestCompilePredicate_Not_PropagatesNestedError(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(stmt.Not{
		Predicate: stmt.Comparison{Column: "id", Op: "bogus", Value: 1},
	}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected propagated error from nested predicate, got nil")
	}
}

func TestCompilePredicate_Not_EmptyNested_ReturnsEmpty(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Not{
		Predicate: stmt.Logical{Op: "and", Predicates: nil},
	}, &argOffset, &args)
	if err != nil || sql != "" {
		t.Fatalf("compilePredicate(Not{empty}) = (%q, %v), want (\"\", nil)", sql, err)
	}
}

func TestDialect_IsConflict_Nil(t *testing.T) {
	d := &dialect{}
	if d.IsConflict(nil) {
		t.Fatal("expected false for nil error")
	}
}

func TestDialect_IsConflict_NonPgError(t *testing.T) {
	d := &dialect{}
	if d.IsConflict(errors.New("plain error, not a PgError")) {
		t.Fatal("expected false for a non-PgError")
	}
}

func TestDialect_IsConflict_PgErrorWrongClass(t *testing.T) {
	d := &dialect{}
	pgErr := &pgconn.PgError{Code: "42601"} // syntax error, class 42, not 23
	if d.IsConflict(pgErr) {
		t.Fatal("expected false for a non-class-23 PgError")
	}
}
