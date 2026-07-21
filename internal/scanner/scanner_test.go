package scanner

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
)

// testStruct exercises every fieldKind.
type testStruct struct {
	IntVal     int
	Int8Val    int8
	Int16Val   int16
	Int32Val   int32
	Int64Val   int64
	UintVal    uint
	Uint8Val   uint8
	Uint16Val  uint16
	Uint32Val  uint32
	Uint64Val  uint64
	Float32Val float32
	Float64Val float64
	StringVal  string
	BoolVal    bool
	BytesVal   []byte
	TimeVal    time.Time
	NullStr    sql.NullString
	NullInt    sql.NullInt64
	NullFloat  sql.NullFloat64
	NullBool   sql.NullBool
	NullTime   sql.NullTime
	PtrStr     *string
	PtrBool    *bool
	PtrInt64   *int64
	CustomVal  customType
}

type customType struct{ X int }

// fakeScannerField implements database/sql.Scanner -- a field type golem
// knows nothing about, proving assignReflect delegates to Scan(raw) instead
// of trying (and failing) a direct field.Set.
type fakeScannerField struct{ inner string }

func (f *fakeScannerField) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("fakeScannerField: cannot scan %T", src)
	}
	f.inner = "scanned:" + s
	return nil
}

type scannerTestStruct struct {
	ID    int64
	Field fakeScannerField
}

// buildMeta builds an EntityMeta for testStruct using entity.New.
func buildMeta(t testing.TB) entity.EntityMeta {
	t.Helper()
	now := time.Now()
	e := entity.New(func(ts *testStruct, b *entity.Table) {
		b.Col(&ts.IntVal, golem.BIGINT())
		b.Col(&ts.Int8Val, golem.SMALLINT())
		b.Col(&ts.Int16Val, golem.SMALLINT())
		b.Col(&ts.Int32Val, golem.INTEGER())
		b.Col(&ts.Int64Val, golem.BIGINT())
		b.Col(&ts.UintVal, golem.BIGINT())
		b.Col(&ts.Uint8Val, golem.SMALLINT())
		b.Col(&ts.Uint16Val, golem.SMALLINT())
		b.Col(&ts.Uint32Val, golem.INTEGER())
		b.Col(&ts.Uint64Val, golem.BIGINT())
		b.Col(&ts.Float32Val, golem.FLOAT(32))
		b.Col(&ts.Float64Val, golem.FLOAT(64))
		b.Col(&ts.StringVal, golem.TEXT())
		b.Col(&ts.BoolVal, golem.BOOLEAN())
		b.Col(&ts.BytesVal, golem.BLOB())
		b.Col(&ts.TimeVal, golem.DATETIME())
		b.Col(&ts.NullStr, golem.TEXT())
		b.Col(&ts.NullInt, golem.BIGINT())
		b.Col(&ts.NullFloat, golem.FLOAT(64))
		b.Col(&ts.NullBool, golem.BOOLEAN())
		b.Col(&ts.NullTime, golem.DATETIME())
		b.Col(&ts.PtrStr, golem.TEXT()).Nullable()
		b.Col(&ts.PtrBool, golem.BOOLEAN()).Nullable()
		b.Col(&ts.PtrInt64, golem.BIGINT()).Nullable()
		b.Col(&ts.CustomVal, golem.TEXT())
		b.PrimaryKey(&ts.Int64Val)
		_ = now
	})
	return e.Describe()
}

func TestCompile_ProducesNonNilPlan(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	if p == nil {
		t.Fatal("Compile returned nil")
	}
	if len(p.assignments) != len(meta.Columns) {
		t.Fatalf("expected %d assignments, got %d", len(meta.Columns), len(p.assignments))
	}
}

func TestScanFromMap_PrimitiveInt(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"int64val": int64(42)}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Int64Val != 42 {
		t.Fatalf("expected 42, got %d", ts.Int64Val)
	}
}

func TestScanFromMap_AllPrimitives(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	now := time.Now().Truncate(time.Second)
	row := map[string]any{
		"intval":     int(1),
		"int8val":    int8(2),
		"int16val":   int16(3),
		"int32val":   int32(4),
		"int64val":   int64(5),
		"uintval":    uint(6),
		"uint8val":   uint8(7),
		"uint16val":  uint16(8),
		"uint32val":  uint32(9),
		"uint64val":  uint64(10),
		"float32val": float32(1.1),
		"float64val": float64(2.2),
		"stringval":  "hello",
		"boolval":    true,
		"bytesval":   []byte{1, 2, 3},
		"timeval":    now,
	}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.IntVal != 1 {
		t.Errorf("IntVal: got %d, want 1", ts.IntVal)
	}
	if ts.Int8Val != 2 {
		t.Errorf("Int8Val: got %d, want 2", ts.Int8Val)
	}
	if ts.Int16Val != 3 {
		t.Errorf("Int16Val: got %d, want 3", ts.Int16Val)
	}
	if ts.Int32Val != 4 {
		t.Errorf("Int32Val: got %d, want 4", ts.Int32Val)
	}
	if ts.Int64Val != 5 {
		t.Errorf("Int64Val: got %d, want 5", ts.Int64Val)
	}
	if ts.UintVal != 6 {
		t.Errorf("UintVal: got %d, want 6", ts.UintVal)
	}
	if ts.Uint8Val != 7 {
		t.Errorf("Uint8Val: got %d, want 7", ts.Uint8Val)
	}
	if ts.Uint16Val != 8 {
		t.Errorf("Uint16Val: got %d, want 8", ts.Uint16Val)
	}
	if ts.Uint32Val != 9 {
		t.Errorf("Uint32Val: got %d, want 9", ts.Uint32Val)
	}
	if ts.Uint64Val != 10 {
		t.Errorf("Uint64Val: got %d, want 10", ts.Uint64Val)
	}
	if ts.StringVal != "hello" {
		t.Errorf("StringVal: got %q, want %q", ts.StringVal, "hello")
	}
	if !ts.BoolVal {
		t.Error("BoolVal: want true")
	}
	if string(ts.BytesVal) != string([]byte{1, 2, 3}) {
		t.Error("BytesVal mismatch")
	}
	if !ts.TimeVal.Equal(now) {
		t.Errorf("TimeVal: got %v, want %v", ts.TimeVal, now)
	}
}

func TestScanFromMap_NullWrappers(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	now := time.Now().Truncate(time.Second)
	row := map[string]any{
		"nullstr":   sql.NullString{String: "x", Valid: true},
		"nullint":   sql.NullInt64{Int64: 7, Valid: true},
		"nullfloat": sql.NullFloat64{Float64: 3.14, Valid: true},
		"nullbool":  sql.NullBool{Bool: true, Valid: true},
		"nulltime":  sql.NullTime{Time: now, Valid: true},
	}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if !ts.NullStr.Valid || ts.NullStr.String != "x" {
		t.Errorf("NullStr: %+v", ts.NullStr)
	}
	if !ts.NullInt.Valid || ts.NullInt.Int64 != 7 {
		t.Errorf("NullInt: %+v", ts.NullInt)
	}
	if !ts.NullFloat.Valid {
		t.Errorf("NullFloat: %+v", ts.NullFloat)
	}
	if !ts.NullBool.Valid || !ts.NullBool.Bool {
		t.Errorf("NullBool: %+v", ts.NullBool)
	}
	if !ts.NullTime.Valid || !ts.NullTime.Time.Equal(now) {
		t.Errorf("NullTime: %+v", ts.NullTime)
	}
}

func TestScanFromMap_NullWrappersFromRaw(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	now := time.Now().Truncate(time.Second)
	row := map[string]any{
		"nullstr":   "raw",
		"nullint":   int64(99),
		"nullfloat": float64(1.5),
		"nullbool":  true,
		"nulltime":  now,
	}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if !ts.NullStr.Valid || ts.NullStr.String != "raw" {
		t.Errorf("NullStr from raw: %+v", ts.NullStr)
	}
	if !ts.NullInt.Valid || ts.NullInt.Int64 != 99 {
		t.Errorf("NullInt from raw: %+v", ts.NullInt)
	}
	if !ts.NullFloat.Valid {
		t.Errorf("NullFloat from raw: %+v", ts.NullFloat)
	}
	if !ts.NullBool.Valid || !ts.NullBool.Bool {
		t.Errorf("NullBool from raw: %+v", ts.NullBool)
	}
	if !ts.NullTime.Valid || !ts.NullTime.Time.Equal(now) {
		t.Errorf("NullTime from raw: %+v", ts.NullTime)
	}
}

func TestScanFromMap_MissingColumn(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Int64Val != 0 {
		t.Error("expected zero value for missing column")
	}
}

func TestScanFromMap_NilValue(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"int64val": nil}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Int64Val != 0 {
		t.Error("nil raw should leave field at zero")
	}
}

func TestScanFromMap_PtrStr(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"ptrstr": "ptr-value"}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.PtrStr == nil || *ts.PtrStr != "ptr-value" {
		t.Errorf("PtrStr: %v", ts.PtrStr)
	}
}

func TestScanFromMap_PtrBool_FromInt(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"ptrbool": int64(1)}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.PtrBool == nil || !*ts.PtrBool {
		t.Errorf("PtrBool from int: %v", ts.PtrBool)
	}
}

func TestScanFromMap_PtrInt64_Convertible(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"ptrint64": int64(77)}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.PtrInt64 == nil || *ts.PtrInt64 != 77 {
		t.Errorf("PtrInt64: %v", ts.PtrInt64)
	}
}

func TestScanFromMap_Bool_FromInt(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"boolval": int64(1)}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if !ts.BoolVal {
		t.Error("bool from int64(1) should be true")
	}
}

func TestScanFromMap_Bool_FromUint(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"boolval": uint8(0)}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.BoolVal {
		t.Error("bool from uint8(0) should be false")
	}
}

func TestScanFromMap_Float_FromInt(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"float64val": int64(3)}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Float64Val != 3.0 {
		t.Errorf("float64 from int64: got %f", ts.Float64Val)
	}
}

func TestScanFromMap_Float32_FromFloat32(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"float32val": float32(1.5)}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Float32Val != 1.5 {
		t.Errorf("float32: got %f", ts.Float32Val)
	}
}

func TestScanFromMap_Int_FromUint(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"intval": uint(5)}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.IntVal != 5 {
		t.Errorf("int from uint: got %d", ts.IntVal)
	}
}

func TestScanFromMap_Uint_FromInt(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"uintval": int(9)}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.UintVal != 9 {
		t.Errorf("uint from int: got %d", ts.UintVal)
	}
}

func TestScanFromMap_FallbackReflect_CustomType(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	// customType is kindOther — goes through assignReflect
	row := map[string]any{"customval": customType{X: 42}}
	ts, err := ScanFromMap[testStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.CustomVal.X != 42 {
		t.Errorf("customVal: %+v", ts.CustomVal)
	}
}

func TestScanFromMap_SqlScanner_DelegatesToScan(t *testing.T) {
	meta := entity.New(func(ts *scannerTestStruct, b *entity.Table) {
		b.TableName("scannerteststruct")
		b.Col(&ts.ID, golem.BIGINT())
		b.Col(&ts.Field, golem.TEXT())
		b.PrimaryKey(&ts.ID)
	}).Describe()
	p := Compile(meta, golem.DefaultParser)

	row := map[string]any{"id": int64(1), "field": "abc"}
	ts, err := ScanFromMap[scannerTestStruct](p, row)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Field.inner != "scanned:abc" {
		t.Errorf("Field.inner = %q, want %q", ts.Field.inner, "scanned:abc")
	}
}

func TestScanFromMap_SqlScanner_PropagatesScanError(t *testing.T) {
	meta := entity.New(func(ts *scannerTestStruct, b *entity.Table) {
		b.TableName("scannerteststruct2")
		b.Col(&ts.ID, golem.BIGINT())
		b.Col(&ts.Field, golem.TEXT())
		b.PrimaryKey(&ts.ID)
	}).Describe()
	p := Compile(meta, golem.DefaultParser)

	// fakeScannerField.Scan only accepts a string -- an int64 makes it error.
	row := map[string]any{"id": int64(1), "field": int64(99)}
	_, err := ScanFromMap[scannerTestStruct](p, row)
	if err == nil {
		t.Fatal("expected error from Scan")
	}
}

func TestScanFromMap_Error_Unconvertible(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	// CustomVal field (customType) can't receive a string
	row := map[string]any{"customval": "not-a-customType"}
	_, err := ScanFromMap[testStruct](p, row)
	if err == nil {
		t.Fatal("expected error for unconvertible type")
	}
}

func TestScanFromMap_String_TypeMismatch_FallsToReflect(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	// stringval expects string but we pass int64 — falls to reflect convertible path
	row := map[string]any{"stringval": customType{X: 1}}
	_, err := ScanFromMap[testStruct](p, row)
	if err == nil {
		t.Fatal("expected error for string from customType")
	}
}

func TestScanFromMap_NullStr_TypeMismatch_FallsToReflect(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	// nullstr expects sql.NullString but we pass customType — fails
	row := map[string]any{"nullstr": customType{X: 1}}
	_, err := ScanFromMap[testStruct](p, row)
	if err == nil {
		t.Fatal("expected error for NullString from customType")
	}
}

func TestScanFromMap_NullInt_TypeMismatch_FallsToReflect(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"nullint": customType{X: 1}}
	_, err := ScanFromMap[testStruct](p, row)
	if err == nil {
		t.Fatal("expected error for NullInt64 from customType")
	}
}

func TestScanFromMap_NullFloat_TypeMismatch_FallsToReflect(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"nullfloat": customType{X: 1}}
	_, err := ScanFromMap[testStruct](p, row)
	if err == nil {
		t.Fatal("expected error for NullFloat64 from customType")
	}
}

func TestScanFromMap_NullBool_TypeMismatch_FallsToReflect(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"nullbool": customType{X: 1}}
	_, err := ScanFromMap[testStruct](p, row)
	if err == nil {
		t.Fatal("expected error for NullBool from customType")
	}
}

func TestScanFromMap_NullTime_TypeMismatch_FallsToReflect(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"nulltime": customType{X: 1}}
	_, err := ScanFromMap[testStruct](p, row)
	if err == nil {
		t.Fatal("expected error for NullTime from customType")
	}
}

func TestScanFromMap_BytesTypeMismatch(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	// customType is not convertible to []byte
	row := map[string]any{"bytesval": customType{X: 1}}
	_, err := ScanFromMap[testStruct](p, row)
	if err == nil {
		t.Fatal("expected error for []byte from customType")
	}
}

func TestScanFromMap_TimeMismatch(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	row := map[string]any{"timeval": "not-a-time"}
	_, err := ScanFromMap[testStruct](p, row)
	if err == nil {
		t.Fatal("expected error for time.Time from string")
	}
}

func TestClassifyGoType_AllKinds(t *testing.T) {
	cases := []struct {
		t    reflect.Type
		want fieldKind
	}{
		{reflect.TypeFor[int](), kindInt},
		{reflect.TypeFor[int8](), kindInt8},
		{reflect.TypeFor[int16](), kindInt16},
		{reflect.TypeFor[int32](), kindInt32},
		{reflect.TypeFor[int64](), kindInt64},
		{reflect.TypeFor[uint](), kindUint},
		{reflect.TypeFor[uint8](), kindUint8},
		{reflect.TypeFor[uint16](), kindUint16},
		{reflect.TypeFor[uint32](), kindUint32},
		{reflect.TypeFor[uint64](), kindUint64},
		{reflect.TypeFor[float32](), kindFloat32},
		{reflect.TypeFor[float64](), kindFloat64},
		{reflect.TypeFor[string](), kindString},
		{reflect.TypeFor[bool](), kindBool},
		{typeBytes, kindBytes},
		{typeTime, kindTime},
		{typeNullString, kindNullString},
		{typeNullInt64, kindNullInt64},
		{typeNullFloat64, kindNullFloat64},
		{typeNullBool, kindNullBool},
		{typeNullTime, kindNullTime},
		{reflect.TypeFor[customType](), kindOther},
	}
	for _, c := range cases {
		got := classifyGoType(c.t)
		if got != c.want {
			t.Errorf("classifyGoType(%v) = %d, want %d", c.t, got, c.want)
		}
	}
}

func TestTargetsPool_Reuses(t *testing.T) {
	meta := buildMeta(t)
	p := Compile(meta, golem.DefaultParser)
	s1 := p.targetsPool.Get().(*[]any)
	p.targetsPool.Put(s1)
	s2 := p.targetsPool.Get().(*[]any)
	if len(*s2) != len(meta.Columns) {
		t.Errorf("pool slice len: got %d, want %d", len(*s2), len(meta.Columns))
	}
	p.targetsPool.Put(s2)
}

func BenchmarkScanFromMap_AllPrimitives(b *testing.B) {
	meta := buildMeta(b)
	p := Compile(meta, golem.DefaultParser)
	now := time.Now()
	row := map[string]any{
		"intval":     int(1),
		"int8val":    int8(2),
		"int16val":   int16(3),
		"int32val":   int32(4),
		"int64val":   int64(5),
		"uintval":    uint(6),
		"uint8val":   uint8(7),
		"uint16val":  uint16(8),
		"uint32val":  uint32(9),
		"uint64val":  uint64(10),
		"float32val": float32(1.1),
		"float64val": float64(2.2),
		"stringval":  "hello",
		"boolval":    true,
		"bytesval":   []byte{1, 2},
		"timeval":    now,
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _ = ScanFromMap[testStruct](p, row)
	}
}

// buildMeta accepts testing.TB to work with both *testing.T and *testing.B.
func init() {
	// ensure buildMeta is exercised via TestCompile via go test
	_ = reflect.TypeFor[testStruct]()
}

// --- coverage gap tests ---

func TestAssignReflect_NilGoType(t *testing.T) {
	if err := assignReflect(nil, nil, "any", golem.DefaultParser); err != nil {
		t.Fatalf("expected nil error for nil goType, got: %v", err)
	}
}

func TestAssignReflect_InvalidRawVal(t *testing.T) {
	var s string
	fp := unsafe.Pointer(&s)
	if err := assignReflect(fp, reflect.TypeOf(""), nil, golem.DefaultParser); err != nil {
		t.Fatalf("expected nil error for nil raw, got: %v", err)
	}
}

func TestAssignReflect_BoolFromIntViaReflect(t *testing.T) {
	// Exercises the goType.Kind() == reflect.Bool branch in assignReflect.
	var b bool
	fp := unsafe.Pointer(&b)
	if err := assignReflect(fp, reflect.TypeOf(false), int64(1), golem.DefaultParser); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !b {
		t.Error("bool should be true after assignReflect with int64(1)")
	}
}

func TestAssignReflect_PtrConvertDifferentType(t *testing.T) {
	// Exercises the rawVal.Type() != elemType branch inside the Pointer path.
	// *int64 field receiving int32 — needs Convert(int64) before wrapping.
	var p *int64
	fp := unsafe.Pointer(&p)
	if err := assignReflect(fp, reflect.TypeOf((*int64)(nil)), int32(7), golem.DefaultParser); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil || *p != 7 {
		t.Errorf("*int64 from int32: got %v", p)
	}
}

func TestToInt64_AllIntTypes(t *testing.T) {
	cases := []struct {
		raw  any
		want int64
	}{
		{int16(3), 3}, {int8(4), 4},
		{uint64(5), 5}, {uint32(6), 6}, {uint16(7), 7}, {uint8(8), 8}, {uint(9), 9},
	}
	for _, c := range cases {
		got, ok := toInt64(c.raw)
		if !ok || got != c.want {
			t.Errorf("toInt64(%v): got (%d, %v), want (%d, true)", c.raw, got, ok, c.want)
		}
	}
	if _, ok := toInt64("not an int"); ok {
		t.Error("toInt64 should return false for string")
	}
}

func TestToUint64_AllUintTypes(t *testing.T) {
	cases := []struct {
		raw  any
		want uint64
	}{
		{int64(1), 1}, {int32(2), 2}, {int16(3), 3}, {int8(4), 4}, {int(5), 5},
		{uint16(6), 6}, {uint8(7), 7},
	}
	for _, c := range cases {
		got, ok := toUint64(c.raw)
		if !ok || got != c.want {
			t.Errorf("toUint64(%v): got (%d, %v), want (%d, true)", c.raw, got, ok, c.want)
		}
	}
	if _, ok := toUint64("not an int"); ok {
		t.Error("toUint64 should return false for string")
	}
}

func TestToBool_AllBranches(t *testing.T) {
	if v, ok := toBool(true); !ok || !v {
		t.Error("toBool(true) failed")
	}
	if v, ok := toBool(false); !ok || v {
		t.Error("toBool(false) failed")
	}
	if v, ok := toBool(int64(1)); !ok || !v {
		t.Error("toBool(int64(1)) failed")
	}
	if _, ok := toBool("nope"); ok {
		t.Error("toBool(string) should return false")
	}
}

func TestAssignReflect_PtrBoolFromNumeric(t *testing.T) {
	var val *bool
	fp := unsafe.Pointer(&val)
	if err := assignReflect(fp, reflect.TypeOf((*bool)(nil)), int64(1), golem.DefaultParser); err != nil {
		t.Fatal(err)
	}
	if val == nil || !*val {
		t.Errorf("expected *true")
	}
}

func TestAssignReflect_PtrBoolFromInvalid(t *testing.T) {
	var val *bool
	fp := unsafe.Pointer(&val)
	// Passes a string, which is not a numeric value, to a *bool.
	// numericToBoolVal will return ok=false, falls through to error.
	err := assignReflect(fp, reflect.TypeOf((*bool)(nil)), "not_a_bool", golem.DefaultParser)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestAssignReflect_PtrExactMatch(t *testing.T) {
	var val *int64
	fp := unsafe.Pointer(&val)
	if err := assignReflect(fp, reflect.TypeOf((*int64)(nil)), int64(42), golem.DefaultParser); err != nil {
		t.Fatal(err)
	}
	if val == nil || *val != 42 {
		t.Errorf("expected *42")
	}
}

func TestAssignReflect_ConvertibleMatch(t *testing.T) {
	type CustomInt int64
	var val CustomInt
	fp := unsafe.Pointer(&val)
	// raw is int32(42), goType is CustomInt. ConvertibleTo is true.
	if err := assignReflect(fp, reflect.TypeOf(CustomInt(0)), int32(42), golem.DefaultParser); err != nil {
		t.Fatal(err)
	}
	if val != CustomInt(42) {
		t.Errorf("expected 42")
	}
}

func TestAssignReflect_NotConvertible(t *testing.T) {
	var val int64
	fp := unsafe.Pointer(&val)
	// raw is string, goType is int64. ConvertibleTo is false.
	err := assignReflect(fp, reflect.TypeOf(int64(0)), "string_val", golem.DefaultParser)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}
