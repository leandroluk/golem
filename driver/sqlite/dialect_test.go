package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/stmt"
)

var _ golem.Dialect = (*dialect)(nil)

// -----------------------------------------------------------------------
// Bind
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Scan
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// quoteIdent / aggregateSQLFunc / projectionSQL
// -----------------------------------------------------------------------

func TestQuoteIdent_Simple(t *testing.T) {
	if got := quoteIdent("col"); got != `"col"` {
		t.Fatalf("quoteIdent(col) = %q", got)
	}
}

func TestQuoteIdent_Dotted(t *testing.T) {
	if got := quoteIdent("table.col"); got != `"table"."col"` {
		t.Fatalf("quoteIdent(table.col) = %q", got)
	}
}

func TestAggregateSQLFunc(t *testing.T) {
	if got := aggregateSQLFunc("count_all", ""); got != "COUNT(*)" {
		t.Fatalf("count_all = %q", got)
	}
	if got := aggregateSQLFunc("count", "col"); got != `COUNT("col")` {
		t.Fatalf("count = %q", got)
	}
	if got := aggregateSQLFunc("sum", "col"); got != `CAST(SUM("col") AS REAL)` {
		t.Fatalf("sum = %q", got)
	}
	if got := aggregateSQLFunc("avg", "col"); got != `CAST(AVG("col") AS REAL)` {
		t.Fatalf("avg = %q", got)
	}
	if got := aggregateSQLFunc("unknown", "col"); got != `"col"` {
		t.Fatalf("unknown = %q", got)
	}
}

func TestProjectionSQL(t *testing.T) {
	p1 := stmt.Projection{Column: "col", Alias: "col"}
	if got := projectionSQL(p1); got != `"col" AS "col"` {
		t.Fatalf("plain projection = %q", got)
	}
	p2 := stmt.Projection{Func: "count", Column: "col", Alias: "cnt"}
	if got := projectionSQL(p2); got != `COUNT("col") AS "cnt"` {
		t.Fatalf("agg projection = %q", got)
	}
}

// -----------------------------------------------------------------------
// compilePredicate
// -----------------------------------------------------------------------

func TestCompilePredicate_Nil(t *testing.T) {
	var args []any
	sql, err := compilePredicate(nil, &args)
	if err != nil || sql != "" {
		t.Fatalf("compilePredicate(nil) = (%q, %v)", sql, err)
	}
}

func TestCompilePredicate_ComparisonOps(t *testing.T) {
	cases := []struct {
		op   string
		want string
	}{
		{"eq", `"age"=?`},
		{"gt", `"age">?`},
		{"gte", `"age">=?`},
		{"lt", `"age"<?`},
		{"lte", `"age"<=?`},
		{"like", `"age" LIKE ?`},
	}
	for _, tc := range cases {
		var args []any
		sql, err := compilePredicate(stmt.Comparison{Column: "age", Op: tc.op, Value: 18}, &args)
		if err != nil {
			t.Fatalf("op %s: %v", tc.op, err)
		}
		if sql != tc.want {
			t.Fatalf("op %s: sql = %q, want %q", tc.op, sql, tc.want)
		}
		if len(args) != 1 || args[0] != 18 {
			t.Fatalf("op %s: args = %v", tc.op, args)
		}
	}
}

func TestCompilePredicate_IsNull(t *testing.T) {
	var args []any
	sql, err := compilePredicate(stmt.Comparison{Column: "id", Op: "is_null"}, &args)
	if err != nil || sql != `"id" IS NULL` || len(args) != 0 {
		t.Fatalf("is_null: sql=%q args=%v err=%v", sql, args, err)
	}
}

func TestCompilePredicate_In_Slice(t *testing.T) {
	var args []any
	sql, err := compilePredicate(stmt.Comparison{Column: "id", Op: "in", Value: []int{1, 2, 3}}, &args)
	if err != nil || sql != `"id" IN (?,?,?)` || len(args) != 3 {
		t.Fatalf("in slice: sql=%q args=%v err=%v", sql, args, err)
	}
}

func TestCompilePredicate_In_EmptySlice(t *testing.T) {
	var args []any
	sql, err := compilePredicate(stmt.Comparison{Column: "id", Op: "in", Value: []int{}}, &args)
	if err != nil || sql != "FALSE" {
		t.Fatalf("in empty: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_In_NonSlice(t *testing.T) {
	var args []any
	sql, err := compilePredicate(stmt.Comparison{Column: "id", Op: "in", Value: 5}, &args)
	if err != nil || sql != `"id" IN (?)` || len(args) != 1 {
		t.Fatalf("in non-slice: sql=%q args=%v err=%v", sql, args, err)
	}
}

func TestCompilePredicate_UnsupportedComparisonOp(t *testing.T) {
	var args []any
	_, err := compilePredicate(stmt.Comparison{Column: "id", Op: "bogus"}, &args)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompilePredicate_Logical_And(t *testing.T) {
	var args []any
	p := stmt.Logical{Op: "and", Predicates: []stmt.Predicate{
		stmt.Comparison{Column: "a", Op: "eq", Value: 1},
		stmt.Comparison{Column: "b", Op: "eq", Value: 2},
	}}
	sql, err := compilePredicate(p, &args)
	if err != nil || sql != `("a"=? AND "b"=?)` {
		t.Fatalf("and: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_Logical_Or(t *testing.T) {
	var args []any
	p := stmt.Logical{Op: "or", Predicates: []stmt.Predicate{
		stmt.Comparison{Column: "a", Op: "eq", Value: 1},
		stmt.Comparison{Column: "b", Op: "eq", Value: 2},
	}}
	sql, err := compilePredicate(p, &args)
	if err != nil || sql != `("a"=? OR "b"=?)` {
		t.Fatalf("or: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_Logical_Empty(t *testing.T) {
	var args []any
	sql, err := compilePredicate(stmt.Logical{Op: "and"}, &args)
	if err != nil || sql != "" {
		t.Fatalf("empty logical: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_Logical_AllPartsEmpty(t *testing.T) {
	var args []any
	p := stmt.Logical{Op: "and", Predicates: []stmt.Predicate{
		stmt.Logical{Op: "and"},
	}}
	sql, err := compilePredicate(p, &args)
	if err != nil || sql != "" {
		t.Fatalf("all parts empty: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_Logical_PropagatesError(t *testing.T) {
	var args []any
	p := stmt.Logical{Op: "and", Predicates: []stmt.Predicate{
		stmt.Comparison{Column: "a", Op: "bogus"},
	}}
	_, err := compilePredicate(p, &args)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompilePredicate_Not(t *testing.T) {
	var args []any
	p := stmt.Not{Predicate: stmt.Comparison{Column: "a", Op: "eq", Value: 1}}
	sql, err := compilePredicate(p, &args)
	if err != nil || sql != `NOT ("a"=?)` {
		t.Fatalf("not: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_Not_EmptyInner(t *testing.T) {
	var args []any
	p := stmt.Not{Predicate: stmt.Logical{Op: "and"}}
	sql, err := compilePredicate(p, &args)
	if err != nil || sql != "" {
		t.Fatalf("not empty: sql=%q err=%v", sql, err)
	}
}

func TestCompilePredicate_Not_PropagatesError(t *testing.T) {
	var args []any
	p := stmt.Not{Predicate: stmt.Comparison{Column: "a", Op: "bogus"}}
	_, err := compilePredicate(p, &args)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompilePredicate_AggregateComparison(t *testing.T) {
	cases := []struct {
		op   string
		want string
	}{
		{"eq", `COUNT("id")=?`},
		{"gt", `COUNT("id")>?`},
		{"gte", `COUNT("id")>=?`},
		{"lt", `COUNT("id")<?`},
		{"lte", `COUNT("id")<=?`},
	}
	for _, tc := range cases {
		var args []any
		sql, err := compilePredicate(stmt.AggregateComparison{Func: "count", Column: "id", Op: tc.op, Value: 1}, &args)
		if err != nil || sql != tc.want {
			t.Fatalf("agg op %s: sql=%q err=%v", tc.op, sql, err)
		}
	}
}

func TestCompilePredicate_AggregateComparison_UnsupportedOp(t *testing.T) {
	var args []any
	_, err := compilePredicate(stmt.AggregateComparison{Func: "count", Column: "id", Op: "bogus"}, &args)
	if err == nil {
		t.Fatal("expected error")
	}
}

// unknownPredicate embeds stmt.Comparison purely to inherit its unexported
// isPredicate() method while remaining a distinct concrete type, the only
// way to reach compilePredicate's default case from outside internal/stmt.
type unknownPredicate struct {
	stmt.Comparison
}

func TestCompilePredicate_UnknownType(t *testing.T) {
	var args []any
	_, err := compilePredicate(unknownPredicate{}, &args)
	if err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// CompileSelect
// -----------------------------------------------------------------------

func TestCompileSelect_Basic(t *testing.T) {
	d := &dialect{}
	sql, args, err := d.CompileSelect(&stmt.Select{Table: "users"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `SELECT * FROM "users"` || len(args) != 0 {
		t.Fatalf("sql=%q args=%v", sql, args)
	}
}

func TestCompileSelect_Columns(t *testing.T) {
	d := &dialect{}
	sql, _, err := d.CompileSelect(&stmt.Select{Table: "users", Columns: []string{"id", "name"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `SELECT "id", "name" FROM "users"` {
		t.Fatalf("sql=%q", sql)
	}
}

func TestCompileSelect_Count(t *testing.T) {
	d := &dialect{}
	sql, _, err := d.CompileSelect(&stmt.Select{Table: "users", Count: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `SELECT COUNT(*) FROM "users"` {
		t.Fatalf("sql=%q", sql)
	}
}

func TestCompileSelect_Projections(t *testing.T) {
	d := &dialect{}
	sql, _, err := d.CompileSelect(&stmt.Select{
		Table:       "users",
		Projections: []stmt.Projection{{Column: "category", Alias: "category"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `SELECT "category" AS "category" FROM "users"` {
		t.Fatalf("sql=%q", sql)
	}
}

func TestCompileSelect_Where(t *testing.T) {
	d := &dialect{}
	sql, args, err := d.CompileSelect(&stmt.Select{
		Table: "users",
		Where: stmt.Comparison{Column: "id", Op: "eq", Value: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `SELECT * FROM "users" WHERE "id"=?` || len(args) != 1 {
		t.Fatalf("sql=%q args=%v", sql, args)
	}
}

func TestCompileSelect_WhereCompileError_Propagates(t *testing.T) {
	d := &dialect{}
	_, _, err := d.CompileSelect(&stmt.Select{
		Table: "users",
		Where: stmt.Comparison{Column: "id", Op: "bogus"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompileSelect_GroupByHaving(t *testing.T) {
	d := &dialect{}
	sql, args, err := d.CompileSelect(&stmt.Select{
		Table:   "users",
		GroupBy: []string{"category"},
		Having:  stmt.AggregateComparison{Func: "count", Column: "id", Op: "gt", Value: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `SELECT * FROM "users" GROUP BY "category" HAVING COUNT("id")>?` || len(args) != 1 {
		t.Fatalf("sql=%q args=%v", sql, args)
	}
}

func TestCompileSelect_HavingCompileError_Propagates(t *testing.T) {
	d := &dialect{}
	_, _, err := d.CompileSelect(&stmt.Select{
		Table:  "users",
		Having: stmt.AggregateComparison{Func: "count", Column: "id", Op: "bogus"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompileSelect_OrderBy(t *testing.T) {
	d := &dialect{}
	sql, _, err := d.CompileSelect(&stmt.Select{
		Table:   "users",
		OrderBy: []stmt.OrderElement{{Column: "id", Desc: true}, {Column: "name"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `SELECT * FROM "users" ORDER BY "id" DESC, "name" ASC` {
		t.Fatalf("sql=%q", sql)
	}
}

func TestCompileSelect_LimitOffset(t *testing.T) {
	d := &dialect{}
	limit, offset := 10, 5
	sql, args, err := d.CompileSelect(&stmt.Select{Table: "users", Limit: &limit, Offset: &offset})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `SELECT * FROM "users" LIMIT ? OFFSET ?` || len(args) != 2 {
		t.Fatalf("sql=%q args=%v", sql, args)
	}
}

func TestCompileSelect_Joins(t *testing.T) {
	d := &dialect{}
	sql, args, err := d.CompileSelect(&stmt.Select{
		Table: "parent",
		Joins: []stmt.Join{{
			Type:  "inner",
			Table: "child",
			On:    []stmt.OnCondition{{LeftCol: "child.parent_id", RightCol: "parent.id"}},
			Where: stmt.Comparison{Column: "child.name", Op: "eq", Value: "x"},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT * FROM "parent" INNER JOIN "child" ON "child"."parent_id" = "parent"."id" AND "child"."name"=?`
	if sql != want || len(args) != 1 {
		t.Fatalf("sql=%q args=%v, want %q", sql, args, want)
	}
}

func TestCompileSelect_JoinWhereCompileError_Propagates(t *testing.T) {
	d := &dialect{}
	_, _, err := d.CompileSelect(&stmt.Select{
		Table: "parent",
		Joins: []stmt.Join{{
			Type:  "inner",
			Table: "child",
			On:    []stmt.OnCondition{{LeftCol: "child.parent_id", RightCol: "parent.id"}},
			Where: stmt.Comparison{Column: "child.name", Op: "bogus"},
		}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCompileSelect_LockError_Propagates(t *testing.T) {
	d := &dialect{}
	_, _, err := d.CompileSelect(&stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: "update"}})
	if err == nil {
		t.Fatal("expected error — SQLite has no row-level locking")
	}
}

// -----------------------------------------------------------------------
// lockClauseSQL — SQLite has no row-level locking clause of any kind;
// every strength errors, unconditionally (spec.md P3 AC7).
// -----------------------------------------------------------------------

func TestLockClauseSQL_AlwaysErrors(t *testing.T) {
	for _, strength := range []string{"update", "no_key_update", "share", "key_share", "bogus"} {
		_, err := lockClauseSQL(&stmt.LockClause{Strength: strength})
		if err == nil {
			t.Fatalf("strength %s: expected error", strength)
		}
	}
}

// -----------------------------------------------------------------------
// CompileDelete
// -----------------------------------------------------------------------

func TestCompileDelete_Basic(t *testing.T) {
	d := &dialect{}
	sql, args, err := d.CompileDelete(&stmt.Delete{Table: "users"})
	if err != nil || sql != `DELETE FROM "users"` || len(args) != 0 {
		t.Fatalf("sql=%q args=%v err=%v", sql, args, err)
	}
}

func TestCompileDelete_Where(t *testing.T) {
	d := &dialect{}
	sql, args, err := d.CompileDelete(&stmt.Delete{Table: "users", Where: stmt.Comparison{Column: "id", Op: "eq", Value: 1}})
	if err != nil || sql != `DELETE FROM "users" WHERE "id"=?` || len(args) != 1 {
		t.Fatalf("sql=%q args=%v err=%v", sql, args, err)
	}
}

func TestCompileDelete_WhereCompileError_Propagates(t *testing.T) {
	d := &dialect{}
	_, _, err := d.CompileDelete(&stmt.Delete{Table: "users", Where: stmt.Comparison{Column: "id", Op: "bogus"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// Insert / Update (single round-trip via RETURNING *)
// -----------------------------------------------------------------------

func newMockDialect(t *testing.T) (*dialect, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &dialect{db: db}, mock
}

func TestDialect_Insert_Success(t *testing.T) {
	d, mock := newMockDialect(t)

	mock.ExpectQuery(`INSERT INTO "users" \("name"\) VALUES \(\?\) RETURNING \*`).WithArgs("Ada").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "Ada"))

	row, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table:   "users",
		Columns: []string{"name"},
		Values:  []driver.Value{"Ada"},
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if row["name"] != "Ada" {
		t.Fatalf("row = %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestDialect_Insert_QueryError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`INSERT INTO "users"`).WillReturnError(errors.New("insert error"))

	_, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table: "users", Columns: []string{"name"}, Values: []driver.Value{"Ada"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_Insert_ZeroRows(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`INSERT INTO "users"`).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	_, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table: "users", Columns: []string{"name"}, Values: []driver.Value{"Ada"},
	})
	if err == nil {
		t.Fatal("expected error for zero rows returned")
	}
}

func TestDialect_Insert_CollectRowsError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`INSERT INTO "users"`).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name"}).RowError(0, errors.New("row scan error")).AddRow(int64(1), "Ada"))

	_, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table: "users", Columns: []string{"name"}, Values: []driver.Value{"Ada"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_Update_Success(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`UPDATE "users" SET "name"=\? WHERE "id"=\? RETURNING \*`).WithArgs("Ada2", int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "Ada2"))

	rows, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "Ada2"}},
		Where: stmt.Comparison{Column: "id", Op: "eq", Value: int64(1)},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(rows) != 1 || rows[0]["name"] != "Ada2" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestDialect_Update_NoWhere(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`UPDATE "users" SET "name"=\? RETURNING \*`).WithArgs("x").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "x").AddRow(int64(2), "x"))

	rows, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestDialect_Update_WhereCompileError_Propagates(t *testing.T) {
	d := &dialect{}
	_, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
		Where: stmt.Comparison{Column: "id", Op: "bogus"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_Update_QueryError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`UPDATE "users"`).WillReturnError(errors.New("update error"))

	_, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_Update_CollectRowsError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery(`UPDATE "users"`).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name"}).RowError(0, errors.New("row scan error")).AddRow(int64(1), "x"))

	_, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// Query / Exec / ExecRaw
// -----------------------------------------------------------------------

func TestDialect_Query_Success(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	rows, err := d.Query(context.Background(), nil, "SELECT 1", nil)
	if err != nil || len(rows) != 1 || rows[0]["id"] != int64(1) {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
}

func TestDialect_Query_Error(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery("SELECT 1").WillReturnError(errors.New("query error"))

	_, err := d.Query(context.Background(), nil, "SELECT 1", nil)
	if err == nil || err.Error() != "query error" {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestDialect_Query_ScanError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"id"}).RowError(0, errors.New("row scan error")).AddRow(int64(1)))

	_, err := d.Query(context.Background(), nil, "SELECT 1", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_Exec_Success(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectExec("UPDATE users").WillReturnResult(sqlmock.NewResult(0, 3))

	n, err := d.Exec(context.Background(), nil, "UPDATE users SET x=1", nil)
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestDialect_Exec_Error(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectExec("UPDATE users").WillReturnError(errors.New("exec error"))

	_, err := d.Exec(context.Background(), nil, "UPDATE users SET x=1", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_ExecRaw_Success(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery("SELECT \\* FROM users").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)).AddRow(int64(2)))

	rows, n, err := d.ExecRaw(context.Background(), nil, "SELECT * FROM users", nil)
	if err != nil || n != 2 || len(rows) != 2 {
		t.Fatalf("rows=%v n=%d err=%v", rows, n, err)
	}
}

func TestDialect_ExecRaw_QueryError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery("SELECT \\* FROM users").WillReturnError(errors.New("query error"))

	_, _, err := d.ExecRaw(context.Background(), nil, "SELECT * FROM users", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_ExecRaw_CollectRowsError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectQuery("SELECT \\* FROM users").WillReturnRows(sqlmock.NewRows([]string{"id"}).RowError(0, errors.New("row scan error")).AddRow(int64(1)))

	_, _, err := d.ExecRaw(context.Background(), nil, "SELECT * FROM users", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_ExecRaw_WriteStatement_Success(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectExec("UPDATE users").WillReturnResult(sqlmock.NewResult(0, 3))

	rows, affected, err := d.ExecRaw(context.Background(), nil, "UPDATE users SET name = ?", []any{"x"})
	if err != nil || affected != 3 || len(rows) != 0 {
		t.Fatalf("rows=%v affected=%d err=%v", rows, affected, err)
	}
}

func TestDialect_ExecRaw_WriteStatement_ExecError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectExec("UPDATE users").WillReturnError(errors.New("exec error"))

	_, _, err := d.ExecRaw(context.Background(), nil, "UPDATE users SET name = ?", []any{"x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDialect_ExecRaw_WriteStatement_RowsAffectedError(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectExec("UPDATE users").WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected error")))

	_, _, err := d.ExecRaw(context.Background(), nil, "UPDATE users SET name = ?", []any{"x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// normalizeCell
// -----------------------------------------------------------------------

func TestNormalizeCell_Time_String(t *testing.T) {
	got := normalizeCell("TIME", "2000-01-01 08:15:30+00:00")
	tm, ok := got.(time.Time)
	if !ok {
		t.Fatalf("got %T, want time.Time", got)
	}
	if tm.Hour() != 8 || tm.Minute() != 15 || tm.Second() != 30 {
		t.Fatalf("got %v, want 08:15:30", tm)
	}
}

func TestNormalizeCell_Time_ByteSlice(t *testing.T) {
	got := normalizeCell("TIME", []byte("2000-01-01 08:15:30.5+00:00"))
	tm, ok := got.(time.Time)
	if !ok {
		t.Fatalf("got %T, want time.Time", got)
	}
	if tm.Hour() != 8 || tm.Nanosecond() != 5e8 {
		t.Fatalf("got %v, want fractional 0.5s", tm)
	}
}

func TestNormalizeCell_Time_ParseError_LeftUntouched(t *testing.T) {
	raw := "not a time"
	got := normalizeCell("TIME", raw)
	if got != raw {
		t.Fatalf("got %v, want raw string left untouched", got)
	}
}

func TestNormalizeCell_Time_NonStringType_LeftUntouched(t *testing.T) {
	got := normalizeCell("TIME", int64(5))
	if got != int64(5) {
		t.Fatalf("got %v, want untouched int64(5)", got)
	}
}

func TestNormalizeCell_NonTimeDeclaredType_LeftUntouched(t *testing.T) {
	got := normalizeCell("DATETIME", "2024-03-15 10:30:45+00:00")
	if got != "2024-03-15 10:30:45+00:00" {
		t.Fatalf("got %v, want untouched (DATETIME already auto-converts, normalizeCell only handles TIME)", got)
	}
}

func TestCollectRows_ColumnsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}
	rows.Close() // Columns() errors once rows are closed

	_, err = collectRows(rows)
	if err == nil {
		t.Fatal("expected error")
	}
}

// -----------------------------------------------------------------------
// mapError / IsConflict — driven against a REAL in-memory SQLite database
// (modernc.org/sqlite has no exported way to construct *sqlite.Error
// directly), which also confirms empirically that Error.Code() returns the
// EXTENDED result code (2067/1555/787) by default, not the primary one
// (19) — the uncertainty design.md flagged.
// -----------------------------------------------------------------------

func realSQLiteConstraintErrors(t *testing.T) (unique, primaryKey, foreignKey, notNull error) {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`CREATE TABLE parent (id INTEGER PRIMARY KEY, name TEXT UNIQUE, req TEXT NOT NULL DEFAULT 'x')`); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL, FOREIGN KEY (parent_id) REFERENCES parent(id))`); err != nil {
		t.Fatalf("create child: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO parent (id, name) VALUES (1, 'a')`); err != nil {
		t.Fatalf("seed parent: %v", err)
	}

	_, unique = db.Exec(`INSERT INTO parent (id, name) VALUES (2, 'a')`)
	_, primaryKey = db.Exec(`INSERT INTO parent (id, name) VALUES (1, 'b')`)
	_, foreignKey = db.Exec(`INSERT INTO child (id, parent_id) VALUES (1, 999)`)
	_, notNull = db.Exec(`INSERT INTO parent (id, name, req) VALUES (3, 'c', NULL)`)

	if unique == nil || primaryKey == nil || foreignKey == nil || notNull == nil {
		t.Fatalf("expected all 4 constraint violations to error: unique=%v pk=%v fk=%v notnull=%v",
			unique, primaryKey, foreignKey, notNull)
	}
	return unique, primaryKey, foreignKey, notNull
}

func TestMapError_Nil(t *testing.T) {
	if mapError(nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestMapError_DuplicateKey(t *testing.T) {
	unique, primaryKey, _, _ := realSQLiteConstraintErrors(t)
	if !errors.Is(mapError(unique), golem.ErrDuplicateKey) {
		t.Fatalf("UNIQUE: expected ErrDuplicateKey, got %v", mapError(unique))
	}
	if !errors.Is(mapError(primaryKey), golem.ErrDuplicateKey) {
		t.Fatalf("PRIMARYKEY: expected ErrDuplicateKey, got %v", mapError(primaryKey))
	}
}

func TestMapError_ForeignKeyViolation(t *testing.T) {
	_, _, foreignKey, _ := realSQLiteConstraintErrors(t)
	if !errors.Is(mapError(foreignKey), golem.ErrForeignKeyViolation) {
		t.Fatalf("expected ErrForeignKeyViolation, got %v", mapError(foreignKey))
	}
}

func TestMapError_UnmappedConstraint_PassesThrough(t *testing.T) {
	_, _, _, notNull := realSQLiteConstraintErrors(t)
	if errors.Is(mapError(notNull), golem.ErrDuplicateKey) || errors.Is(mapError(notNull), golem.ErrForeignKeyViolation) {
		t.Fatalf("NOT NULL violation should not map to a specific sentinel, got %v", mapError(notNull))
	}
	if mapError(notNull) != notNull {
		t.Fatalf("expected unmapped error to pass through unchanged, got %v", mapError(notNull))
	}
}

func TestMapError_NonSQLiteError(t *testing.T) {
	orig := errors.New("plain error")
	if mapError(orig) != orig {
		t.Fatal("expected passthrough")
	}
}

func TestIsConflict_Nil(t *testing.T) {
	d := &dialect{}
	if d.IsConflict(nil) {
		t.Fatal("expected false")
	}
}

func TestIsConflict_True(t *testing.T) {
	d := &dialect{}
	unique, primaryKey, foreignKey, notNull := realSQLiteConstraintErrors(t)
	for name, err := range map[string]error{"unique": unique, "primaryKey": primaryKey, "foreignKey": foreignKey, "notNull": notNull} {
		if !d.IsConflict(err) {
			t.Fatalf("%s: expected true", name)
		}
	}
}

func TestIsConflict_NonSQLiteError(t *testing.T) {
	d := &dialect{}
	if d.IsConflict(errors.New("plain error")) {
		t.Fatal("expected false")
	}
}

// -----------------------------------------------------------------------
// Begin / getExecutor
// -----------------------------------------------------------------------

func TestDialect_Begin_Success(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectBegin()

	tx, err := d.Begin(context.Background(), nil)
	if err != nil || tx == nil {
		t.Fatalf("tx=%v err=%v", tx, err)
	}
}

func TestDialect_Begin_Error(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectBegin().WillReturnError(errors.New("begin error"))

	_, err := d.Begin(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSqliteTx_Commit(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	tx, err := d.Begin(context.Background(), nil)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := tx.(*sqliteTx).Commit(context.Background()); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

func TestSqliteTx_Rollback(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, err := d.Begin(context.Background(), nil)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := tx.(*sqliteTx).Rollback(context.Background()); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
}

func TestDialect_GetExecutor_DefaultsToPool(t *testing.T) {
	d := &dialect{db: nil}
	got := d.getExecutor(nil)
	if got != nil {
		t.Fatalf("expected getExecutor(nil conn) to return d.db (nil), got %v", got)
	}
}

func TestDialect_GetExecutor_UsesTxWhenPresent(t *testing.T) {
	d, mock := newMockDialect(t)
	mock.ExpectBegin()
	txConn, err := d.Begin(context.Background(), nil)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	tx := golem.NewTx(d, txConn, golem.DefaultParser)
	got := d.getExecutor(tx)
	if got == nil {
		t.Fatal("expected non-nil executor")
	}
	if _, ok := got.(*sql.Tx); !ok {
		t.Fatalf("expected *sql.Tx, got %T", got)
	}
}
