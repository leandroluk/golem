// Package core provides the fundamental building blocks of the golem ORM.
// This file contains helper functions for reflection, field mapping,
// condition folding, and common value transformations.
package core

import (
	"fmt"
	"reflect"
	"strings"
	"time"
	"unsafe"
)

// offsetOf returns the memory offset of a struct field selected by the given selector function.
//
// Example:
//
//	type User struct {
//	    ID   int
//	    Name string
//	}
//
//	offset := offsetOf(func(u *User) *string { return &u.Name })
func offsetOf[T any, F any](selector func(*T) *F) uintptr {
	var zero T
	base := uintptr(unsafe.Pointer(&zero))
	ptr := selector(&zero)
	return uintptr(unsafe.Pointer(ptr)) - base
}

// fieldNameFromSelectorFor resolves the Go struct field name from a selector function.
//
// It takes a function of the form func(*T) *F and uses reflection to map it
// back to the struct field name.
//
// Panics if the argument is not a function, or if the function does not return a field pointer.
func fieldNameFromSelectorFor[T any](selector any) string {
	if selector == nil {
		return ""
	}
	selectorValue := reflect.ValueOf(selector)
	if selectorValue.Kind() != reflect.Func {
		panic("selector must be a function")
	}

	// create *T
	var zero T
	typ := reflect.TypeOf(zero)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	arg := reflect.New(typ) // *T

	// execute the selector and obtain its return value
	out := selectorValue.Call([]reflect.Value{arg})
	if len(out) == 0 {
		panic("selector must return a pointer to a field")
	}
	ret := out[0]
	if ret.Kind() != reflect.Pointer {
		panic("selector must return a pointer to a field")
	}

	// calculate offset of the returned pointer relative to *T
	basePtr := arg.Pointer()
	fieldPtr := ret.Pointer()
	offset := uintptr(fieldPtr - basePtr)

	// find the field whose offset matches
	for _, sf := range reflect.VisibleFields(typ) {
		if sf.Offset == offset {
			return sf.Name // Go struct field name
		}
	}
	return "???"
}

// mapToStruct maps a row (map[string]any) into a struct instance of type T.
//
// It uses reflection to assign values to fields, with support for:
//  1. Exact type matching
//  2. Value → pointer conversions (e.g. time.Time → *time.Time)
//  3. Pointer → value conversions (e.g. *time.Time → time.Time)
//  4. Convertible types (e.g. int → float64)
//
// Fields are matched case-insensitively.
func mapToStruct[T any](row map[string]any, out *T) error {
	value := reflect.ValueOf(out).Elem()
	for rowKey, rowValue := range row {
		field := value.FieldByNameFunc(func(name string) bool { return strings.EqualFold(name, rowKey) })
		if !field.IsValid() || !field.CanSet() {
			continue
		}

		if rowValue == nil {
			// If the field is a pointer, set to nil; otherwise skip
			if field.Kind() == reflect.Pointer {
				field.Set(reflect.Zero(field.Type()))
			}
			continue
		}

		rv := reflect.ValueOf(rowValue)

		// 1) exact type match
		if rv.Type().AssignableTo(field.Type()) {
			field.Set(rv)
			continue
		}

		// 2) value → pointer
		if field.Kind() == reflect.Pointer && rv.Type().AssignableTo(field.Type().Elem()) {
			ptr := reflect.New(field.Type().Elem())
			ptr.Elem().Set(rv)
			field.Set(ptr)
			continue
		}

		// 3) pointer → value
		if rv.Kind() == reflect.Pointer && !rv.IsNil() && rv.Type().Elem().AssignableTo(field.Type()) {
			field.Set(rv.Elem())
			continue
		}

		// 4) convertible types
		if rv.Type().ConvertibleTo(field.Type()) {
			field.Set(rv.Convert(field.Type()))
			continue
		}
		if field.Kind() == reflect.Pointer && rv.Type().ConvertibleTo(field.Type().Elem()) {
			ptr := reflect.New(field.Type().Elem())
			ptr.Elem().Set(rv.Convert(field.Type().Elem()))
			field.Set(ptr)
			continue
		}
	}
	return nil
}

// foldConditionsAnd combines multiple conditions into a single condition
// using logical AND. If zero conditions are provided, it returns nil.
// If one condition is provided, it returns that condition.
func foldConditionsAnd(conds ...*Condition) *Condition {
	switch len(conds) {
	case 0:
		return nil
	case 1:
		return conds[0]
	default:
		acc := conds[0]
		for i := 1; i < len(conds); i++ {
			acc = acc.And(conds[i])
		}
		return acc
	}
}

// StructValues extracts field values from a struct according to its schema.
//
// It returns two slices:
//   - values: field values in order
//   - placeholders: parameter placeholders ($1, $2, ...) for SQL queries
//
// Example:
//
//	values, placeholders := StructValues(userSchema, &user)
func StructValues(schema *SchemaCore, doc any) ([]any, []string) {
	value := reflect.ValueOf(doc)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	valueList := []any{}
	placeholderList := []string{}

	for index, field := range schema.Fields {
		fv := value.FieldByName(field.StructFieldName)

		var v any
		if fv.Kind() == reflect.Pointer {
			if fv.IsNil() {
				v = nil
			} else {
				v = fv.Elem().Interface()
			}
		} else {
			v = fv.Interface()
		}

		valueList = append(valueList, v)
		placeholderList = append(placeholderList, fmt.Sprintf("$%d", index+1))
	}

	return valueList, placeholderList
}

// Include returns the Go struct field name given a selector function.
//
// Example:
//
//	nameField := Include(func(u *User) *string { return &u.Name })
func Include[L any, F any](selector func(*L) *F) string {
	return fieldNameFromSelectorFor[L](selector)
}

// setTimeField sets a time.Time value into a struct field, supporting both
// value and pointer kinds.
//
// If the field is a struct time.Time, it sets the value directly.
// If the field is a *time.Time, it sets or allocates as needed.
func setTimeField(field reflect.Value, t time.Time) {
	if !field.IsValid() || !field.CanSet() {
		return
	}
	timeType := reflect.TypeOf(time.Time{})

	switch field.Kind() {
	case reflect.Struct:
		if field.Type() == timeType {
			field.Set(reflect.ValueOf(t))
		}
	case reflect.Pointer:
		if field.Type().Elem() == timeType {
			if field.IsNil() {
				ptr := reflect.New(timeType)
				ptr.Elem().Set(reflect.ValueOf(t))
				field.Set(ptr)
			} else {
				field.Elem().Set(reflect.ValueOf(t))
			}
		}
	}
}
