// Package scanner implements a zero-allocation row scanner for golem's
// Repository. It pre-classifies each struct field into a fieldKind at
// plan-compile time, then uses unsafe pointer arithmetic in the hot loop
// to write directly into the target struct without reflect overhead.
//
// Fast path: primitive types (int*, uint*, float*, string, bool, []byte,
// time.Time, and their sql.Null* wrappers) are cast via unsafe.Add+kind switch.
// Fallback path: all other types (UUID strings, JSON, *T nullable fields that
// don't match a known Null* wrapper) use reflect — same behaviour as the old
// assignFieldValue path in repository.
package scanner

import (
	"database/sql"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
)

// fieldKind classifies a struct field's Go type into a scanner category
// resolved once at Plan compile time, eliminating per-row type switches
// over reflect.Type values.
type fieldKind uint8

const (
	kindOther       fieldKind = iota // fallback: use reflect
	kindInt                          // int
	kindInt8                         // int8
	kindInt16                        // int16
	kindInt32                        // int32
	kindInt64                        // int64
	kindUint                         // uint
	kindUint8                        // uint8
	kindUint16                       // uint16
	kindUint32                       // uint32
	kindUint64                       // uint64
	kindFloat32                      // float32
	kindFloat64                      // float64
	kindString                       // string
	kindBool                         // bool
	kindBytes                        // []byte
	kindTime                         // time.Time
	kindNullString                   // sql.NullString
	kindNullInt64                    // sql.NullInt64
	kindNullFloat64                  // sql.NullFloat64
	kindNullBool                     // sql.NullBool
	kindNullTime                     // sql.NullTime
)

var (
	typeTime        = reflect.TypeOf(time.Time{})
	typeNullString  = reflect.TypeOf(sql.NullString{})
	typeNullInt64   = reflect.TypeOf(sql.NullInt64{})
	typeNullFloat64 = reflect.TypeOf(sql.NullFloat64{})
	typeNullBool    = reflect.TypeOf(sql.NullBool{})
	typeNullTime    = reflect.TypeOf(sql.NullTime{})
	typeBytes       = reflect.TypeOf([]byte(nil))
)

// classifyGoType maps a reflect.Type to its fieldKind.
func classifyGoType(t reflect.Type) fieldKind {
	switch t {
	case typeTime:
		return kindTime
	case typeNullString:
		return kindNullString
	case typeNullInt64:
		return kindNullInt64
	case typeNullFloat64:
		return kindNullFloat64
	case typeNullBool:
		return kindNullBool
	case typeNullTime:
		return kindNullTime
	case typeBytes:
		return kindBytes
	}
	switch t.Kind() {
	case reflect.Int:
		return kindInt
	case reflect.Int8:
		return kindInt8
	case reflect.Int16:
		return kindInt16
	case reflect.Int32:
		return kindInt32
	case reflect.Int64:
		return kindInt64
	case reflect.Uint:
		return kindUint
	case reflect.Uint8:
		return kindUint8
	case reflect.Uint16:
		return kindUint16
	case reflect.Uint32:
		return kindUint32
	case reflect.Uint64:
		return kindUint64
	case reflect.Float32:
		return kindFloat32
	case reflect.Float64:
		return kindFloat64
	case reflect.String:
		return kindString
	case reflect.Bool:
		return kindBool
	}
	return kindOther
}

// fieldAssignment holds everything needed to write one column's value into
// a struct field without looking up types at scan time.
type fieldAssignment struct {
	colName string
	offset  uintptr
	kind    fieldKind
	goType  reflect.Type // used only for kindOther fallback
}

// Plan is compiled once per entity.EntityMeta and reused across all rows.
// The targetsPool recycles the []any slice passed to rows.Scan.
type Plan struct {
	assignments []fieldAssignment
	targetsPool sync.Pool
	parser      golem.Parser
}

// Compile builds a Plan from an entity.EntityMeta and the Parser resolved
// off the golem.Conn the caller is bound to. The plan is intended to be
// stored on Repository[T] and reused for every scanRow call.
func Compile(meta entity.EntityMeta, parser golem.Parser) *Plan {
	n := len(meta.Columns)
	assignments := make([]fieldAssignment, n)
	for i, col := range meta.Columns {
		assignments[i] = fieldAssignment{
			colName: col.Name,
			offset:  col.Offset,
			kind:    classifyGoType(col.GoType),
			goType:  col.GoType,
		}
	}
	p := &Plan{assignments: assignments, parser: parser}
	p.targetsPool.New = func() any {
		s := make([]any, n)
		return &s
	}
	return p
}

// ScanFromMap writes each column in row into a new T using the pre-compiled
// Plan. It is a drop-in replacement for repository.scanRow.
func ScanFromMap[T any](p *Plan, row map[string]any) (T, error) {
	var result T
	base := unsafe.Pointer(&result)
	for i := range p.assignments {
		a := &p.assignments[i]
		raw, ok := row[a.colName]
		if !ok {
			continue
		}
		if raw == nil {
			continue
		}
		if err := assignFast(base, a, raw, p.parser); err != nil {
			return result, err
		}
	}
	return result, nil
}

// assignFast is the hot-loop dispatcher. It handles the fast path for all
// primitive kinds and falls back to reflect for kindOther.
func assignFast(base unsafe.Pointer, a *fieldAssignment, raw any, parser golem.Parser) error {
	fp := unsafe.Add(base, a.offset)
	switch a.kind {
	case kindInt:
		if v, ok := toInt64(raw); ok {
			*(*int)(fp) = int(v)
			return nil
		}
	case kindInt8:
		if v, ok := toInt64(raw); ok {
			*(*int8)(fp) = int8(v)
			return nil
		}
	case kindInt16:
		if v, ok := toInt64(raw); ok {
			*(*int16)(fp) = int16(v)
			return nil
		}
	case kindInt32:
		if v, ok := toInt64(raw); ok {
			*(*int32)(fp) = int32(v)
			return nil
		}
	case kindInt64:
		if v, ok := toInt64(raw); ok {
			*(*int64)(fp) = v
			return nil
		}
	case kindUint:
		if v, ok := toUint64(raw); ok {
			*(*uint)(fp) = uint(v)
			return nil
		}
	case kindUint8:
		if v, ok := toUint64(raw); ok {
			*(*uint8)(fp) = uint8(v)
			return nil
		}
	case kindUint16:
		if v, ok := toUint64(raw); ok {
			*(*uint16)(fp) = uint16(v)
			return nil
		}
	case kindUint32:
		if v, ok := toUint64(raw); ok {
			*(*uint32)(fp) = uint32(v)
			return nil
		}
	case kindUint64:
		if v, ok := toUint64(raw); ok {
			*(*uint64)(fp) = v
			return nil
		}
	case kindFloat32:
		if v, ok := toFloat64(raw); ok {
			*(*float32)(fp) = float32(v)
			return nil
		}
	case kindFloat64:
		if v, ok := toFloat64(raw); ok {
			*(*float64)(fp) = v
			return nil
		}
	case kindString:
		if v, ok := raw.(string); ok {
			*(*string)(fp) = v
			return nil
		}
	case kindBool:
		if v, ok := toBool(raw); ok {
			*(*bool)(fp) = v
			return nil
		}
	case kindBytes:
		if v, ok := raw.([]byte); ok {
			*(*[]byte)(fp) = v
			return nil
		}
	case kindTime:
		if v, ok := raw.(time.Time); ok {
			*(*time.Time)(fp) = v
			return nil
		}
	case kindNullString:
		if v, ok := raw.(sql.NullString); ok {
			*(*sql.NullString)(fp) = v
			return nil
		}
		if v, ok := raw.(string); ok {
			*(*sql.NullString)(fp) = sql.NullString{String: v, Valid: true}
			return nil
		}
	case kindNullInt64:
		if v, ok := raw.(sql.NullInt64); ok {
			*(*sql.NullInt64)(fp) = v
			return nil
		}
		if v, ok := toInt64(raw); ok {
			*(*sql.NullInt64)(fp) = sql.NullInt64{Int64: v, Valid: true}
			return nil
		}
	case kindNullFloat64:
		if v, ok := raw.(sql.NullFloat64); ok {
			*(*sql.NullFloat64)(fp) = v
			return nil
		}
		if v, ok := toFloat64(raw); ok {
			*(*sql.NullFloat64)(fp) = sql.NullFloat64{Float64: v, Valid: true}
			return nil
		}
	case kindNullBool:
		if v, ok := raw.(sql.NullBool); ok {
			*(*sql.NullBool)(fp) = v
			return nil
		}
		if v, ok := toBool(raw); ok {
			*(*sql.NullBool)(fp) = sql.NullBool{Bool: v, Valid: true}
			return nil
		}
	case kindNullTime:
		if v, ok := raw.(sql.NullTime); ok {
			*(*sql.NullTime)(fp) = v
			return nil
		}
		if v, ok := raw.(time.Time); ok {
			*(*sql.NullTime)(fp) = sql.NullTime{Time: v, Valid: true}
			return nil
		}
	}
	// kindOther or type-mismatch on a primitive kind: fall back to the Parser.
	return assignReflect(fp, a.goType, raw, parser)
}

// assignReflect is the slow-path fallback for kindOther (or a raw value
// that didn't match a primitive kind's expected Go type). field.Addr()
// points at the exact struct memory being scanned into (fp itself), so
// parser.FromSQL writes straight into the live field, no extra copy. All
// the resolution logic (sql.Scanner, Get()/Set(T) duck-typing, pointer
// alloc, numeric<->bool, convertible, JSON fallback) lives in the Parser
// now (golem.DefaultParser by default) -- this function is just the glue
// between the unsafe.Pointer world and reflect.Value.
func assignReflect(fp unsafe.Pointer, goType reflect.Type, raw any, parser golem.Parser) error {
	if goType == nil {
		return nil
	}
	field := reflect.NewAt(goType, fp).Elem()
	return parser.FromSQL(field, raw)
}

// toInt64 extracts an integer from any common integer-shaped raw value.
func toInt64(raw any) (int64, bool) {
	switch v := raw.(type) {
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case int16:
		return int64(v), true
	case int8:
		return int64(v), true
	case int:
		return int64(v), true
	case uint64:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint:
		return int64(v), true
	}
	return 0, false
}

// toUint64 extracts an unsigned integer from any common integer-shaped raw value.
func toUint64(raw any) (uint64, bool) {
	switch v := raw.(type) {
	case uint64:
		return v, true
	case uint32:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case uint:
		return uint64(v), true
	case int64:
		return uint64(v), true
	case int32:
		return uint64(v), true
	case int16:
		return uint64(v), true
	case int8:
		return uint64(v), true
	case int:
		return uint64(v), true
	}
	return 0, false
}

// toFloat64 extracts a float from any common numeric raw value.
func toFloat64(raw any) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	}
	if i, ok := toInt64(raw); ok {
		return float64(i), true
	}
	return 0, false
}

// toBool extracts a bool from a raw value, including integer-as-bool convention.
func toBool(raw any) (bool, bool) {
	if v, ok := raw.(bool); ok {
		return v, true
	}
	if i, ok := toInt64(raw); ok {
		return i != 0, true
	}
	return false, false
}
