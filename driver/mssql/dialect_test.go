package mssql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	mssqllib "github.com/microsoft/go-mssqldb"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/stmt"
)

var _ golem.Dialect = (*dialect)(nil)

// -----------------------------------------------------------------------
// Bind
// -----------------------------------------------------------------------

func TestBind_BOOLEAN_BoolValue(t *testing.T) {
	d := dialect{}
	v, err := d.Bind(golem.BOOLEAN(), true)
	if err != nil || v != true {
		t.Fatalf("Bind(BOOLEAN, true) = (%v, %v)", v, err)
	}
}

func TestBind_BOOLEAN_IntValue(t *testing.T) {
	d := dialect{}
	v, err := d.Bind(golem.BOOLEAN(), 1)
	if err != nil || v != true {
		t.Fatalf("Bind(BOOLEAN, 1) = (%v, %v)", v, err)
	}
	v, err = d.Bind(golem.BOOLEAN(), 0)
	if err != nil || v != false {
		t.Fatalf("Bind(BOOLEAN, 0) = (%v, %v)", v, err)
	}
}

func TestBind_BOOLEAN_UnsupportedType(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.BOOLEAN(), "not a bool")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBind_Integers(t *testing.T) {
	d := dialect{}
	cases := []any{int(1), int8(1), int16(1), int32(1), int64(1)}
	for _, kind := range []golem.ColumnType{golem.SMALLINT(), golem.INTEGER(), golem.BIGINT()} {
		for _, v := range cases {
			got, err := d.Bind(kind, v)
			if err != nil {
				t.Fatalf("Bind(%s, %T) error: %v", kind.Kind(), v, err)
			}
			if got != int64(1) {
				t.Fatalf("Bind(%s, %T) = %v, want int64(1)", kind.Kind(), v, got)
			}
		}
	}
}

func TestBind_Integers_UnsupportedType(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.BIGINT(), "nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBind_DECIMAL_Float32AndFloat64(t *testing.T) {
	d := dialect{}
	if v, err := d.Bind(golem.DECIMAL(10, 2), float32(1.5)); err != nil || v != float64(float32(1.5)) {
		t.Fatalf("Bind(DECIMAL, float32) = (%v, %v)", v, err)
	}
	if v, err := d.Bind(golem.DECIMAL(10, 2), float64(1.5)); err != nil || v != 1.5 {
		t.Fatalf("Bind(DECIMAL, float64) = (%v, %v)", v, err)
	}
}

func TestBind_DECIMAL_UnsupportedType(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.DECIMAL(10, 2), "nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBind_Strings(t *testing.T) {
	d := dialect{}
	for _, kind := range []golem.ColumnType{golem.CHAR(10), golem.VARCHAR(50), golem.TEXT()} {
		if v, err := d.Bind(kind, "hello"); err != nil || v != "hello" {
			t.Fatalf("Bind(%s, string) = (%v, %v)", kind.Kind(), v, err)
		}
		if v, err := d.Bind(kind, []byte("hello")); err != nil || v != "hello" {
			t.Fatalf("Bind(%s, []byte) = (%v, %v)", kind.Kind(), v, err)
		}
	}
}

func TestBind_Strings_UnsupportedType(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.VARCHAR(50), 5)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBind_Temporal_TimeValue(t *testing.T) {
	d := dialect{}
	now := time.Now()
	for _, kind := range []golem.ColumnType{golem.DATE(), golem.DATETIME(), golem.TIME()} {
		if v, err := d.Bind(kind, now); err != nil || v != now {
			t.Fatalf("Bind(%s, time.Time) = (%v, %v)", kind.Kind(), v, err)
		}
	}
}

func TestBind_Temporal_PointerValue(t *testing.T) {
	d := dialect{}
	now := time.Now()
	if v, err := d.Bind(golem.DATETIME(), &now); err != nil || v != now {
		t.Fatalf("Bind(DATETIME, *time.Time) = (%v, %v)", v, err)
	}
	var nilTime *time.Time
	if v, err := d.Bind(golem.DATETIME(), nilTime); err != nil || v != nil {
		t.Fatalf("Bind(DATETIME, nil *time.Time) = (%v, %v)", v, err)
	}
}

func TestBind_Temporal_UnsupportedType(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.DATETIME(), "nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBind_BLOB(t *testing.T) {
	d := dialect{}
	if v, err := d.Bind(golem.BLOB(), []byte("data")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if string(v.([]byte)) != "data" {
		t.Fatalf("got %v", v)
	}
	if v, err := d.Bind(golem.BLOB(), "data"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if string(v.([]byte)) != "data" {
		t.Fatalf("got %v", v)
	}
}

func TestBind_BLOB_UnsupportedType(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.BLOB(), 5)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBind_UUID_StringValue(t *testing.T) {
	d := dialect{}
	v, err := d.Bind(golem.UUID(), "123e4567-e89b-12d3-a456-426614174000")
	if err != nil || v != "123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("Bind(UUID, string) = (%v, %v)", v, err)
	}
}

func TestBind_UUID_ByteArrayValue(t *testing.T) {
	d := dialect{}
	var arr [16]byte
	copy(arr[:], []byte{0x12, 0x3e, 0x45, 0x67, 0xe8, 0x9b, 0x12, 0xd3, 0xa4, 0x56, 0x42, 0x66, 0x14, 0x17, 0x40, 0x00})
	v, err := d.Bind(golem.UUID(), arr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("got %v", v)
	}
}

func TestBind_UUID_UnsupportedType(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.UUID(), 5)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBind_JSON(t *testing.T) {
	d := dialect{}
	if v, err := d.Bind(golem.JSON(), `{"a":1}`); err != nil || v != `{"a":1}` {
		t.Fatalf("Bind(JSON, string) = (%v, %v)", v, err)
	}
	if v, err := d.Bind(golem.JSON(), []byte(`{"a":1}`)); err != nil || v != `{"a":1}` {
		t.Fatalf("Bind(JSON, []byte) = (%v, %v)", v, err)
	}
}

func TestBind_JSON_UnsupportedType(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.JSON(), 5)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBind_UnrecognizedKind(t *testing.T) {
	d := dialect{}
	_, err := d.Bind(golem.ColumnType{}, "x")
	if err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// Scan
// -----------------------------------------------------------------------

func TestScan_Nil(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.VARCHAR(50), nil, &dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScan_BOOLEAN(t *testing.T) {
	d := dialect{}
	var dest bool
	if err := d.Scan(golem.BOOLEAN(), true, &dest); err != nil || !dest {
		t.Fatalf("Scan(BOOLEAN, true) dest=%v err=%v", dest, err)
	}
}

func TestScan_BOOLEAN_WrongDestType(t *testing.T) {
	d := dialect{}
	var dest int
	if err := d.Scan(golem.BOOLEAN(), true, &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_BOOLEAN_UnsupportedRawType(t *testing.T) {
	d := dialect{}
	var dest bool
	if err := d.Scan(golem.BOOLEAN(), 3.14, &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_Integers(t *testing.T) {
	d := dialect{}
	for _, kind := range []golem.ColumnType{golem.SMALLINT(), golem.INTEGER(), golem.BIGINT()} {
		var dest int64
		if err := d.Scan(kind, int64(5), &dest); err != nil || dest != 5 {
			t.Fatalf("Scan(%s, int64) dest=%v err=%v", kind.Kind(), dest, err)
		}
	}
}

func TestScan_Integers_WrongDestType(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.BIGINT(), int64(5), &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_Integers_UnsupportedRawType(t *testing.T) {
	d := dialect{}
	var dest int64
	if err := d.Scan(golem.BIGINT(), 3.14, &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_DECIMAL(t *testing.T) {
	d := dialect{}
	var dest float64
	if err := d.Scan(golem.DECIMAL(10, 2), float64(1.5), &dest); err != nil || dest != 1.5 {
		t.Fatalf("Scan(DECIMAL, float64) dest=%v err=%v", dest, err)
	}
}

func TestScan_DECIMAL_WrongDestType(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.DECIMAL(10, 2), float64(1.5), &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_DECIMAL_UnsupportedRawType(t *testing.T) {
	d := dialect{}
	var dest float64
	if err := d.Scan(golem.DECIMAL(10, 2), true, &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_Strings(t *testing.T) {
	d := dialect{}
	for _, kind := range []golem.ColumnType{golem.CHAR(10), golem.VARCHAR(50), golem.TEXT()} {
		var dest string
		if err := d.Scan(kind, "hello", &dest); err != nil || dest != "hello" {
			t.Fatalf("Scan(%s, string) dest=%v err=%v", kind.Kind(), dest, err)
		}
		if err := d.Scan(kind, []byte("hello"), &dest); err != nil || dest != "hello" {
			t.Fatalf("Scan(%s, []byte) dest=%v err=%v", kind.Kind(), dest, err)
		}
	}
}

func TestScan_Strings_WrongDestType(t *testing.T) {
	d := dialect{}
	var dest int
	if err := d.Scan(golem.VARCHAR(50), "hello", &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_Strings_UnsupportedRawType(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.VARCHAR(50), 5, &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_Temporal(t *testing.T) {
	d := dialect{}
	now := time.Now()
	for _, kind := range []golem.ColumnType{golem.DATE(), golem.DATETIME(), golem.TIME()} {
		var dest time.Time
		if err := d.Scan(kind, now, &dest); err != nil || !dest.Equal(now) {
			t.Fatalf("Scan(%s, time.Time) dest=%v err=%v", kind.Kind(), dest, err)
		}
	}
}

func TestScan_Temporal_WrongDestType(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.DATETIME(), time.Now(), &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_Temporal_UnsupportedRawType(t *testing.T) {
	d := dialect{}
	var dest time.Time
	if err := d.Scan(golem.DATETIME(), "not a time", &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_BLOB(t *testing.T) {
	d := dialect{}
	var dest []byte
	if err := d.Scan(golem.BLOB(), []byte("data"), &dest); err != nil || string(dest) != "data" {
		t.Fatalf("Scan(BLOB, []byte) dest=%v err=%v", dest, err)
	}
	if err := d.Scan(golem.BLOB(), "data", &dest); err != nil || string(dest) != "data" {
		t.Fatalf("Scan(BLOB, string) dest=%v err=%v", dest, err)
	}
}

func TestScan_BLOB_WrongDestType(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.BLOB(), []byte("data"), &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_BLOB_UnsupportedRawType(t *testing.T) {
	d := dialect{}
	var dest []byte
	if err := d.Scan(golem.BLOB(), 5, &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_UUID(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.UUID(), "abc", &dest); err != nil || dest != "abc" {
		t.Fatalf("Scan(UUID, string) dest=%v err=%v", dest, err)
	}
	if err := d.Scan(golem.UUID(), []byte("abc"), &dest); err != nil || dest != "abc" {
		t.Fatalf("Scan(UUID, []byte) dest=%v err=%v", dest, err)
	}
}

func TestScan_UUID_WrongDestType(t *testing.T) {
	d := dialect{}
	var dest int
	if err := d.Scan(golem.UUID(), "abc", &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_UUID_UnsupportedRawType(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.UUID(), 5, &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_JSON(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.JSON(), `{"a":1}`, &dest); err != nil || dest != `{"a":1}` {
		t.Fatalf("Scan(JSON, string) dest=%v err=%v", dest, err)
	}
	if err := d.Scan(golem.JSON(), []byte(`{"a":1}`), &dest); err != nil || dest != `{"a":1}` {
		t.Fatalf("Scan(JSON, []byte) dest=%v err=%v", dest, err)
	}
}

func TestScan_JSON_WrongDestType(t *testing.T) {
	d := dialect{}
	var dest int
	if err := d.Scan(golem.JSON(), `{"a":1}`, &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_JSON_UnsupportedRawType(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.JSON(), 5, &dest); err == nil {
		t.Fatal("expected error")
	}
}

func TestScan_UnrecognizedKind(t *testing.T) {
	d := dialect{}
	var dest string
	if err := d.Scan(golem.ColumnType{}, "x", &dest); err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// quoteIdent / aggregateSQLFunc / projectionSQL
// -----------------------------------------------------------------------

func TestQuoteIdent_Simple(t *testing.T) {
	if got := quoteIdent("col"); got != "[col]" {
		t.Fatalf("quoteIdent(col) = %q", got)
	}
}

func TestQuoteIdent_Dotted(t *testing.T) {
	if got := quoteIdent("table.col"); got != "[table].[col]" {
		t.Fatalf("quoteIdent(table.col) = %q", got)
	}
}

func TestAggregateSQLFunc(t *testing.T) {
	if got := aggregateSQLFunc("count_all", ""); got != "COUNT(*)" {
		t.Fatalf("count_all = %q", got)
	}
	if got := aggregateSQLFunc("count", "col"); got != "COUNT([col])" {
		t.Fatalf("count = %q", got)
	}
	if got := aggregateSQLFunc("sum", "col"); got != "CAST(SUM([col]) AS FLOAT)" {
		t.Fatalf("sum = %q", got)
	}
	if got := aggregateSQLFunc("avg", "col"); got != "CAST(AVG([col]) AS FLOAT)" {
		t.Fatalf("avg = %q", got)
	}
	if got := aggregateSQLFunc("unknown", "col"); got != "[col]" {
		t.Fatalf("unknown = %q", got)
	}
}

func TestProjectionSQL(t *testing.T) {
	p1 := stmt.Projection{Column: "col", Alias: "col"}
	if got := projectionSQL(p1); got != "[col] AS [col]" {
		t.Fatalf("plain projection = %q", got)
	}
	p2 := stmt.Projection{Func: "count", Column: "col", Alias: "cnt"}
	if got := projectionSQL(p2); got != "COUNT([col]) AS [cnt]" {
		t.Fatalf("agg projection = %q", got)
	}
}

// -----------------------------------------------------------------------
// compilePredicate
// -----------------------------------------------------------------------

func TestCompilePredicate_Nil(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(nil, &argOffset, &args)
	if err != nil || sql != "" {
		t.Fatalf("compilePredicate(nil) = (%q, %v)", sql, err)
	}
}

func TestCompilePredicate_ComparisonOps(t *testing.T) {
	cases := []struct {
		op   string
		want string
	}{
		{"eq", "[age]=@p1"},
		{"gt", "[age]>@p1"},
		{"gte", "[age]>=@p1"},
		{"lt", "[age]<@p1"},
		{"lte", "[age]<=@p1"},
		{"like", "[age] LIKE @p1"},
	}
	for _, tc := range cases {
		argOffset := 1
		var args []any
		sql, err := compilePredicate(stmt.Comparison{Column: "age", Op: tc.op, Value: 18}, &argOffset, &args)
		if err != nil {
			t.Fatalf("op %s: %v", tc.op, err)
		}
		if sql != tc.want {
			t.Fatalf("op %s: sql = %q, want %q", tc.op, sql, tc.want)
		}
		if len(args) != 1 || args[0] != 18 {
			t.Fatalf("op %s: args = %v", tc.op, args)
		}
		if argOffset != 2 {
			t.Fatalf("op %s: argOffset = %d, want 2", tc.op, argOffset)
		}
	}
}

func TestCompilePredicate_IsNull(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Comparison{Column: "id", Op: "is_null"}, &argOffset, &args)
	if err != nil || sql != "[id] IS NULL" || len(args) != 0 {
		t.Fatalf("is_null: sql=%q args=%v err=%v", sql, args, err)
	}
}

func TestCompilePredicate_In_Slice(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Comparison{Column: "id", Op: "in", Value: []int{1, 2, 3}}, &argOffset, &args)
	if err != nil || sql != "[id] IN (@p1,@p2,@p3)" || len(args) != 3 {
		t.Fatalf("in slice: sql=%q args=%v err=%v", sql, args, err)
	}
}

func TestCompilePredicate_In_EmptySlice(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Comparison{Column: "id", Op: "in", Value: []int{}}, &argOffset, &args)
	if err != nil || sql != "1=0" {
		t.Fatalf("in empty: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_In_NonSlice(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Comparison{Column: "id", Op: "in", Value: 5}, &argOffset, &args)
	if err != nil || sql != "[id] IN (@p1)" || len(args) != 1 {
		t.Fatalf("in non-slice: sql=%q args=%v err=%v", sql, args, err)
	}
}

func TestCompilePredicate_UnsupportedComparisonOp(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(stmt.Comparison{Column: "id", Op: "bogus"}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompilePredicate_Logical_And(t *testing.T) {
	argOffset := 1
	var args []any
	p := stmt.Logical{Op: "and", Predicates: []stmt.Predicate{
		stmt.Comparison{Column: "a", Op: "eq", Value: 1},
		stmt.Comparison{Column: "b", Op: "eq", Value: 2},
	}}
	sql, err := compilePredicate(p, &argOffset, &args)
	if err != nil || sql != "([a]=@p1 AND [b]=@p2)" {
		t.Fatalf("and: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_Logical_Or(t *testing.T) {
	argOffset := 1
	var args []any
	p := stmt.Logical{Op: "or", Predicates: []stmt.Predicate{
		stmt.Comparison{Column: "a", Op: "eq", Value: 1},
		stmt.Comparison{Column: "b", Op: "eq", Value: 2},
	}}
	sql, err := compilePredicate(p, &argOffset, &args)
	if err != nil || sql != "([a]=@p1 OR [b]=@p2)" {
		t.Fatalf("or: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_Logical_Empty(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Logical{Op: "and"}, &argOffset, &args)
	if err != nil || sql != "" {
		t.Fatalf("empty logical: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_Logical_AllPartsEmpty(t *testing.T) {
	argOffset := 1
	var args []any
	p := stmt.Logical{Op: "and", Predicates: []stmt.Predicate{
		stmt.Logical{Op: "and"},
	}}
	sql, err := compilePredicate(p, &argOffset, &args)
	if err != nil || sql != "" {
		t.Fatalf("all parts empty: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_Logical_PropagatesError(t *testing.T) {
	argOffset := 1
	var args []any
	p := stmt.Logical{Op: "and", Predicates: []stmt.Predicate{
		stmt.Comparison{Column: "a", Op: "bogus"},
	}}
	_, err := compilePredicate(p, &argOffset, &args)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompilePredicate_Not(t *testing.T) {
	argOffset := 1
	var args []any
	p := stmt.Not{Predicate: stmt.Comparison{Column: "a", Op: "eq", Value: 1}}
	sql, err := compilePredicate(p, &argOffset, &args)
	if err != nil || sql != "NOT ([a]=@p1)" {
		t.Fatalf("not: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_Not_EmptyInner(t *testing.T) {
	argOffset := 1
	var args []any
	p := stmt.Not{Predicate: stmt.Logical{Op: "and"}}
	sql, err := compilePredicate(p, &argOffset, &args)
	if err != nil || sql != "" {
		t.Fatalf("not empty: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_Not_PropagatesError(t *testing.T) {
	argOffset := 1
	var args []any
	p := stmt.Not{Predicate: stmt.Comparison{Column: "a", Op: "bogus"}}
	_, err := compilePredicate(p, &argOffset, &args)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompilePredicate_AggregateComparison(t *testing.T) {
	cases := []struct {
		op   string
		want string
	}{
		{"eq", "COUNT([id])=@p1"},
		{"gt", "COUNT([id])>@p1"},
		{"gte", "COUNT([id])>=@p1"},
		{"lt", "COUNT([id])<@p1"},
		{"lte", "COUNT([id])<=@p1"},
	}
	for _, tc := range cases {
		argOffset := 1
		var args []any
		sql, err := compilePredicate(stmt.AggregateComparison{Func: "count", Column: "id", Op: tc.op, Value: 1}, &argOffset, &args)
		if err != nil || sql != tc.want {
			t.Fatalf("agg op %s: sql=%q err=%v", tc.op, sql, err)
		}
	}
}

func TestCompilePredicate_AggregateComparison_UnsupportedOp(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(stmt.AggregateComparison{Func: "count", Column: "id", Op: "bogus"}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected error")
	}
}

// unknownPredicate embeds stmt.Comparison purely to inherit its unexported
// isPredicate() method while remaining a distinct concrete type, the only
// way to reach compilePredicate's default case from outside internal/stmt.
type unknownPredicate struct {
	stmt.Comparison
}

func TestCompilePredicate_UnknownType(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(unknownPredicate{}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// CompileSelect
// -----------------------------------------------------------------------

func TestCompileSelect_Basic(t *testing.T) {
	d := &dialect{}
	sql, args, err := d.CompileSelect(&stmt.Select{Table: "users"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT * FROM [users]" || len(args) != 0 {
		t.Fatalf("sql=%q args=%v", sql, args)
	}
}

func TestCompileSelect_Columns(t *testing.T) {
	d := &dialect{}
	sql, _, err := d.CompileSelect(&stmt.Select{Table: "users", Columns: []string{"id", "name"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT [id], [name] FROM [users]" {
		t.Fatalf("sql=%q", sql)
	}
}

func TestCompileSelect_Count(t *testing.T) {
	d := &dialect{}
	sql, _, err := d.CompileSelect(&stmt.Select{Table: "users", Count: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT COUNT(*) FROM [users]" {
		t.Fatalf("sql=%q", sql)
	}
}

func TestCompileSelect_Projections(t *testing.T) {
	d := &dialect{}
	sql, _, err := d.CompileSelect(&stmt.Select{
		Table:       "users",
		Projections: []stmt.Projection{{Column: "category", Alias: "category"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT [category] AS [category] FROM [users]" {
		t.Fatalf("sql=%q", sql)
	}
}

func TestCompileSelect_Where(t *testing.T) {
	d := &dialect{}
	sql, args, err := d.CompileSelect(&stmt.Select{
		Table: "users",
		Where: stmt.Comparison{Column: "id", Op: "eq", Value: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT * FROM [users] WHERE [id]=@p1" || len(args) != 1 {
		t.Fatalf("sql=%q args=%v", sql, args)
	}
}

func TestCompileSelect_WhereCompileError_Propagates(t *testing.T) {
	d := &dialect{}
	_, _, err := d.CompileSelect(&stmt.Select{
		Table: "users",
		Where: stmt.Comparison{Column: "id", Op: "bogus"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompileSelect_GroupByHaving(t *testing.T) {
	d := &dialect{}
	sql, args, err := d.CompileSelect(&stmt.Select{
		Table:   "users",
		GroupBy: []string{"category"},
		Having:  stmt.AggregateComparison{Func: "count", Column: "id", Op: "gt", Value: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT * FROM [users] GROUP BY [category] HAVING COUNT([id])>@p1" || len(args) != 1 {
		t.Fatalf("sql=%q args=%v", sql, args)
	}
}

func TestCompileSelect_HavingCompileError_Propagates(t *testing.T) {
	d := &dialect{}
	_, _, err := d.CompileSelect(&stmt.Select{
		Table:  "users",
		Having: stmt.AggregateComparison{Func: "count", Column: "id", Op: "bogus"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompileSelect_OrderBy(t *testing.T) {
	d := &dialect{}
	sql, _, err := d.CompileSelect(&stmt.Select{
		Table:   "users",
		OrderBy: []stmt.OrderElement{{Column: "id", Desc: true}, {Column: "name"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT * FROM [users] ORDER BY [id] DESC, [name] ASC" {
		t.Fatalf("sql=%q", sql)
	}
}

func TestCompileSelect_LimitOffset_WithOrderBy(t *testing.T) {
	d := &dialect{}
	limit, offset := 10, 5
	sql, args, err := d.CompileSelect(&stmt.Select{
		Table:   "users",
		OrderBy: []stmt.OrderElement{{Column: "id"}},
		Limit:   &limit, Offset: &offset,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "SELECT * FROM [users] ORDER BY [id] ASC OFFSET @p1 ROWS FETCH NEXT @p2 ROWS ONLY"
	if sql != want || len(args) != 2 {
		t.Fatalf("sql=%q args=%v, want %q", sql, args, want)
	}
	if args[0] != 5 || args[1] != 10 {
		t.Fatalf("args=%v, want [5 10]", args)
	}
}

func TestCompileSelect_OffsetOnly_NoFetch(t *testing.T) {
	d := &dialect{}
	offset := 5
	sql, args, err := d.CompileSelect(&stmt.Select{
		Table:   "users",
		OrderBy: []stmt.OrderElement{{Column: "id"}},
		Offset:  &offset,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT * FROM [users] ORDER BY [id] ASC OFFSET @p1 ROWS" || len(args) != 1 {
		t.Fatalf("sql=%q args=%v", sql, args)
	}
}

func TestCompileSelect_LimitOnly_InjectsZeroOffset(t *testing.T) {
	d := &dialect{}
	limit := 10
	sql, args, err := d.CompileSelect(&stmt.Select{
		Table:   "users",
		OrderBy: []stmt.OrderElement{{Column: "id"}},
		Limit:   &limit,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT * FROM [users] ORDER BY [id] ASC OFFSET @p1 ROWS FETCH NEXT @p2 ROWS ONLY" || len(args) != 2 {
		t.Fatalf("sql=%q args=%v", sql, args)
	}
	if args[0] != 0 || args[1] != 10 {
		t.Fatalf("args=%v, want [0 10]", args)
	}
}

func TestCompileSelect_Pagination_NoOrderBy_InjectsPrimaryKey(t *testing.T) {
	d := &dialect{}
	limit := 10
	sql, _, err := d.CompileSelect(&stmt.Select{
		Table:      "users",
		Limit:      &limit,
		PrimaryKey: []string{"id"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT * FROM [users] ORDER BY [id] ASC OFFSET @p1 ROWS FETCH NEXT @p2 ROWS ONLY" {
		t.Fatalf("sql=%q", sql)
	}
}

func TestCompileSelect_Pagination_NoOrderBy_NoPrimaryKey_Errors(t *testing.T) {
	d := &dialect{}
	limit := 10
	_, _, err := d.CompileSelect(&stmt.Select{Table: "users", Limit: &limit})
	if err == nil {
		t.Fatal("expected error — SQL Server requires ORDER BY for OFFSET/FETCH")
	}
}

func TestCompileSelect_CountPagination_NoOrderBy_UsesSelectNull(t *testing.T) {
	d := &dialect{}
	limit := 1
	sql, _, err := d.CompileSelect(&stmt.Select{
		Table: "users",
		Count: true,
		Limit: &limit,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT COUNT(*) FROM [users] ORDER BY (SELECT NULL) OFFSET @p1 ROWS FETCH NEXT @p2 ROWS ONLY" {
		t.Fatalf("sql=%q", sql)
	}
}

func TestCompileSelect_Joins(t *testing.T) {
	d := &dialect{}
	sql, args, err := d.CompileSelect(&stmt.Select{
		Table: "parent",
		Joins: []stmt.Join{{
			Type:  "inner",
			Table: "child",
			On:    []stmt.OnCondition{{LeftCol: "child.parent_id", RightCol: "parent.id"}},
			Where: stmt.Comparison{Column: "child.name", Op: "eq", Value: "x"},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "SELECT * FROM [parent] INNER JOIN [child] ON [child].[parent_id] = [parent].[id] AND [child].[name]=@p1"
	if sql != want || len(args) != 1 {
		t.Fatalf("sql=%q args=%v, want %q", sql, args, want)
	}
}

func TestCompileSelect_JoinWhereCompileError_Propagates(t *testing.T) {
	d := &dialect{}
	_, _, err := d.CompileSelect(&stmt.Select{
		Table: "parent",
		Joins: []stmt.Join{{
			Type:  "inner",
			Table: "child",
			On:    []stmt.OnCondition{{LeftCol: "child.parent_id", RightCol: "parent.id"}},
			Where: stmt.Comparison{Column: "child.name", Op: "bogus"},
		}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompileSelect_Lock(t *testing.T) {
	d := &dialect{}
	sql, _, err := d.CompileSelect(&stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: "update"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "SELECT * FROM [users] WITH (UPDLOCK, ROWLOCK)" {
		t.Fatalf("sql=%q", sql)
	}
}

func TestCompileSelect_LockError_Propagates(t *testing.T) {
	d := &dialect{}
	_, _, err := d.CompileSelect(&stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: "bogus"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// lockClauseSQL
// -----------------------------------------------------------------------

func TestLockClauseSQL(t *testing.T) {
	cases := []struct {
		strength string
		wait     string
		want     string
	}{
		{"update", "", " WITH (UPDLOCK, ROWLOCK)"},
		{"share", "", " WITH (HOLDLOCK, ROWLOCK)"},
		{"update", "nowait", " WITH (UPDLOCK, ROWLOCK, NOWAIT)"},
		{"update", "skip_locked", " WITH (UPDLOCK, ROWLOCK, READPAST)"},
	}
	for _, tc := range cases {
		sql, err := lockClauseSQL(&stmt.LockClause{Strength: tc.strength, Wait: tc.wait})
		if err != nil || sql != tc.want {
			t.Fatalf("strength=%s wait=%s: sql=%q err=%v", tc.strength, tc.wait, sql, err)
		}
	}
}

func TestLockClauseSQL_UnsupportedStrength(t *testing.T) {
	for _, strength := range []string{"no_key_update", "key_share", "bogus"} {
		_, err := lockClauseSQL(&stmt.LockClause{Strength: strength})
		if err == nil {
			t.Fatalf("strength %s: expected error", strength)
		}
	}
}

func TestLockClauseSQL_UnsupportedWait(t *testing.T) {
	_, err := lockClauseSQL(&stmt.LockClause{Strength: "update", Wait: "bogus"})
	if err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// CompileDelete
// -----------------------------------------------------------------------

func TestCompileDelete_Basic(t *testing.T) {
	d := &dialect{}
	sql, args, err := d.CompileDelete(&stmt.Delete{Table: "users"})
	if err != nil || sql != "DELETE FROM [users]" || len(args) != 0 {
		t.Fatalf("sql=%q args=%v err=%v", sql, args, err)
	}
}

func TestCompileDelete_Where(t *testing.T) {
	d := &dialect{}
	sql, args, err := d.CompileDelete(&stmt.Delete{Table: "users", Where: stmt.Comparison{Column: "id", Op: "eq", Value: 1}})
	if err != nil || sql != "DELETE FROM [users] WHERE [id]=@p1" || len(args) != 1 {
		t.Fatalf("sql=%q args=%v err=%v", sql, args, err)
	}
}

func TestCompileDelete_WhereCompileError_Propagates(t *testing.T) {
	d := &dialect{}
	_, _, err := d.CompileDelete(&stmt.Delete{Table: "users", Where: stmt.Comparison{Column: "id", Op: "bogus"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// Insert / Update
// -----------------------------------------------------------------------

func newMockDialect(t *testing.T) (*dialect, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &dialect{db: db}, mock
}

func TestDialect_Insert_Success(t *testing.T) {
	d, mock := newMockDialect(t)

	mock.ExpectQuery(`INSERT INTO \[users\] \(\[name\]\) OUTPUT INSERTED\.\* VALUES \(@p1\)`).WithArgs("Ada").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "Ada"))

	row, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table:   "users",
		Columns: []string{"name"},
		Values:  []driver.Value{"Ada"},
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if row["name"] != "Ada" {
		t.Fatalf("row = %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestDialect_Insert_QueryError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`INSERT INTO \[users\]`).WillReturnError(errors.New("insert error"))

	_, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table: "users", Columns: []string{"name"}, Values: []driver.Value{"Ada"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_Insert_ZeroRows(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`INSERT INTO \[users\]`).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	_, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table: "users", Columns: []string{"name"}, Values: []driver.Value{"Ada"},
	})
	if err == nil {
		t.Fatal("expected error for zero rows returned")
	}
}

func TestDialect_Insert_CollectRowsError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`INSERT INTO \[users\]`).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name"}).RowError(0, errors.New("row scan error")).AddRow(int64(1), "Ada"))

	_, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table: "users", Columns: []string{"name"}, Values: []driver.Value{"Ada"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_Update_Success(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`UPDATE \[users\] SET \[name\]=@p1 OUTPUT INSERTED\.\* WHERE \[id\]=@p2`).WithArgs("Ada2", int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "Ada2"))

	rows, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "Ada2"}},
		Where: stmt.Comparison{Column: "id", Op: "eq", Value: int64(1)},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(rows) != 1 || rows[0]["name"] != "Ada2" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestDialect_Update_NoWhere(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`UPDATE \[users\] SET \[name\]=@p1 OUTPUT INSERTED\.\*`).WithArgs("x").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "x").AddRow(int64(2), "x"))

	rows, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestDialect_Update_WhereCompileError_Propagates(t *testing.T) {
	d := &dialect{}
	_, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
		Where: stmt.Comparison{Column: "id", Op: "bogus"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_Update_QueryError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`UPDATE \[users\]`).WillReturnError(errors.New("update error"))

	_, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_Update_CollectRowsError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`UPDATE \[users\]`).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name"}).RowError(0, errors.New("row scan error")).AddRow(int64(1), "x"))

	_, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// Query / Exec / ExecRaw
// -----------------------------------------------------------------------

func TestDialect_Query_Success(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	rows, err := d.Query(context.Background(), nil, "SELECT 1", nil)
	if err != nil || len(rows) != 1 || rows[0]["id"] != int64(1) {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
}

func TestDialect_Query_Error(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery("SELECT 1").WillReturnError(errors.New("query error"))

	_, err := d.Query(context.Background(), nil, "SELECT 1", nil)
	if err == nil || err.Error() != "query error" {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestDialect_Query_ScanError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"id"}).RowError(0, errors.New("row scan error")).AddRow(int64(1)))

	_, err := d.Query(context.Background(), nil, "SELECT 1", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_Exec_Success(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectExec("UPDATE users").WillReturnResult(sqlmock.NewResult(0, 3))

	n, err := d.Exec(context.Background(), nil, "UPDATE users SET x=1", nil)
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestDialect_Exec_Error(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectExec("UPDATE users").WillReturnError(errors.New("exec error"))

	_, err := d.Exec(context.Background(), nil, "UPDATE users SET x=1", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_ExecRaw_Success(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery("SELECT \\* FROM users").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)).AddRow(int64(2)))

	rows, n, err := d.ExecRaw(context.Background(), nil, "SELECT * FROM users", nil)
	if err != nil || n != 2 || len(rows) != 2 {
		t.Fatalf("rows=%v n=%d err=%v", rows, n, err)
	}
}

func TestDialect_ExecRaw_QueryError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery("SELECT \\* FROM users").WillReturnError(errors.New("query error"))

	_, _, err := d.ExecRaw(context.Background(), nil, "SELECT * FROM users", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_ExecRaw_CollectRowsError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery("SELECT \\* FROM users").WillReturnRows(sqlmock.NewRows([]string{"id"}).RowError(0, errors.New("row scan error")).AddRow(int64(1)))

	_, _, err := d.ExecRaw(context.Background(), nil, "SELECT * FROM users", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_ExecRaw_WriteStatement_Success(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectExec("UPDATE users").WillReturnResult(sqlmock.NewResult(0, 3))

	rows, affected, err := d.ExecRaw(context.Background(), nil, "UPDATE users SET name = @p1", []any{"x"})
	if err != nil || affected != 3 || len(rows) != 0 {
		t.Fatalf("rows=%v affected=%d err=%v", rows, affected, err)
	}
}

func TestDialect_ExecRaw_WriteStatement_ExecError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectExec("UPDATE users").WillReturnError(errors.New("exec error"))

	_, _, err := d.ExecRaw(context.Background(), nil, "UPDATE users SET name = @p1", []any{"x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_ExecRaw_WriteStatement_RowsAffectedError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectExec("UPDATE users").WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected error")))

	_, _, err := d.ExecRaw(context.Background(), nil, "UPDATE users SET name = @p1", []any{"x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCollectRows_ColumnsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	rows.Close() // Columns() errors once rows are closed

	_, err = collectRows(rows)
	if err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// normalizeCell
// -----------------------------------------------------------------------

func TestNormalizeCell_UniqueIdentifier(t *testing.T) {
	raw := []byte{0x67, 0x45, 0x3e, 0x12, 0x9b, 0xe8, 0xd3, 0x12, 0xa4, 0x56, 0x42, 0x66, 0x14, 0x17, 0x40, 0x00}
	got := normalizeCell("UNIQUEIDENTIFIER", raw)
	want := "123e4567-e89b-12d3-a456-426614174000"
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNormalizeCell_UniqueIdentifier_WrongLength_LeftUntouched(t *testing.T) {
	raw := []byte{0x01, 0x02}
	got := normalizeCell("UNIQUEIDENTIFIER", raw)
	b, ok := got.([]byte)
	if !ok || len(b) != 2 {
		t.Fatalf("got %v, want raw bytes left untouched", got)
	}
}

func TestNormalizeCell_UniqueIdentifier_NonByteSlice_LeftUntouched(t *testing.T) {
	got := normalizeCell("UNIQUEIDENTIFIER", "already-a-string")
	if got != "already-a-string" {
		t.Fatalf("got %v, want untouched string", got)
	}
}

func TestNormalizeCell_NonUniqueIdentifierType_LeftUntouched(t *testing.T) {
	got := normalizeCell("NVARCHAR", "hello")
	if got != "hello" {
		t.Fatalf("got %v, want untouched", got)
	}
}

func TestNormalizeCell_Decimal_ParsesByteSliceToFloat64(t *testing.T) {
	got := normalizeCell("DECIMAL", []byte("19.99"))
	f, ok := got.(float64)
	if !ok || f != 19.99 {
		t.Fatalf("got %v (%T), want 19.99 (float64)", got, got)
	}
}

func TestNormalizeCell_Decimal_NonByteSlice_LeftUntouched(t *testing.T) {
	got := normalizeCell("DECIMAL", int64(5))
	if got != int64(5) {
		t.Fatalf("got %v, want untouched", got)
	}
}

func TestNormalizeCell_Decimal_UnparseableBytes_LeftUntouched(t *testing.T) {
	raw := []byte("not-a-number")
	got := normalizeCell("DECIMAL", raw)
	b, ok := got.([]byte)
	if !ok || string(b) != "not-a-number" {
		t.Fatalf("got %v, want raw bytes left untouched", got)
	}
}

func TestReverseBytes(t *testing.T) {
	got := reverseBytes([]byte{1, 2, 3, 4})
	want := []byte{4, 3, 2, 1}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// -----------------------------------------------------------------------
// mapError / IsConflict
// -----------------------------------------------------------------------

func TestMapError_Nil(t *testing.T) {
	if mapError(nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestMapError_DuplicateKey(t *testing.T) {
	for _, num := range []int32{2627, 2601} {
		err := mapError(mssqllib.Error{Number: num, Message: "dup"})
		if !errors.Is(err, golem.ErrDuplicateKey) {
			t.Fatalf("number %d: expected ErrDuplicateKey, got %v", num, err)
		}
	}
}

func TestMapError_ForeignKeyViolation(t *testing.T) {
	err := mapError(mssqllib.Error{Number: 547, Message: "fk"})
	if !errors.Is(err, golem.ErrForeignKeyViolation) {
		t.Fatalf("expected ErrForeignKeyViolation, got %v", err)
	}
}

func TestMapError_UnmappedError(t *testing.T) {
	// mssql.Error contains a []Error field ("All"), so it isn't comparable
	// with == / != — assert identity via errors.As + Number instead.
	orig := mssqllib.Error{Number: 208, Message: "invalid object name"}
	err := mapError(orig)
	var got mssqllib.Error
	if !errors.As(err, &got) || got.Number != 208 {
		t.Fatalf("expected unmapped error to pass through unchanged, got %v", err)
	}
}

func TestMapError_NonMssqlError(t *testing.T) {
	orig := errors.New("plain error")
	if mapError(orig) != orig {
		t.Fatal("expected passthrough")
	}
}

func TestIsConflict_Nil(t *testing.T) {
	d := &dialect{}
	if d.IsConflict(nil) {
		t.Fatal("expected false")
	}
}

func TestIsConflict_True(t *testing.T) {
	d := &dialect{}
	for _, num := range []int32{2627, 2601, 547} {
		if !d.IsConflict(mssqllib.Error{Number: num}) {
			t.Fatalf("number %d: expected true", num)
		}
	}
}

func TestIsConflict_NonConflictError(t *testing.T) {
	d := &dialect{}
	if d.IsConflict(mssqllib.Error{Number: 208}) {
		t.Fatal("expected false")
	}
}

func TestIsConflict_NonMssqlError(t *testing.T) {
	d := &dialect{}
	if d.IsConflict(errors.New("plain error")) {
		t.Fatal("expected false")
	}
}

// -----------------------------------------------------------------------
// Begin / getExecutor
// -----------------------------------------------------------------------

func TestDialect_Begin_Success(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectBegin()

	tx, err := d.Begin(context.Background(), nil)
	if err != nil || tx == nil {
		t.Fatalf("tx=%v err=%v", tx, err)
	}
}

func TestDialect_Begin_Error(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectBegin().WillReturnError(errors.New("begin error"))

	_, err := d.Begin(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMssqlTx_Commit(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	tx, err := d.Begin(context.Background(), nil)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := tx.(*mssqlTx).Commit(context.Background()); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

func TestMssqlTx_Rollback(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, err := d.Begin(context.Background(), nil)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := tx.(*mssqlTx).Rollback(context.Background()); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
}

func TestDialect_GetExecutor_DefaultsToPool(t *testing.T) {
	d := &dialect{db: nil}
	got := d.getExecutor(nil)
	if got != nil {
		t.Fatalf("expected getExecutor(nil conn) to return d.db (nil), got %v", got)
	}
}

func TestDialect_GetExecutor_UsesTxWhenPresent(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectBegin()
	txConn, err := d.Begin(context.Background(), nil)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	tx := golem.NewTx(d, txConn)
	got := d.getExecutor(tx)
	if got == nil {
		t.Fatal("expected non-nil executor")
	}
	if _, ok := got.(*sql.Tx); !ok {
		t.Fatalf("expected *sql.Tx, got %T", got)
	}
}
