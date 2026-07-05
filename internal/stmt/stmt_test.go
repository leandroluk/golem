package stmt

import (
	"reflect"
	"testing"
)

func TestComparison_ImplementsPredicate(t *testing.T) {
	c := Comparison{Column: "age", Op: "eq", Value: 30}
	var p Predicate = c
	p.isPredicate()

	if reflect.TypeOf(p).Name() != "Comparison" {
		t.Fatalf("expected Comparison, got %T", p)
	}
}

func TestLogical_ImplementsPredicate(t *testing.T) {
	l := Logical{
		Op: "and",
		Predicates: []Predicate{
			Comparison{Column: "age", Op: "gt", Value: 18},
			Comparison{Column: "status", Op: "eq", Value: "active"},
		},
	}
	var p Predicate = l
	p.isPredicate()

	if reflect.TypeOf(p).Name() != "Logical" {
		t.Fatalf("expected Logical, got %T", p)
	}
}

func TestNot_ImplementsPredicate(t *testing.T) {
	n := Not{
		Predicate: Comparison{Column: "deleted", Op: "eq", Value: true},
	}
	var p Predicate = n
	p.isPredicate()

	if reflect.TypeOf(p).Name() != "Not" {
		t.Fatalf("expected Not, got %T", p)
	}
}

func TestAggregateComparison_ImplementsPredicate(t *testing.T) {
	a := AggregateComparison{Func: "sum", Column: "amount", Op: "gt", Value: 100}
	var p Predicate = a
	p.isPredicate()

	if reflect.TypeOf(p).Name() != "AggregateComparison" {
		t.Fatalf("expected AggregateComparison, got %T", p)
	}
}

