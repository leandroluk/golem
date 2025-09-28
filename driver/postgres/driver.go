package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/leandroluk/golem/core"
)

type Driver struct {
	dsn     string
	pool    *pgxpool.Pool
	dialect Dialect
}

func NewDriver(dsn string) *Driver {
	return &Driver{dsn: dsn, dialect: Dialect{}}
}

var _ core.Driver[string] = (*Driver)(nil)

func (d *Driver) Connect(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, d.dsn)
	if err != nil {
		return err
	}
	if err := pool.Ping(ctx); err != nil {
		return err
	}
	d.pool = pool
	return nil
}

func (d *Driver) Close() error {
	if d.pool != nil {
		d.pool.Close()
	}
	return nil
}

func (d *Driver) Ping(ctx context.Context) error {
	if d.pool == nil {
		return errors.New("pgx pool nil")
	}
	return d.pool.Ping(ctx)
}

func (d *Driver) Exec(ctx context.Context, stmt string, args ...any) (core.Result, error) {
	ct, err := d.pool.Exec(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	return Result{ct: ct}, nil
}

func (d *Driver) Query(ctx context.Context, stmt string, args ...any) (core.Rows, error) {
	rows, err := d.pool.Query(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{rows: rows}, nil
}

func (d *Driver) Begin(ctx context.Context) (core.Tx[string], error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &Tx{tx: tx}, nil
}

func (d *Driver) Dialect() core.Dialect {
	return d.dialect
}

// ------------------ ADAPTERS ------------------

type Result struct {
	ct pgconn.CommandTag
}

func (r Result) RowsAffected() (int64, error) {
	return int64(r.ct.RowsAffected()), nil
}

type Rows struct {
	rows pgx.Rows
}

var _ core.Rows = (*Rows)(nil)

func (r *Rows) Next() bool {
	return r.rows.Next()
}

func (r *Rows) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

func (r *Rows) Close() error {
	r.rows.Close()
	return nil
}

func (r *Rows) Columns() []string {
	fds := r.rows.FieldDescriptions()
	out := make([]string, len(fds))
	for i, fd := range fds {
		out[i] = string(fd.Name)
	}
	return out
}

type Tx struct {
	tx pgx.Tx
}

var _ core.Tx[string] = (*Tx)(nil)

func (t *Tx) Exec(ctx context.Context, stmt string, args ...any) (core.Result, error) {
	ct, err := t.tx.Exec(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	return Result{ct: ct}, nil
}

func (t *Tx) Query(ctx context.Context, stmt string, args ...any) (core.Rows, error) {
	rows, err := t.tx.Query(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{rows: rows}, nil
}

func (t *Tx) Commit() error {
	return t.tx.Commit(context.Background())
}

func (t *Tx) Rollback() error {
	return t.tx.Rollback(context.Background())
}

type Dialect struct{}

var _ core.Dialect = (*Dialect)(nil)

// ToValue converte valores Go → valores aceitos pelo database/sql
func (d Dialect) ToValue(f *core.FieldMeta, v any) any {
	if undef, ok := f.Meta[core.UndefO]; ok {
		if reflect.DeepEqual(v, undef) {
			// se tiver Default configurado → aplica default
			if def, hasDefault := f.Meta[core.DefaultO]; hasDefault {
				return def
			}
			return nil
		}
	}

	if v == nil {
		return nil
	}

	// se for nullable pointer nil → nil
	if _, ok := f.Meta[core.NullableO]; ok {
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Pointer && rv.IsNil() {
			return nil
		}
	}

	switch val := v.(type) {
	case time.Time:
		return val.UTC()
	case *time.Time:
		if val == nil {
			return nil
		}
		return val.UTC()

	case bool:
		return val
	case *bool:
		if val == nil {
			return nil
		}
		return *val

	case string:
		return val
	case *string:
		if val == nil {
			return nil
		}
		return *val

	case int, int8, int16, int32, int64:
		return val
	case *int, *int8, *int16, *int32, *int64:
		if val == nil {
			return nil
		}
		return reflect.ValueOf(val).Elem().Interface()

	case float32, float64:
		return val
	case *float32, *float64:
		if val == nil {
			return nil
		}
		return reflect.ValueOf(val).Elem().Interface()

	case json.RawMessage:
		return []byte(val) // pgx aceita []byte

	case []byte:
		return val

	default:
		// fallback: tenta JSON
		b, err := json.Marshal(val)
		if err == nil {
			return b
		}
		return val
	}
}

func (d Dialect) FromValue(f *core.FieldMeta, raw any) (any, error) {
	if raw == nil {
		// se for nullable → mantém nil
		if _, ok := f.Meta[core.NullableO]; ok {
			return nil, nil
		}
		// se não for nullable → devolve zero value do tipo Go
		if f.GoType != nil {
			return reflect.Zero(f.GoType).Interface(), nil
		}
		return nil, nil
	}

	switch {
	// Numéricos
	case hasToken(f,
		IntO, Int2O, Int4O, Int8O, SmallIntO, BigIntO,
		DecimalO, NumericO, RealO, DoublePrecisionO, MoneyO):
		return raw, nil

	// Datas/Horas
	case hasToken(f,
		DateO, TimeO, TimeTZO, TimestampO, TimestamptzO, IntervalO):
		if t, ok := raw.(time.Time); ok {
			return t, nil
		}
		return raw, nil

		// Texto
	case hasToken(f, VarCharO, CharO, TextO, CitextO):
		var s string
		switch v := raw.(type) {
		case []byte:
			s = string(v)
		case string:
			s = v
		default:
			return nil, fmt.Errorf("unsupported text type %T", raw)
		}

		if f.GoType != nil {
			switch {
			case f.GoType.Kind() == reflect.Pointer && f.GoType.Elem().Kind() == reflect.String:
				// campo é *string
				return &s, nil

			case f.GoType.Kind() == reflect.String && f.GoType.Name() != "string":
				// campo é um alias (ex.: UserRole)
				rv := reflect.ValueOf(s).Convert(f.GoType)
				return rv.Interface(), nil

			case f.GoType.Kind() == reflect.String:
				// campo é string normal
				return s, nil
			}
		}
		return s, nil

	// Binário
	case hasToken(f, ByteaO):
		return raw, nil

	// JSON
	case hasToken(f, JSONO, JSONBO):
		switch v := raw.(type) {
		case []byte:
			var j json.RawMessage
			if err := json.Unmarshal(v, &j); err != nil {
				return nil, err
			}
			return j, nil
		case string:
			var j json.RawMessage
			if err := json.Unmarshal([]byte(v), &j); err != nil {
				return nil, err
			}
			return j, nil
		default:
			return raw, nil
		}

	// UUID e tipos especiais
	case hasToken(f,
		UUIDO, EnumO, CIDRO, INETO, MACAddrO, MACAddr8O,
		TSVectorO, TSQueryO, HstoreO, LtreeO, CubeO,
		PointO, LineO, LsegO, BoxO, CircleO, PathO, PolygonO,
		LineStringO, MultiPointO, MultiLineStringO, MultiPolygonO,
		GeometryO, GeographyO, GeometryCollectionO,
		Int4RangeO, Int8RangeO, NumRangeO, TSRangeO, TSTZRangeO, DateRangeO,
		Int4MultiRangeO, Int8MultiRangeO, NumMultiRangeO, TSMultiRangeO,
		TSTZMultiRangeO, DateMultiRangeO):
		if b, ok := raw.([]byte); ok {
			return string(b), nil
		}
		return raw, nil

	// Boolean
	case hasToken(f, BooleanO):
		switch v := raw.(type) {
		case bool:
			return v, nil
		case []byte:
			return string(v) == "t", nil
		case string:
			return v == "t" || v == "true" || v == "1", nil
		default:
			return nil, fmt.Errorf("unsupported bool type %T", raw)
		}
	}

	// fallback
	return raw, nil
}

// helper
func hasToken(f *core.FieldMeta, tokens ...core.FieldToken) bool {
	for _, t := range tokens {
		if _, ok := f.Meta[t]; ok {
			return true
		}
	}
	return false
}
