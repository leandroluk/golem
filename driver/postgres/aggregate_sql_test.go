package postgres

import (
	"testing"

	"github.com/leandroluk/golem/internal/stmt"
)

func TestCompileSelect_Aggregate_GroupBySumAvgCount(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "orders",
		Projections: []stmt.Projection{
			{Column: "category", Func: "", Alias: "agg_0"},
			{Column: "amount", Func: "sum", Alias: "agg_1"},
			{Column: "amount", Func: "avg", Alias: "agg_2"},
			{Column: "id", Func: "count", Alias: "agg_3"},
			{Func: "count_all", Alias: "agg_4"},
		},
		GroupBy: []string{"category"},
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT "category" AS "agg_0", CAST(SUM("amount") AS DOUBLE PRECISION) AS "agg_1", CAST(AVG("amount") AS DOUBLE PRECISION) AS "agg_2", COUNT("id") AS "agg_3", COUNT(*) AS "agg_4" FROM "orders" GROUP BY "category"`
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
	if len(args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(args))
	}
}

func TestCompileSelect_Aggregate_WithHaving(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "orders",
		Projections: []stmt.Projection{
			{Column: "category", Alias: "agg_0"},
			{Column: "amount", Func: "sum", Alias: "agg_1"},
		},
		GroupBy: []string{"category"},
		Having:  stmt.AggregateComparison{Func: "sum", Column: "amount", Op: "gt", Value: 1000},
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT "category" AS "agg_0", CAST(SUM("amount") AS DOUBLE PRECISION) AS "agg_1" FROM "orders" GROUP BY "category" HAVING CAST(SUM("amount") AS DOUBLE PRECISION)>$1`
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
	if len(args) != 1 || args[0] != 1000 {
		t.Fatalf("args = %v, want [1000]", args)
	}
}

func TestCompileSelect_Aggregate_WithWhereAndOrderBy(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "orders",
		Projections: []stmt.Projection{
			{Column: "category", Alias: "agg_0"},
			{Column: "amount", Func: "sum", Alias: "agg_1"},
		},
		Where:   stmt.Comparison{Column: "status", Op: "eq", Value: "paid"},
		GroupBy: []string{"category"},
		OrderBy: []stmt.OrderElement{{Column: "agg_1", Desc: true}},
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT "category" AS "agg_0", CAST(SUM("amount") AS DOUBLE PRECISION) AS "agg_1" FROM "orders" WHERE "status"=$1 GROUP BY "category" ORDER BY "agg_1" DESC`
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
	if len(args) != 1 || args[0] != "paid" {
		t.Fatalf("args = %v, want [paid]", args)
	}
}

func TestCompileSelect_Aggregate_HavingError_Propagates(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "orders",
		Projections: []stmt.Projection{
			{Column: "amount", Func: "sum", Alias: "agg_0"},
		},
		Having: stmt.AggregateComparison{Func: "sum", Column: "amount", Op: "like", Value: 1},
	}

	_, _, err := d.CompileSelect(s)
	if err == nil {
		t.Fatal("expected error from an unsupported HAVING operator, got nil")
	}
}

func TestAggregateSQLFunc_UnrecognizedFunc_FallsBackToBareColumn(t *testing.T) {
	got := aggregateSQLFunc("bogus", "amount")
	want := `"amount"`
	if got != want {
		t.Errorf("aggregateSQLFunc(bogus) = %q, want %q", got, want)
	}
}

func TestAggregateComparison_UnsupportedOp_ReturnsError(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(stmt.AggregateComparison{Func: "sum", Column: "amount", Op: "like", Value: 1}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected error for unsupported having comparison operator, got nil")
	}
}

func TestAggregateComparison_AllOps(t *testing.T) {
	cases := []struct {
		op   string
		want string
	}{
		{"eq", `CAST(SUM("amount") AS DOUBLE PRECISION)=$1`},
		{"gt", `CAST(SUM("amount") AS DOUBLE PRECISION)>$1`},
		{"gte", `CAST(SUM("amount") AS DOUBLE PRECISION)>=$1`},
		{"lt", `CAST(SUM("amount") AS DOUBLE PRECISION)<$1`},
		{"lte", `CAST(SUM("amount") AS DOUBLE PRECISION)<=$1`},
	}
	for _, tc := range cases {
		t.Run(tc.op, func(t *testing.T) {
			argOffset := 1
			var args []any
			sql, err := compilePredicate(stmt.AggregateComparison{Func: "sum", Column: "amount", Op: tc.op, Value: 5}, &argOffset, &args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.want {
				t.Errorf("sql = %q, want %q", sql, tc.want)
			}
		})
	}
}
