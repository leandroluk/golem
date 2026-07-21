package core

import (
	"database/sql/driver"
	"errors"
	"reflect"
	"testing"
	"time"
)

// fakeGetSet mirrors the Get()/Set(T) shape of a third-party dirty-tracking
// wrapper (e.g. gonest.Accessor[T]) -- golem must never import gonest, so
// this fake proves the duck-typing works for an arbitrary T without any
// fixed interface or concrete type golem knows about.
type fakeGetSet[T any] struct{ value T }

func (f fakeGetSet[T]) Get() T   { return f.value }
func (f *fakeGetSet[T]) Set(v T) { f.value = v }

type fakeValuer struct{ inner string }

func (f fakeValuer) Value() (driver.Value, error) { return "valued:" + f.inner, nil }

type fakeValuerPtrReceiver struct{ inner string }

func (f *fakeValuerPtrReceiver) Value() (driver.Value, error) { return "ptrvalued:" + f.inner, nil }

// fakeGetPtrOnly has Get() with a POINTER receiver only, forcing callGet's
// fallback branch (fieldVal.Addr().MethodByName) since a non-addressable
// value's method set never includes it.
type fakeGetPtrOnly struct{ value string }

func (f *fakeGetPtrOnly) Get() string { return f.value }

type fakeScanner struct{ inner string }

func (f *fakeScanner) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return errors.New("fakeScanner: not a string")
	}
	f.inner = "scanned:" + s
	return nil
}

type namedString string
type namedInt int32

func valOf(v any) reflect.Value { return reflect.ValueOf(v) }

func addrOf[T any](v T) reflect.Value {
	p := &v
	return reflect.ValueOf(p).Elem()
}

func TestToSQLValue_DriverValuer_ValueReceiver(t *testing.T) {
	dv, err := toSQLValue(addrOf(fakeValuer{inner: "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if dv != "valued:x" {
		t.Fatalf("got %v", dv)
	}
}

func TestToSQLValue_DriverValuer_PointerReceiver(t *testing.T) {
	dv, err := toSQLValue(addrOf(fakeValuerPtrReceiver{inner: "y"}))
	if err != nil {
		t.Fatal(err)
	}
	if dv != "ptrvalued:y" {
		t.Fatalf("got %v", dv)
	}
}

func TestToSQLValue_GetDuckType_String(t *testing.T) {
	dv, err := toSQLValue(addrOf(fakeGetSet[string]{value: "hello"}))
	if err != nil {
		t.Fatal(err)
	}
	if dv != "hello" {
		t.Fatalf("got %v", dv)
	}
}

func TestToSQLValue_GetDuckType_PointerInner(t *testing.T) {
	s := "inner"
	dv, err := toSQLValue(addrOf(fakeGetSet[*string]{value: &s}))
	if err != nil {
		t.Fatal(err)
	}
	if dv != "inner" {
		t.Fatalf("got %v", dv)
	}
}

func TestToSQLValue_GetDuckType_NilPointerInner(t *testing.T) {
	dv, err := toSQLValue(addrOf(fakeGetSet[*string]{value: nil}))
	if err != nil {
		t.Fatal(err)
	}
	if dv != nil {
		t.Fatalf("got %v, want nil", dv)
	}
}

func TestToSQLValue_GetDuckType_PointerReceiverOnly(t *testing.T) {
	dv, err := toSQLValue(addrOf(fakeGetPtrOnly{value: "ptr-recv"}))
	if err != nil {
		t.Fatal(err)
	}
	if dv != "ptr-recv" {
		t.Fatalf("got %v", dv)
	}
}

// nonAddressable wraps a value produced by a map index expression -- one of
// the few ways to get a non-addressable reflect.Value in real code -- to
// exercise toSQLValue's driver.Valuer check that doesn't go through Addr().
func nonAddressableValuer() reflect.Value {
	m := map[string]fakeValuer{"k": {inner: "z"}}
	return reflect.ValueOf(m["k"])
}

func TestToSQLValue_DriverValuer_NonAddressableValue(t *testing.T) {
	v := nonAddressableValuer()
	if v.CanAddr() {
		t.Fatal("test setup: expected a non-addressable reflect.Value")
	}
	dv, err := toSQLValue(v)
	if err != nil {
		t.Fatal(err)
	}
	if dv != "valued:z" {
		t.Fatalf("got %v", dv)
	}
}

func TestToSQLValue_NilPointer(t *testing.T) {
	var p *string
	dv, err := toSQLValue(valOf(p))
	if err != nil {
		t.Fatal(err)
	}
	if dv != nil {
		t.Fatalf("got %v, want nil", dv)
	}
}

func TestToSQLValue_NonNilPointer_Dereferences(t *testing.T) {
	s := "deref"
	dv, err := toSQLValue(valOf(&s))
	if err != nil {
		t.Fatal(err)
	}
	if dv != "deref" {
		t.Fatalf("got %v", dv)
	}
}

func TestToSQLValue_NativePassthrough(t *testing.T) {
	now := time.Now()
	cases := []any{now, []byte("bytes"), "str", true}
	for _, c := range cases {
		dv, err := toSQLValue(valOf(c))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(dv, c) {
			t.Fatalf("got %v, want %v", dv, c)
		}
	}
}

func TestToSQLValue_NamedStringType_ReducesToString(t *testing.T) {
	dv, err := toSQLValue(valOf(namedString("enum-value")))
	if err != nil {
		t.Fatal(err)
	}
	if dv != "enum-value" {
		t.Fatalf("got %v", dv)
	}
}

type namedBool bool

func TestToSQLValue_NamedBoolType_ReducesToBool(t *testing.T) {
	dv, err := toSQLValue(valOf(namedBool(true)))
	if err != nil {
		t.Fatal(err)
	}
	if dv != true {
		t.Fatalf("got %v", dv)
	}
}

func TestToSQLValue_NamedIntType_ReducesToInt64(t *testing.T) {
	dv, err := toSQLValue(valOf(namedInt(42)))
	if err != nil {
		t.Fatal(err)
	}
	if dv != int64(42) {
		t.Fatalf("got %v", dv)
	}
}

func TestToSQLValue_Bool(t *testing.T) {
	dv, err := toSQLValue(valOf(false))
	if err != nil {
		t.Fatal(err)
	}
	if dv != false {
		t.Fatalf("got %v", dv)
	}
}

func TestToSQLValue_Uint_ReducesToInt64(t *testing.T) {
	dv, err := toSQLValue(valOf(uint32(7)))
	if err != nil {
		t.Fatal(err)
	}
	if dv != int64(7) {
		t.Fatalf("got %v", dv)
	}
}

func TestToSQLValue_Float_Passthrough(t *testing.T) {
	dv, err := toSQLValue(valOf(float64(1.5)))
	if err != nil {
		t.Fatal(err)
	}
	if dv != float64(1.5) {
		t.Fatalf("got %v", dv)
	}
}

func TestToSQLValue_JSONFallback_Map(t *testing.T) {
	dv, err := toSQLValue(valOf(map[string]any{"a": 1}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := dv.([]byte); !ok {
		t.Fatalf("got %T, want []byte", dv)
	}
}

func TestToSQLValue_JSONFallback_MarshalError(t *testing.T) {
	_, err := toSQLValue(valOf(make(chan int)))
	if err == nil {
		t.Fatal("expected error marshaling a chan")
	}
}

func TestFromSQLValue_SqlScanner(t *testing.T) {
	var f fakeScanner
	if err := fromSQLValue(addrOf(f).Addr().Elem(), "abc"); err != nil {
		t.Fatal(err)
	}
}

func TestFromSQLValue_SqlScanner_Direct(t *testing.T) {
	var f fakeScanner
	dst := reflect.ValueOf(&f).Elem()
	if err := fromSQLValue(dst, "abc"); err != nil {
		t.Fatal(err)
	}
	if f.inner != "scanned:abc" {
		t.Fatalf("f.inner = %q", f.inner)
	}
}

func TestFromSQLValue_GetSetDuckType(t *testing.T) {
	var f fakeGetSet[string]
	dst := reflect.ValueOf(&f).Elem()
	if err := fromSQLValue(dst, "world"); err != nil {
		t.Fatal(err)
	}
	if f.Get() != "world" {
		t.Fatalf("f.Get() = %q", f.Get())
	}
}

func TestFromSQLValue_GetSetDuckType_PointerInner(t *testing.T) {
	var f fakeGetSet[*string]
	dst := reflect.ValueOf(&f).Elem()
	if err := fromSQLValue(dst, "ptrworld"); err != nil {
		t.Fatal(err)
	}
	if f.Get() == nil || *f.Get() != "ptrworld" {
		t.Fatalf("f.Get() = %v", f.Get())
	}
}

func TestFromSQLValue_GetSetDuckType_InnerError_Propagates(t *testing.T) {
	var f fakeGetSet[int]
	dst := reflect.ValueOf(&f).Elem()
	if err := fromSQLValue(dst, "not-json"); err == nil {
		t.Fatal("expected inner recursion error to propagate")
	}
}

func TestFromSQLValue_Pointer_InnerError_Propagates(t *testing.T) {
	var p *int
	dst := reflect.ValueOf(&p).Elem()
	if err := fromSQLValue(dst, "not-json"); err == nil {
		t.Fatal("expected inner recursion error to propagate")
	}
}

func TestFromSQLValue_Nil_SetsZero(t *testing.T) {
	s := "not-empty"
	dst := reflect.ValueOf(&s).Elem()
	if err := fromSQLValue(dst, nil); err != nil {
		t.Fatal(err)
	}
	if s != "" {
		t.Fatalf("s = %q, want empty", s)
	}
}

func TestFromSQLValue_Pointer_AllocatesAndRecurses(t *testing.T) {
	var p *string
	dst := reflect.ValueOf(&p).Elem()
	if err := fromSQLValue(dst, "alloc"); err != nil {
		t.Fatal(err)
	}
	if p == nil || *p != "alloc" {
		t.Fatalf("p = %v", p)
	}
}

func TestFromSQLValue_DirectMatch(t *testing.T) {
	var i int64
	dst := reflect.ValueOf(&i).Elem()
	if err := fromSQLValue(dst, int64(9)); err != nil {
		t.Fatal(err)
	}
	if i != 9 {
		t.Fatalf("i = %d", i)
	}
}

func TestFromSQLValue_NumericToBool(t *testing.T) {
	var b bool
	dst := reflect.ValueOf(&b).Elem()
	if err := fromSQLValue(dst, int64(1)); err != nil {
		t.Fatal(err)
	}
	if !b {
		t.Fatalf("b = %v, want true", b)
	}
}

func TestFromSQLValue_NumericToBool_UintSource(t *testing.T) {
	var b bool
	dst := reflect.ValueOf(&b).Elem()
	if err := fromSQLValue(dst, uint32(1)); err != nil {
		t.Fatal(err)
	}
	if !b {
		t.Fatalf("b = %v, want true", b)
	}
}

func TestFromSQLValue_Bool_NonNumericRaw_Error(t *testing.T) {
	var b bool
	dst := reflect.ValueOf(&b).Elem()
	if err := fromSQLValue(dst, []int{1, 2}); err == nil {
		t.Fatal("expected error scanning a non-numeric, non-JSON raw into bool")
	}
}

func TestFromSQLValue_NumericToPtrBool(t *testing.T) {
	var p *bool
	dst := reflect.ValueOf(&p).Elem()
	if err := fromSQLValue(dst, int64(1)); err != nil {
		t.Fatal(err)
	}
	if p == nil || !*p {
		t.Fatalf("p = %v, want &true", p)
	}
}

func TestFromSQLValue_Convertible_NamedType(t *testing.T) {
	var n namedString
	dst := reflect.ValueOf(&n).Elem()
	if err := fromSQLValue(dst, "raw"); err != nil {
		t.Fatal(err)
	}
	if n != "raw" {
		t.Fatalf("n = %q", n)
	}
}

func TestFromSQLValue_JSONUnmarshal_FromBytes(t *testing.T) {
	var m map[string]any
	dst := reflect.ValueOf(&m).Elem()
	if err := fromSQLValue(dst, []byte(`{"k":"v"}`)); err != nil {
		t.Fatal(err)
	}
	if m["k"] != "v" {
		t.Fatalf("m = %v", m)
	}
}

func TestFromSQLValue_JSONUnmarshal_FromString(t *testing.T) {
	var m map[string]any
	dst := reflect.ValueOf(&m).Elem()
	if err := fromSQLValue(dst, `{"k":"v2"}`); err != nil {
		t.Fatal(err)
	}
	if m["k"] != "v2" {
		t.Fatalf("m = %v", m)
	}
}

func TestFromSQLValue_JSONUnmarshal_MalformedError(t *testing.T) {
	var m map[string]any
	dst := reflect.ValueOf(&m).Elem()
	if err := fromSQLValue(dst, []byte(`not-json`)); err == nil {
		t.Fatal("expected json unmarshal error")
	}
}

func TestFromSQLValue_UnconvertibleNonJSONRaw_Error(t *testing.T) {
	var m map[string]any
	dst := reflect.ValueOf(&m).Elem()
	if err := fromSQLValue(dst, 42); err == nil {
		t.Fatal("expected error scanning int into map without json source")
	}
}

func TestDefaultParser_ToSQL_FromSQL_RoundTrip(t *testing.T) {
	dv, err := DefaultParser.ToSQL(addrOf(fakeGetSet[string]{value: "roundtrip"}))
	if err != nil {
		t.Fatal(err)
	}
	var f fakeGetSet[string]
	dst := reflect.ValueOf(&f).Elem()
	if err := DefaultParser.FromSQL(dst, dv); err != nil {
		t.Fatal(err)
	}
	if f.Get() != "roundtrip" {
		t.Fatalf("f.Get() = %q", f.Get())
	}
}
