package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
	"github.com/leandroluk/golem/internal/stmt"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
)

type aggOrder struct {
	ID       int64
	Category string
	Amount   float64
}

var aggOrderEntity = entity.New(func(o *aggOrder, b *entity.Table) {
	b.TableName("aggorder")
	b.Col(&o.ID, golem.BIGINT())
	b.Col(&o.Category, golem.VARCHAR(50))
	b.Col(&o.Amount, golem.DECIMAL(10, 2))
	b.PrimaryKey(&o.ID)
})

type aggOrderTotal struct {
	Category string
	Total    float64
	Avg      float64
	Cnt      int64
	All      int64
}

func TestAggregate_GroupBySumAvgCount_ScansResults(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{
			{"agg_0": "food", "agg_1": float64(150.5), "agg_2": float64(75.25), "agg_3": int64(2), "agg_4": int64(2)},
			{"agg_0": "drinks", "agg_1": float64(30), "agg_2": float64(30), "agg_3": int64(1), "agg_4": int64(1)},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	results, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
		a.Sum(&o.Amount, &res.Total)
		a.Avg(&o.Amount, &res.Avg)
		a.Count(&o.Amount, &res.Cnt)
		a.CountAll(&res.All)
	})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Category != "food" || results[0].Total != 150.5 || results[0].Avg != 75.25 || results[0].Cnt != 2 || results[0].All != 2 {
		t.Errorf("results[0] = %+v, unexpected", results[0])
	}

	call := d.selectCalls[0]
	if call.Table != "aggorder" {
		t.Errorf("table = %q, want aggorder", call.Table)
	}
	if len(call.GroupBy) != 1 || call.GroupBy[0] != "category" {
		t.Errorf("GroupBy = %v, want [category]", call.GroupBy)
	}
	if len(call.Projections) != 5 {
		t.Fatalf("expected 5 projections, got %d", len(call.Projections))
	}
	if call.Projections[0].Column != "category" || call.Projections[0].Func != "" {
		t.Errorf("projection[0] = %+v, want GroupBy category", call.Projections[0])
	}
	if call.Projections[1].Column != "amount" || call.Projections[1].Func != "sum" {
		t.Errorf("projection[1] = %+v, want sum(amount)", call.Projections[1])
	}
	if call.Projections[4].Func != "count_all" {
		t.Errorf("projection[4] = %+v, want count_all", call.Projections[4])
	}
}

func TestAggregate_Where_AppliesFilterAndSoftDelete(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
		a.Sum(&o.Amount, &res.Total)
		a.Where(op.Eq(&o.Category, "food"))
	})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	comp, ok := d.selectCalls[0].Where.(stmt.Comparison)
	if !ok || comp.Column != "category" || comp.Op != "eq" || comp.Value != "food" {
		t.Errorf("unexpected Where: %+v", d.selectCalls[0].Where)
	}
}

func TestAggregate_Having_TranslatesToAggregateComparison(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
		a.Sum(&o.Amount, &res.Total)
		a.Having(op.Gt(&res.Total, 100.0))
	})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	having, ok := d.selectCalls[0].Having.(stmt.AggregateComparison)
	if !ok || having.Func != "sum" || having.Column != "amount" || having.Op != "gt" || having.Value != 100.0 {
		t.Errorf("unexpected Having: %+v", d.selectCalls[0].Having)
	}
}

func TestAggregate_Having_MultipleConditions_CombinesWithAnd(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
		a.Sum(&o.Amount, &res.Total)
		a.Count(&o.Amount, &res.Cnt)
		a.Having(op.Gt(&res.Total, 100.0), op.Gt(&res.Cnt, int64(1)))
	})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	logical, ok := d.selectCalls[0].Having.(stmt.Logical)
	if !ok || logical.Op != "and" || len(logical.Predicates) != 2 {
		t.Fatalf("expected logical AND of 2 having predicates, got %+v", d.selectCalls[0].Having)
	}
}

func TestAggregate_Having_UnregisteredField_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
		a.Having(op.Gt(&res.Total, 100.0)) // Total was never registered via Sum/Avg/Count/CountAll
	})
	if err == nil {
		t.Fatal("expected error for Having on an unregistered aggregate field, got nil")
	}
}

func TestAggregate_OrderBy_ByAggregateField(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
		a.Sum(&o.Amount, &res.Total)
		a.OrderBy(op.Desc(&res.Total))
	})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	orderBy := d.selectCalls[0].OrderBy
	if len(orderBy) != 1 || orderBy[0].Column != "agg_1" || !orderBy[0].Desc {
		t.Errorf("unexpected OrderBy: %+v", orderBy)
	}
}

func TestAggregate_OrderBy_ByGroupByField(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
		a.Sum(&o.Amount, &res.Total)
		a.OrderBy(op.Asc(&res.Category))
	})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	orderBy := d.selectCalls[0].OrderBy
	if len(orderBy) != 1 || orderBy[0].Column != "agg_0" || orderBy[0].Desc {
		t.Errorf("unexpected OrderBy: %+v", orderBy)
	}
}

func TestAggregate_OrderBy_UnregisteredField_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
		a.OrderBy(op.Desc(&res.Total)) // Total was never registered
	})
	if err == nil {
		t.Fatal("expected error for OrderBy on an unregistered field, got nil")
	}
}

func TestAggregate_LimitOffset(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
		a.Sum(&o.Amount, &res.Total)
		a.Limit(5)
		a.Offset(2)
	})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	call := d.selectCalls[0]
	if call.Limit == nil || *call.Limit != 5 {
		t.Errorf("Limit = %v, want 5", call.Limit)
	}
	if call.Offset == nil || *call.Offset != 2 {
		t.Errorf("Offset = %v, want 2", call.Offset)
	}
}

func TestAggregate_BadAvgSourceFieldPtr_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	var foreign float64
	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.Avg(&foreign, &res.Avg)
	})
	if err == nil {
		t.Fatal("expected error for an Avg source field pointer that doesn't belong to T, got nil")
	}
}

func TestAggregate_BadCountSourceFieldPtr_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	var foreign float64
	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.Count(&foreign, &res.Cnt)
	})
	if err == nil {
		t.Fatal("expected error for a Count source field pointer that doesn't belong to T, got nil")
	}
}

func TestAggregate_BadWhereFieldPtr_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	var foreign string
	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
		a.Where(op.Eq(&foreign, "x"))
	})
	if err == nil {
		t.Fatal("expected error for a Where field pointer that doesn't belong to T, got nil")
	}
}

func TestAggregate_RowMissingAlias_SkipsField(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{
			{"agg_0": "food"}, // agg_1 (Total) intentionally absent
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	results, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
		a.Sum(&o.Amount, &res.Total)
	})
	if err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if len(results) != 1 || results[0].Category != "food" || results[0].Total != 0 {
		t.Errorf("results = %+v, want [{Category:food Total:0}]", results)
	}
}

func TestAggregate_BadGroupBySourceFieldPtr_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	var foreign string
	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&foreign, &res.Category)
	})
	if err == nil {
		t.Fatal("expected error for a GroupBy source field pointer that doesn't belong to T, got nil")
	}
}

func TestAggregate_BadSourceFieldPtr_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	var foreign float64
	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.Sum(&foreign, &res.Total)
	})
	if err == nil {
		t.Fatal("expected error for a source field pointer that doesn't belong to T, got nil")
	}
}

func TestAggregate_BadDestFieldPtr_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	var foreign float64
	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.Sum(&o.Amount, &foreign)
	})
	if err == nil {
		t.Fatal("expected error for a dest field pointer that doesn't belong to R, got nil")
	}
}

func TestAggregate_BadCountAllDestFieldPtr_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	var foreign int64
	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.CountAll(&foreign)
	})
	if err == nil {
		t.Fatal("expected error for a CountAll dest field pointer that doesn't belong to R, got nil")
	}
}

func TestAggregate_UnmappedSourceField_ReturnsError(t *testing.T) {
	type unmappedSource struct {
		ID     int64
		Amount float64 // never declared via Col
	}
	unmappedEntity := entity.New(func(u *unmappedSource, b *entity.Table) {
		b.TableName("aggunmapped")
		b.Col(&u.ID, golem.BIGINT())
		b.PrimaryKey(&u.ID)
	})

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, unmappedEntity)

	_, err := Aggregate(context.Background(), repo, func(u *unmappedSource, res *aggOrderTotal, a *query.Aggregate[unmappedSource, aggOrderTotal]) {
		a.Sum(&u.Amount, &res.Total)
	})
	if err == nil {
		t.Fatal("expected error for a field not mapped to any column, got nil")
	}
}

func TestAggregate_CompileSelectError_Propagates(t *testing.T) {
	wantErr := errors.New("boom: compile")
	d := &fakeDialect{selectErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

func TestAggregate_QueryError_Propagates(t *testing.T) {
	wantErr := errors.New("boom: query")
	d := &fakeDialect{queryErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

func TestAggregate_ScanError_Propagates(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{
			{"agg_0": struct{}{}}, // unconvertible to string
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, aggOrderEntity)

	_, err := Aggregate(context.Background(), repo, func(o *aggOrder, res *aggOrderTotal, a *query.Aggregate[aggOrder, aggOrderTotal]) {
		a.GroupBy(&o.Category, &res.Category)
	})
	if err == nil {
		t.Fatal("expected scan error for unconvertible column value, got nil")
	}
}
