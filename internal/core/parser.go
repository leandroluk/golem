package core

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// Parser converts a struct field's Go value to/from the driver.Value
// database/sql understands. DataSource uses DefaultParser unless overridden
// via CustomParser at NewDataSource time.
type Parser interface {
	// ToSQL converts fieldVal (a reflect.Value pointing at the struct field
	// being written) into a driver.Value to bind.
	ToSQL(fieldVal reflect.Value) (driver.Value, error)
	// FromSQL writes raw (as read off the row) into dst, an addressable
	// reflect.Value already of the field's exact Go type.
	FromSQL(dst reflect.Value, raw any) error
}

// sqlScanner mirrors database/sql.Scanner. Redeclared locally so this file
// doesn't need to import database/sql just for this one interface.
type sqlScanner interface {
	Scan(src any) error
}

type defaultParser struct{}

// DefaultParser is golem's built-in Parser. See ToSQL/FromSQL for the exact
// resolution order; CustomParser lets a DataSource override or decorate it.
var DefaultParser Parser = defaultParser{}

func (defaultParser) ToSQL(fieldVal reflect.Value) (driver.Value, error) {
	return toSQLValue(fieldVal)
}

func (defaultParser) FromSQL(dst reflect.Value, raw any) error {
	return fromSQLValue(dst, raw)
}

// toSQLValue resolves, in order:
//  1. driver.Valuer (checked via the field's address first -- covers a
//     pointer-receiver Value(), then the value itself)
//  2. a Get() T duck-type -- found by METHOD NAME via reflection, not a
//     fixed interface, so it works for a generic wrapper type (any T)
//     without that type implementing anything golem defines
//  3. pointer dereference (nil -> nil, non-nil -> recurse into the pointee)
//  4. passthrough for types driver.Value already accepts natively
//  5. named/enum type reduced to its underlying kind
//  6. JSON marshal fallback (maps, structs, slices of struct -- jsonb columns)
func toSQLValue(fieldVal reflect.Value) (driver.Value, error) {
	if fieldVal.CanAddr() {
		if v, ok := fieldVal.Addr().Interface().(driver.Valuer); ok {
			return v.Value()
		}
	}
	if fieldVal.IsValid() && fieldVal.CanInterface() {
		if v, ok := fieldVal.Interface().(driver.Valuer); ok {
			return v.Value()
		}
	}

	if inner, ok := callGet(fieldVal); ok {
		return toSQLValue(inner)
	}

	if fieldVal.Kind() == reflect.Pointer {
		if fieldVal.IsNil() {
			return nil, nil
		}
		return toSQLValue(fieldVal.Elem())
	}

	switch v := fieldVal.Interface().(type) {
	case time.Time:
		return v, nil
	case []byte:
		return v, nil
	case string:
		return v, nil
	case bool:
		return v, nil
	}

	switch fieldVal.Kind() {
	case reflect.String:
		return fieldVal.String(), nil
	case reflect.Bool:
		return fieldVal.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fieldVal.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(fieldVal.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return fieldVal.Float(), nil
	}

	raw, err := json.Marshal(fieldVal.Interface())
	if err != nil {
		return nil, fmt.Errorf("golem: json marshal %s: %w", fieldVal.Type(), err)
	}
	return raw, nil
}

// callGet duck-types a Get() T method (0 args, 1 return) on fieldVal or its
// address, returning the unwrapped inner reflect.Value if found.
func callGet(fieldVal reflect.Value) (reflect.Value, bool) {
	if m := fieldVal.MethodByName("Get"); m.IsValid() && m.Type().NumIn() == 0 && m.Type().NumOut() == 1 {
		return m.Call(nil)[0], true
	}
	if fieldVal.CanAddr() {
		if m := fieldVal.Addr().MethodByName("Get"); m.IsValid() && m.Type().NumIn() == 0 && m.Type().NumOut() == 1 {
			return m.Call(nil)[0], true
		}
	}
	return reflect.Value{}, false
}

// fromSQLValue resolves, in order:
//  1. sql.Scanner (via dst's address)
//  2. a Get()/Set(T) duck-type pair -- confirms "this is a value wrapper"
//     (Get alone would be ambiguous), allocates T, recurses, then calls Set
//  3. nil -> zero value
//  4. pointer: allocate and recurse into the pointee
//  5. direct type match / convertible (numeric widening, enum <-> kind)
//  6. JSON unmarshal fallback (jsonb -> map/struct)
func fromSQLValue(dst reflect.Value, raw any) error {
	if dst.CanAddr() {
		if s, ok := dst.Addr().Interface().(sqlScanner); ok {
			return s.Scan(raw)
		}
	}

	if dst.CanAddr() {
		addr := dst.Addr()
		getM := addr.MethodByName("Get")
		setM := addr.MethodByName("Set")
		if getM.IsValid() && setM.IsValid() && setM.Type().NumIn() == 1 {
			innerType := setM.Type().In(0)
			innerPtr := reflect.New(innerType)
			if err := fromSQLValue(innerPtr.Elem(), raw); err != nil {
				return err
			}
			setM.Call([]reflect.Value{innerPtr.Elem()})
			return nil
		}
	}

	if raw == nil {
		dst.Set(reflect.Zero(dst.Type()))
		return nil
	}

	if dst.Kind() == reflect.Pointer {
		elemPtr := reflect.New(dst.Type().Elem())
		if err := fromSQLValue(elemPtr.Elem(), raw); err != nil {
			return err
		}
		dst.Set(elemPtr)
		return nil
	}

	rawVal := reflect.ValueOf(raw)

	if rawVal.Type() == dst.Type() {
		dst.Set(rawVal)
		return nil
	}
	// numeric -> bool (MySQL TINYINT(1) etc. -- not reflect.ConvertibleTo,
	// needs an explicit != 0 check)
	if dst.Kind() == reflect.Bool {
		if b, ok := numericToBool(rawVal); ok {
			dst.SetBool(b)
			return nil
		}
	}
	if rawVal.Type().ConvertibleTo(dst.Type()) {
		dst.Set(rawVal.Convert(dst.Type()))
		return nil
	}

	var jsonRaw []byte
	switch v := raw.(type) {
	case []byte:
		jsonRaw = v
	case string:
		jsonRaw = []byte(v)
	default:
		return fmt.Errorf("golem: cannot scan %T into %s", raw, dst.Type())
	}
	ptr := reflect.New(dst.Type())
	if err := json.Unmarshal(jsonRaw, ptr.Interface()); err != nil {
		return fmt.Errorf("golem: json unmarshal into %s: %w", dst.Type(), err)
	}
	dst.Set(ptr.Elem())
	return nil
}

// numericToBool converts an integer/unsigned-integer reflect.Value to bool
// (nonzero -> true), the MySQL TINYINT(1)-as-BOOLEAN convention. Returns
// false, false for any other kind (including float -- never an implicit
// bool source).
func numericToBool(v reflect.Value) (bool, bool) {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() != 0, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() != 0, true
	}
	return false, false
}
