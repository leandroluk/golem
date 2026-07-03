package postgres

import (
	"reflect"
	"testing"

	"github.com/leandroluk/golem/internal/stmt"
)

func TestCompileSelect_Minimal(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSQL := `SELECT * FROM "users"`
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}
	if len(args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(args))
	}
}

func TestCompileSelect_Complex(t *testing.T) {
	d := dialect{}
	limit := 10
	offset := 20
	s := &stmt.Select{
		Table:   "users",
		Columns: []string{"id", "name"},
		Where: stmt.Logical{
			Op: "and",
			Predicates: []stmt.Predicate{
				stmt.Comparison{Column: "age", Op: "gte", Value: 18},
				stmt.Not{
					Predicate: stmt.Comparison{Column: "status", Op: "eq", Value: "pending"},
				},
				stmt.Logical{
					Op: "or",
					Predicates: []stmt.Predicate{
						stmt.Comparison{Column: "role", Op: "eq", Value: "admin"},
						stmt.Comparison{Column: "role", Op: "eq", Value: "moderator"},
					},
				},
				stmt.Comparison{Column: "deleted_at", Op: "is_null"},
				stmt.Comparison{Column: "tags", Op: "in", Value: []string{"vip", "staff"}},
			},
		},
		OrderBy: []stmt.OrderElement{
			{Column: "name", Desc: false},
			{Column: "id", Desc: true},
		},
		Limit:  &limit,
		Offset: &offset,
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSQL := `SELECT "id", "name" FROM "users" WHERE ("age">=$1 AND NOT ("status"=$2) AND ("role"=$3 OR "role"=$4) AND "deleted_at" IS NULL AND "tags" IN ($5,$6)) ORDER BY "name" ASC, "id" DESC LIMIT $7 OFFSET $8`
	if sql != wantSQL {
		t.Fatalf("sql =\n%q\nwant:\n%q", sql, wantSQL)
	}

	wantArgs := []any{18, "pending", "admin", "moderator", "vip", "staff", 10, 20}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
}

func TestCompileSelect_EmptyInClause(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
		Where: stmt.Comparison{Column: "id", Op: "in", Value: []int{}},
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSQL := `SELECT * FROM "users" WHERE FALSE`
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}
	if len(args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(args))
	}
}

func TestCompileDelete(t *testing.T) {
	d := dialect{}
	s := &stmt.Delete{
		Table: "users",
		Where: stmt.Comparison{Column: "id", Op: "eq", Value: 5},
	}

	sql, args, err := d.CompileDelete(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSQL := `DELETE FROM "users" WHERE "id"=$1`
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}

	wantArgs := []any{5}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
}

func TestCompileSelect_Count(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
		Count: true,
		Where: stmt.Comparison{Column: "active", Op: "eq", Value: true},
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSQL := `SELECT COUNT(*) FROM "users" WHERE "active"=$1`
	if sql != wantSQL {
		t.Fatalf("sql = %q, want %q", sql, wantSQL)
	}

	wantArgs := []any{true}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
}

func TestCompileSelect_WithJoins(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table:   "users",
		Columns: []string{"users.id", "posts.title"},
		Joins: []stmt.Join{
			{
				Type:  "inner",
				Table: "posts",
				On: []stmt.OnCondition{
					{LeftCol: "posts.owner_id", RightCol: "users.id"},
				},
				Where: stmt.Comparison{Column: "posts.published", Op: "eq", Value: true},
			},
		},
		Where: stmt.Comparison{Column: "users.active", Op: "eq", Value: true},
	}

	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSQL := `SELECT "users"."id", "posts"."title" FROM "users" INNER JOIN "posts" ON "posts"."owner_id" = "users"."id" AND "posts"."published"=$1 WHERE "users"."active"=$2`
	if sql != wantSQL {
		t.Fatalf("sql =\n%q\nwant:\n%q", sql, wantSQL)
	}

	wantArgs := []any{true, true}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
}


