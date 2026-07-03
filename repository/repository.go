// Package repository provides Repository[T], a thin CRUD layer bound to one
// entity's metadata (entity.EntityMeta) and a golem.Conn. It builds no SQL
// itself — that's the golem.Dialect's job — repository only shuttles Go
// struct field values to/from the driver.Value/map[string]any shapes Dialect
// deals in, via reflect.
package repository

import (
	"context"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
	"github.com/leandroluk/golem/internal/stmt"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/relation"
)

// Repository[T] is bound to one entity's metadata and a connection.
type Repository[T any] struct {
	conn   golem.Conn
	meta   entity.EntityMeta
	entity *entity.Entity[T]
}

// Get builds a Repository[T] bound to conn and e. T is inferred from e's
// type (*entity.Entity[T]).
func Get[T any](conn golem.Conn, e *entity.Entity[T]) *Repository[T] {
	return &Repository[T]{conn: conn, meta: e.Describe(), entity: e}
}

// fieldToColumn builds a fieldName → columnName map from EntityMeta.
func (r *Repository[T]) fieldToColumn() map[string]string {
	m := make(map[string]string, len(r.meta.Columns))
	for _, col := range r.meta.Columns {
		m[col.FieldName] = col.Name
	}
	return m
}

// pkColumnSet returns a set of column names that form the primary key.
func (r *Repository[T]) pkColumnSet() map[string]bool {
	s := make(map[string]bool, len(r.meta.PrimaryKey))
	for _, pk := range r.meta.PrimaryKey {
		s[pk] = true
	}
	return s
}

// resolveFieldPtrAny resolves a field pointer to a field name using offset
// arithmetic against a struct pointer.
func resolveFieldPtrAny(base any, fieldPtr any) string {
	baseVal := reflect.ValueOf(base)
	if baseVal.Kind() == reflect.Pointer {
		baseVal = baseVal.Elem()
	}
	if baseVal.Kind() != reflect.Struct {
		return ""
	}
	baseAddr := baseVal.Addr().Pointer()
	t := baseVal.Type()
	fpAddr := reflect.ValueOf(fieldPtr).Pointer()
	offset := fpAddr - baseAddr
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Offset == offset {
			return t.Field(i).Name
		}
	}
	return ""
}

// translateCondition recursively maps an op.Condition to a stmt.Predicate AST node.
func (r *Repository[T]) translateCondition(base any, fieldToColumn map[string]string, tableName string, cond op.Condition) (stmt.Predicate, error) {
	if cond.Op == "or" {
		preds := make([]stmt.Predicate, 0, len(cond.SubConditions))
		for _, sub := range cond.SubConditions {
			p, err := r.translateCondition(base, fieldToColumn, tableName, sub)
			if err != nil {
				return nil, err
			}
			if p != nil {
				preds = append(preds, p)
			}
		}
		if len(preds) == 0 {
			return nil, nil
		}
		return stmt.Logical{Op: "or", Predicates: preds}, nil
	}

	if cond.Op == "not" {
		if len(cond.SubConditions) == 0 {
			return nil, nil
		}
		p, err := r.translateCondition(base, fieldToColumn, tableName, cond.SubConditions[0])
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, nil
		}
		return stmt.Not{Predicate: p}, nil
	}

	fieldName := resolveFieldPtrAny(base, cond.FieldPtr)
	if fieldName == "" {
		return nil, fmt.Errorf("repository: field pointer does not belong to entity struct")
	}
	colName, ok := fieldToColumn[fieldName]
	if !ok {
		return nil, fmt.Errorf("repository: field %q is not mapped to any column", fieldName)
	}

	if tableName != "" {
		colName = tableName + "." + colName
	}

	return stmt.Comparison{
		Column: colName,
		Op:     cond.Op,
		Value:  cond.Value,
	}, nil
}

// buildWherePredicate translates []op.Condition into a single combined stmt.Predicate (AND semantics).
func (r *Repository[T]) buildWherePredicate(base any, fieldToColumn map[string]string, tableName string, conditions []op.Condition) (stmt.Predicate, error) {
	if len(conditions) == 0 {
		return nil, nil
	}
	preds := make([]stmt.Predicate, 0, len(conditions))
	for _, c := range conditions {
		p, err := r.translateCondition(base, fieldToColumn, tableName, c)
		if err != nil {
			return nil, err
		}
		if p != nil {
			preds = append(preds, p)
		}
	}
	if len(preds) == 0 {
		return nil, nil
	}
	if len(preds) == 1 {
		return preds[0], nil
	}
	return stmt.Logical{Op: "and", Predicates: preds}, nil
}

// applySoftDeleteFilter wraps a predicate with a soft-delete column check (IS NULL).
func (r *Repository[T]) applySoftDeleteFilter(tableName string, deleteDateField string, qWithDeleted bool, where stmt.Predicate) stmt.Predicate {
	if deleteDateField == "" || qWithDeleted {
		return where
	}
	f2c := r.fieldToColumn()
	colName, ok := f2c[deleteDateField]
	if !ok {
		return where
	}

	if tableName != "" {
		colName = tableName + "." + colName
	}

	sdFilter := stmt.Comparison{
		Column: colName,
		Op:     "is_null",
	}

	if where == nil {
		return sdFilter
	}

	if logical, ok := where.(stmt.Logical); ok && logical.Op == "and" {
		logical.Predicates = append([]stmt.Predicate{sdFilter}, logical.Predicates...)
		return logical
	}

	return stmt.Logical{
		Op:         "and",
		Predicates: []stmt.Predicate{sdFilter, where},
	}
}


// scanRow builds a new T and writes each declared column's value from row
// (keyed by column name) onto the corresponding struct field.
func (r *Repository[T]) scanRow(row map[string]any) (T, error) {
	var result T
	v := reflect.ValueOf(&result).Elem()

	for _, col := range r.meta.Columns {
		raw, ok := row[col.Name]
		if !ok {
			continue
		}
		field := v.FieldByName(col.FieldName)
		if !field.IsValid() || !field.CanSet() {
			continue
		}
		rawVal := reflect.ValueOf(raw)
		if !rawVal.IsValid() {
			continue
		}
		if rawVal.Type() == field.Type() {
			field.Set(rawVal)
		} else if rawVal.Type().ConvertibleTo(field.Type()) {
			field.Set(rawVal.Convert(field.Type()))
		} else {
			return result, fmt.Errorf("repository: column %q (Go value %v, type %s) is not convertible to field %s (%s)", col.Name, raw, rawVal.Type(), col.FieldName, field.Type())
		}
	}
	return result, nil
}

// Insert inserts a single entity. Triggers BeforeCreate/AfterCreate hooks,
// and OnConflictCreate in case of database integrity conflicts.
func (r *Repository[T]) Insert(ctx context.Context, i *T) (T, error) {
	var zero T
	if err := r.entity.TriggerBeforeCreate(ctx, i, r.conn); err != nil {
		return zero, err
	}

	v := reflect.ValueOf(i).Elem()

	columns := make([]string, 0, len(r.meta.Columns))
	values := make([]driver.Value, 0, len(r.meta.Columns))
	for _, col := range r.meta.Columns {
		fieldVal := v.FieldByName(col.FieldName)
		if fieldVal.IsZero() {
			continue
		}
		columns = append(columns, col.Name)
		values = append(values, fieldVal.Interface())
	}

	insPlan := &stmt.Insert{
		Table:   r.meta.TableName,
		Columns: columns,
		Values:  values,
	}

	row, err := r.conn.Dialect().Insert(ctx, r.conn, insPlan)
	if err != nil {
		if r.conn.Dialect().IsConflict(err) {
			if hookErr := r.entity.TriggerOnConflictCreate(ctx, i, r.conn); hookErr != nil {
				return zero, hookErr
			}
		}
		return zero, fmt.Errorf("repository: insert: %w", err)
	}

	res, err := r.scanRow(row)
	if err != nil {
		return zero, err
	}

	if err := r.entity.TriggerAfterCreate(ctx, &res, r.conn); err != nil {
		return zero, err
	}

	return res, nil
}

// InsertMany inserts each item via sequential Insert calls, preserving order.
func (r *Repository[T]) InsertMany(ctx context.Context, items ...*T) ([]T, error) {
	results := make([]T, 0, len(items))
	for _, item := range items {
		result, err := r.Insert(ctx, item)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

// FindMany returns all rows that match the given criteria (AND semantics).
// Without criteria it returns all rows in the table.
func (r *Repository[T]) FindMany(ctx context.Context, criteria ...func(*T, *query.Query[T])) ([]T, error) {
	var zero T
	q := query.New[T]()
	for _, fn := range criteria {
		fn(&zero, q)
	}

	wherePred, err := r.buildWherePredicate(&zero, r.fieldToColumn(), r.meta.TableName, q.Conditions())
	if err != nil {
		return nil, err
	}
	wherePred = r.applySoftDeleteFilter(r.meta.TableName, r.meta.DeleteDateField, q.IsWithDeleted(), wherePred)

	var cols []string
	if len(q.SelectFields()) > 0 {
		f2c := r.fieldToColumn()
		cols = make([]string, 0, len(q.SelectFields()))
		for _, fp := range q.SelectFields() {
			fieldName := resolveFieldPtrAny(&zero, fp)
			if col, ok := f2c[fieldName]; ok {
				cols = append(cols, r.meta.TableName+"."+col)
			}
		}
	} else if len(q.Joins()) > 0 {
		// Project all parent columns explicitly to avoid clashes
		cols = make([]string, len(r.meta.Columns))
		for i, col := range r.meta.Columns {
			cols[i] = r.meta.TableName + "." + col.Name
		}
	}

	var orderBy []stmt.OrderElement
	if len(q.OrderByFields()) > 0 {
		f2c := r.fieldToColumn()
		orderBy = make([]stmt.OrderElement, 0, len(q.OrderByFields()))
		for _, ord := range q.OrderByFields() {
			fieldName := resolveFieldPtrAny(&zero, ord.FieldPtr)
			if col, ok := f2c[fieldName]; ok {
				orderBy = append(orderBy, stmt.OrderElement{Column: r.meta.TableName + "." + col, Desc: ord.Desc})
			}
		}
	}

	var stmtJoins []stmt.Join
	if len(q.Joins()) > 0 {
		stmtJoins = make([]stmt.Join, 0, len(q.Joins()))
		for _, jd := range q.Joins() {
			ons := make([]stmt.OnCondition, 0, len(jd.OnFieldPairs))
			for _, pair := range jd.OnFieldPairs {
				var leftCol, rightCol string

				leftNameParent := resolveFieldPtrAny(&zero, pair.LeftField)
				if leftNameParent != "" {
					leftCol = r.meta.TableName + "." + r.fieldToColumn()[leftNameParent]
				} else {
					leftNameJoined := resolveFieldPtrAny(jd.Zero, pair.LeftField)
					if leftNameJoined != "" {
						leftCol = jd.TableName + "." + jd.FieldToColumn[leftNameJoined]
					} else {
						return nil, fmt.Errorf("repository: join ON clause left field pointer not found in parent or joined entity")
					}
				}

				rightNameParent := resolveFieldPtrAny(&zero, pair.RightField)
				if rightNameParent != "" {
					rightCol = r.meta.TableName + "." + r.fieldToColumn()[rightNameParent]
				} else {
					rightNameJoined := resolveFieldPtrAny(jd.Zero, pair.RightField)
					if rightNameJoined != "" {
						rightCol = jd.TableName + "." + jd.FieldToColumn[rightNameJoined]
					} else {
						return nil, fmt.Errorf("repository: join ON clause right field pointer not found in parent or joined entity")
					}
				}

				ons = append(ons, stmt.OnCondition{LeftCol: leftCol, RightCol: rightCol})
			}

			var joinWhere stmt.Predicate
			if len(jd.Conditions) > 0 {
				joinWhere, err = r.buildWherePredicate(jd.Zero, jd.FieldToColumn, jd.TableName, jd.Conditions)
				if err != nil {
					return nil, err
				}
			}

			if jd.DeleteDateField != "" && !jd.WithDeleted {
				colName := jd.FieldToColumn[jd.DeleteDateField]
				sdFilter := stmt.Comparison{
					Column: jd.TableName + "." + colName,
					Op:     "is_null",
				}
				if joinWhere == nil {
					joinWhere = sdFilter
				} else {
					if logical, ok := joinWhere.(stmt.Logical); ok && logical.Op == "and" {
						logical.Predicates = append([]stmt.Predicate{sdFilter}, logical.Predicates...)
						joinWhere = logical
					} else {
						joinWhere = stmt.Logical{
							Op:         "and",
							Predicates: []stmt.Predicate{sdFilter, joinWhere},
						}
					}
				}
			}

			stmtJoins = append(stmtJoins, stmt.Join{
				Type:  jd.Type,
				Table: jd.TableName,
				On:    ons,
				Where: joinWhere,
			})
		}
	}

	selPlan := &stmt.Select{
		Table:   r.meta.TableName,
		Columns: cols,
		Where:   wherePred,
		OrderBy: orderBy,
		Limit:   q.GetLimit(),
		Offset:  q.GetOffset(),
		Joins:   stmtJoins,
	}

	sql, args, err := r.conn.Dialect().CompileSelect(selPlan)
	if err != nil {
		return nil, fmt.Errorf("repository: compile select: %w", err)
	}

	rows, err := r.conn.Dialect().Query(ctx, r.conn, sql, args)
	if err != nil {
		return nil, fmt.Errorf("repository: find many: %w", err)
	}

	// A 1:N join fans out: one SQL row per matched child row for the same
	// parent. Since only parent columns are ever projected here, dedupe by
	// the parent's own PK so the caller sees one T per matching parent row,
	// not one per (parent, matched child) pair.
	var seen map[string]bool
	if len(q.Joins()) > 0 && len(r.meta.PrimaryKey) > 0 {
		seen = make(map[string]bool, len(rows))
	}

	results := make([]T, 0, len(rows))
	for _, row := range rows {
		if seen != nil {
			key := pkRowKey(row, r.meta.PrimaryKey)
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		item, err := r.scanRow(row)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, nil
}

// pkRowKey builds a composite dedup key from a raw result row's primary key
// column values.
func pkRowKey(row map[string]any, pkColumns []string) string {
	parts := make([]string, len(pkColumns))
	for i, col := range pkColumns {
		parts[i] = fmt.Sprintf("%v", row[col])
	}
	return strings.Join(parts, "\x00")
}

// FindOne returns the first row that matches the given criteria. Returns
// golem.ErrNotFound when no rows match.
func (r *Repository[T]) FindOne(ctx context.Context, criteria ...func(*T, *query.Query[T])) (T, error) {
	var zero T
	results, err := r.FindMany(ctx, criteria...)
	if err != nil {
		return zero, err
	}
	if len(results) == 0 {
		return zero, golem.ErrNotFound
	}
	return results[0], nil
}

// SaveOne re-persists an existing T: issues UPDATE WHERE <pk columns> SET
// <all non-PK columns>, including zero values (unlike Insert).
func (r *Repository[T]) SaveOne(ctx context.Context, i *T) (T, error) {
	var zero T
	if err := r.entity.TriggerBeforeUpdate(ctx, i, r.conn); err != nil {
		return zero, err
	}

	v := reflect.ValueOf(i).Elem()
	pkSet := r.pkColumnSet()

	setClauses := make([]stmt.UpdateClause, 0, len(r.meta.Columns))
	var pkPreds []stmt.Predicate

	for _, col := range r.meta.Columns {
		fieldVal := v.FieldByName(col.FieldName)
		if pkSet[col.Name] {
			pkPreds = append(pkPreds, stmt.Comparison{
				Column: col.Name,
				Op:     "eq",
				Value:  fieldVal.Interface(),
			})
		} else {
			setClauses = append(setClauses, stmt.UpdateClause{
				Column: col.Name,
				Value:  fieldVal.Interface(),
			})
		}
	}

	if len(pkPreds) == 0 {
		return zero, fmt.Errorf("repository: cannot save entity without primary key")
	}

	var wherePred stmt.Predicate
	if len(pkPreds) == 1 {
		wherePred = pkPreds[0]
	} else {
		wherePred = stmt.Logical{Op: "and", Predicates: pkPreds}
	}

	updPlan := &stmt.Update{
		Table: r.meta.TableName,
		Sets:  setClauses,
		Where: wherePred,
	}

	rows, err := r.conn.Dialect().Update(ctx, r.conn, updPlan)
	if err != nil {
		if r.conn.Dialect().IsConflict(err) {
			if hookErr := r.entity.TriggerOnConflictUpdate(ctx, i, r.conn); hookErr != nil {
				return zero, hookErr
			}
		}
		return zero, fmt.Errorf("repository: save one: %w", err)
	}
	if len(rows) == 0 {
		return zero, golem.ErrNotFound
	}

	res, err := r.scanRow(rows[0])
	if err != nil {
		return zero, err
	}

	if err := r.entity.TriggerAfterUpdate(ctx, &res, r.conn); err != nil {
		return zero, err
	}

	return res, nil
}

// SaveMany re-persists multiple instances via sequential SaveOne calls.
func (r *Repository[T]) SaveMany(ctx context.Context, items ...*T) ([]T, error) {
	results := make([]T, 0, len(items))
	for _, item := range items {
		result, err := r.SaveOne(ctx, item)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

// UpdateOne applies WHERE + SET criteria and returns the single updated row.
// Returns golem.ErrNotFound when no rows are affected.
func (r *Repository[T]) UpdateOne(ctx context.Context, criteria func(*T, *query.Update[T])) (T, error) {
	var zero T
	u := query.NewUpdate[T]()
	criteria(&zero, u)

	wherePred, err := r.buildWherePredicate(&zero, r.fieldToColumn(), r.meta.TableName, u.Conditions())
	if err != nil {
		return zero, err
	}
	wherePred = r.applySoftDeleteFilter(r.meta.TableName, r.meta.DeleteDateField, u.IsWithDeleted(), wherePred)

	f2c := r.fieldToColumn()
	setClauses := make([]stmt.UpdateClause, 0, len(u.Sets()))
	for _, s := range u.Sets() {
		fieldName := resolveFieldPtrAny(&zero, s.FieldPtr)
		if colName, ok := f2c[fieldName]; ok {
			setClauses = append(setClauses, stmt.UpdateClause{
				Column: colName,
				Value:  s.Value,
			})
		}
	}

	updPlan := &stmt.Update{
		Table: r.meta.TableName,
		Sets:  setClauses,
		Where: wherePred,
	}

	rows, err := r.conn.Dialect().Update(ctx, r.conn, updPlan)
	if err != nil {
		return zero, fmt.Errorf("repository: update one: %w", err)
	}
	if len(rows) == 0 {
		return zero, golem.ErrNotFound
	}
	res, err := r.scanRow(rows[0])
	if err != nil {
		return zero, err
	}
	if err := r.entity.TriggerAfterUpdate(ctx, &res, r.conn); err != nil {
		return zero, err
	}
	return res, nil
}

// UpdateMany applies WHERE + SET criteria and returns all affected rows.
// Zero rows affected is not an error.
func (r *Repository[T]) UpdateMany(ctx context.Context, criteria func(*T, *query.Update[T])) ([]T, error) {
	var zero T
	u := query.NewUpdate[T]()
	criteria(&zero, u)

	wherePred, err := r.buildWherePredicate(&zero, r.fieldToColumn(), r.meta.TableName, u.Conditions())
	if err != nil {
		return nil, err
	}
	wherePred = r.applySoftDeleteFilter(r.meta.TableName, r.meta.DeleteDateField, u.IsWithDeleted(), wherePred)

	f2c := r.fieldToColumn()
	setClauses := make([]stmt.UpdateClause, 0, len(u.Sets()))
	for _, s := range u.Sets() {
		fieldName := resolveFieldPtrAny(&zero, s.FieldPtr)
		if colName, ok := f2c[fieldName]; ok {
			setClauses = append(setClauses, stmt.UpdateClause{
				Column: colName,
				Value:  s.Value,
			})
		}
	}

	updPlan := &stmt.Update{
		Table: r.meta.TableName,
		Sets:  setClauses,
		Where: wherePred,
	}

	rows, err := r.conn.Dialect().Update(ctx, r.conn, updPlan)
	if err != nil {
		return nil, fmt.Errorf("repository: update many: %w", err)
	}

	results := make([]T, 0, len(rows))
	for _, row := range rows {
		item, err := r.scanRow(row)
		if err != nil {
			return nil, err
		}
		if err := r.entity.TriggerAfterUpdate(ctx, &item, r.conn); err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, nil
}

// cascadeActionable reports whether action requires golem to do something at
// delete time (as opposed to OnDeleteDefault/OnDeleteNoAction, which both
// mean "golem does nothing — defer to whatever real DB constraint, if any,
// exists outside golem's knowledge").
func cascadeActionable(action relation.OnDeleteAction) bool {
	switch action {
	case relation.OnDeleteCascade, relation.OnDeleteSetNull, relation.OnDeleteRestrict:
		return true
	default:
		return false
	}
}

// beginCascadeTx opens a transaction for a Delete call that has 1+
// cascade-actionable incoming FKs, so the cascade side effects and the
// parent row's own delete commit or roll back together. If conn is already
// a golem.Tx, it's reused as-is (commit/rollback become no-ops — the caller
// that opened that Tx owns its boundary). If no FK is cascade-actionable,
// r.conn is returned unchanged with no-op commit/rollback.
func (r *Repository[T]) beginCascadeTx(ctx context.Context, fkRegs []entity.FKRegistration) (golem.Conn, func() error, func(), error) {
	needsTx := false
	for _, reg := range fkRegs {
		if cascadeActionable(reg.Options.ResolvedOnDelete()) {
			needsTx = true
			break
		}
	}
	noop := func() error { return nil }
	if !needsTx {
		return r.conn, noop, func() {}, nil
	}
	if tx, ok := r.conn.(golem.Tx); ok {
		return tx, noop, func() {}, nil
	}
	txConn, err := r.conn.Dialect().Begin(ctx, r.conn)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("repository: delete: begin cascade tx: %w", err)
	}
	tx := golem.NewTx(r.conn.Dialect(), txConn)
	return tx, func() error { return tx.Commit(ctx) }, func() { _ = tx.Rollback(ctx) }, nil
}

// applyDeleteCascades runs every cascade-actionable OnDelete action
// registered against r.meta.TableName, for the row identified by pkValue.
// Restrict checks run before any Cascade/SetNull mutation, so a rejected
// delete never leaves a partial cascade behind (the caller's transaction,
// opened by beginCascadeTx, is the actual safety net either way).
func applyDeleteCascades(ctx context.Context, conn golem.Conn, fkRegs []entity.FKRegistration, pkValue any) error {
	for _, reg := range fkRegs {
		if reg.Options.ResolvedOnDelete() != relation.OnDeleteRestrict {
			continue
		}
		where := stmt.Predicate(stmt.Comparison{Column: reg.ChildColumn, Op: "eq", Value: pkValue})
		if reg.ChildDeleteDateColumn != "" {
			where = stmt.Logical{Op: "and", Predicates: []stmt.Predicate{
				where,
				stmt.Comparison{Column: reg.ChildDeleteDateColumn, Op: "is_null"},
			}}
		}
		selPlan := &stmt.Select{Table: reg.ChildTableName, Where: where, Count: true}
		sql, args, err := conn.Dialect().CompileSelect(selPlan)
		if err != nil {
			return fmt.Errorf("repository: delete: restrict check on %q: %w", reg.ChildTableName, err)
		}
		rows, err := conn.Dialect().Query(ctx, conn, sql, args)
		if err != nil {
			return fmt.Errorf("repository: delete: restrict check on %q: %w", reg.ChildTableName, err)
		}
		if len(rows) == 0 {
			continue
		}
		for _, v := range rows[0] {
			if countValue(v) > 0 {
				return golem.ErrForeignKeyViolation
			}
		}
	}

	for _, reg := range fkRegs {
		switch reg.Options.ResolvedOnDelete() {
		case relation.OnDeleteCascade:
			if reg.ChildDeleteDateColumn != "" {
				updPlan := &stmt.Update{
					Table: reg.ChildTableName,
					Sets:  []stmt.UpdateClause{{Column: reg.ChildDeleteDateColumn, Value: time.Now()}},
					Where: stmt.Comparison{Column: reg.ChildColumn, Op: "eq", Value: pkValue},
				}
				if _, err := conn.Dialect().Update(ctx, conn, updPlan); err != nil {
					return fmt.Errorf("repository: delete: cascade soft-delete %q: %w", reg.ChildTableName, err)
				}
			} else {
				delPlan := &stmt.Delete{
					Table: reg.ChildTableName,
					Where: stmt.Comparison{Column: reg.ChildColumn, Op: "eq", Value: pkValue},
				}
				sql, args, err := conn.Dialect().CompileDelete(delPlan)
				if err != nil {
					return fmt.Errorf("repository: delete: cascade delete %q: %w", reg.ChildTableName, err)
				}
				if _, err := conn.Dialect().Exec(ctx, conn, sql, args); err != nil {
					return fmt.Errorf("repository: delete: cascade delete %q: %w", reg.ChildTableName, err)
				}
			}

		case relation.OnDeleteSetNull:
			updPlan := &stmt.Update{
				Table: reg.ChildTableName,
				Sets:  []stmt.UpdateClause{{Column: reg.ChildColumn, Value: nil}},
				Where: stmt.Comparison{Column: reg.ChildColumn, Op: "eq", Value: pkValue},
			}
			if _, err := conn.Dialect().Update(ctx, conn, updPlan); err != nil {
				return fmt.Errorf("repository: delete: cascade set-null %q: %w", reg.ChildTableName, err)
			}
		}
	}
	return nil
}

// countValue normalizes the handful of integer types a COUNT(*) projection
// can come back as across dialects/drivers.
func countValue(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int:
		return int64(n)
	default:
		return 0
	}
}

// Delete deletes the given items. If the entity has soft-delete enabled,
// it performs a soft-delete (UPDATE setting the delete column to the current time);
// otherwise it runs a hard DELETE. Before the delete itself, applies every
// cascade-actionable OnDelete action (relation.OnDeleteCascade/SetNull/
// Restrict) registered by other entities' ForeignKey declarations pointing
// at this one — see entity.ForeignKeysReferencing. Triggers BeforeDelete/
// AfterDelete hooks, and OnConflictDelete on database integrity conflicts.
func (r *Repository[T]) Delete(ctx context.Context, items ...*T) error {
	if len(items) == 0 {
		return nil
	}
	pkSet := r.pkColumnSet()
	f2c := r.fieldToColumn()
	fkRegs := entity.ForeignKeysReferencing(r.meta.TableName)

	for _, item := range items {
		if err := r.entity.TriggerBeforeDelete(ctx, item, r.conn); err != nil {
			return err
		}

		v := reflect.ValueOf(item).Elem()
		var pkPreds []stmt.Predicate
		var pkValue any
		for _, col := range r.meta.Columns {
			if pkSet[col.Name] {
				fieldVal := v.FieldByName(col.FieldName)
				pkPreds = append(pkPreds, stmt.Comparison{
					Column: col.Name,
					Op:     "eq",
					Value:  fieldVal.Interface(),
				})
				pkValue = fieldVal.Interface()
			}
		}
		if len(pkPreds) == 0 {
			return fmt.Errorf("repository: cannot delete entity without primary key")
		}
		var wherePred stmt.Predicate
		if len(pkPreds) == 1 {
			wherePred = pkPreds[0]
		} else {
			wherePred = stmt.Logical{Op: "and", Predicates: pkPreds}
		}

		conn, commit, rollback, err := r.beginCascadeTx(ctx, fkRegs)
		if err != nil {
			return err
		}

		if err := applyDeleteCascades(ctx, conn, fkRegs, pkValue); err != nil {
			rollback()
			return err
		}

		var delErr error
		if r.meta.DeleteDateField != "" {
			colName, ok := f2c[r.meta.DeleteDateField]
			if !ok {
				rollback()
				return fmt.Errorf("repository: delete date field %q is not mapped to any column", r.meta.DeleteDateField)
			}
			updPlan := &stmt.Update{
				Table: r.meta.TableName,
				Sets: []stmt.UpdateClause{
					{Column: colName, Value: time.Now()},
				},
				Where: wherePred,
			}
			_, delErr = conn.Dialect().Update(ctx, conn, updPlan)
		} else {
			delPlan := &stmt.Delete{
				Table: r.meta.TableName,
				Where: wherePred,
			}
			var sql string
			var args []any
			sql, args, delErr = conn.Dialect().CompileDelete(delPlan)
			if delErr == nil {
				_, delErr = conn.Dialect().Exec(ctx, conn, sql, args)
			}
		}

		if delErr != nil {
			rollback()
			if r.conn.Dialect().IsConflict(delErr) {
				if hookErr := r.entity.TriggerOnConflictDelete(ctx, item, r.conn); hookErr != nil {
					return hookErr
				}
			}
			return fmt.Errorf("repository: delete: %w", delErr)
		}

		if err := commit(); err != nil {
			return fmt.Errorf("repository: delete: commit cascade: %w", err)
		}

		if err := r.entity.TriggerAfterDelete(ctx, item, r.conn); err != nil {
			return err
		}
	}
	return nil
}

// Restore soft-restores (sets soft-delete column back to NULL) the given items.
// Returns an error if soft-delete is not configured.
func (r *Repository[T]) Restore(ctx context.Context, items ...*T) error {
	if r.meta.DeleteDateField == "" {
		return fmt.Errorf("repository: cannot restore entity %q without a soft-delete field", r.meta.TableName)
	}
	if len(items) == 0 {
		return nil
	}
	pkSet := r.pkColumnSet()
	f2c := r.fieldToColumn()
	colName, ok := f2c[r.meta.DeleteDateField]
	if !ok {
		return fmt.Errorf("repository: delete date field %q is not mapped to any column", r.meta.DeleteDateField)
	}

	for _, item := range items {
		v := reflect.ValueOf(item).Elem()
		var pkPreds []stmt.Predicate
		for _, col := range r.meta.Columns {
			if pkSet[col.Name] {
				fieldVal := v.FieldByName(col.FieldName)
				pkPreds = append(pkPreds, stmt.Comparison{
					Column: col.Name,
					Op:     "eq",
					Value:  fieldVal.Interface(),
				})
			}
		}
		if len(pkPreds) == 0 {
			return fmt.Errorf("repository: cannot restore entity without primary key")
		}
		var wherePred stmt.Predicate
		if len(pkPreds) == 1 {
			wherePred = pkPreds[0]
		} else {
			wherePred = stmt.Logical{Op: "and", Predicates: pkPreds}
		}

		updPlan := &stmt.Update{
			Table: r.meta.TableName,
			Sets: []stmt.UpdateClause{
				{Column: colName, Value: nil},
			},
			Where: wherePred,
		}
		_, err := r.conn.Dialect().Update(ctx, r.conn, updPlan)
		if err != nil {
			return fmt.Errorf("repository: restore: %w", err)
		}
	}
	return nil
}

// Count returns the number of entities matching the criteria.
func (r *Repository[T]) Count(ctx context.Context, criteria ...func(*T, *query.Count[T])) (int64, error) {
	var zero T
	c := query.NewCount[T]()
	for _, fn := range criteria {
		fn(&zero, c)
	}

	wherePred, err := r.buildWherePredicate(&zero, r.fieldToColumn(), r.meta.TableName, c.Conditions())
	if err != nil {
		return 0, err
	}
	wherePred = r.applySoftDeleteFilter(r.meta.TableName, r.meta.DeleteDateField, c.IsWithDeleted(), wherePred)

	selPlan := &stmt.Select{
		Table: r.meta.TableName,
		Where: wherePred,
		Count: true,
	}

	sql, args, err := r.conn.Dialect().CompileSelect(selPlan)
	if err != nil {
		return 0, fmt.Errorf("repository: compile count: %w", err)
	}

	rows, err := r.conn.Dialect().Query(ctx, r.conn, sql, args)
	if err != nil {
		return 0, fmt.Errorf("repository: count query: %w", err)
	}

	if len(rows) == 0 {
		return 0, nil
	}

	for _, v := range rows[0] {
		switch val := v.(type) {
		case int64:
			return val, nil
		case int32:
			return int64(val), nil
		case int:
			return int64(val), nil
		}
	}
	return 0, fmt.Errorf("repository: count: failed to parse result from row: %v", rows[0])
}

// Exists checks if any entity matches the criteria.
func (r *Repository[T]) Exists(ctx context.Context, criteria ...func(*T, *query.Count[T])) (bool, error) {
	var zero T
	c := query.NewCount[T]()
	for _, fn := range criteria {
		fn(&zero, c)
	}

	wherePred, err := r.buildWherePredicate(&zero, r.fieldToColumn(), r.meta.TableName, c.Conditions())
	if err != nil {
		return false, err
	}
	wherePred = r.applySoftDeleteFilter(r.meta.TableName, r.meta.DeleteDateField, c.IsWithDeleted(), wherePred)

	limit := 1
	selPlan := &stmt.Select{
		Table: r.meta.TableName,
		Where: wherePred,
		Limit: &limit,
		Count: true,
	}

	sql, args, err := r.conn.Dialect().CompileSelect(selPlan)
	if err != nil {
		return false, fmt.Errorf("repository: compile exists: %w", err)
	}

	rows, err := r.conn.Dialect().Query(ctx, r.conn, sql, args)
	if err != nil {
		return false, fmt.Errorf("repository: exists query: %w", err)
	}

	if len(rows) == 0 {
		return false, nil
	}

	for _, v := range rows[0] {
		switch val := v.(type) {
		case int64:
			return val > 0, nil
		case int32:
			return val > 0, nil
		case int:
			return val > 0, nil
		}
	}
	return false, nil
}

// Exec executes a raw SQL query and maps the returned rows to a slice of T.
func (r *Repository[T]) Exec(ctx context.Context, sql string, args ...any) ([]T, error) {
	res, err := r.conn.Exec(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	var results []T
	for res.Next() {
		row, err := res.Scan()
		if err != nil {
			return nil, err
		}
		item, err := r.scanRow(row)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, nil
}

