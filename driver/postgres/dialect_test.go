package postgres

import (
	"context"
	"database/sql/driver"
	"errors"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/stmt"
	"github.com/pashagolub/pgxmock/v4"
)

var _ golem.Dialect = (*dialect)(nil)

// -----------------------------------------------------------------------
// Bind
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Scan
// -----------------------------------------------------------------------

// wrong: BIGINT expects *int64

// -----------------------------------------------------------------------
// CompileSelect / CompileDelete (moved here from dialect_sql_test.go)
// -----------------------------------------------------------------------

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

func TestCompileSelect_WhereCompileError_Propagates(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
		Where: stmt.Comparison{Column: "id", Op: "bogus", Value: 1},
	}
	_, _, err := d.CompileSelect(s)
	if err == nil {
		t.Fatal("expected error from an unsupported Where operator, got nil")
	}
}

func TestCompileSelect_JoinWhereCompileError_Propagates(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
		Joins: []stmt.Join{
			{
				Type:  "inner",
				Table: "posts",
				On:    []stmt.OnCondition{{LeftCol: "posts.owner_id", RightCol: "users.id"}},
				Where: stmt.Comparison{Column: "posts.id", Op: "bogus", Value: 1},
			},
		},
	}
	_, _, err := d.CompileSelect(s)
	if err == nil {
		t.Fatal("expected error from an unsupported join Where operator, got nil")
	}
}

func TestCompileDelete_WhereCompileError_Propagates(t *testing.T) {
	d := dialect{}
	s := &stmt.Delete{
		Table: "users",
		Where: stmt.Comparison{Column: "id", Op: "bogus", Value: 1},
	}
	_, _, err := d.CompileDelete(s)
	if err == nil {
		t.Fatal("expected error from an unsupported Where operator, got nil")
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

// -----------------------------------------------------------------------
// Aggregate projections / HAVING (moved here from aggregate_sql_test.go)
// -----------------------------------------------------------------------

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

// -----------------------------------------------------------------------
// Row locking (moved here from lock_sql_test.go)
// -----------------------------------------------------------------------

func TestCompileSelect_Lock_ForUpdate(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{
		Table: "users",
		Lock:  &stmt.LockClause{Strength: "update"},
	}
	sql, _, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT * FROM "users" FOR UPDATE`
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
}

func TestCompileSelect_Lock_AllStrengths(t *testing.T) {
	cases := []struct {
		strength string
		want     string
	}{
		{"update", "FOR UPDATE"},
		{"no_key_update", "FOR NO KEY UPDATE"},
		{"share", "FOR SHARE"},
		{"key_share", "FOR KEY SHARE"},
	}
	for _, tc := range cases {
		t.Run(tc.strength, func(t *testing.T) {
			d := dialect{}
			s := &stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: tc.strength}}
			sql, _, err := d.CompileSelect(s)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := `SELECT * FROM "users" ` + tc.want
			if sql != want {
				t.Errorf("sql = %q, want %q", sql, want)
			}
		})
	}
}

func TestCompileSelect_Lock_WaitPolicies(t *testing.T) {
	cases := []struct {
		wait string
		want string
	}{
		{"", `SELECT * FROM "users" FOR UPDATE`},
		{"nowait", `SELECT * FROM "users" FOR UPDATE NOWAIT`},
		{"skip_locked", `SELECT * FROM "users" FOR UPDATE SKIP LOCKED`},
	}
	for _, tc := range cases {
		t.Run(tc.wait, func(t *testing.T) {
			d := dialect{}
			s := &stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: "update", Wait: tc.wait}}
			sql, _, err := d.CompileSelect(s)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.want {
				t.Errorf("sql = %q, want %q", sql, tc.want)
			}
		})
	}
}

func TestCompileSelect_Lock_UnsupportedStrength_ReturnsError(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: "bogus"}}
	_, _, err := d.CompileSelect(s)
	if err == nil {
		t.Fatal("expected error for unsupported lock strength, got nil")
	}
}

func TestCompileSelect_Lock_UnsupportedWait_ReturnsError(t *testing.T) {
	d := dialect{}
	s := &stmt.Select{Table: "users", Lock: &stmt.LockClause{Strength: "update", Wait: "bogus"}}
	_, _, err := d.CompileSelect(s)
	if err == nil {
		t.Fatal("expected error for unsupported lock wait policy, got nil")
	}
}

func TestCompileSelect_Lock_WithOrderByLimitOffset_AppearsLast(t *testing.T) {
	d := dialect{}
	limit := 10
	s := &stmt.Select{
		Table:   "users",
		OrderBy: []stmt.OrderElement{{Column: "id", Desc: false}},
		Limit:   &limit,
		Lock:    &stmt.LockClause{Strength: "update"},
	}
	sql, args, err := d.CompileSelect(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT * FROM "users" ORDER BY "id" ASC LIMIT $1 FOR UPDATE`
	if sql != want {
		t.Fatalf("sql = %q, want %q", sql, want)
	}
	if len(args) != 1 || args[0] != 10 {
		t.Fatalf("args = %v, want [10]", args)
	}
}

// -----------------------------------------------------------------------
// mapError (moved here from errors_test.go)
// -----------------------------------------------------------------------

func TestMapError_Nil_ReturnsNil(t *testing.T) {
	if err := mapError(nil); err != nil {
		t.Fatalf("mapError(nil) = %v, want nil", err)
	}
}

func TestMapError_UniqueViolation_WrapsErrDuplicateKey(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}

	got := mapError(pgErr)

	if !errors.Is(got, golem.ErrDuplicateKey) {
		t.Fatalf("mapError(23505) = %v, want wrapped golem.ErrDuplicateKey", got)
	}
	var asPgErr *pgconn.PgError
	if !errors.As(got, &asPgErr) {
		t.Fatalf("mapError(23505) = %v, want original *pgconn.PgError reachable via errors.As", got)
	}
	if asPgErr.Code != "23505" {
		t.Fatalf("unwrapped PgError.Code = %q, want 23505", asPgErr.Code)
	}
}

func TestMapError_ForeignKeyViolation_WrapsErrForeignKeyViolation(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23503", Message: "violates foreign key constraint"}

	got := mapError(pgErr)

	if !errors.Is(got, golem.ErrForeignKeyViolation) {
		t.Fatalf("mapError(23503) = %v, want wrapped golem.ErrForeignKeyViolation", got)
	}
	var asPgErr *pgconn.PgError
	if !errors.As(got, &asPgErr) {
		t.Fatalf("mapError(23503) = %v, want original *pgconn.PgError reachable via errors.As", got)
	}
}

func TestMapError_UnmappedPgCode_PassesThroughUnchanged(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "42601", Message: "syntax error"}

	got := mapError(pgErr)

	if got != error(pgErr) {
		t.Fatalf("mapError(42601) = %v, want unchanged original error", got)
	}
	if errors.Is(got, golem.ErrDuplicateKey) || errors.Is(got, golem.ErrForeignKeyViolation) {
		t.Fatalf("mapError(42601) should not match any sentinel, got %v", got)
	}
}

func TestMapError_NonPgError_PassesThroughUnchanged(t *testing.T) {
	plain := errors.New("connection refused")

	got := mapError(plain)

	if got != plain {
		t.Fatalf("mapError(plain) = %v, want unchanged original error", got)
	}
}

// -----------------------------------------------------------------------
// Execute-path dialect methods, mocked via pgxmock (moved here from
// postgres_test.go)
// -----------------------------------------------------------------------

func TestDialect_IsConflict(t *testing.T) {
	d := &dialect{}
	if d.IsConflict(errors.New("some error")) {
		t.Fatal("expected false")
	}
	pgErr := &pgconn.PgError{Code: "23505"}
	if !d.IsConflict(pgErr) {
		t.Fatal("expected true")
	}
}

func TestDialect_Query(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery("SELECT 1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(1))

	rows, err := d.Query(context.Background(), nil, "SELECT 1", nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(rows) != 1 || rows[0]["id"] != 1 {
		t.Fatalf("expected row id 1, got %v", rows)
	}
}

func TestDialect_Query_Error(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery("SELECT 1").WillReturnError(errors.New("query error"))

	_, err := d.Query(context.Background(), nil, "SELECT 1", nil)
	if err == nil || err.Error() != "query error" {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestDialect_Query_CollectRowsError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery("SELECT 1").WillReturnRows(pgxmock.NewRows([]string{"id"}).RowError(0, errors.New("row scan error")).AddRow(1))

	_, err := d.Query(context.Background(), nil, "SELECT 1", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDialect_Exec(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectExec("DELETE FROM users").WillReturnResult(pgxmock.NewResult("DELETE", 2))

	affected, err := d.Exec(context.Background(), nil, "DELETE FROM users", nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if affected != 2 {
		t.Fatalf("expected 2 affected, got %d", affected)
	}
}

func TestDialect_ExecRaw(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery("INSERT INTO users").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(1))

	rows, affected, err := d.ExecRaw(context.Background(), nil, "INSERT INTO users", nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(rows) != 1 || rows[0]["id"] != 1 {
		t.Fatalf("expected row id 1, got %v", rows)
	}
	if affected != 0 {
		t.Fatalf("expected 0 affected, got %d", affected)
	}
}

func TestDialect_Begin(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectBegin()

	tx, err := d.Begin(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if tx == nil {
		t.Fatal("expected tx, got nil")
	}
}

func TestDialect_TxConn(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectBegin()
	mock.ExpectCommit()

	tx, _ := d.Begin(context.Background(), nil)
	err := tx.Commit(context.Background())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDialect_TxConn_Rollback(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectBegin()
	mock.ExpectRollback()

	tx, _ := d.Begin(context.Background(), nil)
	err := tx.Rollback(context.Background())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDialect_Begin_Error(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectBegin().WillReturnError(errors.New("begin error"))

	_, err := d.Begin(context.Background(), nil)
	if err == nil || err.Error() != "postgres: begin: begin error" {
		t.Fatalf("expected begin error, got %v", err)
	}
}

func TestDialect_GetExecutor_UsesTxWhenPresent(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectBegin()
	pgxTx, err := mock.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	tx := golem.NewTx(d, &pgTx{tx: pgxTx, d: d}, golem.DefaultParser)

	mock.ExpectQuery("SELECT 1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(1))

	rows, err := d.Query(context.Background(), tx, "SELECT 1", nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}

func TestDialect_Insert_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery(`INSERT INTO "users"`).WithArgs("Ada").WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).AddRow(1, "Ada"))

	row, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table:   "users",
		Columns: []string{"name"},
		Values:  []driver.Value{"Ada"},
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if row["id"] != int32(1) && row["id"] != 1 {
		t.Errorf("expected id=1, got %v", row["id"])
	}
}

func TestDialect_Insert_QueryError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery(`INSERT INTO "users"`).WithArgs("Ada").WillReturnError(errors.New("insert failed"))

	_, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table:   "users",
		Columns: []string{"name"},
		Values:  []driver.Value{"Ada"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDialect_Insert_CollectRowError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	// Zero rows returned -> pgx.CollectOneRow errors (ErrNoRows).
	mock.ExpectQuery(`INSERT INTO "users"`).WithArgs("Ada").WillReturnRows(pgxmock.NewRows([]string{"id"}))

	_, err := d.Insert(context.Background(), nil, &stmt.Insert{
		Table:   "users",
		Columns: []string{"name"},
		Values:  []driver.Value{"Ada"},
	})
	if err == nil {
		t.Fatal("expected error for zero returned rows, got nil")
	}
}

func TestDialect_Update_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery(`UPDATE "users"`).WithArgs("Ada2", 1).WillReturnRows(pgxmock.NewRows([]string{"id", "name"}).AddRow(1, "Ada2"))

	rows, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "Ada2"}},
		Where: stmt.Comparison{Column: "id", Op: "eq", Value: 1},
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}

func TestDialect_Update_NoWhere(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery(`UPDATE "users"`).WithArgs("same").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))

	rows, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "same"}},
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestDialect_Update_WhereCompileError(t *testing.T) {
	d := &dialect{}
	_, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
		Where: stmt.AggregateComparison{Func: "sum", Column: "amount", Op: "like", Value: 1},
	})
	if err == nil {
		t.Fatal("expected error from an unsupported Where predicate, got nil")
	}
}

func TestDialect_Update_QueryError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery(`UPDATE "users"`).WithArgs("x").WillReturnError(errors.New("update failed"))

	_, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDialect_Update_CollectRowsError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery(`UPDATE "users"`).WithArgs("x").WillReturnRows(pgxmock.NewRows([]string{"id"}).RowError(0, errors.New("row scan error")).AddRow(1))

	_, err := d.Update(context.Background(), nil, &stmt.Update{
		Table: "users",
		Sets:  []stmt.UpdateClause{{Column: "name", Value: "x"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDialect_ExecRaw_ExecError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery("INSERT INTO users").WillReturnError(errors.New("exec raw error"))

	_, _, err := d.ExecRaw(context.Background(), nil, "INSERT INTO users", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDialect_ExecRaw_CollectRowsError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectQuery("INSERT INTO users").WillReturnRows(pgxmock.NewRows([]string{"id"}).RowError(0, errors.New("row scan error")).AddRow(1))

	_, _, err := d.ExecRaw(context.Background(), nil, "INSERT INTO users", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDialect_Exec_Error(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	d := &dialect{pool: mock}

	mock.ExpectExec("DELETE FROM users").WillReturnError(errors.New("exec error"))

	_, err := d.Exec(context.Background(), nil, "DELETE FROM users", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCompilePredicate_Nil_ReturnsEmpty(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(nil, &argOffset, &args)
	if err != nil || sql != "" {
		t.Fatalf("compilePredicate(nil) = (%q, %v), want (\"\", nil)", sql, err)
	}
}

// unknownPredicate embeds stmt.Comparison purely to inherit its unexported
// isPredicate() method (the only way to satisfy stmt.Predicate from outside
// internal/stmt) while remaining a distinct concrete type. compilePredicate's
// type-switch has no case for it, so this is the only way to reach the
// switch's default branch from outside internal/stmt.
type unknownPredicate struct {
	stmt.Comparison
}

func TestCompilePredicate_UnknownType_ReturnsError(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(unknownPredicate{}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected error for unrecognized predicate type")
	}
}

func TestCompilePredicate_Comparison_AllOps(t *testing.T) {
	cases := []struct {
		op   string
		want string
	}{
		{"eq", `"age"=$1`},
		{"gt", `"age">$1`},
		{"gte", `"age">=$1`},
		{"lt", `"age"<$1`},
		{"lte", `"age"<=$1`},
		{"like", `"age" LIKE $1`},
	}
	for _, tc := range cases {
		t.Run(tc.op, func(t *testing.T) {
			argOffset := 1
			var args []any
			sql, err := compilePredicate(stmt.Comparison{Column: "age", Op: tc.op, Value: 18}, &argOffset, &args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.want {
				t.Errorf("sql = %q, want %q", sql, tc.want)
			}
		})
	}
}

func TestCompilePredicate_Comparison_In_NonSliceValue(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Comparison{Column: "id", Op: "in", Value: 5}, &argOffset, &args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `"id" IN ($1)` {
		t.Fatalf("sql = %q, want %q", sql, `"id" IN ($1)`)
	}
}

func TestCompilePredicate_Comparison_UnsupportedOp_ReturnsError(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(stmt.Comparison{Column: "id", Op: "bogus", Value: 1}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected error for unsupported comparison operator, got nil")
	}
}

func TestCompilePredicate_Logical_EmptyPredicates_ReturnsEmpty(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Logical{Op: "and", Predicates: nil}, &argOffset, &args)
	if err != nil || sql != "" {
		t.Fatalf("compilePredicate(empty Logical) = (%q, %v), want (\"\", nil)", sql, err)
	}
}

func TestCompilePredicate_Logical_AllNestedEmpty_ReturnsEmpty(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Logical{
		Op:         "and",
		Predicates: []stmt.Predicate{stmt.Logical{Op: "and", Predicates: nil}},
	}, &argOffset, &args)
	if err != nil || sql != "" {
		t.Fatalf("compilePredicate(nested empty Logical) = (%q, %v), want (\"\", nil)", sql, err)
	}
}

func TestCompilePredicate_Logical_PropagatesNestedError(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(stmt.Logical{
		Op:         "and",
		Predicates: []stmt.Predicate{stmt.Comparison{Column: "id", Op: "bogus", Value: 1}},
	}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected propagated error from nested predicate, got nil")
	}
}

func TestCompilePredicate_Not_PropagatesNestedError(t *testing.T) {
	argOffset := 1
	var args []any
	_, err := compilePredicate(stmt.Not{
		Predicate: stmt.Comparison{Column: "id", Op: "bogus", Value: 1},
	}, &argOffset, &args)
	if err == nil {
		t.Fatal("expected propagated error from nested predicate, got nil")
	}
}

func TestCompilePredicate_Not_EmptyNested_ReturnsEmpty(t *testing.T) {
	argOffset := 1
	var args []any
	sql, err := compilePredicate(stmt.Not{
		Predicate: stmt.Logical{Op: "and", Predicates: nil},
	}, &argOffset, &args)
	if err != nil || sql != "" {
		t.Fatalf("compilePredicate(Not{empty}) = (%q, %v), want (\"\", nil)", sql, err)
	}
}

func TestDialect_IsConflict_Nil(t *testing.T) {
	d := &dialect{}
	if d.IsConflict(nil) {
		t.Fatal("expected false for nil error")
	}
}

func TestDialect_IsConflict_NonPgError(t *testing.T) {
	d := &dialect{}
	if d.IsConflict(errors.New("plain error, not a PgError")) {
		t.Fatal("expected false for a non-PgError")
	}
}

func TestDialect_IsConflict_PgErrorWrongClass(t *testing.T) {
	d := &dialect{}
	pgErr := &pgconn.PgError{Code: "42601"} // syntax error, class 42, not 23
	if d.IsConflict(pgErr) {
		t.Fatal("expected false for a non-class-23 PgError")
	}
}

// -----------------------------------------------------------------------
// normalizeRow / normalizeRows
// -----------------------------------------------------------------------

func TestNormalizeRow_Numeric(t *testing.T) {
	row := map[string]any{"price": pgtype.Numeric{Int: big.NewInt(1999), Exp: -2, Valid: true}}
	normalizeRow(row)
	if row["price"] != 19.99 {
		t.Fatalf("price = %v, want 19.99", row["price"])
	}
}

func TestNormalizeRow_Time(t *testing.T) {
	row := map[string]any{"duration": pgtype.Time{Microseconds: 29700000000, Valid: true}}
	normalizeRow(row)
	got, ok := row["duration"].(time.Time)
	if !ok {
		t.Fatalf("duration = %T, want time.Time", row["duration"])
	}
	want := time.Date(0, 1, 1, 8, 15, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("duration = %v, want %v", got, want)
	}
}

func TestNormalizeRow_Time_Invalid_LeftUntouched(t *testing.T) {
	raw := pgtype.Time{Valid: false}
	row := map[string]any{"duration": raw}
	normalizeRow(row)
	if row["duration"] != raw {
		t.Fatalf("expected invalid pgtype.Time to be left untouched, got %v", row["duration"])
	}
}

func TestNormalizeRow_UUID(t *testing.T) {
	row := map[string]any{"uid": [16]byte{0x12, 0x3e, 0x45, 0x67, 0xe8, 0x9b, 0x12, 0xd3, 0xa4, 0x56, 0x42, 0x66, 0x14, 0x17, 0x40, 0x00}}
	normalizeRow(row)
	if row["uid"] != "123e4567-e89b-12d3-a456-426614174000" {
		t.Fatalf("uid = %v, want 123e4567-e89b-12d3-a456-426614174000", row["uid"])
	}
}

func TestNormalizeRow_JSONObject(t *testing.T) {
	row := map[string]any{"meta": map[string]any{"key": "value"}}
	normalizeRow(row)
	if row["meta"] != `{"key":"value"}` {
		t.Fatalf("meta = %v, want {\"key\":\"value\"}", row["meta"])
	}
}

func TestNormalizeRow_JSONArray(t *testing.T) {
	row := map[string]any{"meta": []any{"a", "b"}}
	normalizeRow(row)
	if row["meta"] != `["a","b"]` {
		t.Fatalf("meta = %v, want [\"a\",\"b\"]", row["meta"])
	}
}

func TestNormalizeRow_JSONMarshalError_LeftUntouched(t *testing.T) {
	unmarshalable := map[string]any{"bad": make(chan int)}
	row := map[string]any{"meta": unmarshalable}
	normalizeRow(row)
	if v, ok := row["meta"].(map[string]any); !ok || &v == nil {
		t.Fatalf("expected meta to be left as the original map on marshal error, got %#v", row["meta"])
	}
}

func TestNormalizeRow_UnrecognizedType_LeftUntouched(t *testing.T) {
	row := map[string]any{"name": "Ada"}
	normalizeRow(row)
	if row["name"] != "Ada" {
		t.Fatalf("name = %v, want Ada (untouched)", row["name"])
	}
}

func TestNormalizeRows(t *testing.T) {
	rows := []map[string]any{
		{"price": pgtype.Numeric{Int: big.NewInt(100), Exp: 0, Valid: true}},
		{"price": pgtype.Numeric{Int: big.NewInt(200), Exp: 0, Valid: true}},
	}
	normalizeRows(rows)
	if rows[0]["price"] != float64(100) || rows[1]["price"] != float64(200) {
		t.Fatalf("normalizeRows did not normalize every row: %+v", rows)
	}
}
