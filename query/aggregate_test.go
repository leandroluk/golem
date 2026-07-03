package query

import (
	"testing"

	"github.com/leandroluk/golem/op"
)

type aggTestSource struct {
	Category string
	Amount   float64
}

type aggTestResult struct {
	Category string
	Total    float64
	Count    int64
}

func TestNewAggregate_IsEmpty(t *testing.T) {
	a := NewAggregate[aggTestSource, aggTestResult]()
	if len(a.GroupByMappings()) != 0 {
		t.Error("expected empty GroupByMappings")
	}
	if len(a.SumMappings()) != 0 {
		t.Error("expected empty SumMappings")
	}
	if len(a.AvgMappings()) != 0 {
		t.Error("expected empty AvgMappings")
	}
	if len(a.CountMappings()) != 0 {
		t.Error("expected empty CountMappings")
	}
	if len(a.CountAllFields()) != 0 {
		t.Error("expected empty CountAllFields")
	}
	if len(a.Conditions()) != 0 {
		t.Error("expected empty Conditions")
	}
	if len(a.HavingConditions()) != 0 {
		t.Error("expected empty HavingConditions")
	}
	if len(a.OrderByFields()) != 0 {
		t.Error("expected empty OrderByFields")
	}
	if a.GetLimit() != nil {
		t.Error("expected nil Limit")
	}
	if a.GetOffset() != nil {
		t.Error("expected nil Offset")
	}
	if a.IsWithDeleted() {
		t.Error("expected WithDeleted = false")
	}
}

func TestAggregate_GroupBy_Accumulates(t *testing.T) {
	var s aggTestSource
	var res aggTestResult
	a := NewAggregate[aggTestSource, aggTestResult]()
	a.GroupBy(&s.Category, &res.Category)
	if len(a.GroupByMappings()) != 1 {
		t.Fatalf("expected 1 GroupBy mapping, got %d", len(a.GroupByMappings()))
	}
	if a.GroupByMappings()[0].SourceFieldPtr != &s.Category || a.GroupByMappings()[0].DestFieldPtr != &res.Category {
		t.Error("unexpected GroupBy mapping pointers")
	}
}

func TestAggregate_Sum_Avg_Count_CountAll(t *testing.T) {
	var s aggTestSource
	var res aggTestResult
	a := NewAggregate[aggTestSource, aggTestResult]()
	a.Sum(&s.Amount, &res.Total)
	a.Avg(&s.Amount, &res.Total)
	a.Count(&s.Amount, &res.Count)
	a.CountAll(&res.Count)

	if len(a.SumMappings()) != 1 {
		t.Errorf("expected 1 Sum mapping, got %d", len(a.SumMappings()))
	}
	if len(a.AvgMappings()) != 1 {
		t.Errorf("expected 1 Avg mapping, got %d", len(a.AvgMappings()))
	}
	if len(a.CountMappings()) != 1 {
		t.Errorf("expected 1 Count mapping, got %d", len(a.CountMappings()))
	}
	if len(a.CountAllFields()) != 1 || a.CountAllFields()[0] != &res.Count {
		t.Errorf("expected 1 CountAll field pointing at res.Count, got %+v", a.CountAllFields())
	}
}

func TestAggregate_Where_Having_OrderBy_Accumulate(t *testing.T) {
	var s aggTestSource
	var res aggTestResult
	a := NewAggregate[aggTestSource, aggTestResult]()
	a.Where(op.Eq(&s.Category, "food"))
	a.Where(op.Gt(&s.Amount, 0.0))
	a.Having(op.Gt(&res.Total, 100.0))
	a.OrderBy(op.Desc(&res.Total))

	if len(a.Conditions()) != 2 {
		t.Errorf("expected 2 Where conditions, got %d", len(a.Conditions()))
	}
	if len(a.HavingConditions()) != 1 {
		t.Errorf("expected 1 Having condition, got %d", len(a.HavingConditions()))
	}
	if len(a.OrderByFields()) != 1 || !a.OrderByFields()[0].Desc {
		t.Errorf("expected 1 descending OrderBy field, got %+v", a.OrderByFields())
	}
}

func TestAggregate_LimitOffsetWithDeleted(t *testing.T) {
	a := NewAggregate[aggTestSource, aggTestResult]()
	a.Limit(10)
	a.Offset(5)
	a.WithDeleted()

	if a.GetLimit() == nil || *a.GetLimit() != 10 {
		t.Errorf("Limit = %v, want 10", a.GetLimit())
	}
	if a.GetOffset() == nil || *a.GetOffset() != 5 {
		t.Errorf("Offset = %v, want 5", a.GetOffset())
	}
	if !a.IsWithDeleted() {
		t.Error("expected WithDeleted = true")
	}
}

func TestAggregate_ChainReturnsSameInstance(t *testing.T) {
	var s aggTestSource
	var res aggTestResult
	a := NewAggregate[aggTestSource, aggTestResult]()
	got := a.GroupBy(&s.Category, &res.Category).
		Sum(&s.Amount, &res.Total).
		Avg(&s.Amount, &res.Total).
		Count(&s.Amount, &res.Count).
		CountAll(&res.Count).
		Where(op.Eq(&s.Category, "food")).
		Having(op.Gt(&res.Total, 1.0)).
		OrderBy(op.Asc(&res.Category)).
		Limit(1).
		Offset(1).
		WithDeleted()
	if got != a {
		t.Error("chained calls should all return the same *Aggregate instance")
	}
}
