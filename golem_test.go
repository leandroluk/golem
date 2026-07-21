package golem

import (
	"context"
	"testing"

	"github.com/leandroluk/golem/internal/stmt"
)

// fakeDialect is a minimal Dialect stub -- only exists to satisfy the
// interface so a real Conn can be constructed for these tests. The
// wrappers under test (NewRepository/RunAggregate/Preload) are thin
// delegations already covered end-to-end by internal/repository's own
// tests; these tests only prove golem.go's one-line wrappers call through.
type fakeDialect struct{}

func (fakeDialect) Insert(ctx context.Context, conn Conn, s *stmt.Insert) (map[string]any, error) {
	return map[string]any{}, nil
}
func (fakeDialect) Update(ctx context.Context, conn Conn, s *stmt.Update) ([]map[string]any, error) {
	return nil, nil
}
func (fakeDialect) CompileSelect(s *stmt.Select) (string, []any, error) { return "", nil, nil }
func (fakeDialect) CompileDelete(s *stmt.Delete) (string, []any, error) { return "", nil, nil }
func (fakeDialect) Query(ctx context.Context, conn Conn, sql string, args []any) ([]map[string]any, error) {
	return nil, nil
}
func (fakeDialect) Exec(ctx context.Context, conn Conn, sql string, args []any) (int64, error) {
	return 0, nil
}
func (fakeDialect) IsConflict(err error) bool { return false }
func (fakeDialect) ExecRaw(ctx context.Context, conn Conn, sql string, args []any) ([]map[string]any, int64, error) {
	return nil, 0, nil
}
func (fakeDialect) Begin(ctx context.Context, conn Conn) (TxConn, error) { return nil, nil }

type fakeConnector struct{}

func (fakeConnector) Connect() (Dialect, error) { return fakeDialect{}, nil }
func (fakeConnector) Close() error              { return nil }

// testConn builds a real, connected DataSource -- Conn is a sealed
// interface (unexported isConn method), only internal/core can implement
// it, so a working DataSource via NewDataSource is the only way to get one
// from this package.
func testConn(t *testing.T) Conn {
	t.Helper()
	ds, err := NewDataSource(WithConnector(fakeConnector{}), DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource: %v", err)
	}
	if err := ds.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { ds.Close() })
	return ds
}

type golemTestSubject struct {
	ID   int64
	Name string
}

func newGolemTestEntity() *Entity[golemTestSubject] {
	return NewTable(func(s *golemTestSubject, b *Table) {
		b.TableName("golemtestsubject")
		b.Col(&s.ID, BIGINT())
		b.Col(&s.Name, TEXT())
		b.PrimaryKey(&s.ID)
	})
}

func TestNewTable_BuildsEntity(t *testing.T) {
	e := newGolemTestEntity()
	if e == nil {
		t.Fatal("NewTable returned nil")
	}
	if e.Describe().TableName != "golemtestsubject" {
		t.Fatalf("TableName = %q", e.Describe().TableName)
	}
}

func TestAddHook_RegistersHook(t *testing.T) {
	e := newGolemTestEntity()
	called := false
	AddHook(e).BeforeCreate(func(ctx context.Context, s *golemTestSubject, conn Conn) error {
		called = true
		return nil
	})
	if err := e.TriggerBeforeCreate(context.Background(), &golemTestSubject{}, testConn(t)); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("hook was not called")
	}
}

func TestNewRepository_BuildsRepository(t *testing.T) {
	conn := testConn(t)
	e := newGolemTestEntity()
	repo := NewRepository(conn, e)
	if repo == nil {
		t.Fatal("NewRepository returned nil")
	}
}

func TestRunAggregate_Delegates(t *testing.T) {
	conn := testConn(t)
	repo := NewRepository(conn, newGolemTestEntity())
	_, _ = RunAggregate(context.Background(), repo, func(s *golemTestSubject, res *struct{ Count int64 }, a *Aggregate[golemTestSubject, struct{ Count int64 }]) {
		a.CountAll(&res.Count)
	})
}

func TestPreload_Delegates(t *testing.T) {
	conn := testConn(t)
	repo := NewRepository(conn, newGolemTestEntity())
	_, _ = Preload(context.Background(), repo, []golemTestSubject{}, newGolemTestEntity())
}

func TestNewAggregate_BuildsBuilder(t *testing.T) {
	if NewAggregate[golemTestSubject, struct{}]() == nil {
		t.Fatal("NewAggregate returned nil")
	}
}

func TestNewQuery_BuildsBuilder(t *testing.T) {
	if NewQuery[golemTestSubject]() == nil {
		t.Fatal("NewQuery returned nil")
	}
}

func TestNewUpdate_BuildsBuilder(t *testing.T) {
	if NewUpdate[golemTestSubject]() == nil {
		t.Fatal("NewUpdate returned nil")
	}
}

func TestNewCount_BuildsBuilder(t *testing.T) {
	if NewCount[golemTestSubject]() == nil {
		t.Fatal("NewCount returned nil")
	}
}

func TestNewJoin_BuildsBuilder(t *testing.T) {
	if NewJoin[golemTestSubject]() == nil {
		t.Fatal("NewJoin returned nil")
	}
}

type golemJoinSubject struct {
	ID        int64
	SubjectID int64
}

func newGolemJoinEntity() *Entity[golemJoinSubject] {
	return NewTable(func(s *golemJoinSubject, b *Table) {
		b.TableName("golemjoinsubject")
		b.Col(&s.ID, BIGINT())
		b.Col(&s.SubjectID, BIGINT())
		b.PrimaryKey(&s.ID)
	})
}

func TestJoinInner_RegistersJoin(t *testing.T) {
	q := NewQuery[golemTestSubject]()
	JoinInner(q, newGolemJoinEntity(), func(j *golemJoinSubject, q1 *Join[golemJoinSubject]) {})
	if len(q.Joins()) != 1 {
		t.Fatalf("Joins() len = %d, want 1", len(q.Joins()))
	}
}

func TestJoinLeft_RegistersJoin(t *testing.T) {
	q := NewQuery[golemTestSubject]()
	JoinLeft(q, newGolemJoinEntity(), func(j *golemJoinSubject, q1 *Join[golemJoinSubject]) {})
	if len(q.Joins()) != 1 {
		t.Fatalf("Joins() len = %d, want 1", len(q.Joins()))
	}
}

func TestJoinRight_RegistersJoin(t *testing.T) {
	q := NewQuery[golemTestSubject]()
	JoinRight(q, newGolemJoinEntity(), func(j *golemJoinSubject, q1 *Join[golemJoinSubject]) {})
	if len(q.Joins()) != 1 {
		t.Fatalf("Joins() len = %d, want 1", len(q.Joins()))
	}
}

func TestJoinFull_RegistersJoin(t *testing.T) {
	q := NewQuery[golemTestSubject]()
	JoinFull(q, newGolemJoinEntity(), func(j *golemJoinSubject, q1 *Join[golemJoinSubject]) {})
	if len(q.Joins()) != 1 {
		t.Fatalf("Joins() len = %d, want 1", len(q.Joins()))
	}
}
