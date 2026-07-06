package oracle

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

	goOra "github.com/sijms/go-ora/v2"
	goraNetwork "github.com/sijms/go-ora/v2/network"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/must"
	"github.com/leandroluk/golem/internal/sqlutil"
	"github.com/leandroluk/golem/internal/stmt"
)

// dialect is the Oracle implementation of golem.Dialect.
type dialect struct {
	db sqlIface
}

var _ golem.Dialect = (*dialect)(nil)

// Bind converts a Go value to a driver.Value suitable for Oracle, according
// to the declared ColumnType. "uuid" binds a plain string (VARCHAR2(36) —
// design decision, not RAW(16); see design.md).
func (dialect) Bind(t golem.ColumnType, value any) (driver.Value, error) {
	switch t.Kind() {
	case "boolean":
		switch v := value.(type) {
		case bool:
			return v, nil
		case int, int8, int16, int32, int64:
			return fmt.Sprintf("%v", v) != "0", nil
		}
		return nil, fmt.Errorf("oracle: bind boolean: unsupported Go type %T", value)

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
		return nil, fmt.Errorf("oracle: bind %s: unsupported Go type %T", t.Kind(), value)

	case "decimal", "float":
		switch v := value.(type) {
		case float32:
			return float64(v), nil
		case float64:
			return v, nil
		}
		return nil, fmt.Errorf("oracle: bind %s: unsupported Go type %T", t.Kind(), value)

	case "char", "varchar", "text":
		switch v := value.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		}
		return nil, fmt.Errorf("oracle: bind %s: unsupported Go type %T", t.Kind(), value)

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
		return nil, fmt.Errorf("oracle: bind %s: unsupported Go type %T", t.Kind(), value)

	case "blob":
		switch v := value.(type) {
		case []byte:
			return v, nil
		case string:
			return []byte(v), nil
		}
		return nil, fmt.Errorf("oracle: bind blob: unsupported Go type %T", value)

	case "uuid":
		switch v := value.(type) {
		case string:
			return v, nil
		case [16]byte:
			return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
				v[0:4], v[4:6], v[6:8], v[8:10], v[10:16]), nil
		}
		return nil, fmt.Errorf("oracle: bind uuid: unsupported Go type %T", value)

	case "json":
		switch v := value.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		}
		return nil, fmt.Errorf("oracle: bind json: unsupported Go type %T", value)
	}

	return nil, fmt.Errorf("oracle: bind: unrecognized column kind %q", t.Kind())
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
			return fmt.Errorf("oracle: scan boolean: dest must be *bool, got %T", dest)
		}
		switch v := raw.(type) {
		case bool:
			*d = v
		case int64:
			*d = v != 0
		default:
			return fmt.Errorf("oracle: scan boolean: unsupported raw type %T", raw)
		}

	case "smallint", "integer", "bigint":
		d, ok := dest.(*int64)
		if !ok {
			return fmt.Errorf("oracle: scan %s: dest must be *int64, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case int64:
			*d = v
		default:
			return fmt.Errorf("oracle: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "decimal", "float":
		d, ok := dest.(*float64)
		if !ok {
			return fmt.Errorf("oracle: scan %s: dest must be *float64, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case float64:
			*d = v
		default:
			return fmt.Errorf("oracle: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "char", "varchar", "text":
		d, ok := dest.(*string)
		if !ok {
			return fmt.Errorf("oracle: scan %s: dest must be *string, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case string:
			*d = v
		case []byte:
			*d = string(v)
		default:
			return fmt.Errorf("oracle: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "date", "datetime", "time":
		d, ok := dest.(*time.Time)
		if !ok {
			return fmt.Errorf("oracle: scan %s: dest must be *time.Time, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case time.Time:
			*d = v
		default:
			return fmt.Errorf("oracle: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "blob":
		d, ok := dest.(*[]byte)
		if !ok {
			return fmt.Errorf("oracle: scan blob: dest must be *[]byte, got %T", dest)
		}
		switch v := raw.(type) {
		case []byte:
			*d = v
		case string:
			*d = []byte(v)
		default:
			return fmt.Errorf("oracle: scan blob: unsupported raw type %T", raw)
		}

	case "uuid":
		d, ok := dest.(*string)
		if !ok {
			return fmt.Errorf("oracle: scan uuid: dest must be *string, got %T", dest)
		}
		switch v := raw.(type) {
		case string:
			*d = v
		case []byte:
			*d = string(v)
		default:
			return fmt.Errorf("oracle: scan uuid: unsupported raw type %T", raw)
		}

	case "json":
		d, ok := dest.(*string)
		if !ok {
			return fmt.Errorf("oracle: scan json: dest must be *string, got %T", dest)
		}
		switch v := raw.(type) {
		case string:
			*d = v
		case []byte:
			*d = string(v)
		default:
			return fmt.Errorf("oracle: scan json: unsupported raw type %T", raw)
		}

	default:
		return fmt.Errorf("oracle: scan: unrecognized column kind %q", t.Kind())
	}

	return nil
}

// quoteIdent double-quote-quotes a SQL identifier, preserving table
// prefixes — Oracle's ANSI-standard identifier quoting.
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
// cast to FLOAT so the driver always decodes them consistently, mirroring
// every other adapter's same-purpose cast.
func aggregateSQLFunc(fn, column string) string {
	switch fn {
	case "count_all":
		return "COUNT(*)"
	case "count":
		return fmt.Sprintf("COUNT(%s)", quoteIdent(column))
	case "sum", "avg":
		return fmt.Sprintf("CAST(%s(%s) AS FLOAT)", strings.ToUpper(fn), quoteIdent(column))
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

// compilePredicate recursively builds the Oracle WHERE clause from a
// stmt.Predicate. Uses ":N"-numbered placeholders (confirmed via design.md's
// probe — go-ora's driver expects colon-prefixed numbered binds), same
// numbered-placeholder shape as driver/postgres's $N/driver/mssql's @pN.
func compilePredicate(p stmt.Predicate, argOffset *int, args *[]any) (string, error) {
	if p == nil {
		return "", nil
	}

	switch v := p.(type) {
	case stmt.Comparison:
		switch v.Op {
		case "eq":
			sql := fmt.Sprintf("%s=:%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "gt":
			sql := fmt.Sprintf("%s>:%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "gte":
			sql := fmt.Sprintf("%s>=:%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "lt":
			sql := fmt.Sprintf("%s<:%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "lte":
			sql := fmt.Sprintf("%s<=:%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "like":
			sql := fmt.Sprintf("%s LIKE :%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "is_null":
			return fmt.Sprintf("%s IS NULL", quoteIdent(v.Column)), nil
		case "in":
			valVal := reflect.ValueOf(v.Value)
			if valVal.Kind() != reflect.Slice {
				sql := fmt.Sprintf("%s IN (:%d)", quoteIdent(v.Column), *argOffset)
				*args = append(*args, v.Value)
				*argOffset++
				return sql, nil
			}
			n := valVal.Len()
			if n == 0 {
				// Oracle (pre-23c, this adapter's floor is 12c) has no SQL
				// boolean literal — "1=0" is the universal always-false
				// idiom, same choice driver/mssql made.
				return "1=0", nil
			}
			placeholders := make([]string, n)
			for i := 0; i < n; i++ {
				placeholders[i] = fmt.Sprintf(":%d", *argOffset)
				*args = append(*args, valVal.Index(i).Interface())
				*argOffset++
			}
			return fmt.Sprintf("%s IN (%s)", quoteIdent(v.Column), strings.Join(placeholders, ",")), nil
		}
		return "", fmt.Errorf("oracle: unsupported comparison operator %q", v.Op)

	case stmt.Logical:
		if len(v.Predicates) == 0 {
			return "", nil
		}
		parts := make([]string, 0, len(v.Predicates))
		for _, pred := range v.Predicates {
			part, err := compilePredicate(pred, argOffset, args)
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
		part, err := compilePredicate(v.Predicate, argOffset, args)
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
			sql := fmt.Sprintf("%s=:%d", expr, *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "gt":
			sql := fmt.Sprintf("%s>:%d", expr, *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "gte":
			sql := fmt.Sprintf("%s>=:%d", expr, *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "lt":
			sql := fmt.Sprintf("%s<:%d", expr, *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "lte":
			sql := fmt.Sprintf("%s<=:%d", expr, *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		}
		return "", fmt.Errorf("oracle: unsupported having comparison operator %q", v.Op)

	default:
		return "", fmt.Errorf("oracle: unsupported predicate type %T", p)
	}
}

// CompileSelect compiles a Select statement plan to Oracle SQL and args.
//
// Unlike driver/mssql, Oracle's OFFSET/FETCH pagination does NOT require an
// accompanying ORDER BY (confirmed via design.md's probe) — s.PrimaryKey is
// never read here as a result; ORDER BY is only emitted when the caller
// explicitly set s.OrderBy. For a Count query, ORDER BY is never emitted at
// all (even if s.OrderBy were set) — confirmed via probe that Oracle
// rejects ORDER BY on any expression (a real column OR the MSSQL-style
// "ORDER BY (SELECT NULL)" idiom) against a bare COUNT(*) with no GROUP BY
// ("ORA-00937: not a single-group group function"); omitting it entirely is
// the actual fix, not a workaround, since OFFSET/FETCH doesn't need it here.
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
	argOffset := 1

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
			whereSQL, err := compilePredicate(j.Where, &argOffset, &args)
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
		sql, err := compilePredicate(s.Where, &argOffset, &args)
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
		sql, err := compilePredicate(s.Having, &argOffset, &args)
		if err != nil {
			return "", nil, err
		}
		if sql != "" {
			sb.WriteString(" HAVING ")
			sb.WriteString(sql)
		}
	}

	if !s.Count && len(s.OrderBy) > 0 {
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

	if s.Limit != nil || s.Offset != nil {
		offsetVal := 0
		if s.Offset != nil {
			offsetVal = *s.Offset
		}
		sb.WriteString(fmt.Sprintf(" OFFSET :%d ROWS", argOffset))
		args = append(args, offsetVal)
		argOffset++
		if s.Limit != nil {
			sb.WriteString(fmt.Sprintf(" FETCH NEXT :%d ROWS ONLY", argOffset))
			args = append(args, *s.Limit)
			argOffset++
		}
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

// lockClauseSQL builds the trailing `FOR UPDATE [NOWAIT|SKIP LOCKED]`
// row-locking clause (confirmed via design.md's probe) — same trailing-
// clause shape as Postgres, not a table hint like driver/mssql. Oracle has
// no FOR SHARE clause at all (a permanent SQL-dialect gap, not a version
// gate — Oracle's only row-locking read clause has ever been FOR UPDATE),
// and no NO KEY UPDATE/KEY SHARE equivalent (Postgres-specific concepts) —
// all three fall into the default case below and error.
func lockClauseSQL(l *stmt.LockClause) (string, error) {
	if l.Strength != "update" {
		return "", fmt.Errorf("oracle: unsupported lock strength %q", l.Strength)
	}

	sql := " FOR UPDATE"

	switch l.Wait {
	case "":
		// block (Oracle default) — no suffix
	case "nowait":
		sql += " NOWAIT"
	case "skip_locked":
		sql += " SKIP LOCKED"
	default:
		return "", fmt.Errorf("oracle: unsupported lock wait policy %q", l.Wait)
	}

	return sql, nil
}

// CompileDelete compiles a Delete statement plan to Oracle SQL and args.
func (d *dialect) CompileDelete(s *stmt.Delete) (string, []any, error) {
	var sb strings.Builder
	sb.WriteString("DELETE FROM ")
	sb.WriteString(quoteIdent(s.Table))

	var args []any
	argOffset := 1

	if s.Where != nil {
		sql, err := compilePredicate(s.Where, &argOffset, &args)
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

// indexOf returns the index of s in ss, or -1 if not found.
func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

// Insert executes an INSERT statement against Oracle. Unlike Postgres/
// SQLite's RETURNING */driver/mssql's OUTPUT INSERTED.*, Oracle's
// RETURNING ... INTO needs one pre-typed out-bind per returned column —
// not viable to build generically for "return every column" (design.md's
// correction). Instead, this is shaped like driver/mysql's Insert: a plain
// INSERT, then a follow-up SELECT * WHERE <primary key> to read the row
// back, using stmt.Insert.PrimaryKey (M16/AD-038) for exactly this
// purpose. The one Oracle-specific piece: when an auto-generated primary
// key column is NOT present in s.Columns (relies on GENERATED ALWAYS AS
// IDENTITY rather than a client-assigned key), sql.Result.LastInsertId()
// is silently unsupported (confirmed via design.md's probe: always
// returns (0, nil)) — RETURNING <pk> INTO :N (via goOra.Out{Dest:
// &generatedID}, executed through ExecContext) is the only way to recover
// the generated value in one round-trip.
func (d *dialect) Insert(ctx context.Context, conn golem.Conn, s *stmt.Insert) (map[string]any, error) {
	executor := d.getExecutor(conn)

	quotedCols := make([]string, len(s.Columns))
	placeholders := make([]string, len(s.Columns))
	args := make([]any, len(s.Values))
	for i, c := range s.Columns {
		quotedCols[i] = quoteIdent(c)
		placeholders[i] = fmt.Sprintf(":%d", i+1)
		args[i] = s.Values[i]
	}
	argOffset := len(s.Columns) + 1

	var sb strings.Builder
	fmt.Fprintf(&sb, "INSERT INTO %s (%s) VALUES (%s)",
		quoteIdent(s.Table), strings.Join(quotedCols, ","), strings.Join(placeholders, ","))

	type generatedPK struct {
		column string
		dest   *int64
	}
	var generated []generatedPK

	whereClauses := make([]string, 0, len(s.PrimaryKey))
	whereArgs := make([]any, 0, len(s.PrimaryKey))
	for _, pk := range s.PrimaryKey {
		if idx := indexOf(s.Columns, pk); idx >= 0 {
			whereClauses = append(whereClauses, pk)
			whereArgs = append(whereArgs, s.Values[idx])
			continue
		}
		generated = append(generated, generatedPK{column: pk, dest: new(int64)})
	}

	if len(generated) > 0 {
		returningCols := make([]string, len(generated))
		returningPlaceholders := make([]string, len(generated))
		for i, g := range generated {
			returningCols[i] = quoteIdent(g.column)
			returningPlaceholders[i] = fmt.Sprintf(":%d", argOffset)
			args = append(args, goOra.Out{Dest: g.dest})
			argOffset++
		}
		fmt.Fprintf(&sb, " RETURNING %s INTO %s",
			strings.Join(returningCols, ","), strings.Join(returningPlaceholders, ","))
	}

	if _, err := executor.ExecContext(ctx, sb.String(), args...); err != nil {
		return nil, fmt.Errorf("oracle: insert: %w", mapError(err))
	}

	for _, g := range generated {
		whereClauses = append(whereClauses, g.column)
		whereArgs = append(whereArgs, *g.dest)
	}

	if len(whereClauses) == 0 {
		return nil, fmt.Errorf("oracle: insert: table %q has no primary key, cannot read back the inserted row", s.Table)
	}

	selectArgOffset := 1
	selectConds := make([]string, len(whereClauses))
	for i, col := range whereClauses {
		selectConds[i] = fmt.Sprintf("%s=:%d", quoteIdent(col), selectArgOffset)
		selectArgOffset++
	}
	selectSQL := fmt.Sprintf("SELECT * FROM %s WHERE %s", quoteIdent(s.Table), strings.Join(selectConds, " AND "))

	rows, err := executor.QueryContext(ctx, selectSQL, whereArgs...)
	if err != nil {
		return nil, fmt.Errorf("oracle: insert: read back: %w", mapError(err))
	}
	defer rows.Close()

	results, err := collectRows(rows)
	if err != nil {
		return nil, fmt.Errorf("oracle: insert: read back: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("oracle: insert: read back: no row found after insert")
	}
	return results[0], nil
}

// Update executes an UPDATE statement, then reads every affected row back
// via a follow-up SELECT — same multi-round-trip shape and reasoning as
// driver/mysql's Update: primary keys matching s.Where are captured BEFORE
// running the UPDATE, since re-running the original WHERE afterward can't
// work when Sets modifies a column Where itself filters on. This is also
// what SaveOne/SaveMany need (they route through this same method — Oracle
// has no upsert semantics golem implements either, see design.md's
// Correction section).
//
// Unlike driver/mysql's unnumbered "?", Oracle's ":N" placeholders are
// numbered per-statement, so whereSQL (compiled once against the SET
// clauses' own running argOffset) can't be reused verbatim in the two
// standalone follow-up statements below — each recompiles s.Where with its
// own fresh argOffset starting at 1. Recompiling the exact same (already
// proven compilable) predicate value cannot fail a second time, so those 2
// error checks are provably unreachable — extracted via internal/must so
// they're exercised through the deferred must.Recover instead of sitting
// at a permanent 0%, same established pattern as every adapter's
// collectRows (STATE.md AD-043).
func (d *dialect) Update(ctx context.Context, conn golem.Conn, s *stmt.Update) (results []map[string]any, err error) {
	defer must.Recover(&err)

	setClauses := make([]string, 0, len(s.Sets))
	args := make([]any, 0, len(s.Sets))
	argOffset := 1
	for _, set := range s.Sets {
		setClauses = append(setClauses, fmt.Sprintf("%s=:%d", quoteIdent(set.Column), argOffset))
		args = append(args, set.Value)
		argOffset++
	}

	var sb strings.Builder
	sb.WriteString("UPDATE ")
	sb.WriteString(quoteIdent(s.Table))
	sb.WriteString(" SET ")
	sb.WriteString(strings.Join(setClauses, ","))

	var whereArgs []any
	whereSQL := ""
	if s.Where != nil {
		sql, err := compilePredicate(s.Where, &argOffset, &whereArgs)
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
		captureArgOffset := 1
		var captureArgs []any
		captureWhereSQL := must.Value(compilePredicate(s.Where, &captureArgOffset, &captureArgs))
		selectPKSQL := fmt.Sprintf("SELECT %s FROM %s WHERE %s", strings.Join(quotedPKs, ","), quoteIdent(s.Table), captureWhereSQL)
		rows, err := executor.QueryContext(ctx, selectPKSQL, captureArgs...)
		if err != nil {
			return nil, fmt.Errorf("oracle: update: capture primary keys: %w", mapError(err))
		}
		pkRows, err = collectRows(rows)
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("oracle: update: capture primary keys: %w", err)
		}
	}

	if _, err := executor.ExecContext(ctx, sb.String(), args...); err != nil {
		return nil, fmt.Errorf("oracle: update: %w", mapError(err))
	}

	var selectSQL string
	var selectArgs []any
	switch {
	case usePKReadBack && len(pkRows) == 0:
		return []map[string]any{}, nil
	case usePKReadBack:
		selectArgOffset := 1
		orClauses := make([]string, len(pkRows))
		for i, row := range pkRows {
			andClauses := make([]string, len(s.PrimaryKey))
			for j, pk := range s.PrimaryKey {
				andClauses[j] = fmt.Sprintf("%s=:%d", quoteIdent(pk), selectArgOffset)
				selectArgs = append(selectArgs, row[pk])
				selectArgOffset++
			}
			orClauses[i] = "(" + strings.Join(andClauses, " AND ") + ")"
		}
		selectSQL = "SELECT * FROM " + quoteIdent(s.Table) + " WHERE " + strings.Join(orClauses, " OR ")
	case whereSQL != "":
		captureArgOffset := 1
		var captureArgs []any
		captureWhereSQL := must.Value(compilePredicate(s.Where, &captureArgOffset, &captureArgs))
		selectSQL = "SELECT * FROM " + quoteIdent(s.Table) + " WHERE " + captureWhereSQL
		selectArgs = captureArgs
	default:
		selectSQL = "SELECT * FROM " + quoteIdent(s.Table)
	}

	rows, err := executor.QueryContext(ctx, selectSQL, selectArgs...)
	if err != nil {
		return nil, fmt.Errorf("oracle: update: read back: %w", mapError(err))
	}
	defer rows.Close()

	results, err = collectRows(rows)
	if err != nil {
		return nil, fmt.Errorf("oracle: update: read back: %w", err)
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

// collectRows scans every row from rows into a column-name-keyed map, then
// normalizes each value per its reported SQL column type (see
// normalizeCell). Extracted via internal/must so the accepted-unreachable
// defensive checks are exercised through the deferred must.Recover, same
// pattern as every other adapter's collectRows (STATE.md AD-043).
func collectRows(rows *sql.Rows) (results []map[string]any, err error) {
	defer must.Recover(&err)

	cols := must.Value(rows.Columns())
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

// normalizeCell converts raw values whose driver-level representation is
// ambiguous into their proper Go type. Confirmed via a real-server probe
// (design.md): every numeric golem.ColumnType kind maps to a NUMBER
// variant in Oracle — there is no separate INTEGER/SMALLINT/BIGINT SQL
// type — and DatabaseTypeName() reports the literal string "NUMBER" for
// all of them, with a generic Scan(&any) always returning a Go string
// (e.g. "123", "19.99"), never a native int64/float64.
//
// sql.ColumnType.DecimalSize()'s scale looked like the obvious
// disambiguating signal at first, but isn't reliable: a bare COUNT(*)
// expression reports the exact same "unconstrained NUMBER" sentinel scale
// (confirmed via probe: precision=38, scale=255) as a genuine FLOAT
// column, yet COUNT(*)'s value is always integral and
// repository.Count()'s row-parsing only accepts int64/int32/int, not
// float64 — using scale alone mis-parsed COUNT(*) into a float64 and
// broke SoftDelete's Count call. The actual fix: try strconv.ParseInt
// first regardless of scale; only fall back to strconv.ParseFloat when
// the string isn't a plain integer (i.e., it has a fractional part, which
// a genuinely-scaled NUMBER(p,s) column's string form always shows, since
// Oracle pads to the declared scale — confirmed empirically that
// NUMBER(10,2) renders as "19.99", never "19.99000" or bare "19"). A
// float-kind column whose value happens to be a whole number (e.g. a SUM
// that nets to an exact integer) parses as int64 here, same as COUNT(*) —
// harmless, since repository.assignFieldValue's reflect-based
// ConvertibleTo fallback already converts int64 into a float64 struct
// field without issue (same mechanism M16/AD-039 relies on for
// numeric-to-bool).
//
// Every other declared type (VARCHAR2/CHAR/CLOB as string, DATE/TIMESTAMP
// as time.Time, BLOB/RAW as []byte) already scans as the correct Go type
// natively and is left untouched.
func normalizeCell(dbType string, raw any) any {
	if dbType != "NUMBER" {
		return raw
	}
	s, ok := raw.(string)
	if !ok {
		return raw
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return raw
}

// mapError wraps err with the matching golem sentinel (ErrDuplicateKey,
// ErrForeignKeyViolation) when it's a recognized Oracle error code, keeping
// the original *network.OracleError reachable via errors.As/errors.Unwrap.
// Unmapped errors (including non-Oracle ones) pass through unchanged.
// Unlike driver/mssql's single 547 covering both FK directions, Oracle uses
// two distinct codes: 2291 (insert/update-side, "parent key not found") and
// 2292 (delete-side, "child record found") — both confirmed via design.md's
// probe and mapped identically to golem.ErrForeignKeyViolation.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	var oraErr *goraNetwork.OracleError
	if errors.As(err, &oraErr) {
		switch oraErr.ErrCode {
		case 1: // ORA-00001: unique constraint violated
			return fmt.Errorf("%w: %w", golem.ErrDuplicateKey, err)
		case 2291, 2292: // ORA-02291/ORA-02292: integrity constraint violated
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
	var oraErr *goraNetwork.OracleError
	if errors.As(err, &oraErr) {
		switch oraErr.ErrCode {
		case 1, 2291, 2292:
			return true
		}
	}
	return false
}

// Begin starts a new transaction on the database.
func (d *dialect) Begin(ctx context.Context, conn golem.Conn) (golem.TxConn, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("oracle: begin: %w", err)
	}
	return &oracleTx{tx: tx}, nil
}

type oracleExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (d *dialect) getExecutor(conn golem.Conn) oracleExecutor {
	if tx, ok := conn.(golem.Tx); ok {
		if otx, ok := tx.Underlying().(*oracleTx); ok {
			return otx.tx
		}
	}
	return d.db
}

type oracleTx struct {
	tx *sql.Tx
}

func (t *oracleTx) Commit(ctx context.Context) error {
	return t.tx.Commit()
}

func (t *oracleTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback()
}
