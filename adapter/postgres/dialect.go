package postgres

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/leandroluk/golem"
)

// dialect is the Postgres implementation of golem.Dialect. Bind/Scan have no
// real ColumnType set to bind/scan yet (that's a future milestone) so those
// calls return a descriptive error instead of panicking. Insert/FindByID are
// real, backed by the pool established by connector.Connect.
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

// buildFindByIDSQL builds `SELECT * FROM "table" WHERE "pkColumn" = $1` with
// double-quoted identifiers.
func buildFindByIDSQL(table string, pkColumn string) string {
	return fmt.Sprintf(`SELECT * FROM %s WHERE %s = $1`, quoteIdent(table), quoteIdent(pkColumn))
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

// FindByID fetches one row by a single-column primary key from d's own pool.
// A no-rows result is a normal outcome (found=false, err=nil), not an error.
func (d *dialect) FindByID(ctx context.Context, conn golem.Conn, table string, pkColumn string, id driver.Value) (map[string]any, bool, error) {
	sql := buildFindByIDSQL(table, pkColumn)

	rows, err := d.pool.Query(ctx, sql, id)
	if err != nil {
		return nil, false, fmt.Errorf("postgres: find by id: %w", err)
	}
	row, err := pgx.CollectOneRow(rows, pgx.RowToMap)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("postgres: find by id: %w", err)
	}
	return row, true, nil
}
