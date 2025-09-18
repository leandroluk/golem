package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/leandroluk/golem/core"
)

//region postgresTransaction

type postgresTransaction struct {
	transaction pgx.Tx
}

func (transaction *postgresTransaction) Commit(ctx context.Context) error {
	return transaction.transaction.Commit(ctx)
}

func (transaction *postgresTransaction) Rollback(ctx context.Context) error {
	return transaction.transaction.Rollback(ctx)
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

func (driver *PostgresDriver) formatTable(schema *core.SchemaCore) string {
	if schema.Database != "" {
		return fmt.Sprintf("%q.%q", schema.Database, schema.Collection)
	}
	return fmt.Sprintf("%q", schema.Collection)
}

func (driver *PostgresDriver) buildCondition(condition *core.Condition, argList *[]any) string {
	if condition == nil {
		return "1=1"
	}
	if len(condition.Children) > 0 {
		partList := []string{}
		for _, child := range condition.Children {
			partList = append(partList, driver.buildCondition(child, argList))
		}
		switch *condition.Operator {
		case core.OpAnd:
			return "(" + strings.Join(partList, " AND ") + ")"
		case core.OpOr:
			return "(" + strings.Join(partList, " OR ") + ")"
		case core.OpNot:
			return "NOT (" + strings.Join(partList, " AND ") + ")"
		}
	}

	column := fmt.Sprintf("%q", condition.FieldName)
	switch *condition.Operator {
	case core.OpNil:
		return column + " IS NULL"
	case core.OpEq:
		*argList = append(*argList, condition.Value)
		return fmt.Sprintf("%s = $%d", column, len(*argList))
	case core.OpGt:
		*argList = append(*argList, condition.Value)
		return fmt.Sprintf("%s > $%d", column, len(*argList))
	case core.OpGte:
		*argList = append(*argList, condition.Value)
		return fmt.Sprintf("%s >= $%d", column, len(*argList))
	case core.OpLt:
		*argList = append(*argList, condition.Value)
		return fmt.Sprintf("%s < $%d", column, len(*argList))
	case core.OpLte:
		*argList = append(*argList, condition.Value)
		return fmt.Sprintf("%s <= $%d", column, len(*argList))
	case core.OpLike:
		*argList = append(*argList, condition.Value)
		return fmt.Sprintf("%s ILIKE $%d", column, len(*argList))
	case core.OpIn:
		valueList := condition.Value.([]any)
		placeholderList := []string{}
		for _, v := range valueList {
			*argList = append(*argList, v)
			placeholderList = append(placeholderList, fmt.Sprintf("$%d", len(*argList)))
		}
		return fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholderList, ", "))
	}
	return "1=1"
}

// --- helpers para executar com/sem transação ---

func (driver *PostgresDriver) exec(ctx context.Context, sqlQuery string, args ...any) error {
	if tx := core.TransactionFrom(ctx); tx != nil {
		if pgTx, ok := tx.(*postgresTransaction); ok {
			_, err := pgTx.transaction.Exec(ctx, sqlQuery, args...)
			return err
		}
	}
	_, err := driver.pool.Exec(ctx, sqlQuery, args...)
	return err
}

func (driver *PostgresDriver) query(ctx context.Context, sqlQuery string, args ...any) (pgx.Rows, error) {
	if tx := core.TransactionFrom(ctx); tx != nil {
		if pgTx, ok := tx.(*postgresTransaction); ok {
			return pgTx.transaction.Query(ctx, sqlQuery, args...)
		}
	}
	return driver.pool.Query(ctx, sqlQuery, args...)
}

func (driver *PostgresDriver) queryRow(ctx context.Context, sqlQuery string, args ...any) pgx.Row {
	if tx := core.TransactionFrom(ctx); tx != nil {
		if pgTx, ok := tx.(*postgresTransaction); ok {
			return pgTx.transaction.QueryRow(ctx, sqlQuery, args...)
		}
	}
	return driver.pool.QueryRow(ctx, sqlQuery, args...)
}

func (driver *PostgresDriver) find(ctx context.Context, schema *core.SchemaCore, query *core.Where, single bool) ([]map[string]any, error) {
	columnNameList := []string{}
	for _, field := range schema.Fields {
		columnNameList = append(columnNameList, fmt.Sprintf("%q", field.DatabaseColumnName))
	}
	selectColumns := strings.Join(columnNameList, ", ")

	argList := []any{}
	whereClause := driver.buildCondition(query.Condition, &argList)

	sqlQuery := fmt.Sprintf("SELECT %s FROM %s WHERE %s", selectColumns, driver.formatTable(schema), whereClause)

	if len(query.Sort) > 0 {
		orderPartList := []string{}
		for _, sortItem := range query.Sort {
			direction := "ASC"
			if sortItem.Order < 0 {
				direction = "DESC"
			}
			orderPartList = append(orderPartList, fmt.Sprintf("%q %s", sortItem.FieldName, direction))
		}
		sqlQuery += " ORDER BY " + strings.Join(orderPartList, ", ")
	}
	if single {
		sqlQuery += " LIMIT 1"
	} else {
		if query.Limit > 0 {
			sqlQuery += fmt.Sprintf(" LIMIT %d", query.Limit)
		}
		if query.Offset > 0 {
			sqlQuery += fmt.Sprintf(" OFFSET %d", query.Offset)
		}
	}

	rowList, err := driver.query(ctx, sqlQuery, argList...)
	if err != nil {
		return nil, err
	}
	defer rowList.Close()

	columnDescriptionList := rowList.FieldDescriptions()
	var resultList []map[string]any

	for rowList.Next() {
		valueList, err := rowList.Values()
		if err != nil {
			return nil, err
		}
		rowMap := make(map[string]any)
		for i, col := range columnDescriptionList {
			rowMap[string(col.Name)] = valueList[i]
		}
		resultList = append(resultList, rowMap)
		if single {
			break
		}
	}
	return resultList, nil
}

func (driver *PostgresDriver) Connect(ctx context.Context) error {
	return driver.pool.Ping(ctx)
}

func (driver *PostgresDriver) Ping(ctx context.Context) error {
	return driver.pool.Ping(ctx)
}

func (driver *PostgresDriver) Close(ctx context.Context) error {
	driver.pool.Close()
	return nil
}

func (driver *PostgresDriver) Transaction(ctx context.Context) (core.Transaction, error) {
	tx, err := driver.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	return &postgresTransaction{transaction: tx}, nil
}

func (driver *PostgresDriver) Insert(ctx context.Context, schema *core.SchemaCore, documents ...any) error {
	if len(documents) == 0 {
		return nil
	}

	columnNameList := []string{}
	for _, field := range schema.Fields {
		columnNameList = append(columnNameList, fmt.Sprintf("%q", field.DatabaseColumnName))
	}
	columnList := "(" + strings.Join(columnNameList, ", ") + ")"

	for _, doc := range documents {
		valueList, placeholderList := core.StructValues(schema, doc)
		sqlQuery := fmt.Sprintf("INSERT INTO %s %s VALUES (%s)",
			driver.formatTable(schema), columnList, strings.Join(placeholderList, ", "))

		if err := driver.exec(ctx, sqlQuery, valueList...); err != nil {
			return err
		}
	}
	return nil
}

func (driver *PostgresDriver) FindOne(ctx context.Context, schema *core.SchemaCore, query *core.Where) (any, error) {
	rowList, err := driver.find(ctx, schema, query, true)
	if err != nil {
		return nil, err
	}
	if len(rowList) == 0 {
		return nil, nil
	}
	return rowList[0], nil
}

func (driver *PostgresDriver) FindMany(ctx context.Context, schema *core.SchemaCore, query *core.Where) (any, error) {
	return driver.find(ctx, schema, query, false)
}

func (driver *PostgresDriver) Update(ctx context.Context, schema *core.SchemaCore, condition *core.Condition, changes core.Changes) error {
	argList := []any{}
	whereClause := driver.buildCondition(condition, &argList)

	setPartList := []string{}
	for column, value := range changes {
		argList = append(argList, value)
		setPartList = append(setPartList, fmt.Sprintf("%q = $%d", column, len(argList)))
	}

	sqlQuery := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		driver.formatTable(schema), strings.Join(setPartList, ", "), whereClause)

	return driver.exec(ctx, sqlQuery, argList...)
}

func (driver *PostgresDriver) Delete(ctx context.Context, schema *core.SchemaCore, condition *core.Condition) error {
	argList := []any{}
	whereClause := driver.buildCondition(condition, &argList)
	sqlQuery := fmt.Sprintf("DELETE FROM %s WHERE %s", driver.formatTable(schema), whereClause)
	return driver.exec(ctx, sqlQuery, argList...)
}

func (driver *PostgresDriver) Count(ctx context.Context, schema *core.SchemaCore, condition *core.Condition) (int64, error) {
	argList := []any{}
	whereClause := driver.buildCondition(condition, &argList)
	sqlQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", driver.formatTable(schema), whereClause)

	var count int64
	if err := driver.queryRow(ctx, sqlQuery, argList...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

//endregion
