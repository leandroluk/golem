package repository

import (
	"context"
	"fmt"
	"reflect"

	"github.com/leandroluk/golem/entity"
	"github.com/leandroluk/golem/internal/stmt"
	"github.com/leandroluk/golem/query"
)

// Preload fetches every J row related to items (a parent Repository[T]'s
// FindMany/FindOne result), grouped by the parent row's primary key value,
// and returns that grouping as a plain map — it never attaches related rows
// onto items themselves. There's no navigational collection field to attach
// to by design (AD-001/AD-024): "eager loading" in golem is a query-level
// concern, not a struct shape one.
//
// The join column is discovered automatically from the FK registry entity.
// New/ForeignKey populates (see entity.ForeignKeysReferencing) — target
// must have a ForeignKey declared against r's entity (in either direction:
// either target references r's table, or r's own entity references
// target's table; see design.md for why both directions are supported).
// r's entity must have a single-column primary key.
func Preload[T, J any](ctx context.Context, r *Repository[T], items []T, target *entity.Entity[J], criteria ...func(*J, *query.Query[J])) (map[any][]J, error) {
	if len(items) == 0 {
		return map[any][]J{}, nil
	}

	childRepo := Get(r.conn, target)
	childMeta := childRepo.meta

	fkCol, parentKeyCol, _, err := resolvePreloadJoin(r.meta, childMeta)
	if err != nil {
		return nil, err
	}

	parentPKField := ""
	for _, col := range r.meta.Columns {
		if col.Name == parentKeyCol {
			parentPKField = col.FieldName
			break
		}
	}

	keys := make([]any, 0, len(items))
	seenKeys := make(map[any]bool, len(items))
	for i := range items {
		v := reflect.ValueOf(items[i]).FieldByName(parentPKField)
		key := v.Interface()
		if !seenKeys[key] {
			seenKeys[key] = true
			keys = append(keys, key)
		}
	}

	var zero J
	q := query.New[J]()
	for _, fn := range criteria {
		fn(&zero, q)
	}
	extraWhere, err := childRepo.buildWherePredicate(&zero, childRepo.fieldToColumn(), "", q.Conditions())
	if err != nil {
		return nil, err
	}

	// fkCol is always the join column as it appears on the FETCHED
	// (target/J) side — whether target is the "many" side (fkCol is
	// target's own FK column, e.g. Post.owner_user_id) or the "one" side
	// (fkCol is target's PK, e.g. User.id, when r/T is the one holding the
	// ForeignKey). Either way it's both the IN-filter column AND the
	// column every returned row can be grouped by.
	inPred := stmt.Predicate(stmt.Comparison{Column: fkCol, Op: "in", Value: keys})
	where := inPred
	if extraWhere != nil {
		where = stmt.Logical{Op: "and", Predicates: []stmt.Predicate{inPred, extraWhere}}
	}
	where = childRepo.applySoftDeleteFilter(childMeta.TableName, childMeta.DeleteDateField, q.IsWithDeleted(), where)

	var orderBy []stmt.OrderElement
	for _, ord := range q.OrderByFields() {
		fieldName := resolveFieldPtrAny(&zero, ord.FieldPtr)
		if col, ok := childRepo.fieldToColumn()[fieldName]; ok {
			orderBy = append(orderBy, stmt.OrderElement{Column: col, Desc: ord.Desc})
		}
	}

	selPlan := &stmt.Select{
		Table:   childMeta.TableName,
		Where:   where,
		OrderBy: orderBy,
		Limit:   q.GetLimit(),
		Offset:  q.GetOffset(),
	}

	sql, args, err := childRepo.conn.Dialect().CompileSelect(selPlan)
	if err != nil {
		return nil, fmt.Errorf("repository: preload: compile select: %w", err)
	}
	rows, err := childRepo.conn.Dialect().Query(ctx, childRepo.conn, sql, args)
	if err != nil {
		return nil, fmt.Errorf("repository: preload: %w", err)
	}

	result := make(map[any][]J, len(keys))
	for _, row := range rows {
		item, err := childRepo.scanRow(row)
		if err != nil {
			return nil, err
		}
		result[row[fkCol]] = append(result[row[fkCol]], item)
	}
	return result, nil
}

// resolvePreloadJoin finds the single FK relationship between parentMeta
// (the entity items belong to) and childMeta (target's entity), in either
// direction, using entity.ForeignKeysReferencing. Returns: fkCol (the
// join column as it appears on childMeta's table — always present on every
// fetched row, used both for the IN-filter and for grouping results),
// parentKeyCol (the column, on parentMeta's OWN table, whose values become
// the IN-filter's key list — parentMeta's PK when childMeta holds the
// ForeignKey, or parentMeta's own FK column when parentMeta holds it
// instead), childSide (true in the first case, false in the second).

func resolvePreloadJoin(parentMeta, childMeta entity.EntityMeta) (fkCol string, parentKeyCol string, childSide bool, err error) {
	if len(parentMeta.PrimaryKey) != 1 {
		return "", "", false, fmt.Errorf("repository: preload: %q must have a single-column primary key", parentMeta.TableName)
	}

	for _, reg := range entity.ForeignKeysReferencing(parentMeta.TableName) {
		if reg.ChildTableName == childMeta.TableName {
			return reg.ChildColumn, parentMeta.PrimaryKey[0], true, nil
		}
	}

	if len(childMeta.PrimaryKey) == 1 {
		for _, reg := range entity.ForeignKeysReferencing(childMeta.TableName) {
			if reg.ChildTableName == parentMeta.TableName {
				return childMeta.PrimaryKey[0], reg.ChildColumn, false, nil
			}
		}
	}

	return "", "", false, fmt.Errorf("repository: preload: no ForeignKey is registered between %q and %q", parentMeta.TableName, childMeta.TableName)
}
