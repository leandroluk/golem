package postgres

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/stmt"
)

// dialect is the Postgres implementation of golem.Dialect.
type dialect struct {
	pool *pgxpool.Pool
}

var _ golem.Dialect = (*dialect)(nil)

// Bind converts a Go value to a driver.Value suitable for Postgres, according
// to the declared ColumnType.
func (dialect) Bind(t golem.ColumnType, value any) (driver.Value, error) {
	switch t.Kind() {
	case "boolean":
		switch v := value.(type) {
		case bool:
			return v, nil
		case int, int8, int16, int32, int64:
			return fmt.Sprintf("%v", v) != "0", nil
		}
		return nil, fmt.Errorf("postgres: bind boolean: unsupported Go type %T", value)

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
		return nil, fmt.Errorf("postgres: bind %s: unsupported Go type %T", t.Kind(), value)

	case "decimal", "float":
		switch v := value.(type) {
		case float32:
			return float64(v), nil
		case float64:
			return v, nil
		}
		return nil, fmt.Errorf("postgres: bind %s: unsupported Go type %T", t.Kind(), value)

	case "char", "varchar", "text":
		switch v := value.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		}
		return nil, fmt.Errorf("postgres: bind %s: unsupported Go type %T", t.Kind(), value)

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
		return nil, fmt.Errorf("postgres: bind %s: unsupported Go type %T", t.Kind(), value)

	case "blob":
		switch v := value.(type) {
		case []byte:
			return v, nil
		case string:
			return []byte(v), nil
		}
		return nil, fmt.Errorf("postgres: bind blob: unsupported Go type %T", value)

	case "uuid":
		switch v := value.(type) {
		case string:
			return v, nil
		case [16]byte:
			return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
				v[0:4], v[4:6], v[6:8], v[8:10], v[10:16]), nil
		}
		return nil, fmt.Errorf("postgres: bind uuid: unsupported Go type %T", value)

	case "json":
		switch v := value.(type) {
		case string:
			return v, nil
		case []byte:
			return string(v), nil
		}
		return nil, fmt.Errorf("postgres: bind json: unsupported Go type %T", value)
	}

	return nil, fmt.Errorf("postgres: bind: unrecognized column kind %q", t.Kind())
}

// Scan converts a raw value returned by pgx into dest, according to the
// declared ColumnType.
func (dialect) Scan(t golem.ColumnType, raw any, dest any) error {
	if raw == nil {
		return nil
	}

	switch t.Kind() {
	case "boolean":
		d, ok := dest.(*bool)
		if !ok {
			return fmt.Errorf("postgres: scan boolean: dest must be *bool, got %T", dest)
		}
		switch v := raw.(type) {
		case bool:
			*d = v
		case int64:
			*d = v != 0
		default:
			return fmt.Errorf("postgres: scan boolean: unsupported raw type %T", raw)
		}

	case "smallint", "integer", "bigint":
		d, ok := dest.(*int64)
		if !ok {
			return fmt.Errorf("postgres: scan %s: dest must be *int64, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case int64:
			*d = v
		case int32:
			*d = int64(v)
		case int16:
			*d = int64(v)
		default:
			return fmt.Errorf("postgres: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "decimal", "float":
		d, ok := dest.(*float64)
		if !ok {
			return fmt.Errorf("postgres: scan %s: dest must be *float64, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case float64:
			*d = v
		case float32:
			*d = float64(v)
		default:
			return fmt.Errorf("postgres: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "char", "varchar", "text":
		d, ok := dest.(*string)
		if !ok {
			return fmt.Errorf("postgres: scan %s: dest must be *string, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case string:
			*d = v
		case []byte:
			*d = string(v)
		default:
			return fmt.Errorf("postgres: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "date", "datetime", "time":
		d, ok := dest.(*time.Time)
		if !ok {
			return fmt.Errorf("postgres: scan %s: dest must be *time.Time, got %T", t.Kind(), dest)
		}
		switch v := raw.(type) {
		case time.Time:
			*d = v
		default:
			return fmt.Errorf("postgres: scan %s: unsupported raw type %T", t.Kind(), raw)
		}

	case "blob":
		d, ok := dest.(*[]byte)
		if !ok {
			return fmt.Errorf("postgres: scan blob: dest must be *[]byte, got %T", dest)
		}
		switch v := raw.(type) {
		case []byte:
			*d = v
		case string:
			*d = []byte(v)
		default:
			return fmt.Errorf("postgres: scan blob: unsupported raw type %T", raw)
		}

	case "uuid":
		d, ok := dest.(*string)
		if !ok {
			return fmt.Errorf("postgres: scan uuid: dest must be *string, got %T", dest)
		}
		switch v := raw.(type) {
		case string:
			*d = v
		case [16]byte:
			*d = fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
				v[0:4], v[4:6], v[6:8], v[8:10], v[10:16])
		default:
			return fmt.Errorf("postgres: scan uuid: unsupported raw type %T", raw)
		}

	case "json":
		d, ok := dest.(*string)
		if !ok {
			return fmt.Errorf("postgres: scan json: dest must be *string, got %T", dest)
		}
		switch v := raw.(type) {
		case string:
			*d = v
		case []byte:
			*d = string(v)
		default:
			return fmt.Errorf("postgres: scan json: unsupported raw type %T", raw)
		}

	default:
		return fmt.Errorf("postgres: scan: unrecognized column kind %q", t.Kind())
	}

	return nil
}

// quoteIdent double-quotes a SQL identifier, preserving table prefixes.
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

// compilePredicate recursively builds the Postgres SQL WHERE clause from a stmt.Predicate
func compilePredicate(p stmt.Predicate, argOffset *int, args *[]any) (string, error) {
	if p == nil {
		return "", nil
	}

	switch v := p.(type) {
	case stmt.Comparison:
		switch v.Op {
		case "eq":
			sql := fmt.Sprintf("%s=$%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "gt":
			sql := fmt.Sprintf("%s>$%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "gte":
			sql := fmt.Sprintf("%s>=$%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "lt":
			sql := fmt.Sprintf("%s<$%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "lte":
			sql := fmt.Sprintf("%s<=$%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "like":
			sql := fmt.Sprintf("%s LIKE $%d", quoteIdent(v.Column), *argOffset)
			*args = append(*args, v.Value)
			*argOffset++
			return sql, nil
		case "is_null":
			return fmt.Sprintf("%s IS NULL", quoteIdent(v.Column)), nil
		case "in":
			valVal := reflect.ValueOf(v.Value)
			if valVal.Kind() != reflect.Slice {
				sql := fmt.Sprintf("%s IN ($%d)", quoteIdent(v.Column), *argOffset)
				*args = append(*args, v.Value)
				*argOffset++
				return sql, nil
			}
			n := valVal.Len()
			if n == 0 {
				return "FALSE", nil
			}
			placeholders := make([]string, n)
			for i := 0; i < n; i++ {
				placeholders[i] = fmt.Sprintf("$%d", *argOffset)
				*args = append(*args, valVal.Index(i).Interface())
				*argOffset++
			}
			return fmt.Sprintf("%s IN (%s)", quoteIdent(v.Column), strings.Join(placeholders, ",")), nil
		}
		return "", fmt.Errorf("postgres: unsupported comparison operator %q", v.Op)

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

	default:
		return "", fmt.Errorf("postgres: unsupported predicate type %T", p)
	}
}

// CompileSelect compiles a Select statement plan to Postgres SQL and args.
func (d *dialect) CompileSelect(s *stmt.Select) (string, []any, error) {
	var sb strings.Builder
	sb.WriteString("SELECT ")
	if s.Count {
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
		sb.WriteString(fmt.Sprintf(" LIMIT $%d", argOffset))
		args = append(args, *s.Limit)
		argOffset++
	}

	if s.Offset != nil {
		sb.WriteString(fmt.Sprintf(" OFFSET $%d", argOffset))
		args = append(args, *s.Offset)
		argOffset++
	}

	return sb.String(), args, nil
}

// CompileDelete compiles a Delete statement plan to Postgres SQL and args.
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

// Insert executes an INSERT statement against Postgres returning the mapped result row.
func (d *dialect) Insert(ctx context.Context, conn golem.Conn, s *stmt.Insert) (map[string]any, error) {
	quotedCols := make([]string, len(s.Columns))
	placeholders := make([]string, len(s.Columns))
	for i, c := range s.Columns {
		quotedCols[i] = quoteIdent(c)
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	sql := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s) RETURNING *`,
		quoteIdent(s.Table), strings.Join(quotedCols, ","), strings.Join(placeholders, ","))

	args := make([]any, len(s.Values))
	for i, v := range s.Values {
		args[i] = v
	}

	rows, err := d.getExecutor(conn).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: insert: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToMap)
	if err != nil {
		return nil, fmt.Errorf("postgres: insert: %w", err)
	}
	return row, nil
}

// Update executes an UPDATE statement and returns all updated rows.
func (d *dialect) Update(ctx context.Context, conn golem.Conn, s *stmt.Update) ([]map[string]any, error) {
	setClauses := make([]string, 0, len(s.Sets))
	args := make([]any, 0, len(s.Sets))
	argOffset := 1
	for _, set := range s.Sets {
		setClauses = append(setClauses, fmt.Sprintf("%s=$%d", quoteIdent(set.Column), argOffset))
		args = append(args, set.Value)
		argOffset++
	}

	var sb strings.Builder
	sb.WriteString("UPDATE ")
	sb.WriteString(quoteIdent(s.Table))
	sb.WriteString(" SET ")
	sb.WriteString(strings.Join(setClauses, ","))

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

	sb.WriteString(" RETURNING *")

	rows, err := d.getExecutor(conn).Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: update: %w", err)
	}
	results, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		return nil, fmt.Errorf("postgres: update: %w", err)
	}
	return results, nil
}

// Query executes a raw compiled SELECT query statement.
func (d *dialect) Query(ctx context.Context, conn golem.Conn, sql string, args []any) ([]map[string]any, error) {
	rows, err := d.getExecutor(conn).Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToMap)
}

// Exec executes a raw compiled command query (command executing, no rows returned).
func (d *dialect) Exec(ctx context.Context, conn golem.Conn, sql string, args []any) (int64, error) {
	ct, err := d.getExecutor(conn).Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

// IsConflict returns true if the error represents a database integrity constraint violation.
func (d *dialect) IsConflict(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// Class 23 — Integrity Constraint Violation
		// Reference: https://www.postgresql.org/docs/current/errcodes-appendix.html
		return strings.HasPrefix(pgErr.Code, "23")
	}
	return false
}

// Begin starts a new transaction on the database.
func (d *dialect) Begin(ctx context.Context, conn golem.Conn) (golem.TxConn, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: begin: %w", err)
	}
	return &pgTx{tx: tx, d: d}, nil
}

type pgExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func (d *dialect) getExecutor(conn golem.Conn) pgExecutor {
	if tx, ok := conn.(golem.Tx); ok {
		if pgTx, ok := tx.Underlying().(*pgTx); ok {
			return pgTx.tx
		}
	}
	return d.pool
}

type pgTx struct {
	tx pgx.Tx
	d  *dialect
}

func (t *pgTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *pgTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

