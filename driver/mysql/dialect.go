package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/stmt"
)

// dialect is the MySQL/MariaDB implementation of golem.Dialect.
type dialect struct {
	db sqlIface
}

var _ golem.Dialect = (*dialect)(nil)

// Bind converts a Go value to a driver.Value suitable for MySQL, according
// to the declared ColumnType. Per INSIGHT.md, only "boolean" (TINYINT(1))
// and "uuid" (CHAR(36)) differ from native MySQL types at all, and neither
// actually needs a different Go-value representation than Postgres's
// version — MySQL has no native UUID type, so a string already round-trips
// through CHAR(36) untouched.
func (dialect) Bind(t golem.ColumnType, value any) (driver.Value, error) {
	switch t.Kind() {
	case "boolean":
		switch v := value.(type) {
		case bool:
			return v, nil
		case int, int8, int16, int32, int64:
			return fmt.Sprintf("%v", v) != "0", nil
		}
		return nil, fmt.Errorf("mysql: bind boolean: unsupported Go type %T", value)

	case "smallint", "integer", "bigint":
		switch v := value.(type) {
		case int:
			return int64(v), nil
		case int8:
			return int64(v), nil
		case int16:
			return int64(v), nil
		case int32:
			return int64(v), nil
		case int64:
			return v, nil
		}
		return nil, fmt.Errorf("mysql: bind %s: unsupported Go type %T", t.Kind(), value)

	case "decimal", "float":
		switch v := value.(type) {
		case float32:
			return float64(v), nil
		case float64:
			return v, nil
		}
		return nil, fmt.Errorf("mysql: bind %s: unsupported Go type %T", t.Kind(), value)

	case "char", "varchar", "text":
		switch v := value.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		}
		return nil, fmt.Errorf("mysql: bind %s: unsupported Go type %T", t.Kind(), value)

	case "date", "datetime", "time":
		switch v := value.(type) {
		case time.Time:
			return v, nil
		case *time.Time:
			if v == nil {
				return nil, nil
			}
			return *v, nil
		}
		return nil, fmt.Errorf("mysql: bind %s: unsupported Go type %T", t.Kind(), value)

	case "blob":
		switch v := value.(type) {
		case []byte:
			return v, nil
		case string:
			return []byte(v), nil
		}
		return nil, fmt.Errorf("mysql: bind blob: unsupported Go type %T", value)

	case "uuid":
		switch v := value.(type) {
		case string:
			return v, nil
		case [16]byte:
			return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
				v[0:4], v[4:6], v[6:8], v[8:10], v[10:16]), nil
		}
		return nil, fmt.Errorf("mysql: bind uuid: unsupported Go type %T", value)

	case "json":
		switch v := value.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		}
		return nil, fmt.Errorf("mysql: bind json: unsupported Go type %T", value)
	}

	return nil, fmt.Errorf("mysql: bind: unrecognized column kind %q", t.Kind())
}

// Scan converts a raw value returned by database/sql into dest, according
// to the declared ColumnType. go-sql-driver/mysql's default Go
// representations diverge from pgx's in several places (DECIMAL as
// []byte/string rather than a numeric wrapper type, booleans/integers as
// int64 regardless of column width), so this accepts a broader set of raw
// types per kind than driver/postgres's version needs to.
func (dialect) Scan(t golem.ColumnType, raw any, dest any) error {
	if raw == nil {
		return nil
	}

	switch t.Kind() {
	case "boolean":
		d, ok := dest.(*bool)
		if !ok {
			return fmt.Errorf("mysql: scan boolean: dest must be *bool, got %T", dest)
		}
		switch v := raw.(type) {
		case bool:
			*d = v
		case int64:
			*d = v != 0
		case []byte:
			*d = string(v) != "0" && string(v) != ""
		default:
			return fmt.Errorf("mysql: scan boolean: unsupported raw type %T", raw)
		}

	case "smallint", "integer", "bigint":
		d, ok := dest.(*int64)
		if !ok {
			return fmt.Errorf("mysql: scan %s: dest must be *int64, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case int64:
			*d = v
		case int32:
			*d = int64(v)
		case int16:
			*d = int64(v)
		case []byte:
			n, err := strconv.ParseInt(string(v), 10, 64)
			if err != nil {
				return fmt.Errorf("mysql: scan %s: %w", t.Kind(), err)
			}
			*d = n
		default:
			return fmt.Errorf("mysql: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "decimal", "float":
		d, ok := dest.(*float64)
		if !ok {
			return fmt.Errorf("mysql: scan %s: dest must be *float64, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case float64:
			*d = v
		case float32:
			*d = float64(v)
		case []byte:
			// go-sql-driver returns DECIMAL/NUMERIC columns as []byte (a
			// decimal-text representation), not a numeric wrapper type.
			f, err := strconv.ParseFloat(string(v), 64)
			if err != nil {
				return fmt.Errorf("mysql: scan %s: %w", t.Kind(), err)
			}
			*d = f
		default:
			return fmt.Errorf("mysql: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "char", "varchar", "text":
		d, ok := dest.(*string)
		if !ok {
			return fmt.Errorf("mysql: scan %s: dest must be *string, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case string:
			*d = v
		case []byte:
			*d = string(v)
		default:
			return fmt.Errorf("mysql: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "date", "datetime", "time":
		d, ok := dest.(*time.Time)
		if !ok {
			return fmt.Errorf("mysql: scan %s: dest must be *time.Time, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case time.Time:
			*d = v
		default:
			return fmt.Errorf("mysql: scan %s: unsupported raw type %T (did ParseTime get disabled?)", t.Kind(), raw)
		}

	case "blob":
		d, ok := dest.(*[]byte)
		if !ok {
			return fmt.Errorf("mysql: scan blob: dest must be *[]byte, got %T", dest)
		}
		switch v := raw.(type) {
		case []byte:
			*d = v
		case string:
			*d = []byte(v)
		default:
			return fmt.Errorf("mysql: scan blob: unsupported raw type %T", raw)
		}

	case "uuid":
		d, ok := dest.(*string)
		if !ok {
			return fmt.Errorf("mysql: scan uuid: dest must be *string, got %T", dest)
		}
		switch v := raw.(type) {
		case string:
			*d = v
		case []byte:
			*d = string(v)
		default:
			return fmt.Errorf("mysql: scan uuid: unsupported raw type %T", raw)
		}

	case "json":
		d, ok := dest.(*string)
		if !ok {
			return fmt.Errorf("mysql: scan json: dest must be *string, got %T", dest)
		}
		switch v := raw.(type) {
		case string:
			*d = v
		case []byte:
			*d = string(v)
		default:
			return fmt.Errorf("mysql: scan json: unsupported raw type %T", raw)
		}

	default:
		return fmt.Errorf("mysql: scan: unrecognized column kind %q", t.Kind())
	}

	return nil
}

// quoteIdent backtick-quotes a SQL identifier, preserving table prefixes.
func quoteIdent(name string) string {
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		quoted := make([]string, len(parts))
		for i, p := range parts {
			quoted[i] = "`" + p + "`"
		}
		return strings.Join(quoted, ".")
	}
	return "`" + name + "`"
}

// aggregateSQLFunc builds a SUM/AVG/COUNT(column) expression. SUM/AVG are
// cast to DOUBLE so the driver always decodes them as []byte/float64
// consistently regardless of the source column's exact numeric type,
// mirroring driver/postgres's DOUBLE PRECISION cast for the same reason.
func aggregateSQLFunc(fn, column string) string {
	switch fn {
	case "count_all":
		return "COUNT(*)"
	case "count":
		return fmt.Sprintf("COUNT(%s)", quoteIdent(column))
	case "sum", "avg":
		return fmt.Sprintf("CAST(%s(%s) AS DOUBLE)", strings.ToUpper(fn), quoteIdent(column))
	default:
		return quoteIdent(column)
	}
}

// projectionSQL builds one SELECT-list expression (with its alias) from a
// stmt.Projection.
func projectionSQL(p stmt.Projection) string {
	if p.Func == "" {
		return fmt.Sprintf("%s AS %s", quoteIdent(p.Column), quoteIdent(p.Alias))
	}
	return fmt.Sprintf("%s AS %s", aggregateSQLFunc(p.Func, p.Column), quoteIdent(p.Alias))
}

// compilePredicate recursively builds the MySQL SQL WHERE clause from a
// stmt.Predicate. Unlike Postgres's $N numbered placeholders, MySQL uses
// unnumbered "?" — args are still appended in call order, but no argOffset
// counter is needed to render the placeholder text itself.
func compilePredicate(p stmt.Predicate, args *[]any) (string, error) {
	if p == nil {
		return "", nil
	}

	switch v := p.(type) {
	case stmt.Comparison:
		switch v.Op {
		case "eq":
			*args = append(*args, v.Value)
			return fmt.Sprintf("%s=?", quoteIdent(v.Column)), nil
		case "gt":
			*args = append(*args, v.Value)
			return fmt.Sprintf("%s>?", quoteIdent(v.Column)), nil
		case "gte":
			*args = append(*args, v.Value)
			return fmt.Sprintf("%s>=?", quoteIdent(v.Column)), nil
		case "lt":
			*args = append(*args, v.Value)
			return fmt.Sprintf("%s<?", quoteIdent(v.Column)), nil
		case "lte":
			*args = append(*args, v.Value)
			return fmt.Sprintf("%s<=?", quoteIdent(v.Column)), nil
		case "like":
			*args = append(*args, v.Value)
			return fmt.Sprintf("%s LIKE ?", quoteIdent(v.Column)), nil
		case "is_null":
			return fmt.Sprintf("%s IS NULL", quoteIdent(v.Column)), nil
		case "in":
			valVal := reflect.ValueOf(v.Value)
			if valVal.Kind() != reflect.Slice {
				*args = append(*args, v.Value)
				return fmt.Sprintf("%s IN (?)", quoteIdent(v.Column)), nil
			}
			n := valVal.Len()
			if n == 0 {
				return "FALSE", nil
			}
			placeholders := make([]string, n)
			for i := 0; i < n; i++ {
				placeholders[i] = "?"
				*args = append(*args, valVal.Index(i).Interface())
			}
			return fmt.Sprintf("%s IN (%s)", quoteIdent(v.Column), strings.Join(placeholders, ",")), nil
		}
		return "", fmt.Errorf("mysql: unsupported comparison operator %q", v.Op)

	case stmt.Logical:
		if len(v.Predicates) == 0 {
			return "", nil
		}
		parts := make([]string, 0, len(v.Predicates))
		for _, pred := range v.Predicates {
			part, err := compilePredicate(pred, args)
			if err != nil {
				return "", err
			}
			if part != "" {
				parts = append(parts, part)
			}
		}
		if len(parts) == 0 {
			return "", nil
		}
		opName := " AND "
		if v.Op == "or" {
			opName = " OR "
		}
		return "(" + strings.Join(parts, opName) + ")", nil

	case stmt.Not:
		part, err := compilePredicate(v.Predicate, args)
		if err != nil {
			return "", err
		}
		if part == "" {
			return "", nil
		}
		return "NOT (" + part + ")", nil

	case stmt.AggregateComparison:
		expr := aggregateSQLFunc(v.Func, v.Column)
		switch v.Op {
		case "eq":
			*args = append(*args, v.Value)
			return fmt.Sprintf("%s=?", expr), nil
		case "gt":
			*args = append(*args, v.Value)
			return fmt.Sprintf("%s>?", expr), nil
		case "gte":
			*args = append(*args, v.Value)
			return fmt.Sprintf("%s>=?", expr), nil
		case "lt":
			*args = append(*args, v.Value)
			return fmt.Sprintf("%s<?", expr), nil
		case "lte":
			*args = append(*args, v.Value)
			return fmt.Sprintf("%s<=?", expr), nil
		}
		return "", fmt.Errorf("mysql: unsupported having comparison operator %q", v.Op)

	default:
		return "", fmt.Errorf("mysql: unsupported predicate type %T", p)
	}
}

// CompileSelect compiles a Select statement plan to MySQL SQL and args.
func (d *dialect) CompileSelect(s *stmt.Select) (string, []any, error) {
	var sb strings.Builder
	sb.WriteString("SELECT ")
	if len(s.Projections) > 0 {
		parts := make([]string, len(s.Projections))
		for i, p := range s.Projections {
			parts[i] = projectionSQL(p)
		}
		sb.WriteString(strings.Join(parts, ", "))
	} else if s.Count {
		sb.WriteString("COUNT(*)")
	} else if len(s.Columns) == 0 {
		sb.WriteString("*")
	} else {
		quotedCols := make([]string, len(s.Columns))
		for i, col := range s.Columns {
			quotedCols[i] = quoteIdent(col)
		}
		sb.WriteString(strings.Join(quotedCols, ", "))
	}
	sb.WriteString(" FROM ")
	sb.WriteString(quoteIdent(s.Table))

	var args []any

	for _, j := range s.Joins {
		sb.WriteString(" ")
		sb.WriteString(strings.ToUpper(j.Type))
		sb.WriteString(" JOIN ")
		sb.WriteString(quoteIdent(j.Table))
		sb.WriteString(" ON ")
		onClauses := make([]string, len(j.On))
		for idx, cond := range j.On {
			onClauses[idx] = fmt.Sprintf("%s = %s", quoteIdent(cond.LeftCol), quoteIdent(cond.RightCol))
		}
		sb.WriteString(strings.Join(onClauses, " AND "))
		if j.Where != nil {
			whereSQL, err := compilePredicate(j.Where, &args)
			if err != nil {
				return "", nil, err
			}
			if whereSQL != "" {
				sb.WriteString(" AND ")
				sb.WriteString(whereSQL)
			}
		}
	}

	if s.Where != nil {
		sql, err := compilePredicate(s.Where, &args)
		if err != nil {
			return "", nil, err
		}
		if sql != "" {
			sb.WriteString(" WHERE ")
			sb.WriteString(sql)
		}
	}

	if len(s.GroupBy) > 0 {
		sb.WriteString(" GROUP BY ")
		groupCols := make([]string, len(s.GroupBy))
		for i, col := range s.GroupBy {
			groupCols[i] = quoteIdent(col)
		}
		sb.WriteString(strings.Join(groupCols, ", "))
	}

	if s.Having != nil {
		sql, err := compilePredicate(s.Having, &args)
		if err != nil {
			return "", nil, err
		}
		if sql != "" {
			sb.WriteString(" HAVING ")
			sb.WriteString(sql)
		}
	}

	if len(s.OrderBy) > 0 {
		sb.WriteString(" ORDER BY ")
		orderByClauses := make([]string, len(s.OrderBy))
		for i, ord := range s.OrderBy {
			dir := "ASC"
			if ord.Desc {
				dir = "DESC"
			}
			orderByClauses[i] = fmt.Sprintf("%s %s", quoteIdent(ord.Column), dir)
		}
		sb.WriteString(strings.Join(orderByClauses, ", "))
	}

	if s.Limit != nil {
		sb.WriteString(" LIMIT ?")
		args = append(args, *s.Limit)
	}

	if s.Offset != nil {
		sb.WriteString(" OFFSET ?")
		args = append(args, *s.Offset)
	}

	if s.Lock != nil {
		sql, err := lockClauseSQL(s.Lock)
		if err != nil {
			return "", nil, err
		}
		sb.WriteString(sql)
	}

	return sb.String(), args, nil
}

// lockClauseSQL builds the trailing `FOR {UPDATE|SHARE} [NOWAIT|SKIP
// LOCKED]` row-locking clause (MySQL 8.0+; NOWAIT/SKIP LOCKED need 8.0.1+).
// MySQL has no equivalent of Postgres's NO KEY UPDATE/KEY SHARE — those
// strengths fall into the default case below and error, which is what
// actually enforces Capabilities.Locking's NoKeyUpdate/KeyShare being false
// for real (query.Query[T] has no dialect awareness of its own).
func lockClauseSQL(l *stmt.LockClause) (string, error) {
	var strength string
	switch l.Strength {
	case "update":
		strength = "UPDATE"
	case "share":
		strength = "SHARE"
	default:
		return "", fmt.Errorf("mysql: unsupported lock strength %q", l.Strength)
	}

	sql := " FOR " + strength

	switch l.Wait {
	case "":
		// block (MySQL default) — no suffix
	case "nowait":
		sql += " NOWAIT"
	case "skip_locked":
		sql += " SKIP LOCKED"
	default:
		return "", fmt.Errorf("mysql: unsupported lock wait policy %q", l.Wait)
	}

	return sql, nil
}

// CompileDelete compiles a Delete statement plan to MySQL SQL and args.
func (d *dialect) CompileDelete(s *stmt.Delete) (string, []any, error) {
	var sb strings.Builder
	sb.WriteString("DELETE FROM ")
	sb.WriteString(quoteIdent(s.Table))

	var args []any

	if s.Where != nil {
		sql, err := compilePredicate(s.Where, &args)
		if err != nil {
			return "", nil, err
		}
		if sql != "" {
			sb.WriteString(" WHERE ")
			sb.WriteString(sql)
		}
	}

	return sb.String(), args, nil
}

// Insert executes an INSERT statement against MySQL. MySQL has no
// RETURNING, so this is a multi-round-trip: INSERT, resolve every
// PrimaryKey column's value (from s.Columns/s.Values when the caller
// provided it, or LastInsertId() for the one auto-increment column that was
// omitted), then a follow-up SELECT by that PK to return the full row.
func (d *dialect) Insert(ctx context.Context, conn golem.Conn, s *stmt.Insert) (map[string]any, error) {
	quotedCols := make([]string, len(s.Columns))
	placeholders := make([]string, len(s.Columns))
	for i, c := range s.Columns {
		quotedCols[i] = quoteIdent(c)
		placeholders[i] = "?"
	}
	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteIdent(s.Table), strings.Join(quotedCols, ","), strings.Join(placeholders, ","))

	args := make([]any, len(s.Values))
	for i, v := range s.Values {
		args[i] = v
	}

	executor := d.getExecutor(conn)
	result, err := executor.ExecContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: insert: %w", mapError(err))
	}

	whereClauses := make([]string, 0, len(s.PrimaryKey))
	whereArgs := make([]any, 0, len(s.PrimaryKey))
	for _, pk := range s.PrimaryKey {
		if idx := indexOf(s.Columns, pk); idx >= 0 {
			whereClauses = append(whereClauses, fmt.Sprintf("%s=?", quoteIdent(pk)))
			whereArgs = append(whereArgs, s.Values[idx])
			continue
		}
		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("mysql: insert: resolve primary key %q: %w", pk, err)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("%s=?", quoteIdent(pk)))
		whereArgs = append(whereArgs, id)
	}

	if len(whereClauses) == 0 {
		return nil, fmt.Errorf("mysql: insert: table %q has no primary key, cannot read back the inserted row", s.Table)
	}

	selectSQL := fmt.Sprintf("SELECT * FROM %s WHERE %s", quoteIdent(s.Table), strings.Join(whereClauses, " AND "))
	rows, err := executor.QueryContext(ctx, selectSQL, whereArgs...)
	if err != nil {
		return nil, fmt.Errorf("mysql: insert: read back: %w", mapError(err))
	}
	defer rows.Close()

	results, err := collectRows(rows)
	if err != nil {
		return nil, fmt.Errorf("mysql: insert: read back: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("mysql: insert: read back: no row found after insert")
	}
	return results[0], nil
}

func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

// Update executes an UPDATE statement, then a follow-up SELECT to return
// all updated rows (MySQL has no RETURNING). If Where matches at least one
// row and PrimaryKey is known, the matching rows' primary key values are
// captured BEFORE the UPDATE runs, and the follow-up SELECT reads them back
// by primary key instead of re-running Where verbatim — Sets may modify a
// column Where itself filters on (e.g. Set(&t.Category, "hardware") after
// Where(op.Eq(&t.Category, "tools"))), which would otherwise make the
// post-update Where match zero rows. Falls back to re-running Where as-is
// when PrimaryKey is empty (callers that don't populate it never modify a
// column their own Where depends on) or when Where is empty (already
// matches every row, nothing to disambiguate).
func (d *dialect) Update(ctx context.Context, conn golem.Conn, s *stmt.Update) ([]map[string]any, error) {
	setClauses := make([]string, 0, len(s.Sets))
	args := make([]any, 0, len(s.Sets))
	for _, set := range s.Sets {
		setClauses = append(setClauses, fmt.Sprintf("%s=?", quoteIdent(set.Column)))
		args = append(args, set.Value)
	}

	var sb strings.Builder
	sb.WriteString("UPDATE ")
	sb.WriteString(quoteIdent(s.Table))
	sb.WriteString(" SET ")
	sb.WriteString(strings.Join(setClauses, ","))

	var whereArgs []any
	whereSQL := ""
	if s.Where != nil {
		sql, err := compilePredicate(s.Where, &whereArgs)
		if err != nil {
			return nil, err
		}
		whereSQL = sql
		if whereSQL != "" {
			sb.WriteString(" WHERE ")
			sb.WriteString(whereSQL)
			args = append(args, whereArgs...)
		}
	}

	executor := d.getExecutor(conn)

	usePKReadBack := whereSQL != "" && len(s.PrimaryKey) > 0
	var pkRows []map[string]any
	if usePKReadBack {
		quotedPKs := make([]string, len(s.PrimaryKey))
		for i, pk := range s.PrimaryKey {
			quotedPKs[i] = quoteIdent(pk)
		}
		selectPKSQL := fmt.Sprintf("SELECT %s FROM %s WHERE %s", strings.Join(quotedPKs, ","), quoteIdent(s.Table), whereSQL)
		rows, err := executor.QueryContext(ctx, selectPKSQL, whereArgs...)
		if err != nil {
			return nil, fmt.Errorf("mysql: update: capture primary keys: %w", mapError(err))
		}
		pkRows, err = collectRows(rows)
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("mysql: update: capture primary keys: %w", err)
		}
	}

	if _, err := executor.ExecContext(ctx, sb.String(), args...); err != nil {
		return nil, fmt.Errorf("mysql: update: %w", mapError(err))
	}

	var selectSQL string
	var selectArgs []any
	switch {
	case usePKReadBack && len(pkRows) == 0:
		return []map[string]any{}, nil
	case usePKReadBack:
		orClauses := make([]string, len(pkRows))
		for i, row := range pkRows {
			andClauses := make([]string, len(s.PrimaryKey))
			for j, pk := range s.PrimaryKey {
				andClauses[j] = fmt.Sprintf("%s=?", quoteIdent(pk))
				selectArgs = append(selectArgs, row[pk])
			}
			orClauses[i] = "(" + strings.Join(andClauses, " AND ") + ")"
		}
		selectSQL = "SELECT * FROM " + quoteIdent(s.Table) + " WHERE " + strings.Join(orClauses, " OR ")
	case whereSQL != "":
		selectSQL = "SELECT * FROM " + quoteIdent(s.Table) + " WHERE " + whereSQL
		selectArgs = whereArgs
	default:
		selectSQL = "SELECT * FROM " + quoteIdent(s.Table)
	}

	rows, err := executor.QueryContext(ctx, selectSQL, selectArgs...)
	if err != nil {
		return nil, fmt.Errorf("mysql: update: read back: %w", mapError(err))
	}
	defer rows.Close()

	results, err := collectRows(rows)
	if err != nil {
		return nil, fmt.Errorf("mysql: update: read back: %w", err)
	}
	return results, nil
}

// Query executes a raw compiled SELECT query statement.
func (d *dialect) Query(ctx context.Context, conn golem.Conn, sql string, args []any) ([]map[string]any, error) {
	rows, err := d.getExecutor(conn).QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	return collectRows(rows)
}

// Exec executes a raw compiled command query (command executing, no rows returned).
func (d *dialect) Exec(ctx context.Context, conn golem.Conn, sql string, args []any) (int64, error) {
	result, err := d.getExecutor(conn).ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, mapError(err)
	}
	return result.RowsAffected()
}

// ExecRaw executes a raw SQL statement, returning the list of returned rows (if any) and rows affected count.
func (d *dialect) ExecRaw(ctx context.Context, conn golem.Conn, sql string, args []any) ([]map[string]any, int64, error) {
	executor := d.getExecutor(conn)
	rows, err := executor.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, 0, mapError(err)
	}
	defer rows.Close()

	results, err := collectRows(rows)
	if err != nil {
		return nil, 0, err
	}

	// database/sql's *sql.Rows has no CommandTag-equivalent RowsAffected —
	// that's only available from sql.Result (ExecContext), not
	// QueryContext. For a raw statement that might be either a SELECT (rows,
	// no meaningful affected-count) or a write issued via Query (unusual but
	// not prevented), len(results) is the best available affected-count
	// signal when rows are present.
	return results, int64(len(results)), nil
}

// collectRows scans every row from rows into a column-name-keyed map,
// mirroring driver/postgres's pgx.RowToMap via database/sql's generic
// (*sql.Rows).Scan([]any) into freshly-allocated any destinations, then
// normalizes each value per its reported SQL column type (see normalizeCell).
func collectRows(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	// ColumnTypes() fails for the same reasons Columns() would (e.g. rows
	// already closed) — since Columns() above already returned successfully,
	// this is defensive and not independently reachable via any real driver
	// behavior; same accepted-unreachable class as the Scan() check below.
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	dbTypes := make([]string, len(colTypes))
	for i, ct := range colTypes {
		dbTypes[i] = ct.DatabaseTypeName()
	}

	var results []map[string]any
	for rows.Next() {
		dest := make([]any, len(cols))
		destPtrs := make([]any, len(cols))
		for i := range dest {
			destPtrs[i] = &dest[i]
		}
		// Scanning into *any never actually fails in practice (any driver
		// value assigns into an interface{} pointer; destPtrs always has
		// exactly len(cols) entries) — kept as a defensive check anyway,
		// same class of accepted-unreachable branch as compilePredicate's
		// default case (driver/postgres) and Numeric.Float64Value()'s error
		// path (AD-037).
		if err := rows.Scan(destPtrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = normalizeCell(dbTypes[i], dest[i])
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// normalizeCell converts a raw []byte value that go-sql-driver/mysql
// returns as text for certain SQL column types into the plain Go type
// golem's Dialect boundary expects, so pgx-style leaks (AD-037's original
// finding, for Postgres) don't recur here in a MySQL-specific form.
//
// Unlike driver/postgres's normalizeRow, this needs the column's reported
// SQL type name (dbType, from *sql.ColumnType.DatabaseTypeName()) — many
// different MySQL column types (DECIMAL, DATE, TIME, VARCHAR, BLOB, JSON,
// ...) can all come back as the exact same Go type ([]byte), so the Go
// type alone is ambiguous, unlike pgx's distinct pgtype.Numeric/pgtype.Time
// wrapper types.
//
// Only DECIMAL and TIME need this: ParseTime=true (always set by
// resolveDSN) already makes DATE/DATETIME/TIMESTAMP columns come back as
// native time.Time — but go-sql-driver deliberately never does this for
// TIME, since MySQL's TIME range (-838:59:59 to 838:59:59) doesn't fit
// time.Time's time-of-day model.
func normalizeCell(dbType string, raw any) any {
	b, ok := raw.([]byte)
	if !ok {
		return raw
	}
	switch dbType {
	case "DECIMAL":
		if f, err := strconv.ParseFloat(string(b), 64); err == nil {
			return f
		}
	case "TIME":
		if t, err := parseMySQLTime(string(b)); err == nil {
			return t
		}
	}
	return raw
}

// parseMySQLTime parses MySQL's TIME text representation ("HH:MM:SS" or
// "HH:MM:SS.ffffff") into a time.Time on a fixed reference date (2000-01-01)
// — matching golem's "time" ColumnType kind, which is always represented as
// time.Time (see Bind's "date"/"datetime"/"time" case). MySQL's full TIME
// range (durations beyond 24h, negative values) isn't representable this
// way; not handled here — no conformance case exercises it.
func parseMySQLTime(s string) (time.Time, error) {
	layout := "15:04:05"
	if strings.Contains(s, ".") {
		layout = "15:04:05.999999"
	}
	parsed, err := time.Parse(layout, s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(2000, 1, 1, parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), time.UTC), nil
}

// mapError wraps err with the matching golem sentinel (ErrDuplicateKey,
// ErrForeignKeyViolation) when it's a recognized MySQL error number,
// keeping the original *mysql.MySQLError reachable via errors.As/errors.Unwrap.
// Unmapped errors (including non-MySQL ones) pass through unchanged.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) {
		switch myErr.Number {
		case 1062: // ER_DUP_ENTRY
			return fmt.Errorf("%w: %w", golem.ErrDuplicateKey, err)
		case 1451, 1452: // ER_ROW_IS_REFERENCED_2, ER_NO_REFERENCED_ROW_2
			return fmt.Errorf("%w: %w", golem.ErrForeignKeyViolation, err)
		}
	}
	return err
}

// IsConflict returns true if the error represents a database integrity constraint violation.
func (d *dialect) IsConflict(err error) bool {
	if err == nil {
		return false
	}
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) {
		switch myErr.Number {
		case 1062, 1451, 1452:
			return true
		}
	}
	return false
}

// Begin starts a new transaction on the database.
func (d *dialect) Begin(ctx context.Context, conn golem.Conn) (golem.TxConn, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mysql: begin: %w", err)
	}
	return &mysqlTx{tx: tx}, nil
}

type mysqlExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (d *dialect) getExecutor(conn golem.Conn) mysqlExecutor {
	if tx, ok := conn.(golem.Tx); ok {
		if mtx, ok := tx.Underlying().(*mysqlTx); ok {
			return mtx.tx
		}
	}
	return d.db
}

type mysqlTx struct {
	tx *sql.Tx
}

func (t *mysqlTx) Commit(ctx context.Context) error {
	return t.tx.Commit()
}

func (t *mysqlTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback()
}
