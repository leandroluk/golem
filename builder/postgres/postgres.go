package postgres

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/leandroluk/golem/core"
)

// ColumnsFromFields resolve colunas (com aspas) a partir de referências de campo.
// Aceita: []int (índice do campo), "coluna" (db), "GoName" (nome do campo Go) ou literais SQL.
func ColumnsFromFields(schema *core.Schema, fields ...any) []string {
	out := make([]string, 0, len(fields))
	for _, fd := range fields {
		out = append(out, resolveColumn(fd, schema))
	}
	return out
}

// resolveColumn - resolve o nome da coluna no schema (com aspas) a partir da referência de campo.
func resolveColumn(field any, schema *core.Schema) string {
	switch v := field.(type) {
	case []int:
		if f := schema.FieldByIndex(v); f != nil {
			return `"` + f.Column + `"`
		}
		return fmt.Sprintf("/* coluna (index) não encontrada: %v */", v)

	case string:
		// tenta por nome de coluna
		if f := schema.FieldByColumn(v); f != nil {
			return `"` + f.Column + `"`
		}
		// tenta por nome Go do campo
		if f := schema.FieldByName(v); f != nil {
			return `"` + f.Column + `"`
		}
		// se não achou no schema, assume literal/expressão já válida
		return v

	default:
		// fallback: stringificação (evitar pânico)
		return fmt.Sprint(field)
	}
}

// ~=~=~= WHERE / LIMIT / UPDATE builders ~=~=~=

func BuildWhere(q *core.Query, schema *core.Schema, startIndex int) (string, []any, int) {
	if q == nil || len(q.Conds) == 0 {
		return "", nil, startIndex
	}
	parts := []string{}
	args := []any{}
	i := startIndex

	for _, c := range q.Conds {
		switch c.Op {
		// básicos
		case core.EqO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("%s = $%d", col, i))
			args = append(args, c.Args[0])
			i++
		case core.NeqO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("%s <> $%d", col, i))
			args = append(args, c.Args[0])
			i++
		case core.LikeO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("%s LIKE $%d", col, i))
			args = append(args, c.Args[0])
			i++
		case core.NLikeO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("%s NOT LIKE $%d", col, i))
			args = append(args, c.Args[0])
			i++

		// comparadores
		case core.GtO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("%s > $%d", col, i))
			args = append(args, c.Args[0])
			i++
		case core.NGtO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("NOT (%s > $%d)", col, i))
			args = append(args, c.Args[0])
			i++
		case core.GteO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("%s >= $%d", col, i))
			args = append(args, c.Args[0])
			i++
		case core.NGteO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("NOT (%s >= $%d)", col, i))
			args = append(args, c.Args[0])
			i++
		case core.LtO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("%s < $%d", col, i))
			args = append(args, c.Args[0])
			i++
		case core.NLtO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("NOT (%s < $%d)", col, i))
			args = append(args, c.Args[0])
			i++
		case core.LteO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("%s <= $%d", col, i))
			args = append(args, c.Args[0])
			i++
		case core.NLteO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("NOT (%s <= $%d)", col, i))
			args = append(args, c.Args[0])
			i++

		// conjuntos
		case core.InO, core.NinO:
			col := resolveColumn(c.Field, schema)
			ph := []string{}
			for range c.Args {
				ph = append(ph, fmt.Sprintf("$%d", i))
				i++
			}
			op := "IN"
			if c.Op == core.NinO {
				op = "NOT IN"
			}
			parts = append(parts, fmt.Sprintf("%s %s (%s)", col, op, strings.Join(ph, ",")))
			args = append(args, c.Args...)

		// nulls
		case core.NullO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("%s IS NULL", col))
		case core.NotNullO:
			col := resolveColumn(c.Field, schema)
			parts = append(parts, fmt.Sprintf("%s IS NOT NULL", col))

		// lógicos (AND/OR)
		case core.AndO, core.OrO:
			subQueries := c.Args[0].([]*core.Query)
			subParts := []string{}
			for _, sq := range subQueries {
				subSQL, subArgs, next := BuildWhere(sq, schema, i)
				if after, ok := strings.CutPrefix(subSQL, "WHERE "); ok {
					subSQL = after
				}
				subParts = append(subParts, "("+subSQL+")")
				args = append(args, subArgs...)
				i = next
			}
			connector := "AND"
			if c.Op == core.OrO {
				connector = "OR"
			}
			parts = append(parts, strings.Join(subParts, " "+connector+" "))
		}
	}

	if len(parts) == 0 {
		return "", args, i
	}
	return "WHERE " + strings.Join(parts, " AND "), args, i
}

func BuildLimitOffset(q *core.Query, startIndex int) (string, []any, int) {
	if q == nil || len(q.Conds) == 0 {
		return "", nil, startIndex
	}
	parts := []string{}
	args := []any{}
	i := startIndex

	for _, c := range q.Conds {
		switch c.Op {
		case core.LimitO:
			parts = append(parts, fmt.Sprintf("LIMIT $%d", i))
			args = append(args, c.Args[0])
			i++
		case core.SkipO:
			parts = append(parts, fmt.Sprintf("OFFSET $%d", i))
			args = append(args, c.Args[0])
			i++
		}
	}
	return strings.Join(parts, " "), args, i
}

func BuildUpdate(u *core.Update, schema *core.Schema, startIndex int) (string, []any, int) {
	if u == nil || len(u.Sets) == 0 {
		return "", nil, startIndex
	}
	parts := []string{}
	args := []any{}
	i := startIndex

	for _, c := range u.Sets {
		col := resolveColumn(c.Field, schema)
		parts = append(parts, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, c.Args[0])
		i++
	}
	return "SET " + strings.Join(parts, ", "), args, i
}

// ~=~=~= JOIN builder (numeração consistente) ~=~=~=

// removido parâmetro não usado
func buildJoins(q *core.Query, startIndex int) (string, []any, int) {
	if q == nil {
		return "", nil, startIndex
	}
	parts := []string{}
	args := []any{}
	i := startIndex

	for _, c := range q.Conds {
		if c.Op != core.JoinO {
			continue
		}
		joinType := c.Args[0].(core.JoinType)
		rightSchema := c.Args[1].(*core.Schema)
		onQuery := c.Args[2].(*core.Query)

		onSQL, onArgs, next := BuildWhere(onQuery, rightSchema, i)
		if after, ok := strings.CutPrefix(onSQL, "WHERE "); ok {
			onSQL = after
		}
		parts = append(parts, fmt.Sprintf(`%s JOIN "%s" ON %s`, joinType, rightSchema.TableName, onSQL))
		args = append(args, onArgs...)
		i = next
	}
	return strings.Join(parts, " "), args, i
}

// ~=~=~= SELECT / INSERT / UPDATE / DELETE ~=~=~=

// BuildSelect: SELECT ... FROM table [JOIN ...] [WHERE ...] [ORDER BY ...] [LIMIT/OFFSET]
func BuildSelect(schema *core.Schema, table string, q *core.Query, selectCols ...string) (string, []any) {
	cols := "*"
	if len(selectCols) > 0 {
		cols = strings.Join(selectCols, ", ")
	}

	joinSQL, joinArgs, next := buildJoins(q, 1)
	whereSQL, whereArgs, next2 := BuildWhere(q, schema, next)
	limSQL, limArgs, _ := BuildLimitOffset(q, next2)

	// ORDER BY (só colunas do schema "base")
	orderParts := []string{}
	for _, c := range q.Conds {
		if c.Op == core.OrderO {
			col := resolveColumn(c.Field, schema)
			dir := c.Args[0].(core.OrderDirection)
			orderParts = append(orderParts, fmt.Sprintf("%s %s", col, dir))
		}
	}
	orderSQL := ""
	if len(orderParts) > 0 {
		orderSQL = "ORDER BY " + strings.Join(orderParts, ", ")
	}

	sqlParts := []string{table}
	for _, p := range []string{joinSQL, whereSQL, orderSQL, limSQL} {
		if strings.TrimSpace(p) != "" {
			sqlParts = append(sqlParts, p)
		}
	}

	sql := fmt.Sprintf("SELECT %s FROM %s", cols, strings.Join(sqlParts, " "))
	args := append(joinArgs, whereArgs...)
	args = append(args, limArgs...)
	return sql, args
}

func BuildUpdateStmt(schema *core.Schema, table string, u *core.Update, q *core.Query, returningCols []string) (string, []any) {
	setSQL, setArgs, next := BuildUpdate(u, schema, 1)
	whereSQL, whereArgs, _ := BuildWhere(q, schema, next)

	sql := strings.TrimSpace(fmt.Sprintf(`UPDATE %s %s %s`, table, setSQL, whereSQL))
	sql = addReturning(sql, returningCols)
	args := append(setArgs, whereArgs...)
	return sql, args
}

func BuildInsertStmt(schema *core.Schema, table string, entities []any, returningCols []string) (string, []any) {
	if len(entities) == 0 {
		return "", nil
	}

	cols := []string{}
	for _, f := range schema.Fields {
		// Se for PK e zero value → ignora a coluna (deixa o DEFAULT/serial agir)
		if f.Meta[core.PrimaryO] == true {
			v := reflect.ValueOf(entities[0]).Elem().FieldByName(f.Name).Interface()
			if isZero(v) {
				continue
			}
		}
		cols = append(cols, `"`+f.Column+`"`)
	}

	valueRows := []string{}
	args := []any{}
	argIndex := 1

	for _, e := range entities {
		v := reflect.ValueOf(e).Elem()
		placeholders := []string{}

		for _, f := range schema.Fields {
			// se PK zero → ignora
			if f.Meta[core.PrimaryO] == true {
				val := v.FieldByName(f.Name).Interface()
				if isZero(val) {
					continue
				}
			}
			placeholders = append(placeholders, fmt.Sprintf("$%d", argIndex))
			args = append(args, v.FieldByName(f.Name).Interface())
			argIndex++
		}
		valueRows = append(valueRows, "("+strings.Join(placeholders, ",")+")")
	}

	sql := fmt.Sprintf(`INSERT INTO %s (%s) VALUES %s`,
		table, strings.Join(cols, ", "), strings.Join(valueRows, ", "))
	sql = addReturning(sql, returningCols)
	return sql, args
}

func BuildDeleteStmt(schema *core.Schema, table string, q *core.Query, returningCols []string) (string, []any) {
	whereSQL, whereArgs, _ := BuildWhere(q, schema, 1)
	sql := strings.TrimSpace(fmt.Sprintf(`DELETE FROM %s %s`, table, whereSQL))
	sql = addReturning(sql, returningCols)
	return sql, whereArgs
}

func BuildCountStmt(schema *core.Schema, table string, q *core.Query) (string, []any) {
	joinSQL, joinArgs, next := buildJoins(q, 1)
	whereSQL, whereArgs, _ := BuildWhere(q, schema, next)
	sql := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s %s`, table, joinSQL, whereSQL)
	args := append(joinArgs, whereArgs...)
	return sql, args
}

func addReturning(sql string, cols []string) string {
	if len(cols) == 0 {
		return sql
	}
	if len(cols) == 1 && cols[0] == "*" {
		return sql + " RETURNING *"
	}
	return sql + " RETURNING " + strings.Join(cols, ", ")
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
