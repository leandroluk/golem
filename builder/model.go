package builder

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	psql "github.com/leandroluk/golem/builder/postgres"
	"github.com/leandroluk/golem/core"
)

type ReturningMode int

const (
	ReturningNone ReturningMode = iota
	ReturningPK
	ReturningAll
)

type Model[T any, S any] struct {
	schema    *core.Schema
	client    *core.Client[S]
	returning ReturningMode
}

func NewModel[T any, S any](schema *core.Schema, client *core.Client[S]) *Model[T, S] {
	return &Model[T, S]{schema: schema, client: client}
}

func (m *Model[T, S]) WithReturning(mode ReturningMode) *Model[T, S] {
	m2 := *m
	m2.returning = mode
	return &m2
}

func tableName(s *core.Schema) string {
	name := fmt.Sprintf(`"%s"`, s.TableName)
	if s.SchemaName != "" {
		return fmt.Sprintf(`"%s".%s`, s.SchemaName, name)
	}
	return name
}

func runHook(schema *core.Schema, name string, entity any) error {
	if fn, ok := schema.Hooks[name]; ok {
		return fn(entity)
	}
	return nil
}

func scanRows[T any](rows core.Rows, schema *core.Schema, dialect core.Dialect) ([]T, error) {
	defer rows.Close()
	out := []T{}

	for rows.Next() {
		var t T
		v := reflect.ValueOf(&t).Elem()
		rawDest := make([]any, len(schema.Fields))

		for i := range schema.Fields {
			var tmp any
			rawDest[i] = &tmp
		}

		if err := rows.Scan(rawDest...); err != nil {
			return nil, err
		}

		for i, f := range schema.Fields {
			val := reflect.ValueOf(rawDest[i]).Elem().Interface()
			converted, err := dialect.FromValue(f, val)
			if err != nil {
				return nil, err
			}
			fv := v.FieldByName(f.Name)
			if fv.IsValid() && fv.CanSet() && converted != nil {
				fv.Set(reflect.ValueOf(converted))
			}
		}

		if err := runHook(schema, "AfterSelect", &t); err != nil {
			return nil, err
		}

		out = append(out, t)
	}
	return out, nil
}

func isZero(x any) bool {
	v := reflect.ValueOf(x)
	switch v.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	default:
		z := reflect.Zero(v.Type()).Interface()
		return reflect.DeepEqual(x, z)
	}
}

func (m *Model[T, S]) returningCols() []string {
	switch m.returning {
	case ReturningAll:
		return []string{"*"}
	case ReturningPK:
		if pk := m.schema.FindPrimaryKey(); pk != nil {
			return []string{`"` + pk.Column + `"`}
		}
	}
	return []string{}
}

func (m *Model[T, S]) Save(ctx context.Context, entity *T) error {
	m.returning = ReturningAll

	pk := m.schema.FindPrimaryKey()
	if pk == nil {
		result := m.Insert(ctx, entity)
		m.returning = ReturningNone
		return result
	}

	v := reflect.ValueOf(entity).Elem()
	fv := v.FieldByName(pk.Name)

	if !fv.IsValid() || isZero(fv.Interface()) {
		result := m.Insert(ctx, entity)
		m.returning = ReturningNone
		return result
	}

	_, err := m.Update(ctx, func(t2 *T, u *core.Update, q *core.Query) {
		ev := reflect.ValueOf(entity).Elem()
		tv := reflect.ValueOf(t2).Elem()

		for _, f := range m.schema.Fields {
			if f == pk {
				continue
			}
			tf := tv.FieldByName(f.Name)
			if tf.IsValid() && tf.CanAddr() {
				u.Set(tf.Addr().Interface(), ev.FieldByName(f.Name).Interface())
			}
		}

		pkf := tv.FieldByName(pk.Name)
		if pkf.IsValid() && pkf.CanAddr() {
			q.Eq(pkf.Addr().Interface(), fv.Interface())
		}
	})

	m.returning = ReturningNone

	return err
}

func (m *Model[T, S]) Insert(ctx context.Context, entities ...*T) error {
	if len(entities) == 0 {
		return nil
	}
	table := tableName(m.schema)
	cache := core.BuildFieldCache(m.schema, reflect.TypeOf((*T)(nil)).Elem())

	// BeforeInsert + Getters + timestamps
	for _, e := range entities {
		if err := runHook(m.schema, "BeforeInsert", e); err != nil {
			return err
		}
		v := reflect.ValueOf(e).Elem()
		now := time.Now()

		for _, f := range m.schema.Fields {
			fv := v.FieldByName(f.Name)
			if !fv.IsValid() || !fv.CanSet() {
				continue
			}
			// timestamps
			switch f.Column {
			case "created_at", "updated_at":
				if fv.Kind() == reflect.Pointer {
					fv.Set(reflect.ValueOf(&now))
				} else {
					fv.Set(reflect.ValueOf(now))
				}
			}
		}
		// getters (por campo correto)
		// mapeia: coluna -> idx de cache
		colToIdx := make(map[string]int, len(m.schema.Fields))
		for i, f := range m.schema.Fields {
			colToIdx[f.Column] = i
		}
		for _, f := range m.schema.Fields {
			idx := colToIdx[f.Column]
			if idx < len(cache) && cache[idx].Getter != nil {
				fv := v.FieldByName(f.Name)
				if fv.IsValid() && fv.CanSet() {
					fv.Set(reflect.ValueOf(cache[idx].Getter(fv.Interface())))
				}
			}
		}
	}

	anySlice := make([]any, len(entities))
	for i, e := range entities {
		anySlice[i] = e
	}

	sql, args := psql.BuildInsertStmt(m.schema, table, anySlice, m.returningCols())

	dialect := m.client.Dialect
	// Converte os primeiros N args conforme os metadados dos campos do schema.
	// (Se o seu BuildInsertStmt gera colunas dinâmicas, adapte esta parte para usar a lista de colunas retornada.)
	for i, f := range m.schema.Fields {
		if i < len(args) {
			args[i] = dialect.ToValue(f, args[i])
		}
	}

	switch m.returning {
	case ReturningAll:
		rows, err := m.client.Query(ctx, any(sql).(S), args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		i := 0
		for rows.Next() && i < len(entities) {
			v := reflect.ValueOf(entities[i]).Elem()
			dest := make([]any, 0, len(m.schema.Fields))
			for _, f := range m.schema.Fields {
				fv := v.FieldByName(f.Name)
				if fv.IsValid() {
					dest = append(dest, fv.Addr().Interface())
				} else {
					var dummy any
					dest = append(dest, &dummy)
				}
			}
			if err := rows.Scan(dest...); err != nil {
				return err
			}
			if err := runHook(m.schema, "AfterInsert", entities[i]); err != nil {
				return err
			}
			i++
		}
	case ReturningPK:
		pk := m.schema.FindPrimaryKey()
		if pk == nil {
			_, err := m.client.Exec(ctx, any(sql).(S), args...)
			return err
		}
		rows, err := m.client.Query(ctx, any(sql).(S), args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		i := 0
		for rows.Next() && i < len(entities) {
			v := reflect.ValueOf(entities[i]).Elem()
			fv := v.FieldByName(pk.Name)
			if fv.IsValid() {
				if err := rows.Scan(fv.Addr().Interface()); err != nil {
					return err
				}
			}
			if err := runHook(m.schema, "AfterInsert", entities[i]); err != nil {
				return err
			}
			i++
		}
	default:
		if _, err := m.client.Exec(ctx, any(sql).(S), args...); err != nil {
			return err
		}
		for _, e := range entities {
			if err := runHook(m.schema, "AfterInsert", e); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Model[T, S]) Update(ctx context.Context, fn func(*T, *core.Update, *core.Query)) ([]T, error) {
	var t T

	q := core.NewQuery(m.schema, &t)
	u := core.NewUpdate(m.schema, &t)

	fn(&t, u, q)

	if len(u.Sets) == 0 {
		return nil, errors.New("update sem SET")
	}

	// mapa para achar FieldMeta e aplicar getter / timestamp / ToValue
	colToIdx := make(map[string]int, len(m.schema.Fields))
	for i, f := range m.schema.Fields {
		colToIdx[f.Column] = i
	}
	cache := core.BuildFieldCache(m.schema, reflect.TypeOf((*T)(nil)).Elem())
	dialect := m.client.Dialect
	now := time.Now()

	// Normaliza valores de SET com base no FieldMeta correto
	for i, set := range u.Sets {
		var fm *core.FieldMeta
		switch idx := set.Field.(type) {
		case []int:
			fm = m.schema.FieldByIndex(idx)
		case string:
			if fm = m.schema.FieldByColumn(idx); fm == nil {
				fm = m.schema.FieldByName(idx)
			}
		}
		if fm == nil {
			continue
		}

		// getter (cache alinhado com ordem dos fields do schema)
		if ci, ok := colToIdx[fm.Column]; ok && ci < len(cache) && cache[ci].Getter != nil {
			u.Sets[i].Args[0] = cache[ci].Getter(set.Args[0])
		}

		// timestamps automáticos
		if fm.Column == "updated_at" {
			fv := reflect.ValueOf(&t).Elem().FieldByName(fm.Name)
			if fv.IsValid() && fv.CanSet() {
				if fv.Kind() == reflect.Pointer {
					fv.Set(reflect.ValueOf(&now))
					u.Sets[i].Args[0] = &now
				} else {
					fv.Set(reflect.ValueOf(now))
					u.Sets[i].Args[0] = now
				}
			}
		}

		// ToValue
		u.Sets[i].Args[0] = dialect.ToValue(fm, u.Sets[i].Args[0])
	}

	if err := runHook(m.schema, "BeforeUpdate", &t); err != nil {
		return nil, err
	}

	table := tableName(m.schema)
	sql, args := psql.BuildUpdateStmt(m.schema, table, u, q, m.returningCols())

	switch m.returning {
	case ReturningAll:
		rows, err := m.client.Query(ctx, any(sql).(S), args...)
		if err != nil {
			return nil, err
		}
		res, err := scanRows[T](rows, m.schema, m.client.Dialect)
		if err != nil {
			return nil, err
		}
		for i := range res {
			if err := runHook(m.schema, "AfterUpdate", &res[i]); err != nil {
				return nil, err
			}
		}
		return res, nil
	case ReturningPK:
		pk := m.schema.FindPrimaryKey()
		if pk == nil {
			_, err := m.client.Exec(ctx, any(sql).(S), args...)
			return nil, err
		}
		rows, err := m.client.Query(ctx, any(sql).(S), args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		ids := []T{}
		for rows.Next() {
			var tmp T
			v := reflect.ValueOf(&tmp).Elem()
			fv := v.FieldByName(pk.Name)
			if fv.IsValid() {
				if err := rows.Scan(fv.Addr().Interface()); err != nil {
					return nil, err
				}
			}
			if err := runHook(m.schema, "AfterUpdate", &tmp); err != nil {
				return nil, err
			}
			ids = append(ids, tmp)
		}
		return ids, nil
	default:
		_, err := m.client.Exec(ctx, any(sql).(S), args...)
		if err != nil {
			return nil, err
		}
		return nil, runHook(m.schema, "AfterUpdate", &t)
	}
}

func (m *Model[T, S]) Delete(ctx context.Context, fn func(*T, *core.Query)) ([]T, error) {
	var t T
	q := core.NewQuery(m.schema, &t)
	fn(&t, q)

	if err := runHook(m.schema, "BeforeDelete", &t); err != nil {
		return nil, err
	}

	table := tableName(m.schema)
	sql, args := psql.BuildDeleteStmt(m.schema, table, q, m.returningCols())

	switch m.returning {
	case ReturningAll:
		rows, err := m.client.Query(ctx, any(sql).(S), args...)
		if err != nil {
			return nil, err
		}
		res, err := scanRows[T](rows, m.schema, m.client.Dialect)
		if err != nil {
			return nil, err
		}
		for i := range res {
			if err := runHook(m.schema, "AfterDelete", &res[i]); err != nil {
				return nil, err
			}
		}
		return res, nil
	case ReturningPK:
		pk := m.schema.FindPrimaryKey()
		if pk == nil {
			_, err := m.client.Exec(ctx, any(sql).(S), args...)
			return nil, err
		}
		rows, err := m.client.Query(ctx, any(sql).(S), args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		ids := []T{}
		for rows.Next() {
			var tmp T
			v := reflect.ValueOf(&tmp).Elem()
			fv := v.FieldByName(pk.Name)
			if fv.IsValid() {
				if err := rows.Scan(fv.Addr().Interface()); err != nil {
					return nil, err
				}
			}
			if err := runHook(m.schema, "AfterDelete", &tmp); err != nil {
				return nil, err
			}
			ids = append(ids, tmp)
		}
		return ids, nil
	default:
		_, err := m.client.Exec(ctx, any(sql).(S), args...)
		if err != nil {
			return nil, err
		}
		return nil, runHook(m.schema, "AfterDelete", &t)
	}
}

func (m *Model[T, S]) Count(ctx context.Context, fn func(*T, *core.Query)) (int64, error) {
	var t T
	q := core.NewQuery(m.schema, &t)
	fn(&t, q)

	table := tableName(m.schema)
	sql, args := psql.BuildCountStmt(m.schema, table, q)
	rows, err := m.client.Query(ctx, any(sql).(S), args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var n int64
	if rows.Next() {
		if err := rows.Scan(&n); err != nil {
			return 0, err
		}
		return n, nil
	}
	return 0, nil
}

func (m *Model[T, S]) FindOne(ctx context.Context, fn func(*T, *core.Query)) (*T, error) {
	var t T
	q := core.NewQuery(m.schema, &t)
	fn(&t, q)
	q.Limit(1)

	table := tableName(m.schema)
	sql, args := psql.BuildSelect(m.schema, table, q)
	rows, err := m.client.Query(ctx, any(sql).(S), args...)
	if err != nil {
		return nil, err
	}
	list, err := scanRows[T](rows, m.schema, m.client.Dialect)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("nenhum registro encontrado")
	}
	return &list[0], nil
}

func (m *Model[T, S]) FindMany(ctx context.Context, fn func(*T, *core.Query)) ([]T, error) {
	var t T
	q := core.NewQuery(m.schema, &t)
	fn(&t, q)

	table := tableName(m.schema)
	sql, args := psql.BuildSelect(m.schema, table, q)
	rows, err := m.client.Query(ctx, any(sql).(S), args...)
	if err != nil {
		return nil, err
	}
	return scanRows[T](rows, m.schema, m.client.Dialect)
}
