package repository

import (
	"context"
	"fmt"
	"reflect"

	"github.com/leandroluk/golem/entity"
	"github.com/leandroluk/golem/internal/scanner"
	"github.com/leandroluk/golem/internal/stmt"
	"github.com/leandroluk/golem/query"
)

// aggProjection is one resolved GROUP BY column or aggregate expression:
// which T column it reads from (empty for CountAll), which SQL function (if
// any) wraps it, the SQL alias it's projected under, and which R field the
// result gets written to.
type aggProjection struct {
	column          string // "" for count_all
	fn              string // "", "sum", "avg", "count", "count_all"
	alias           string
	resultFieldName string
}

// Aggregate runs a GROUP BY/aggregate query against T's table and scans the
// results into R — an arbitrary plain struct (not necessarily an
// entity.Entity) whose fields are resolved by pointer offset against a zero
// R value, same mechanism as everywhere else in golem. R is never attached
// onto T (there's nothing to attach onto — this is a read-only, shape-
// changing query, unlike Preload).
func Aggregate[T, R any](ctx context.Context, r *Repository[T], fn func(t *T, res *R, a *query.Aggregate[T, R])) ([]R, error) {
	var zeroT T
	var zeroR R
	a := query.NewAggregate[T, R]()
	fn(&zeroT, &zeroR, a)

	f2c := r.fieldToColumn()

	var projs []aggProjection
	groupByCols := make([]string, 0, len(a.GroupByMappings()))

	resolveMapping := func(m query.AggMapping, kind string) (aggProjection, error) {
		srcField := resolveFieldPtrAny(&zeroT, m.SourceFieldPtr)
		if srcField == "" {
			return aggProjection{}, fmt.Errorf("repository: aggregate: source field pointer does not belong to entity struct")
		}
		col, ok := f2c[srcField]
		if !ok {
			return aggProjection{}, fmt.Errorf("repository: aggregate: field %q is not mapped to any column", srcField)
		}
		dstField := resolveFieldPtrAny(&zeroR, m.DestFieldPtr)
		if dstField == "" {
			return aggProjection{}, fmt.Errorf("repository: aggregate: dest field pointer does not belong to result struct")
		}
		return aggProjection{column: col, fn: kind, resultFieldName: dstField}, nil
	}

	idx := 0
	nextAlias := func() string {
		alias := fmt.Sprintf("agg_%d", idx)
		idx++
		return alias
	}

	for _, m := range a.GroupByMappings() {
		p, err := resolveMapping(m, "")
		if err != nil {
			return nil, err
		}
		p.alias = nextAlias()
		projs = append(projs, p)
		groupByCols = append(groupByCols, p.column)
	}
	for _, m := range a.SumMappings() {
		p, err := resolveMapping(m, "sum")
		if err != nil {
			return nil, err
		}
		p.alias = nextAlias()
		projs = append(projs, p)
	}
	for _, m := range a.AvgMappings() {
		p, err := resolveMapping(m, "avg")
		if err != nil {
			return nil, err
		}
		p.alias = nextAlias()
		projs = append(projs, p)
	}
	for _, m := range a.CountMappings() {
		p, err := resolveMapping(m, "count")
		if err != nil {
			return nil, err
		}
		p.alias = nextAlias()
		projs = append(projs, p)
	}
	for _, destPtr := range a.CountAllFields() {
		dstField := resolveFieldPtrAny(&zeroR, destPtr)
		if dstField == "" {
			return nil, fmt.Errorf("repository: aggregate: dest field pointer does not belong to result struct")
		}
		projs = append(projs, aggProjection{fn: "count_all", alias: nextAlias(), resultFieldName: dstField})
	}

	aggByField := make(map[string]aggProjection, len(projs))
	for _, p := range projs {
		if p.fn != "" {
			aggByField[p.resultFieldName] = p
		}
	}
	groupByField := make(map[string]aggProjection, len(projs))
	for _, p := range projs {
		if p.fn == "" {
			groupByField[p.resultFieldName] = p
		}
	}

	wherePred, err := r.buildWherePredicate(&zeroT, f2c, "", a.Conditions())
	if err != nil {
		return nil, err
	}
	wherePred = r.applySoftDeleteFilter("", r.meta.DeleteDateField, a.IsWithDeleted(), wherePred)

	var havingPred stmt.Predicate
	for _, cond := range a.HavingConditions() {
		rField := resolveFieldPtrAny(&zeroR, cond.FieldPtr)
		p, ok := aggByField[rField]
		if !ok {
			return nil, fmt.Errorf("repository: aggregate: having field %q is not a registered aggregate (Sum/Avg/Count/CountAll)", rField)
		}
		cmp := stmt.AggregateComparison{Func: p.fn, Column: p.column, Op: cond.Op, Value: cond.Value}
		if havingPred == nil {
			havingPred = cmp
		} else {
			havingPred = stmt.Logical{Op: "and", Predicates: []stmt.Predicate{havingPred, cmp}}
		}
	}

	var orderBy []stmt.OrderElement
	for _, ord := range a.OrderByFields() {
		rField := resolveFieldPtrAny(&zeroR, ord.FieldPtr)
		if p, ok := aggByField[rField]; ok {
			orderBy = append(orderBy, stmt.OrderElement{Column: p.alias, Desc: ord.Desc})
			continue
		}
		if p, ok := groupByField[rField]; ok {
			orderBy = append(orderBy, stmt.OrderElement{Column: p.alias, Desc: ord.Desc})
			continue
		}
		return nil, fmt.Errorf("repository: aggregate: order by field %q is neither a GroupBy nor an aggregate field", rField)
	}

	stmtProjs := make([]stmt.Projection, len(projs))
	for i, p := range projs {
		stmtProjs[i] = stmt.Projection{Column: p.column, Func: p.fn, Alias: p.alias}
	}

	selPlan := &stmt.Select{
		Table:       r.meta.TableName,
		Projections: stmtProjs,
		Where:       wherePred,
		GroupBy:     groupByCols,
		Having:      havingPred,
		OrderBy:     orderBy,
		Limit:       a.GetLimit(),
		Offset:      a.GetOffset(),
	}

	sql, args, err := r.conn.Dialect().CompileSelect(selPlan)
	if err != nil {
		return nil, fmt.Errorf("repository: aggregate: compile select: %w", err)
	}
	rows, err := r.conn.Dialect().Query(ctx, r.conn, sql, args)
	if err != nil {
		return nil, fmt.Errorf("repository: aggregate: %w", err)
	}

	cols := make([]entity.ColumnMeta, 0, len(projs))
	rType := reflect.TypeOf(new(R)).Elem()
	for _, p := range projs {
		if sf, ok := rType.FieldByName(p.resultFieldName); ok {
			cols = append(cols, entity.ColumnMeta{
				Name:      p.alias,
				FieldName: p.resultFieldName,
				GoType:    sf.Type,
				Offset:    sf.Offset,
			})
		}
	}
	plan := scanner.Compile(entity.EntityMeta{Columns: cols}, r.conn.Parser())

	results := make([]R, 0, len(rows))
	for _, row := range rows {
		item, err := scanner.ScanFromMap[R](plan, row)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, nil
}
