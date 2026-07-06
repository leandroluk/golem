package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	sqlitelib "modernc.org/sqlite"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/must"
	"github.com/leandroluk/golem/internal/sqlutil"
	"github.com/leandroluk/golem/internal/stmt"
)

// dialect is the SQLite implementation of golem.Dialect.
type dialect struct {
	db sqlIface
}

var _ golem.Dialect = (*dialect)(nil)

// sqliteTimeFormatLayout matches modernc.org/sqlite's own "_time_format=
// sqlite" serialization (forced by resolveDSN) — the layout the driver
// uses when binding a raw time.Time arg, and the layout normalizeCell
// parses back for the one declared type (TIME) the driver doesn't already
// auto-convert to time.Time on Scan (see normalizeCell's doc comment).
const sqliteTimeFormatLayout = "2006-01-02 15:04:05.999999999-07:00"

// Bind converts a Go value to a driver.Value suitable for SQLite, according
// to the declared ColumnType. Unlike an earlier version of this file,
// date/datetime/time values are NOT formatted to text here — repository
// never actually calls Bind (it's dead code, same as every other adapter,
// see AD-037/M15), so the real serialization path is resolveDSN's forced
// _time_format=sqlite DSN option acting on the raw time.Time value
// repository hands the driver directly. Bind/Scan mirror driver/postgres's
// simple time.Time passthrough for contract consistency.
func (dialect) Bind(t golem.ColumnType, value any) (driver.Value, error) {
	switch t.Kind() {
	case "boolean":
		switch v := value.(type) {
		case bool:
			return v, nil
		case int, int8, int16, int32, int64:
			return fmt.Sprintf("%v", v) != "0", nil
		}
		return nil, fmt.Errorf("sqlite: bind boolean: unsupported Go type %T", value)

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
		return nil, fmt.Errorf("sqlite: bind %s: unsupported Go type %T", t.Kind(), value)

	case "decimal", "float":
		switch v := value.(type) {
		case float32:
			return float64(v), nil
		case float64:
			return v, nil
		}
		return nil, fmt.Errorf("sqlite: bind %s: unsupported Go type %T", t.Kind(), value)

	case "char", "varchar", "text":
		switch v := value.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		}
		return nil, fmt.Errorf("sqlite: bind %s: unsupported Go type %T", t.Kind(), value)

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
		return nil, fmt.Errorf("sqlite: bind %s: unsupported Go type %T", t.Kind(), value)

	case "blob":
		switch v := value.(type) {
		case []byte:
			return v, nil
		case string:
			return []byte(v), nil
		}
		return nil, fmt.Errorf("sqlite: bind blob: unsupported Go type %T", value)

	case "uuid":
		switch v := value.(type) {
		case string:
			return v, nil
		case [16]byte:
			return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
				v[0:4], v[4:6], v[6:8], v[8:10], v[10:16]), nil
		}
		return nil, fmt.Errorf("sqlite: bind uuid: unsupported Go type %T", value)

	case "json":
		switch v := value.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		}
		return nil, fmt.Errorf("sqlite: bind json: unsupported Go type %T", value)
	}

	return nil, fmt.Errorf("sqlite: bind: unrecognized column kind %q", t.Kind())
}

// Scan converts a raw value returned by database/sql into dest, according
// to the declared ColumnType.
func (dialect) Scan(t golem.ColumnType, raw any, dest any) error {
	if raw == nil {
		return nil
	}

	switch t.Kind() {
	case "boolean":
		d, ok := dest.(*bool)
		if !ok {
			return fmt.Errorf("sqlite: scan boolean: dest must be *bool, got %T", dest)
		}
		switch v := raw.(type) {
		case bool:
			*d = v
		case int64:
			*d = v != 0
		default:
			return fmt.Errorf("sqlite: scan boolean: unsupported raw type %T", raw)
		}

	case "smallint", "integer", "bigint":
		d, ok := dest.(*int64)
		if !ok {
			return fmt.Errorf("sqlite: scan %s: dest must be *int64, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case int64:
			*d = v
		default:
			return fmt.Errorf("sqlite: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "decimal", "float":
		d, ok := dest.(*float64)
		if !ok {
			return fmt.Errorf("sqlite: scan %s: dest must be *float64, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case float64:
			*d = v
		default:
			return fmt.Errorf("sqlite: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "char", "varchar", "text":
		d, ok := dest.(*string)
		if !ok {
			return fmt.Errorf("sqlite: scan %s: dest must be *string, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case string:
			*d = v
		case []byte:
			*d = string(v)
		default:
			return fmt.Errorf("sqlite: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "date", "datetime", "time":
		d, ok := dest.(*time.Time)
		if !ok {
			return fmt.Errorf("sqlite: scan %s: dest must be *time.Time, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case time.Time:
			*d = v
		default:
			return fmt.Errorf("sqlite: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "blob":
		d, ok := dest.(*[]byte)
		if !ok {
			return fmt.Errorf("sqlite: scan blob: dest must be *[]byte, got %T", dest)
		}
		switch v := raw.(type) {
		case []byte:
			*d = v
		case string:
			*d = []byte(v)
		default:
			return fmt.Errorf("sqlite: scan blob: unsupported raw type %T", raw)
		}

	case "uuid":
		d, ok := dest.(*string)
		if !ok {
			return fmt.Errorf("sqlite: scan uuid: dest must be *string, got %T", dest)
		}
		switch v := raw.(type) {
		case string:
			*d = v
		case []byte:
			*d = string(v)
		default:
			return fmt.Errorf("sqlite: scan uuid: unsupported raw type %T", raw)
		}

	case "json":
		d, ok := dest.(*string)
		if !ok {
			return fmt.Errorf("sqlite: scan json: dest must be *string, got %T", dest)
		}
		switch v := raw.(type) {
		case string:
			*d = v
		case []byte:
			*d = string(v)
		default:
			return fmt.Errorf("sqlite: scan json: unsupported raw type %T", raw)
		}

	default:
		return fmt.Errorf("sqlite: scan: unrecognized column kind %q", t.Kind())
	}

	return nil
}

// quoteIdent double-quotes a SQL identifier, preserving table prefixes —
// SQLite follows the SQL standard here, so this is a straight port of
// driver/postgres's version, not driver/mysql's backtick one.
func quoteIdent(name string) string {
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		quoted := make([]string, len(parts))
		for i, p := range parts {
			quoted[i] = `"` + p + `"`
		}
		return strings.Join(quoted, ".")
	}
	return `"` + name + `"`
}

// aggregateSQLFunc builds a SUM/AVG/COUNT(column) expression. SUM/AVG are
// cast to REAL so the driver always decodes them as float64 consistently
// regardless of the source column's exact numeric affinity, mirroring the
// other two adapters' same-purpose cast.
func aggregateSQLFunc(fn, column string) string {
	switch fn {
	case "count_all":
		return "COUNT(*)"
	case "count":
		return fmt.Sprintf("COUNT(%s)", quoteIdent(column))
	case "sum", "avg":
		return fmt.Sprintf("CAST(%s(%s) AS REAL)", strings.ToUpper(fn), quoteIdent(column))
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

// compilePredicate recursively builds the SQLite SQL WHERE clause from a
// stmt.Predicate. Uses unnumbered "?" placeholders, same as driver/mysql —
// args are still appended in call order, no argOffset counter needed to
// render the placeholder text itself.
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
		return "", fmt.Errorf("sqlite: unsupported comparison operator %q", v.Op)

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
		return "", fmt.Errorf("sqlite: unsupported having comparison operator %q", v.Op)

	default:
		return "", fmt.Errorf("sqlite: unsupported predicate type %T", p)
	}
}

// CompileSelect compiles a Select statement plan to SQLite SQL and args.
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
		// SQLite has no row-level locking clause of any kind — lockClauseSQL
		// always errors for every Strength (see its doc comment), so there's
		// no success value to ever append here.
		if _, err := lockClauseSQL(s.Lock); err != nil {
			return "", nil, err
		}
	}

	return sb.String(), args, nil
}

// lockClauseSQL always errors: SQLite has no row-level locking clause of
// any kind (it locks at the whole-database-file level instead). Every
// Strength value hits this same branch — a deliberate, spec'd outcome
// (spec.md P3 AC7), not a placeholder. This is what enforces
// Capabilities.Locking being entirely false for real: query.Query[T] has
// no dialect awareness of its own, so nothing stops a caller from writing
// .ForUpdate() against a SQLite DataSource — it compiles fine up to here,
// which then errors, same "loud failure over silent no-op" pattern as
// every other unsupported-lock-strength case in this codebase.
func lockClauseSQL(l *stmt.LockClause) (string, error) {
	return "", fmt.Errorf("sqlite: row-level locking is not supported (strength %q)", l.Strength)
}

// CompileDelete compiles a Delete statement plan to SQLite SQL and args.
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

// Insert executes an INSERT statement against SQLite, using RETURNING * for
// a single round-trip — SQLite (3.35+) supports RETURNING natively, unlike
// MySQL, so this mirrors driver/postgres's shape, not driver/mysql's
// multi-round-trip one. s.PrimaryKey is read from stmt.Insert (added for
// M16) but unused here, same as driver/postgres.
func (d *dialect) Insert(ctx context.Context, conn golem.Conn, s *stmt.Insert) (map[string]any, error) {
	quotedCols := make([]string, len(s.Columns))
	placeholders := make([]string, len(s.Columns))
	for i, c := range s.Columns {
		quotedCols[i] = quoteIdent(c)
		placeholders[i] = "?"
	}
	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *",
		quoteIdent(s.Table), strings.Join(quotedCols, ","), strings.Join(placeholders, ","))

	args := make([]any, len(s.Values))
	for i, v := range s.Values {
		args[i] = v
	}

	rows, err := d.getExecutor(conn).QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: insert: %w", mapError(err))
	}
	defer rows.Close()

	results, err := collectRows(rows)
	if err != nil {
		return nil, fmt.Errorf("sqlite: insert: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("sqlite: insert: no row returned")
	}
	return results[0], nil
}

// Update executes an UPDATE statement with RETURNING *, returning all
// updated rows in the same round-trip — no PK-capture-before-write dance
// needed (that's a MySQL/no-RETURNING workaround only).
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

	if s.Where != nil {
		sql, err := compilePredicate(s.Where, &args)
		if err != nil {
			return nil, err
		}
		if sql != "" {
			sb.WriteString(" WHERE ")
			sb.WriteString(sql)
		}
	}

	sb.WriteString(" RETURNING *")

	rows, err := d.getExecutor(conn).QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: update: %w", mapError(err))
	}
	defer rows.Close()

	results, err := collectRows(rows)
	if err != nil {
		return nil, fmt.Errorf("sqlite: update: %w", err)
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

	// See driver/mysql's identical comment: QueryContext can't report a
	// real affected-row-count for a write statement (no result set), so a
	// write is run via ExecContext instead — confirmed necessary via a
	// real .examples test failure (golem.DataSource.Exec("UPDATE ...")
	// always reported 0 rows affected).
	if !sqlutil.IsRowReturning(sql) {
		result, err := executor.ExecContext(ctx, sql, args...)
		if err != nil {
			return nil, 0, mapError(err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, 0, err
		}
		return nil, affected, nil
	}

	rows, err := executor.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, 0, mapError(err)
	}
	defer rows.Close()

	results, err := collectRows(rows)
	if err != nil {
		return nil, 0, err
	}

	return results, int64(len(results)), nil
}

// collectRows scans every row from rows into a column-name-keyed map.
// repository never calls golem.Dialect.Scan (it's dead code, same as every
// other adapter — see AD-037/M15) — it reads whatever Go value comes back
// in this map directly. modernc.org/sqlite already auto-converts DATE/
// DATETIME-declared TEXT columns back to time.Time on a generic Scan(&any)
// (confirmed empirically), but NOT TIME-declared columns — those still
// arrive as the raw "_time_format=sqlite"-formatted string. normalizeCell
// fixes up that one remaining case, the same declared-SQL-type-name
// disambiguation mechanism (via rows.ColumnTypes()) driver/mysql's
// normalizeCell uses for its own (different) ambiguous cases.
func collectRows(rows *sql.Rows) (results []map[string]any, err error) {
	defer must.Recover(&err)
	cols := must.Value(rows.Columns())
	// ColumnTypes() fails for the same reasons Columns() would (e.g. rows
	// already closed) — since Columns() above already returned successfully,
	// this is defensive and not independently reachable via any real driver
	// behavior; same accepted-unreachable class as the Scan() check below.
	colTypes := must.Value(rows.ColumnTypes())
	dbTypes := make([]string, len(colTypes))
	for i, ct := range colTypes {
		dbTypes[i] = ct.DatabaseTypeName()
	}

	results = make([]map[string]any, 0)
	for rows.Next() {
		dest := make([]any, len(cols))
		destPtrs := make([]any, len(cols))
		for i := range dest {
			destPtrs[i] = &dest[i]
		}
		// Scanning into *any never actually fails in practice (any driver
		// value assigns into an interface{} pointer) — kept as a defensive
		// check anyway, same accepted-unreachable class as
		// driver/mysql/driver/postgres's equivalent checks.
		must.Exec(rows.Scan(destPtrs...))
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = normalizeCell(dbTypes[i], dest[i])
		}
		results = append(results, row)
	}
	must.Exec(rows.Err())
	return results, nil
}

// normalizeCell parses a TIME-declared column's raw string (or []byte)
// value — the modernc.org/sqlite driver's own "_time_format=sqlite"
// serialization (see resolveDSN) — back into a time.Time. Every other
// declared type (or a parse failure, which shouldn't happen for a value
// this driver itself wrote) is left untouched.
func normalizeCell(dbType string, raw any) any {
	if dbType != "TIME" {
		return raw
	}

	var s string
	switch v := raw.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return raw
	}

	if t, err := time.Parse(sqliteTimeFormatLayout, s); err == nil {
		return t
	}
	return raw
}

// mapError wraps err with the matching golem sentinel (ErrDuplicateKey,
// ErrForeignKeyViolation) when it's a recognized SQLite constraint
// violation, keeping the original *sqlite.Error reachable via
// errors.As/errors.Unwrap. Unmapped errors (including non-SQLite ones) pass
// through unchanged.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	var sqliteErr *sqlitelib.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case sqliteConstraintUnique, sqliteConstraintPrimaryKey:
			return fmt.Errorf("%w: %w", golem.ErrDuplicateKey, err)
		case sqliteConstraintForeignKey:
			return fmt.Errorf("%w: %w", golem.ErrForeignKeyViolation, err)
		}
	}
	return err
}

// SQLite extended result codes for constraint violations (sqlite.org's
// result-code list) — confirmed via Context7 (pkg.go.dev/modernc.org/sqlite/lib).
// An extended code's low byte (code & 0xFF) always equals its primary
// result code (19 = SQLITE_CONSTRAINT), which IsConflict uses as a
// Postgres-style "whole class" catch-all.
const (
	sqliteConstraintPrimary    = 19
	sqliteConstraintUnique     = 2067
	sqliteConstraintPrimaryKey = 1555
	sqliteConstraintForeignKey = 787
)

// IsConflict returns true if the error represents a database integrity constraint violation.
func (d *dialect) IsConflict(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *sqlitelib.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code()&0xFF == sqliteConstraintPrimary
	}
	return false
}

// Begin starts a new transaction on the database.
func (d *dialect) Begin(ctx context.Context, conn golem.Conn) (golem.TxConn, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("sqlite: begin: %w", err)
	}
	return &sqliteTx{tx: tx}, nil
}

type sqliteExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (d *dialect) getExecutor(conn golem.Conn) sqliteExecutor {
	if tx, ok := conn.(golem.Tx); ok {
		if stx, ok := tx.Underlying().(*sqliteTx); ok {
			return stx.tx
		}
	}
	return d.db
}

type sqliteTx struct {
	tx *sql.Tx
}

func (t *sqliteTx) Commit(ctx context.Context) error {
	return t.tx.Commit()
}

func (t *sqliteTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback()
}
