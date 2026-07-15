package mssql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	mssqllib "github.com/microsoft/go-mssqldb"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/must"
	"github.com/leandroluk/golem/internal/sqlutil"
	"github.com/leandroluk/golem/internal/stmt"
)

// dialect is the SQL Server implementation of golem.Dialect.
type dialect struct {
	db sqlIface
}

var _ golem.Dialect = (*dialect)(nil)

// Bind converts a Go value to a driver.Value suitable for SQL Server,
// according to the declared ColumnType. Confirmed via a real SQL Server
// probe (design.md): BIT/DATETIME2 round-trip natively as bool/time.Time,
// and a plain Go string binds correctly against a UNIQUEIDENTIFIER column
// (SQL Server's implicit string conversion applies to parameters, not just
// literals) — no byte-level handling needed for writes, only for reads
// (see normalizeCell).
// dead code — never called in production path (see AD-037, M22/design.md)

// Scan converts a raw value returned by database/sql into dest, according
// to the declared ColumnType.
// dead code — never called in production path (see AD-037, M22/design.md)

// quoteIdent square-bracket-quotes a SQL identifier, preserving table
// prefixes — T-SQL's own quoting convention.
func quoteIdent(name string) string {
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		quoted := make([]string, len(parts))
		for i, p := range parts {
			quoted[i] = "[" + p + "]"
		}
		return strings.Join(quoted, ".")
	}
	return "[" + name + "]"
}

// aggregateSQLFunc builds a SUM/AVG/COUNT(column) expression. SUM/AVG are
// cast to FLOAT so the driver always decodes them as float64 consistently,
// mirroring every other adapter's same-purpose cast.
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

// compilePredicate recursively builds the SQL Server WHERE clause from a
// stmt.Predicate. Uses @p-numbered placeholders (confirmed via design.md's
// probe — the "sqlserver" driver does no ?-style pre-processing), same
// numbered-placeholder shape as driver/postgres's $N, s/$/@p/.
func compilePredicate(p stmt.Predicate, argOffset *int, args *[]any) (string, error) {
	if p == nil {
		return "", nil
	}

	switch v := p.(type) {
	case stmt.Comparison:
		switch v.Op {
		case "eq":
			sql := fmt.Sprintf("%s=@p%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "gt":
			sql := fmt.Sprintf("%s>@p%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "gte":
			sql := fmt.Sprintf("%s>=@p%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "lt":
			sql := fmt.Sprintf("%s<@p%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "lte":
			sql := fmt.Sprintf("%s<=@p%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "like":
			sql := fmt.Sprintf("%s LIKE @p%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "is_null":
			return fmt.Sprintf("%s IS NULL", quoteIdent(v.Column)), nil
		case "in":
			valVal := reflect.ValueOf(v.Value)
			if valVal.Kind() != reflect.Slice {
				sql := fmt.Sprintf("%s IN (@p%d)", quoteIdent(v.Column), *argOffset)
				*args = append(*args, v.Value)
				*argOffset++
				return sql, nil
			}
			n := valVal.Len()
			if n == 0 {
				return "1=0", nil
			}
			placeholders := make([]string, n)
			for i := 0; i < n; i++ {
				placeholders[i] = fmt.Sprintf("@p%d", *argOffset)
				*args = append(*args, valVal.Index(i).Interface())
				*argOffset++
			}
			return fmt.Sprintf("%s IN (%s)", quoteIdent(v.Column), strings.Join(placeholders, ",")), nil
		}
		return "", fmt.Errorf("mssql: unsupported comparison operator %q", v.Op)

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
			sql := fmt.Sprintf("%s=@p%d", expr, *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "gt":
			sql := fmt.Sprintf("%s>@p%d", expr, *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "gte":
			sql := fmt.Sprintf("%s>=@p%d", expr, *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "lt":
			sql := fmt.Sprintf("%s<@p%d", expr, *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "lte":
			sql := fmt.Sprintf("%s<=@p%d", expr, *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		}
		return "", fmt.Errorf("mssql: unsupported having comparison operator %q", v.Op)

	default:
		return "", fmt.Errorf("mssql: unsupported predicate type %T", p)
	}
}

// CompileSelect compiles a Select statement plan to SQL Server SQL and args.
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

	if s.Lock != nil {
		hint, err := lockClauseSQL(s.Lock)
		if err != nil {
			return "", nil, err
		}
		sb.WriteString(hint)
	}

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

	// SQL Server's OFFSET/FETCH pagination is a hard syntax error without an
	// accompanying ORDER BY (confirmed via design.md's probe) — when a
	// caller pages without one, inject a default ORDER BY instead of
	// letting that cryptic error surface.
	needsPagination := s.Limit != nil || s.Offset != nil
	orderBy := s.OrderBy
	rawOrderBy := ""
	if needsPagination && len(orderBy) == 0 {
		switch {
		case s.Count:
			// A bare aggregate (COUNT/SUM/AVG without GROUP BY) always
			// collapses to a single row, so any ORDER BY column reference
			// would be rejected by SQL Server ("invalid in the ORDER BY
			// clause because it is not contained in either an aggregate
			// function or the GROUP BY clause") — confirmed via a real
			// probe against repository.Exists's Count+Limit query. ORDER
			// BY (SELECT NULL) is the standard T-SQL idiom for "OFFSET/
			// FETCH needs syntax, order is meaningless here."
			rawOrderBy = "(SELECT NULL)"
		case len(s.PrimaryKey) > 0:
			orderBy = make([]stmt.OrderElement, len(s.PrimaryKey))
			for i, pk := range s.PrimaryKey {
				orderBy[i] = stmt.OrderElement{Column: pk}
			}
		default:
			// repository.Aggregate's own stmt.Select never populates
			// PrimaryKey — an aggregate's result columns don't include
			// the source entity's PK — so fail clearly instead of
			// emitting invalid SQL Server syntax.
			return "", nil, fmt.Errorf("mssql: OFFSET/FETCH pagination requires an ORDER BY, and no OrderBy or PrimaryKey was available to build a default one")
		}
	}

	if rawOrderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(rawOrderBy)
	} else if len(orderBy) > 0 {
		sb.WriteString(" ORDER BY ")
		orderByClauses := make([]string, len(orderBy))
		for i, ord := range orderBy {
			dir := "ASC"
			if ord.Desc {
				dir = "DESC"
			}
			orderByClauses[i] = fmt.Sprintf("%s %s", quoteIdent(ord.Column), dir)
		}
		sb.WriteString(strings.Join(orderByClauses, ", "))
	}

	if needsPagination {
		offsetVal := 0
		if s.Offset != nil {
			offsetVal = *s.Offset
		}
		sb.WriteString(fmt.Sprintf(" OFFSET @p%d ROWS", argOffset))
		args = append(args, offsetVal)
		argOffset++
		if s.Limit != nil {
			sb.WriteString(fmt.Sprintf(" FETCH NEXT @p%d ROWS ONLY", argOffset))
			args = append(args, *s.Limit)
			argOffset++
		}
	}

	return sb.String(), args, nil
}

// lockClauseSQL builds the `WITH (...)` table hint attached to the FROM
// clause's table reference (confirmed via design.md's probe) — unlike
// every other adapter, SQL Server expresses row locking as a table hint,
// not a trailing clause.
func lockClauseSQL(l *stmt.LockClause) (string, error) {
	var hints []string
	switch l.Strength {
	case "update":
		hints = []string{"UPDLOCK", "ROWLOCK"}
	case "share":
		hints = []string{"HOLDLOCK", "ROWLOCK"}
	default:
		return "", fmt.Errorf("mssql: unsupported lock strength %q", l.Strength)
	}

	switch l.Wait {
	case "":
		// block (SQL Server default) — no extra hint
	case "nowait":
		hints = append(hints, "NOWAIT")
	case "skip_locked":
		hints = append(hints, "READPAST")
	default:
		return "", fmt.Errorf("mssql: unsupported lock wait policy %q", l.Wait)
	}

	return " WITH (" + strings.Join(hints, ", ") + ")", nil
}

// CompileDelete compiles a Delete statement plan to SQL Server SQL and args.
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

// Insert executes an INSERT statement against SQL Server, using
// OUTPUT INSERTED.* for a single round-trip — confirmed via design.md's
// probe. s.PrimaryKey is read from stmt.Insert (added for M16) but unused
// here, same as driver/postgres/driver/sqlite.
func (d *dialect) Insert(ctx context.Context, conn golem.Conn, s *stmt.Insert) (map[string]any, error) {
	quotedCols := make([]string, len(s.Columns))
	placeholders := make([]string, len(s.Columns))
	for i, c := range s.Columns {
		quotedCols[i] = quoteIdent(c)
		placeholders[i] = fmt.Sprintf("@p%d", i+1)
	}
	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) OUTPUT INSERTED.* VALUES (%s)",
		quoteIdent(s.Table), strings.Join(quotedCols, ","), strings.Join(placeholders, ","))

	args := make([]any, len(s.Values))
	for i, v := range s.Values {
		args[i] = v
	}

	rows, err := d.getExecutor(conn).QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("mssql: insert: %w", mapError(err))
	}
	defer rows.Close()

	// go-mssqldb's QueryContext is lazy: a constraint violation on the
	// statement itself (confirmed via a real probe against SQL Server —
	// e.g. a UNIQUE/FOREIGN KEY conflict) does NOT surface as an error from
	// QueryContext above; it only appears via rows.Err() once the row
	// iteration inside collectRows completes. mapError must be applied
	// here too, not just on the QueryContext error above, or constraint
	// violations never get mapped to golem.ErrDuplicateKey/
	// ErrForeignKeyViolation at all.
	results, err := collectRows(rows)
	if err != nil {
		return nil, fmt.Errorf("mssql: insert: %w", mapError(err))
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("mssql: insert: no row returned")
	}
	return results[0], nil
}

// Update executes an UPDATE statement with OUTPUT INSERTED.*, returning all
// updated rows in the same round-trip — this is also all SaveOne/SaveMany
// need (they route through this same method; SQL Server's lack of an
// ON CONFLICT/MERGE equivalent turned out not to matter, since golem never
// implemented upsert semantics on any adapter — see design.md's Correction
// section).
func (d *dialect) Update(ctx context.Context, conn golem.Conn, s *stmt.Update) ([]map[string]any, error) {
	setClauses := make([]string, 0, len(s.Sets))
	args := make([]any, 0, len(s.Sets))
	argOffset := 1
	for _, set := range s.Sets {
		setClauses = append(setClauses, fmt.Sprintf("%s=@p%d", quoteIdent(set.Column), argOffset))
		args = append(args, set.Value)
		argOffset++
	}

	var sb strings.Builder
	sb.WriteString("UPDATE ")
	sb.WriteString(quoteIdent(s.Table))
	sb.WriteString(" SET ")
	sb.WriteString(strings.Join(setClauses, ","))
	sb.WriteString(" OUTPUT INSERTED.*")

	if s.Where != nil {
		sql, err := compilePredicate(s.Where, &argOffset, &args)
		if err != nil {
			return nil, err
		}
		if sql != "" {
			sb.WriteString(" WHERE ")
			sb.WriteString(sql)
		}
	}

	rows, err := d.getExecutor(conn).QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("mssql: update: %w", mapError(err))
	}
	defer rows.Close()

	// See Insert's identical comment: QueryContext is lazy, so a
	// constraint violation on this statement surfaces via rows.Err()
	// inside collectRows, not the QueryContext error above.
	results, err := collectRows(rows)
	if err != nil {
		return nil, fmt.Errorf("mssql: update: %w", mapError(err))
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
	results, err := collectRows(rows)
	if err != nil {
		return nil, mapError(err)
	}
	return results, nil
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
		return nil, 0, mapError(err)
	}

	return results, int64(len(results)), nil
}

// collectRows scans every row from rows into a column-name-keyed map, then
// normalizes each value per its reported SQL column type (see
// normalizeCell). Extracted via internal/must so the accepted-unreachable
// defensive checks (io errors that can't happen given database/sql's *any
// Scan contract) are exercised through the deferred must.Recover, same
// pattern as driver/mysql's/driver/sqlite's collectRows (STATE.md AD-043).
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

// normalizeCell converts raw []byte values from columns whose driver-level
// representation is ambiguous into their proper Go type. Two cases,
// confirmed via a real-server probe:
//   - UNIQUEIDENTIFIER: generic Scan(&any) returns raw bytes in SQL Server's
//     mixed-endian layout — the first 3 groups (4/2/2 bytes) are
//     byte-reversed relative to a standard UUID's byte order, the last 2
//     groups (2/6 bytes) are not. Converted to a standard hyphenated string.
//   - DECIMAL: generic Scan(&any) returns raw ASCII decimal text (e.g.
//     "19.99") as []byte, not a native float64 (unlike FLOAT columns, which
//     scan natively) — mirrors driver/mysql's own DECIMAL handling
//     (AD-039). Parsed into a float64.
//
// Every other declared type is left untouched.
func normalizeCell(dbType string, raw any) any {
	switch dbType {
	case "UNIQUEIDENTIFIER":
		b, ok := raw.([]byte)
		if !ok || len(b) != 16 {
			return raw
		}
		return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			reverseBytes(b[0:4]), reverseBytes(b[4:6]), reverseBytes(b[6:8]), b[8:10], b[10:16])
	case "DECIMAL":
		b, ok := raw.([]byte)
		if !ok {
			return raw
		}
		f, err := strconv.ParseFloat(string(b), 64)
		if err != nil {
			return raw
		}
		return f
	default:
		return raw
	}
}

// reverseBytes returns a new slice with bs's bytes in reverse order.
func reverseBytes(bs []byte) []byte {
	out := make([]byte, len(bs))
	for i, v := range bs {
		out[len(bs)-1-i] = v
	}
	return out
}

// mapError wraps err with the matching golem sentinel (ErrDuplicateKey,
// ErrForeignKeyViolation) when it's a recognized SQL Server error number,
// keeping the original mssql.Error reachable via errors.As/errors.Unwrap.
// Unmapped errors (including non-SQL-Server ones) pass through unchanged.
// Note: 547 covers both foreign key AND CHECK constraint violations in real
// SQL Server — golem has no CHECK concept (AD-012), so this maps it
// uniformly to ErrForeignKeyViolation, a simplification driver/mysql/
// driver/sqlite didn't need but this adapter does.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	var msErr mssqllib.Error
	if errors.As(err, &msErr) {
		switch msErr.Number {
		case 2627, 2601:
			return fmt.Errorf("%w: %w", golem.ErrDuplicateKey, err)
		case 547:
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
	var msErr mssqllib.Error
	if errors.As(err, &msErr) {
		switch msErr.Number {
		case 2627, 2601, 547:
			return true
		}
	}
	return false
}

// Begin starts a new transaction on the database.
func (d *dialect) Begin(ctx context.Context, conn golem.Conn) (golem.TxConn, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mssql: begin: %w", err)
	}
	return &mssqlTx{tx: tx}, nil
}

type mssqlExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (d *dialect) getExecutor(conn golem.Conn) mssqlExecutor {
	if tx, ok := conn.(golem.Tx); ok {
		if mtx, ok := tx.Underlying().(*mssqlTx); ok {
			return mtx.tx
		}
	}
	return d.db
}

type mssqlTx struct {
	tx *sql.Tx
}

func (t *mssqlTx) Commit(ctx context.Context) error {
	return t.tx.Commit()
}

func (t *mssqlTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback()
}
