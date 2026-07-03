package postgres

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/leandroluk/golem"
)

// dialect is the Postgres implementation of golem.Dialect. Bind/Scan have no
// real ColumnType set to bind/scan yet (that's a future milestone) so those
// calls return a descriptive error instead of panicking. Insert, Select, and
// Update are real, backed by the pool established by connector.Connect.
type dialect struct {
	pool *pgxpool.Pool
}

var _ golem.Dialect = (*dialect)(nil)

func (dialect) Bind(t golem.ColumnType, value any) (driver.Value, error) {
	return nil, fmt.Errorf("postgres: unrecognized column type %v", t)
}

func (dialect) Scan(t golem.ColumnType, raw any, dest any) error {
	return fmt.Errorf("postgres: unrecognized column type %v", t)
}

// quoteIdent double-quotes a SQL identifier. Plain concatenation (rather than
// fmt's %q) is used deliberately: %q applies Go string-escaping rules, which
// only coincidentally match SQL identifier-quoting rules; table/column names
// in this codebase are always simple snake_case identifiers with no embedded
// quotes, but this keeps the quoting unambiguous regardless.
func quoteIdent(name string) string {
	return `"` + name + `"`
}

// buildInsertSQL builds `INSERT INTO "table" ("col1","col2") VALUES ($1,$2)
// RETURNING *` with double-quoted identifiers and $N placeholders in column
// order.
func buildInsertSQL(table string, columns []string) string {
	quotedCols := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	for i, c := range columns {
		quotedCols[i] = quoteIdent(c)
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	return fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s) RETURNING *`,
		quoteIdent(table), strings.Join(quotedCols, ","), strings.Join(placeholders, ","))
}

// Insert executes an INSERT ... RETURNING * against d's own pool.
//
// TODO(M8): route through conn once golem.Tx exists; this pass's Postgres
// dialect always uses its own d.pool directly, so conn is accepted (to
// satisfy golem.Dialect) but otherwise unused.
func (d *dialect) Insert(ctx context.Context, conn golem.Conn, table string, columns []string, values []driver.Value) (map[string]any, error) {
	sql := buildInsertSQL(table, columns)

	args := make([]any, len(values))
	for i, v := range values {
		args[i] = v
	}

	rows, err := d.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: insert: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToMap)
	if err != nil {
		return nil, fmt.Errorf("postgres: insert: %w", err)
	}
	return row, nil
}

// buildSelectSQL builds `SELECT * FROM "table"` and, when whereColumns is
// non-empty, appends `WHERE "c1"=$1 AND "c2"=$2 ...` with double-quoted
// identifiers and $N placeholders in column order.
func buildSelectSQL(table string, whereColumns []string) string {
	sql := fmt.Sprintf(`SELECT * FROM %s`, quoteIdent(table))
	if len(whereColumns) == 0 {
		return sql
	}
	whereClauses := make([]string, len(whereColumns))
	for i, c := range whereColumns {
		whereClauses[i] = fmt.Sprintf("%s=$%d", quoteIdent(c), i+1)
	}
	return sql + ` WHERE ` + strings.Join(whereClauses, " AND ")
}

// buildUpdateSQL builds `UPDATE "table" SET "s1"=$1,"s2"=$2 WHERE "w1"=$3
// AND "w2"=$4 ... RETURNING *` with double-quoted identifiers. SET
// placeholders are numbered first, in setColumns order; WHERE placeholders
// (when whereColumns is non-empty) continue the sequence from there.
func buildUpdateSQL(table string, setColumns []string, whereColumns []string) string {
	setClauses := make([]string, len(setColumns))
	for i, c := range setColumns {
		setClauses[i] = fmt.Sprintf("%s=$%d", quoteIdent(c), i+1)
	}
	sql := fmt.Sprintf(`UPDATE %s SET %s`, quoteIdent(table), strings.Join(setClauses, ","))
	if len(whereColumns) > 0 {
		whereClauses := make([]string, len(whereColumns))
		for i, c := range whereColumns {
			whereClauses[i] = fmt.Sprintf("%s=$%d", quoteIdent(c), len(setColumns)+i+1)
		}
		sql += ` WHERE ` + strings.Join(whereClauses, " AND ")
	}
	return sql + ` RETURNING *`
}

// Select executes a SELECT * (optionally filtered by a WHERE clause) against
// d's own pool. Zero matching rows is a normal outcome: it returns an empty
// slice and a nil error, not an error.
//
// TODO(M8): route through conn once golem.Tx exists; this pass's Postgres
// dialect always uses its own d.pool directly, so conn is accepted (to
// satisfy golem.Dialect) but otherwise unused.
func (d *dialect) Select(ctx context.Context, conn golem.Conn, table string, whereColumns []string, whereValues []driver.Value) ([]map[string]any, error) {
	sql := buildSelectSQL(table, whereColumns)

	args := make([]any, len(whereValues))
	for i, v := range whereValues {
		args[i] = v
	}

	rows, err := d.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: select: %w", err)
	}
	results, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		return nil, fmt.Errorf("postgres: select: %w", err)
	}
	return results, nil
}

// Update executes an UPDATE ... RETURNING * against d's own pool. Zero
// updated rows is a normal outcome: it returns an empty slice and a nil
// error. Deciding whether zero rows constitutes a "not found" error is left
// to the caller (repository), not this Dialect method.
//
// TODO(M8): route through conn once golem.Tx exists; this pass's Postgres
// dialect always uses its own d.pool directly, so conn is accepted (to
// satisfy golem.Dialect) but otherwise unused.
func (d *dialect) Update(ctx context.Context, conn golem.Conn, table string, setColumns []string, setValues []driver.Value, whereColumns []string, whereValues []driver.Value) ([]map[string]any, error) {
	sql := buildUpdateSQL(table, setColumns, whereColumns)

	args := make([]any, 0, len(setValues)+len(whereValues))
	for _, v := range setValues {
		args = append(args, v)
	}
	for _, v := range whereValues {
		args = append(args, v)
	}

	rows, err := d.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: update: %w", err)
	}
	results, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		return nil, fmt.Errorf("postgres: update: %w", err)
	}
	return results, nil
}
