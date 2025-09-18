package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/leandroluk/golem/core"
)

// region postgresTransaction

type postgresTransaction struct {
	transaction pgx.Tx
}

func (t *postgresTransaction) Commit(ctx context.Context) error {
	return t.transaction.Commit(ctx)
}

func (t *postgresTransaction) Rollback(ctx context.Context) error {
	return t.transaction.Rollback(ctx)
}

//endregion

//region PostgresDriver

type PostgresDriver struct {
	pool *pgxpool.Pool
}

var _ core.Driver = (*PostgresDriver)(nil)

func NewPostgresDriver(ctx context.Context, connString string) (*PostgresDriver, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}
	return &PostgresDriver{pool: pool}, nil
}

func (d *PostgresDriver) formatTable(schema *core.SchemaCore) string {
	if schema.Database != "" {
		return fmt.Sprintf("%q.%q", schema.Database, schema.Collection)
	}
	return fmt.Sprintf("%q", schema.Collection)
}

func (d *PostgresDriver) buildCondition(c *core.Condition, args *[]any) string {
	if c == nil {
		return "1=1"
	}
	if len(c.Children) > 0 {
		parts := []string{}
		for _, child := range c.Children {
			parts = append(parts, d.buildCondition(child, args))
		}
		switch *c.Operator {
		case core.OpAnd:
			return "(" + strings.Join(parts, " AND ") + ")"
		case core.OpOr:
			return "(" + strings.Join(parts, " OR ") + ")"
		case core.OpNot:
			return "NOT (" + strings.Join(parts, " AND ") + ")"
		}
	}

	col := fmt.Sprintf("%q", c.FieldName)
	switch *c.Operator {
	case core.OpNil:
		return col + " IS NULL"
	case core.OpEq:
		*args = append(*args, c.Value)
		return fmt.Sprintf("%s = $%d", col, len(*args))
	case core.OpGt:
		*args = append(*args, c.Value)
		return fmt.Sprintf("%s > $%d", col, len(*args))
	case core.OpGte:
		*args = append(*args, c.Value)
		return fmt.Sprintf("%s >= $%d", col, len(*args))
	case core.OpLt:
		*args = append(*args, c.Value)
		return fmt.Sprintf("%s < $%d", col, len(*args))
	case core.OpLte:
		*args = append(*args, c.Value)
		return fmt.Sprintf("%s <= $%d", col, len(*args))
	case core.OpLike:
		*args = append(*args, c.Value)
		return fmt.Sprintf("%s ILIKE $%d", col, len(*args))
	case core.OpIn:
		values := c.Value.([]any)
		placeholders := []string{}
		for _, v := range values {
			*args = append(*args, v)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(*args)))
		}
		return fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", "))
	}
	return "1=1"
}

// --- helpers para executar com/sem transação ---

func (d *PostgresDriver) exec(ctx context.Context, sql string, args ...any) error {
	if tx := core.TransactionFrom(ctx); tx != nil {
		if pgTx, ok := tx.(*postgresTransaction); ok {
			_, err := pgTx.transaction.Exec(ctx, sql, args...)
			return err
		}
	}
	_, err := d.pool.Exec(ctx, sql, args...)
	return err
}

func (d *PostgresDriver) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx := core.TransactionFrom(ctx); tx != nil {
		if pgTx, ok := tx.(*postgresTransaction); ok {
			return pgTx.transaction.Query(ctx, sql, args...)
		}
	}
	return d.pool.Query(ctx, sql, args...)
}

func (d *PostgresDriver) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx := core.TransactionFrom(ctx); tx != nil {
		if pgTx, ok := tx.(*postgresTransaction); ok {
			return pgTx.transaction.QueryRow(ctx, sql, args...)
		}
	}
	return d.pool.QueryRow(ctx, sql, args...)
}

func (d *PostgresDriver) find(ctx context.Context, schema *core.SchemaCore, options *core.Query, single bool) ([]map[string]any, error) {
	cols := []string{}
	for _, f := range schema.Fields {
		cols = append(cols, fmt.Sprintf("%q", f.DatabaseColumnName))
	}
	selectCols := strings.Join(cols, ", ")

	args := []any{}
	where := d.buildCondition(options.Condition, &args)

	sqlStr := fmt.Sprintf("SELECT %s FROM %s WHERE %s", selectCols, d.formatTable(schema), where)

	if len(options.Sort) > 0 {
		orderParts := []string{}
		for _, s := range options.Sort {
			dir := "ASC"
			if s.Order < 0 {
				dir = "DESC"
			}
			orderParts = append(orderParts, fmt.Sprintf("%q %s", s.FieldName, dir))
		}
		sqlStr += " ORDER BY " + strings.Join(orderParts, ", ")
	}
	if single {
		sqlStr += " LIMIT 1"
	} else {
		if options.Limit > 0 {
			sqlStr += fmt.Sprintf(" LIMIT %d", options.Limit)
		}
		if options.Offset > 0 {
			sqlStr += fmt.Sprintf(" OFFSET %d", options.Offset)
		}
	}

	rows, err := d.query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colsNames := rows.FieldDescriptions()
	var results []map[string]any

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		rowMap := make(map[string]any)
		for i, col := range colsNames {
			rowMap[string(col.Name)] = values[i]
		}
		results = append(results, rowMap)
		if single {
			break
		}
	}
	return results, nil
}

func (d *PostgresDriver) Connect(ctx context.Context) error {
	return d.pool.Ping(ctx)
}

func (d *PostgresDriver) Ping(ctx context.Context) error {
	return d.pool.Ping(ctx)
}

func (d *PostgresDriver) Close(ctx context.Context) error {
	d.pool.Close()
	return nil
}

func (d *PostgresDriver) Transaction(ctx context.Context) (core.Transaction, error) {
	transaction, err := d.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	return &postgresTransaction{transaction: transaction}, nil
}

func (d *PostgresDriver) Insert(ctx context.Context, schema *core.SchemaCore, documents ...any) error {
	if len(documents) == 0 {
		return nil
	}

	cols := []string{}
	for _, f := range schema.Fields {
		cols = append(cols, fmt.Sprintf("%q", f.DatabaseColumnName))
	}
	colList := "(" + strings.Join(cols, ", ") + ")"

	for _, doc := range documents {
		vals, placeholders := core.StructValues(schema, doc)
		sqlStr := fmt.Sprintf("INSERT INTO %s %s VALUES (%s)",
			d.formatTable(schema), colList, strings.Join(placeholders, ", "))

		if err := d.exec(ctx, sqlStr, vals...); err != nil {
			return err
		}
	}
	return nil
}

func (d *PostgresDriver) FindOne(ctx context.Context, schema *core.SchemaCore, options *core.Query) (any, error) {
	rows, err := d.find(ctx, schema, options, true)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (d *PostgresDriver) FindMany(ctx context.Context, schema *core.SchemaCore, options *core.Query) (any, error) {
	return d.find(ctx, schema, options, false)
}

func (d *PostgresDriver) Update(ctx context.Context, schema *core.SchemaCore, condition *core.Condition, changes core.Changes) error {
	args := []any{}
	where := d.buildCondition(condition, &args)

	setParts := []string{}
	for col, val := range changes {
		args = append(args, val)
		setParts = append(setParts, fmt.Sprintf("%q = $%d", col, len(args)))
	}

	sqlStr := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		d.formatTable(schema), strings.Join(setParts, ", "), where)

	return d.exec(ctx, sqlStr, args...)
}

func (d *PostgresDriver) Delete(ctx context.Context, schema *core.SchemaCore, condition *core.Condition) error {
	args := []any{}
	where := d.buildCondition(condition, &args)
	sqlStr := fmt.Sprintf("DELETE FROM %s WHERE %s", d.formatTable(schema), where)
	return d.exec(ctx, sqlStr, args...)
}

func (d *PostgresDriver) Count(ctx context.Context, schema *core.SchemaCore, condition *core.Condition) (int64, error) {
	args := []any{}
	where := d.buildCondition(condition, &args)
	sqlStr := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", d.formatTable(schema), where)

	var count int64
	if err := d.queryRow(ctx, sqlStr, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

//endregion
