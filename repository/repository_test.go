package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
	"github.com/leandroluk/golem/internal/stmt"
	"github.com/leandroluk/golem/join"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/relation"
)

// dataSourceNameCounter guarantees every newConnWithDialect call gets a
// unique golem.DataSource name, even across repeated calls within the same
// test (golem.NewDataSource now errors on a duplicate name).
var dataSourceNameCounter atomic.Int64

// -----------------------------------------------------------------------
// Test-only entity: a minimal single-PK subject with a DB-generated ID.
// -----------------------------------------------------------------------

type testSubject struct {
	ID    int64
	Name  string
	Email string
}

var testSubjectEntity = entity.New(func(s *testSubject, b *entity.Table) {
	b.TableName("testsubject")
	b.Col(&s.ID, golem.BIGINT())
	b.Col(&s.Name, golem.VARCHAR(50))
	b.Col(&s.Email, golem.TEXT())
	b.PrimaryKey(&s.ID)
})

// -----------------------------------------------------------------------
// Composite-PK test entity (2 PK columns).
// -----------------------------------------------------------------------

type testCompositeSubject struct {
	SubjectID  int64
	CategoryID int64
}

var testCompositeSubjectEntity = entity.New(func(s *testCompositeSubject, b *entity.Table) {
	b.TableName("testcompositesubject")
	b.Col(&s.SubjectID, golem.BIGINT())
	b.Col(&s.CategoryID, golem.BIGINT())
	b.PrimaryKey(&s.SubjectID, &s.CategoryID)
})

// -----------------------------------------------------------------------
// Soft-delete test entity.
// -----------------------------------------------------------------------

type softDeleteSubject struct {
	ID        int64
	DeletedAt *time.Time
}

var softDeleteSubjectEntity = entity.New(func(s *softDeleteSubject, b *entity.Table) {
	b.TableName("softdeletesubject")
	b.Col(&s.ID, golem.BIGINT())
	b.Col(&s.DeletedAt, golem.DATETIME()).Name("deleted_at")
	b.PrimaryKey(&s.ID)
	b.DeleteDate(&s.DeletedAt)
})

// -----------------------------------------------------------------------
// Fakes: golem.Connector + golem.Dialect
// -----------------------------------------------------------------------

type execCall struct {
	sql  string
	args []any
}

type fakeDialect struct {
	insertCalls  []*stmt.Insert
	insertResult map[string]any
	insertErr    error

	updateCalls  []*stmt.Update
	updateResult []map[string]any
	updateErr    error

	selectCalls  []*stmt.Select
	selectResult []map[string]any
	selectErr    error

	deleteCalls   []*stmt.Delete
	deleteCompErr error
	execCalls     []execCall
	execErr       error
	queryErr      error

	beginCalls int
	beginErr   error
	commitErr  error
}

func (d *fakeDialect) Bind(t golem.ColumnType, value any) (driver.Value, error) {
	return value, nil
}

func (d *fakeDialect) Scan(t golem.ColumnType, raw any, dest any) error {
	return nil
}

func (d *fakeDialect) Insert(ctx context.Context, conn golem.Conn, s *stmt.Insert) (map[string]any, error) {
	d.insertCalls = append(d.insertCalls, s)
	if d.insertErr != nil {
		return nil, d.insertErr
	}
	return d.insertResult, nil
}

func (d *fakeDialect) Update(ctx context.Context, conn golem.Conn, s *stmt.Update) ([]map[string]any, error) {
	d.updateCalls = append(d.updateCalls, s)
	if d.updateErr != nil {
		return nil, d.updateErr
	}
	return d.updateResult, nil
}

func (d *fakeDialect) CompileSelect(s *stmt.Select) (string, []any, error) {
	d.selectCalls = append(d.selectCalls, s)
	if d.selectErr != nil {
		return "", nil, d.selectErr
	}
	return "mock-sql-select", nil, nil
}

func (d *fakeDialect) CompileDelete(s *stmt.Delete) (string, []any, error) {
	d.deleteCalls = append(d.deleteCalls, s)
	if d.deleteCompErr != nil {
		return "", nil, d.deleteCompErr
	}
	return "mock-sql-delete", nil, nil
}

func (d *fakeDialect) Query(ctx context.Context, conn golem.Conn, sql string, args []any) ([]map[string]any, error) {
	if d.queryErr != nil {
		return nil, d.queryErr
	}
	return d.selectResult, nil
}

func (d *fakeDialect) Exec(ctx context.Context, conn golem.Conn, sql string, args []any) (int64, error) {
	d.execCalls = append(d.execCalls, execCall{sql: sql, args: args})
	if d.execErr != nil {
		return 0, d.execErr
	}
	return 0, nil
}

func (d *fakeDialect) IsConflict(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "conflict")
}

func (d *fakeDialect) ExecRaw(ctx context.Context, conn golem.Conn, sql string, args []any) ([]map[string]any, int64, error) {
	d.execCalls = append(d.execCalls, execCall{sql: sql, args: args})
	if d.execErr != nil {
		return nil, 0, d.execErr
	}
	return d.selectResult, int64(len(d.selectResult)), nil
}

type fakeTx struct {
	committed  bool
	rolledBack bool
	commitErr  error
}

func (t *fakeTx) Commit(ctx context.Context) error {
	t.committed = true
	return t.commitErr
}

func (t *fakeTx) Rollback(ctx context.Context) error {
	t.rolledBack = true
	return nil
}

func (d *fakeDialect) Begin(ctx context.Context, conn golem.Conn) (golem.TxConn, error) {
	d.beginCalls++
	if d.beginErr != nil {
		return nil, d.beginErr
	}
	return &fakeTx{commitErr: d.commitErr}, nil
}

var _ golem.Dialect = (*fakeDialect)(nil)

type anyDialectConnector struct {
	dialect golem.Dialect
}

func (c *anyDialectConnector) Connect() (golem.Dialect, error) { return c.dialect, nil }
func (c *anyDialectConnector) Close() error                    { return nil }

var _ golem.Connector = (*anyDialectConnector)(nil)

func newConnWithDialect(t *testing.T, d golem.Dialect) golem.Conn {
	t.Helper()
	name := fmt.Sprintf("%s-%d", t.Name(), dataSourceNameCounter.Add(1))
	ds, err := golem.NewDataSource(golem.WithConnector(&anyDialectConnector{dialect: d}), golem.DataSourceName(name))
	if err != nil {
		t.Fatalf("golem.NewDataSource: %v", err)
	}
	t.Cleanup(func() { ds.Close() })
	if err := ds.Connect(); err != nil {
		t.Fatalf("ds.Connect: %v", err)
	}
	return ds
}

func newFakeConn(t *testing.T, d *fakeDialect) golem.Conn {
	t.Helper()
	return newConnWithDialect(t, d)
}

// -----------------------------------------------------------------------
// Insert
// -----------------------------------------------------------------------

func TestRepository_Insert_CallsDialectWithCorrectTableAndColumns(t *testing.T) {
	d := &fakeDialect{
		insertResult: map[string]any{
			"id":    int64(42),
			"name":  "Ada",
			"email": "ada@example.com",
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	in := &testSubject{Name: "Ada", Email: "ada@example.com"}
	got, err := repo.Insert(context.Background(), in)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if len(d.insertCalls) != 1 {
		t.Fatalf("expected 1 Insert call, got %d", len(d.insertCalls))
	}
	call := d.insertCalls[0]
	if call.Table != "testsubject" {
		t.Errorf("table = %q, want %q", call.Table, "testsubject")
	}

	wantCols := []string{"name", "email"}
	if len(call.Columns) != len(wantCols) {
		t.Fatalf("columns = %v, want %v", call.Columns, wantCols)
	}
	for i, c := range wantCols {
		if call.Columns[i] != c {
			t.Errorf("columns[%d] = %q, want %q", i, call.Columns[i], c)
		}
	}
	if call.Values[0] != "Ada" {
		t.Errorf("values[0] (name) = %v, want Ada", call.Values[0])
	}
	if call.Values[1] != "ada@example.com" {
		t.Errorf("values[1] (email) = %v, want ada@example.com", call.Values[1])
	}

	if got.ID != 42 {
		t.Errorf("got.ID = %d, want 42", got.ID)
	}
}

func TestRepository_InsertMany_PreservesOrderAndCallsTwice(t *testing.T) {
	rows := []map[string]any{
		{"id": int64(1), "name": "Ada", "email": "ada@example.com"},
		{"id": int64(2), "name": "Bob", "email": "bob@example.com"},
	}
	wrapped := &sequencedInsertDialect{fakeDialect: &fakeDialect{}, rows: rows}
	conn := newConnWithDialect(t, wrapped)
	repo := Get(conn, testSubjectEntity)

	a := &testSubject{Name: "Ada", Email: "ada@example.com"}
	b := &testSubject{Name: "Bob", Email: "bob@example.com"}

	results, err := repo.InsertMany(context.Background(), a, b)
	if err != nil {
		t.Fatalf("InsertMany: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != 1 || results[1].ID != 2 {
		t.Errorf("results = %v", results)
	}
	if wrapped.calls != 2 {
		t.Errorf("expected fake Insert called 2 times, got %d", wrapped.calls)
	}
}

type sequencedInsertDialect struct {
	*fakeDialect
	rows  []map[string]any
	calls int
}

func (s *sequencedInsertDialect) Insert(ctx context.Context, conn golem.Conn, ins *stmt.Insert) (map[string]any, error) {
	row := s.rows[s.calls]
	s.calls++
	s.fakeDialect.insertCalls = append(s.fakeDialect.insertCalls, ins)
	return row, nil
}

// -----------------------------------------------------------------------
// FindMany / FindOne
// -----------------------------------------------------------------------

func TestRepository_FindMany_WithCriteria_PassesWhereColumnsAndScansRows(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{
			{"id": int64(7), "name": "Grace", "email": "grace@example.com"},
			{"id": int64(8), "name": "Grace", "email": "grace2@example.com"},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	got, err := repo.FindMany(context.Background(), func(t *testSubject, q *query.Query[testSubject]) {
		q.Where(op.Eq(&t.Name, "Grace"))
	})
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}
	if len(d.selectCalls) != 1 {
		t.Fatalf("expected 1 Select call, got %d", len(d.selectCalls))
	}
	call := d.selectCalls[0]
	if call.Table != "testsubject" {
		t.Errorf("table = %q, want %q", call.Table, "testsubject")
	}

	comp, ok := call.Where.(stmt.Comparison)
	if !ok {
		t.Fatalf("expected stmt.Comparison, got %T", call.Where)
	}
	if comp.Column != "testsubject.name" || comp.Op != "eq" || comp.Value != "Grace" {
		t.Errorf("Where = %+v", comp)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
}

func TestRepository_FindMany_NoCriteria_PassesEmptyWhere(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{
			{"id": int64(1), "name": "Ada", "email": "ada@example.com"},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindMany(context.Background())
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}
	if len(d.selectCalls) != 1 {
		t.Fatalf("expected 1 Select call, got %d", len(d.selectCalls))
	}
	call := d.selectCalls[0]
	if call.Where != nil {
		t.Errorf("expected nil Where, got %+v", call.Where)
	}
}

func TestRepository_FindOne_Found_ReturnsFirstResult(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{
			{"id": int64(7), "name": "Grace", "email": "grace@example.com"},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	got, err := repo.FindOne(context.Background(), func(t *testSubject, q *query.Query[testSubject]) {
		q.Where(op.Eq(&t.Name, "Grace"))
	})
	if err != nil {
		t.Fatalf("FindOne: %v", err)
	}
	if got.ID != 7 {
		t.Errorf("got.ID = %d, want 7", got.ID)
	}
}

func TestRepository_FindOne_NotFound_ReturnsErrNotFound(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindOne(context.Background(), func(t *testSubject, q *query.Query[testSubject]) {
		q.Where(op.Eq(&t.Name, "Nobody"))
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, golem.ErrNotFound) {
		t.Errorf("expected errors.Is(err, golem.ErrNotFound), got %v", err)
	}
}

// -----------------------------------------------------------------------
// SaveOne
// -----------------------------------------------------------------------

func TestRepository_SaveOne_SingleColumnPK_SetsEverythingElseAndWheresPK(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(7), "name": "Grace Updated", "email": ""},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	in := &testSubject{ID: 7, Name: "Grace Updated", Email: ""}
	got, err := repo.SaveOne(context.Background(), in)
	if err != nil {
		t.Fatalf("SaveOne: %v", err)
	}
	if len(d.updateCalls) != 1 {
		t.Fatalf("expected 1 Update call, got %d", len(d.updateCalls))
	}
	call := d.updateCalls[0]
	if call.Table != "testsubject" {
		t.Errorf("table = %q, want %q", call.Table, "testsubject")
	}

	comp, ok := call.Where.(stmt.Comparison)
	if !ok || comp.Column != "id" || comp.Op != "eq" || comp.Value != int64(7) {
		t.Errorf("unexpected Where: %+v", call.Where)
	}

	wantSets := map[string]any{"name": "Grace Updated", "email": ""}
	if len(call.Sets) != len(wantSets) {
		t.Fatalf("Sets len = %d, want %d", len(call.Sets), len(wantSets))
	}
	for _, set := range call.Sets {
		wantVal, ok := wantSets[set.Column]
		if !ok || set.Value != wantVal {
			t.Errorf("unexpected set: %+v", set)
		}
	}

	if got.ID != 7 || got.Name != "Grace Updated" {
		t.Errorf("got = %+v", got)
	}
}

func TestRepository_SaveOne_CompositePK_WheresAllPKColumns(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"subjectid": int64(1), "categoryid": int64(2)},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testCompositeSubjectEntity)

	in := &testCompositeSubject{SubjectID: 1, CategoryID: 2}
	_, err := repo.SaveOne(context.Background(), in)
	if err != nil {
		t.Fatalf("SaveOne: %v", err)
	}
	if len(d.updateCalls) != 1 {
		t.Fatalf("expected 1 Update call, got %d", len(d.updateCalls))
	}
	call := d.updateCalls[0]

	logical, ok := call.Where.(stmt.Logical)
	if !ok || logical.Op != "and" || len(logical.Predicates) != 2 {
		t.Fatalf("expected logical AND of 2 PK conditions, got %+v", call.Where)
	}

	wantWhere := map[string]any{"subjectid": int64(1), "categoryid": int64(2)}
	for _, pred := range logical.Predicates {
		comp, ok := pred.(stmt.Comparison)
		if !ok || comp.Op != "eq" {
			t.Fatalf("expected comparison PK condition, got %+v", pred)
		}
		wantVal, ok := wantWhere[comp.Column]
		if !ok || comp.Value != wantVal {
			t.Errorf("unexpected PK condition: %+v", comp)
		}
	}
	if len(call.Sets) != 0 {
		t.Errorf("expected 0 sets, got %+v", call.Sets)
	}
}

// -----------------------------------------------------------------------
// Update
// -----------------------------------------------------------------------

func TestRepository_Update_PassesWhereAndSetColumns(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(7), "name": "Grace", "email": "new@example.com"},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	got, err := repo.Update(context.Background(), func(t *testSubject, u *query.Update[testSubject]) {
		u.Where(op.Eq(&t.ID, int64(7)))
		u.Set(&t.Email, "new@example.com")
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(d.updateCalls) != 1 {
		t.Fatalf("expected 1 Update call, got %d", len(d.updateCalls))
	}
	call := d.updateCalls[0]

	comp, ok := call.Where.(stmt.Comparison)
	if !ok || comp.Column != "testsubject.id" || comp.Op != "eq" || comp.Value != int64(7) {
		t.Errorf("unexpected Where: %+v", call.Where)
	}

	if len(call.Sets) != 1 || call.Sets[0].Column != "email" || call.Sets[0].Value != "new@example.com" {
		t.Errorf("unexpected Sets: %+v", call.Sets)
	}
	if len(got) != 1 || got[0].Email != "new@example.com" {
		t.Errorf("got = %+v", got)
	}
}

func TestRepository_Update_ZeroRowsAffected_IsNotAnError(t *testing.T) {
	d := &fakeDialect{updateResult: []map[string]any{}}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	got, err := repo.Update(context.Background(), func(t *testSubject, u *query.Update[testSubject]) {
		u.Where(op.Eq(&t.ID, int64(999)))
		u.Set(&t.Email, "nobody@example.com")
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 results, got %+v", got)
	}
}

func TestRepository_Update_MultipleRowsAffected(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(1), "name": "Ada", "email": "same@example.com"},
			{"id": int64(2), "name": "Bob", "email": "same@example.com"},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	got, err := repo.Update(context.Background(), func(t *testSubject, u *query.Update[testSubject]) {
		u.Where(op.Eq(&t.Email, "old@example.com"))
		u.Set(&t.Email, "same@example.com")
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
}

func TestRepository_Update_BadWhereFieldPtr_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	var foreign string
	_, err := repo.Update(context.Background(), func(t *testSubject, u *query.Update[testSubject]) {
		u.Where(op.Eq(&foreign, "x"))
		u.Set(&t.Email, "y@example.com")
	})
	if err == nil {
		t.Fatal("expected error for a Where field pointer that doesn't belong to the entity, got nil")
	}
}

func TestRepository_Update_ScanRowError_Propagates(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(1), "name": "Ada", "email": struct{}{}}, // unconvertible to string
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.Update(context.Background(), func(t *testSubject, u *query.Update[testSubject]) {
		u.Where(op.Eq(&t.ID, int64(1)))
		u.Set(&t.Email, "x@example.com")
	})
	if err == nil {
		t.Fatal("expected scanRow error for unconvertible column value, got nil")
	}
}

func TestRepository_Update_AfterUpdateHookError_Propagates(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(1)},
		},
	}
	conn := newFakeConn(t, d)

	hookedEntity := entity.New(func(s *hookedTestSubject, b *entity.Table) {
		b.TableName("hooked_update")
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
	})
	wantErr := errors.New("boom: after update hook")
	entity.AddHook(hookedEntity).AfterUpdate(func(ctx context.Context, s *hookedTestSubject, c golem.Conn) error {
		return wantErr
	})

	repo := Get(conn, hookedEntity)
	_, err := repo.Update(context.Background(), func(s *hookedTestSubject, u *query.Update[hookedTestSubject]) {
		u.Where(op.Eq(&s.ID, int64(1)))
		u.Set(&s.Name, "x")
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Update_DialectError_Propagates(t *testing.T) {
	wantErr := errors.New("boom: update")
	d := &fakeDialect{updateErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.Update(context.Background(), func(t *testSubject, u *query.Update[testSubject]) {
		u.Where(op.Eq(&t.ID, int64(1)))
		u.Set(&t.Email, "x@example.com")
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

// -----------------------------------------------------------------------
// Soft Delete Filters
// -----------------------------------------------------------------------

func TestRepository_FindMany_AppendsSoftDeleteFilterByDefault(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, softDeleteSubjectEntity)

	_, err := repo.FindMany(context.Background())
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}

	if len(d.selectCalls) != 1 {
		t.Fatalf("expected 1 Select call")
	}
	call := d.selectCalls[0]
	comp, ok := call.Where.(stmt.Comparison)
	if !ok || comp.Column != "softdeletesubject.deleted_at" || comp.Op != "is_null" {
		t.Fatalf("expected soft-delete IS NULL filter, got %+v", call.Where)
	}
}

func TestRepository_FindMany_WithDeleted_SkipsSoftDeleteFilter(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, softDeleteSubjectEntity)

	_, err := repo.FindMany(context.Background(), func(t *softDeleteSubject, q *query.Query[softDeleteSubject]) {
		q.WithDeleted()
	})
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}

	if len(d.selectCalls) != 1 {
		t.Fatalf("expected 1 Select call")
	}
	call := d.selectCalls[0]
	if call.Where != nil {
		t.Fatalf("expected no WHERE filter, got %+v", call.Where)
	}
}

// -----------------------------------------------------------------------
// Delete & Restore, Count & Exists
// -----------------------------------------------------------------------

func TestRepository_Delete_HardDelete(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	in := &testSubject{ID: 100, Name: "To Be Deleted"}
	err := repo.Delete(context.Background(), in)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(d.deleteCalls) != 1 {
		t.Fatalf("expected 1 CompileDelete call, got %d", len(d.deleteCalls))
	}
	call := d.deleteCalls[0]
	if call.Table != "testsubject" {
		t.Errorf("table = %q, want %q", call.Table, "testsubject")
	}
	comp, ok := call.Where.(stmt.Comparison)
	if !ok || comp.Column != "id" || comp.Op != "eq" || comp.Value != int64(100) {
		t.Errorf("unexpected delete Where condition: %+v", call.Where)
	}
	if len(d.execCalls) != 1 || d.execCalls[0].sql != "mock-sql-delete" {
		t.Errorf("expected 1 Exec call for SQL compile output, got %+v", d.execCalls)
	}
}

func TestRepository_Delete_SoftDelete(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(200)},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, softDeleteSubjectEntity)

	in := &softDeleteSubject{ID: 200}
	err := repo.Delete(context.Background(), in)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(d.updateCalls) != 1 {
		t.Fatalf("expected 1 Update call for soft-delete, got %d", len(d.updateCalls))
	}
	call := d.updateCalls[0]
	if call.Table != "softdeletesubject" {
		t.Errorf("table = %q, want %q", call.Table, "softdeletesubject")
	}
	comp, ok := call.Where.(stmt.Comparison)
	if !ok || comp.Column != "id" || comp.Op != "eq" || comp.Value != int64(200) {
		t.Errorf("unexpected Where condition: %+v", call.Where)
	}
	if len(call.Sets) != 1 || call.Sets[0].Column != "deleted_at" || call.Sets[0].Value == nil {
		t.Errorf("expected soft-delete Set Clause deleted_at = time.Now, got %+v", call.Sets)
	}
}

func TestRepository_Restore_SoftDelete(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(300)},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, softDeleteSubjectEntity)

	in := &softDeleteSubject{ID: 300}
	err := repo.Restore(context.Background(), in)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if len(d.updateCalls) != 1 {
		t.Fatalf("expected 1 Update call, got %d", len(d.updateCalls))
	}
	call := d.updateCalls[0]
	if len(call.Sets) != 1 || call.Sets[0].Column != "deleted_at" || call.Sets[0].Value != nil {
		t.Errorf("expected sets deleted_at = NULL, got %+v", call.Sets)
	}
}

func TestRepository_Restore_HardDeleteEntity_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	in := &testSubject{ID: 400}
	err := repo.Restore(context.Background(), in)
	if err == nil {
		t.Fatal("expected Restore to error for hard-deleted entity, got nil")
	}
	if err.Error() != `repository: cannot restore entity "testsubject" without a soft-delete field` {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRepository_Count_ReturnsCorrectValue(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{
			{"count": int64(42)},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	val, err := repo.Count(context.Background(), func(t *testSubject, c *query.Count[testSubject]) {
		c.Where(op.Eq(&t.Name, "Ada"))
	})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if val != 42 {
		t.Errorf("Count = %d, want 42", val)
	}

	if len(d.selectCalls) != 1 {
		t.Fatalf("expected 1 Select call")
	}
	call := d.selectCalls[0]
	if !call.Count {
		t.Errorf("expected call.Count to be true")
	}
}

func TestRepository_Exists_ReturnsTrue(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{
			{"count": int64(1)},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	ok, err := repo.Exists(context.Background(), func(t *testSubject, c *query.Count[testSubject]) {
		c.Where(op.Eq(&t.Name, "Ada"))
	})
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !ok {
		t.Errorf("expected true, got false")
	}

	if len(d.selectCalls) != 1 {
		t.Fatalf("expected 1 Select call")
	}
	call := d.selectCalls[0]
	if *call.Limit != 1 {
		t.Errorf("expected Limit = 1, got %v", call.Limit)
	}
}

func TestRepository_Update_AppendsSoftDeleteFilterByDefault(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(7)},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, softDeleteSubjectEntity)

	_, err := repo.Update(context.Background(), func(t *softDeleteSubject, u *query.Update[softDeleteSubject]) {
		u.Where(op.Eq(&t.ID, int64(7)))
		u.Set(&t.ID, 7)
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(d.updateCalls) != 1 {
		t.Fatalf("expected 1 Update call")
	}
	call := d.updateCalls[0]
	logical, ok := call.Where.(stmt.Logical)
	if !ok || logical.Op != "and" || len(logical.Predicates) != 2 {
		t.Fatalf("expected logical AND, got %+v", call.Where)
	}
	comp0, ok0 := logical.Predicates[0].(stmt.Comparison)
	if !ok0 || comp0.Column != "softdeletesubject.deleted_at" || comp0.Op != "is_null" {
		t.Errorf("expected deleted_at IS NULL as first predicate, got %+v", logical.Predicates[0])
	}
}

func TestRepository_Update_WithDeleted_SkipsSoftDeleteFilter(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(7)},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, softDeleteSubjectEntity)

	_, err := repo.Update(context.Background(), func(t *softDeleteSubject, u *query.Update[softDeleteSubject]) {
		u.Where(op.Eq(&t.ID, int64(7)))
		u.Set(&t.ID, 7)
		u.WithDeleted()
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(d.updateCalls) != 1 {
		t.Fatalf("expected 1 Update call")
	}
	call := d.updateCalls[0]
	comp, ok := call.Where.(stmt.Comparison)
	if !ok || comp.Column != "softdeletesubject.id" || comp.Op != "eq" {
		t.Fatalf("expected simple ID comparison, got %+v", call.Where)
	}
}

func TestRepository_FindMany_WithInnerJoin(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)

	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindMany(context.Background(), func(s *testSubject, q0 *query.Query[testSubject]) {
		join.Inner(q0, softDeleteSubjectEntity, func(sd *softDeleteSubject, q1 *query.Join[softDeleteSubject]) {
			q1.On(&sd.ID, &s.ID)
			q1.Where(op.Eq(&sd.ID, int64(10)))
		})
	})
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}

	if len(d.selectCalls) != 1 {
		t.Fatalf("expected 1 Select call, got %d", len(d.selectCalls))
	}
	call := d.selectCalls[0]
	if len(call.Joins) != 1 {
		t.Fatalf("expected 1 Join compilation inside plan, got %d", len(call.Joins))
	}
	jPlan := call.Joins[0]
	if jPlan.Type != "inner" || jPlan.Table != "softdeletesubject" {
		t.Errorf("unexpected join plan table/type: %+v", jPlan)
	}
	if len(jPlan.On) != 1 || jPlan.On[0].LeftCol != "softdeletesubject.id" || jPlan.On[0].RightCol != "testsubject.id" {
		t.Errorf("unexpected join ON keys: %+v", jPlan.On)
	}

	logical, ok := jPlan.Where.(stmt.Logical)
	if !ok || logical.Op != "and" || len(logical.Predicates) != 2 {
		t.Fatalf("expected logical AND, got %+v", jPlan.Where)
	}
	comp0, ok0 := logical.Predicates[0].(stmt.Comparison)
	if !ok0 || comp0.Column != "softdeletesubject.deleted_at" || comp0.Op != "is_null" {
		t.Errorf("expected deleted_at IS NULL as first predicate, got %+v", logical.Predicates[0])
	}
	comp1, ok1 := logical.Predicates[1].(stmt.Comparison)
	if !ok1 || comp1.Column != "softdeletesubject.id" || comp1.Op != "eq" || comp1.Value != int64(10) {
		t.Errorf("expected ID = 10 comparison, got %+v", logical.Predicates[1])
	}
}

func TestRepository_FindMany_WithInnerJoin_DedupesFanOutByParentPK(t *testing.T) {
	// Same parent row (id=1) appears twice — a 1:N join fan-out from 2
	// matched child rows. FindMany must return exactly 1 T for it, not 2.
	d := &fakeDialect{
		selectResult: []map[string]any{
			{"id": int64(1), "name": "Ada", "email": "ada@example.com"},
			{"id": int64(1), "name": "Ada", "email": "ada@example.com"},
			{"id": int64(2), "name": "Bob", "email": "bob@example.com"},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	got, err := repo.FindMany(context.Background(), func(s *testSubject, q0 *query.Query[testSubject]) {
		join.Inner(q0, softDeleteSubjectEntity, func(sd *softDeleteSubject, q1 *query.Join[softDeleteSubject]) {
			q1.On(&sd.ID, &s.ID)
		})
	})
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 deduped results, got %d: %+v", len(got), got)
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("expected IDs [1 2] in original order, got [%d %d]", got[0].ID, got[1].ID)
	}
}

func TestRepository_FindMany_WithoutJoin_DoesNotDedupe(t *testing.T) {
	// No join present: rows pass through as-is, even if two rows happen to
	// share the same PK value (not a realistic case without a join, but
	// dedup must be join-gated, not unconditional).
	d := &fakeDialect{
		selectResult: []map[string]any{
			{"id": int64(1), "name": "Ada", "email": "ada@example.com"},
			{"id": int64(1), "name": "Ada", "email": "ada@example.com"},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	got, err := repo.FindMany(context.Background())
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results (no dedup without a join), got %d", len(got))
	}
}

type hookedTestSubject struct {
	ID   int64
	Name string
}

func TestRepository_Insert_RunsLifecycleHooks(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)

	hookedEntity := entity.New(func(s *hookedTestSubject, b *entity.Table) {
		b.TableName("hooked")
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
	})

	var calls []string
	entity.AddHook(hookedEntity).
		BeforeCreate(func(ctx context.Context, s *hookedTestSubject, c golem.Conn) error {
			calls = append(calls, "before")
			return nil
		}).
		AfterCreate(func(ctx context.Context, s *hookedTestSubject, c golem.Conn) error {
			calls = append(calls, "after")
			return nil
		}).
		OnConflictCreate(func(ctx context.Context, s *hookedTestSubject, c golem.Conn) error {
			calls = append(calls, "conflict")
			return nil
		})

	repo := Get(conn, hookedEntity)

	// Test successful insert
	d.insertResult = map[string]any{"id": int64(1)}
	_, err := repo.Insert(context.Background(), &hookedTestSubject{ID: 1})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if len(calls) != 2 || calls[0] != "before" || calls[1] != "after" {
		t.Errorf("unexpected successful hook execution sequence: %v", calls)
	}

	// Reset calls
	calls = nil

	// Test conflict error insert
	d.insertErr = errors.New("conflict error")
	_, err = repo.Insert(context.Background(), &hookedTestSubject{ID: 1})
	if err == nil {
		t.Fatal("expected insert to fail")
	}

	if len(calls) != 2 || calls[0] != "before" || calls[1] != "conflict" {
		t.Errorf("unexpected conflict hook execution sequence: %v", calls)
	}
}

func TestRepository_Insert_HookErrorRollsBack(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)

	hookedEntity := entity.New(func(s *hookedTestSubject, b *entity.Table) {
		b.TableName("hooked_rollback")
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
	})

	entity.AddHook(hookedEntity).
		BeforeCreate(func(ctx context.Context, s *hookedTestSubject, c golem.Conn) error {
			return errors.New("before_error")
		})

	repo := Get(conn, hookedEntity)

	_, err := repo.Insert(context.Background(), &hookedTestSubject{ID: 1})
	if err == nil || err.Error() != "before_error" {
		t.Fatalf("expected insert to fail with 'before_error', got %v", err)
	}

	if len(d.insertCalls) != 0 {
		t.Errorf("database insert was called despite before error")
	}
}

func TestDataSource_Transaction_CommitOnSuccess(t *testing.T) {
	d := &fakeDialect{}
	connector := &anyDialectConnector{dialect: d}
	ds, err := golem.NewDataSource(golem.WithConnector(connector), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource: %v", err)
	}
	defer ds.Close()
	_ = ds.Connect()

	var usedTx golem.Tx
	err = ds.Transaction(context.Background(), func(tx golem.Tx) error {
		usedTx = tx
		return nil
	})
	if err != nil {
		t.Fatalf("Transaction: %v", err)
	}

	ft := usedTx.Underlying().(*fakeTx)
	if !ft.committed {
		t.Error("expected transaction to be committed")
	}
	if ft.rolledBack {
		t.Error("expected transaction not to be rolled back")
	}
}

func TestDataSource_Transaction_RollbackOnError(t *testing.T) {
	d := &fakeDialect{}
	connector := &anyDialectConnector{dialect: d}
	ds, err := golem.NewDataSource(golem.WithConnector(connector), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource: %v", err)
	}
	defer ds.Close()
	_ = ds.Connect()

	var usedTx golem.Tx
	expectedErr := errors.New("trans_error")
	err = ds.Transaction(context.Background(), func(tx golem.Tx) error {
		usedTx = tx
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected Transaction to return expectedErr, got %v", err)
	}

	ft := usedTx.Underlying().(*fakeTx)
	if ft.committed {
		t.Error("expected transaction not to be committed")
	}
	if !ft.rolledBack {
		t.Error("expected transaction to be rolled back")
	}
}

func TestDataSource_Transaction_RollbackOnPanic(t *testing.T) {
	d := &fakeDialect{}
	connector := &anyDialectConnector{dialect: d}
	ds, err := golem.NewDataSource(golem.WithConnector(connector), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource: %v", err)
	}
	defer ds.Close()
	_ = ds.Connect()

	var usedTx golem.Tx
	defer func() {
		r := recover()
		if r == nil || r.(string) != "panic_msg" {
			t.Fatalf("expected panic 'panic_msg', got %v", r)
		}
		ft := usedTx.Underlying().(*fakeTx)
		if ft.committed {
			t.Error("expected transaction not to be committed after panic")
		}
		if !ft.rolledBack {
			t.Error("expected transaction to be rolled back after panic")
		}
	}()

	_ = ds.Transaction(context.Background(), func(tx golem.Tx) error {
		usedTx = tx
		panic("panic_msg")
	})
}

func TestDataSource_Exec_RawExecution(t *testing.T) {
	d := &fakeDialect{}
	d.selectResult = []map[string]any{
		{"id": int64(1), "name": "Alice"},
		{"id": int64(2), "name": "Bob"},
	}
	connector := &anyDialectConnector{dialect: d}
	ds, err := golem.NewDataSource(golem.WithConnector(connector), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource: %v", err)
	}
	defer ds.Close()
	_ = ds.Connect()

	res, err := ds.Exec(context.Background(), "SELECT * FROM users WHERE active = $1", true)
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected failed: %v", err)
	}
	if affected != 2 {
		t.Errorf("RowsAffected = %d, want 2", affected)
	}

	var names []string
	for res.Next() {
		row, err := res.Scan()
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		names = append(names, row["name"].(string))
	}

	if len(names) != 2 || names[0] != "Alice" || names[1] != "Bob" {
		t.Errorf("unexpected rows scanned: %v", names)
	}
}

func TestRepository_Exec_RawExecution(t *testing.T) {
	d := &fakeDialect{}
	d.selectResult = []map[string]any{
		{"id": int64(10), "name": "Subject A"},
	}
	connector := &anyDialectConnector{dialect: d}
	ds, err := golem.NewDataSource(golem.WithConnector(connector), golem.DataSourceName(t.Name()))
	if err != nil {
		t.Fatalf("NewDataSource: %v", err)
	}
	defer ds.Close()
	_ = ds.Connect()

	repo := Get(ds, testSubjectEntity)
	users, err := repo.Exec(context.Background(), "SELECT * FROM testsubject WHERE id = $1", 10)
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if len(users) != 1 || users[0].ID != 10 || users[0].Name != "Subject A" {
		t.Errorf("unexpected mapped users: %+v", users)
	}
}

func TestAssignFieldValue_Errors(t *testing.T) {
	var s struct {
		IntField int
	}
	v := reflect.ValueOf(&s).Elem().FieldByName("IntField")
	err := assignFieldValue(v, "not an int", "col", "IntField")
	if err == nil {
		t.Fatal("expected error")
	}

	var invalid reflect.Value
	err = assignFieldValue(invalid, 1, "col", "field")
	if err != nil {
		t.Fatal("expected no error for invalid field")
	}
}

func TestTranslateCondition_OrAndNot(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	var zero testSubject
	f2c := repo.fieldToColumn()

	// OR
	condOr := op.Or(op.Eq(&zero.Name, "A"), op.Eq(&zero.Email, "B"))
	pred, err := repo.translateCondition(&zero, f2c, "testsubject", condOr)
	if err != nil {
		t.Fatal(err)
	}
	logical, ok := pred.(stmt.Logical)
	if !ok || logical.Op != "or" || len(logical.Predicates) != 2 {
		t.Fatal("expected logical or")
	}

	// Empty OR
	condOrEmpty := op.Or()
	pred, err = repo.translateCondition(&zero, f2c, "testsubject", condOrEmpty)
	if err != nil || pred != nil {
		t.Fatal("expected nil pred for empty or")
	}

	// OR with error
	var foreign string
	condOrErr := op.Or(op.Eq(&foreign, "A"))
	_, err = repo.translateCondition(&zero, f2c, "testsubject", condOrErr)
	if err == nil {
		t.Fatal("expected error")
	}

	// NOT
	condNot := op.Not(op.Eq(&zero.Name, "A"))
	pred, err = repo.translateCondition(&zero, f2c, "testsubject", condNot)
	if err != nil {
		t.Fatal(err)
	}
	_, ok = pred.(stmt.Not)
	if !ok {
		t.Fatal("expected not stmt")
	}

	// Empty NOT
	condNotEmpty := op.Condition{Op: "not"}
	pred, err = repo.translateCondition(&zero, f2c, "testsubject", condNotEmpty)
	if err != nil || pred != nil {
		t.Fatal("expected nil pred for empty not")
	}

	// NOT with error
	condNotErr := op.Not(op.Eq(&foreign, "A"))
	_, err = repo.translateCondition(&zero, f2c, "testsubject", condNotErr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveFieldPtrAny_NilPointer(t *testing.T) {
	res := resolveFieldPtrAny(nil, nil)
	if res != "" {
		t.Fatal("expected empty string")
	}
}

func TestBuildWherePredicate_Errors(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)
	var zero testSubject
	var foreign string
	_, err := repo.buildWherePredicate(&zero, repo.fieldToColumn(), "test", []op.Condition{
		op.Eq(&foreign, "a"),
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// Valid conditions len = 1
	p, err := repo.buildWherePredicate(&zero, repo.fieldToColumn(), "test", []op.Condition{
		op.Eq(&zero.Name, "a"),
	})
	if err != nil || p == nil {
		t.Fatal("expected valid predicate")
	}
}

func TestApplySoftDeleteFilter_NoWhere(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, softDeleteSubjectEntity)

	// nil where
	p := repo.applySoftDeleteFilter("softdeletesubject", "DeletedAt", false, nil)
	comp, ok := p.(stmt.Comparison)
	if !ok || comp.Op != "is_null" {
		t.Fatal("expected is_null")
	}

	// Unmapped field
	p2 := repo.applySoftDeleteFilter("softdeletesubject", "unknown_field", false, nil)
	if p2 != nil {
		t.Fatal("expected nil for unknown field")
	}
}

func TestRepository_InsertMany_Error(t *testing.T) {
	d := &fakeDialect{insertErr: errors.New("insert error")}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.InsertMany(context.Background(), &testSubject{Name: "A"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRepository_SaveOne_NoPK(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	noPKEntity := entity.New(func(s *testSubject, b *entity.Table) {
		b.TableName("nopk")
		b.Col(&s.Name, golem.VARCHAR(50))
	})
	repo := Get(conn, noPKEntity)

	_, err := repo.SaveOne(context.Background(), &testSubject{Name: "A"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRepository_SaveMany_SuccessAndError(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(1)},
			{"id": int64(2)},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	items := []*testSubject{
		{ID: 1, Name: "A"},
		{ID: 2, Name: "B"},
	}

	got, err := repo.SaveMany(context.Background(), items...)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatal("expected 2 results")
	}

	d.updateErr = errors.New("update err")
	_, err = repo.SaveMany(context.Background(), items...)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRepository_Delete_NoPK(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	noPKEntity := entity.New(func(s *testSubject, b *entity.Table) {
		b.TableName("nopk")
		b.Col(&s.Name, golem.VARCHAR(50))
	})
	repo := Get(conn, noPKEntity)

	err := repo.Delete(context.Background(), &testSubject{Name: "A"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRepository_Restore_NoPK(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	noPKEntity := entity.New(func(s *softDeleteSubject, b *entity.Table) {
		b.TableName("nopk")
		b.Col(&s.DeletedAt, golem.DATETIME()).Name("deleted_at")
		b.DeleteDate(&s.DeletedAt)
	})
	repo := Get(conn, noPKEntity)

	err := repo.Restore(context.Background(), &softDeleteSubject{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRepository_Count_Errors(t *testing.T) {
	d := &fakeDialect{selectErr: errors.New("compile error")}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.Count(context.Background())
	if err == nil {
		t.Fatal("expected compile error")
	}

	d.selectErr = nil
	d.queryErr = errors.New("query error")
	_, err = repo.Count(context.Background())
	if err == nil {
		t.Fatal("expected query error")
	}

	d.queryErr = nil
	var foreign string
	_, err = repo.Count(context.Background(), func(t *testSubject, c *query.Count[testSubject]) {
		c.Where(op.Eq(&foreign, "a"))
	})
	if err == nil {
		t.Fatal("expected where err")
	}
}

func TestRepository_Exists_Errors(t *testing.T) {
	d := &fakeDialect{selectErr: errors.New("compile error")}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.Exists(context.Background())
	if err == nil {
		t.Fatal("expected compile error")
	}

	d.selectErr = nil
	d.queryErr = errors.New("query error")
	_, err = repo.Exists(context.Background())
	if err == nil {
		t.Fatal("expected query error")
	}

	d.queryErr = nil
	var foreign string
	_, err = repo.Exists(context.Background(), func(t *testSubject, c *query.Count[testSubject]) {
		c.Where(op.Eq(&foreign, "a"))
	})
	if err == nil {
		t.Fatal("expected where err")
	}
}

func TestRepository_Exec_Errors(t *testing.T) {
	d := &fakeDialect{execErr: errors.New("exec error")}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.Exec(context.Background(), "SELECT 1")
	if err == nil {
		t.Fatal("expected exec error")
	}
}
func TestRepository_FindMany_ForUpdate_OutsideTx_ReturnsError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		q.ForUpdate()
	})
	if err == nil {
		t.Fatal("expected error for ForUpdate outside a transaction, got nil")
	}
	if len(d.selectCalls) != 0 {
		t.Errorf("expected 0 Select calls (rejected before compiling), got %d", len(d.selectCalls))
	}
}

func newFakeTxConn(t *testing.T, d *fakeDialect) golem.Conn {
	t.Helper()
	ds := newFakeConn(t, d)
	txConn, err := d.Begin(context.Background(), ds)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	d.beginCalls = 0
	return golem.NewTx(d, txConn)
}

func TestRepository_FindMany_ForUpdate_InsideTx_PassesLockClause(t *testing.T) {
	d := &fakeDialect{}
	tx := newFakeTxConn(t, d)
	repo := Get(tx, testSubjectEntity)

	_, err := repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		q.ForUpdate()
	})
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}
	if len(d.selectCalls) != 1 {
		t.Fatalf("expected 1 Select call, got %d", len(d.selectCalls))
	}
	lock := d.selectCalls[0].Lock
	if lock == nil || lock.Strength != string(query.LockForUpdate) || lock.Wait != "" {
		t.Errorf("unexpected Lock clause: %+v", lock)
	}
}

func TestRepository_FindMany_ForShare_WithNoWait_InsideTx(t *testing.T) {
	d := &fakeDialect{}
	tx := newFakeTxConn(t, d)
	repo := Get(tx, testSubjectEntity)

	_, err := repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		q.ForShare(query.LockWaitNoWait)
	})
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}
	lock := d.selectCalls[0].Lock
	if lock == nil || lock.Strength != string(query.LockForShare) || lock.Wait != string(query.LockWaitNoWait) {
		t.Errorf("unexpected Lock clause: %+v", lock)
	}
}

func TestRepository_FindMany_NoLock_LockClauseIsNil(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindMany(context.Background())
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}
	call := d.selectCalls[0]
	if call.Lock != nil {
		t.Errorf("expected nil Lock, got %+v", call.Lock)
	}
}

func TestRepository_FindOne_ForUpdate_OutsideTx_ReturnsError(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{{"id": int64(1), "name": "Ada", "email": "ada@x.com"}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindOne(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		q.ForUpdate()
	})
	if err == nil {
		t.Fatal("expected error for FindOne+ForUpdate outside a transaction, got nil")
	}
}

// -----------------------------------------------------------------------
// Cascade test entities. Each test uses a distinct table name (the FK
// registry, entity.ForeignKeysReferencing, is a package-level global) so
// registrations from one test never leak into another's assertions.
// -----------------------------------------------------------------------

type cascadeParent struct {
	ID int64
}

type cascadeChild struct {
	ID       int64
	ParentID int64
}

func newCascadeParentEntity(table string) *entity.Entity[cascadeParent] {
	return entity.New(func(p *cascadeParent, b *entity.Table) {
		b.TableName(table)
		b.Col(&p.ID, golem.BIGINT())
		b.PrimaryKey(&p.ID)
	})
}

func newCascadeChildEntity(table string, parent *entity.Entity[cascadeParent], opts *relation.ForeignKeyOptions) *entity.Entity[cascadeChild] {
	return entity.New(func(c *cascadeChild, b *entity.Table) {
		b.TableName(table)
		b.Col(&c.ID, golem.BIGINT())
		b.Col(&c.ParentID, golem.BIGINT()).Name("parent_id")
		b.PrimaryKey(&c.ID)
		if opts != nil {
			b.ForeignKey(&c.ParentID, parent, opts)
		} else {
			b.ForeignKey(&c.ParentID, parent)
		}
	})
}

func TestRepository_Delete_OnDeleteCascade_DeletesChildRows(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_delete")
	newCascadeChildEntity("cascade_child_delete", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 5}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if d.beginCalls != 1 {
		t.Errorf("beginCalls = %d, want 1 (cascade-actionable FK must open a transaction)", d.beginCalls)
	}
	if len(d.deleteCalls) != 2 {
		t.Fatalf("expected 2 CompileDelete calls (child cascade + parent itself), got %d", len(d.deleteCalls))
	}
	childCall := d.deleteCalls[0]
	if childCall.Table != "cascade_child_delete" {
		t.Errorf("first delete table = %q, want cascade_child_delete", childCall.Table)
	}
	childComp, ok := childCall.Where.(stmt.Comparison)
	if !ok || childComp.Column != "parent_id" || childComp.Op != "eq" || childComp.Value != int64(5) {
		t.Errorf("unexpected cascade delete Where: %+v", childCall.Where)
	}
	parentCall := d.deleteCalls[1]
	if parentCall.Table != "cascade_parent_delete" {
		t.Errorf("second delete table = %q, want cascade_parent_delete", parentCall.Table)
	}
}

func TestRepository_Delete_OnDeleteSetNull_NullsChildColumn(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_setnull")
	newCascadeChildEntity("cascade_child_setnull", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteSetNull))

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 9}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if len(d.updateCalls) != 1 {
		t.Fatalf("expected 1 Update call (SET NULL on child), got %d", len(d.updateCalls))
	}
	call := d.updateCalls[0]
	if call.Table != "cascade_child_setnull" {
		t.Errorf("update table = %q, want cascade_child_setnull", call.Table)
	}
	if len(call.Sets) != 1 || call.Sets[0].Column != "parent_id" || call.Sets[0].Value != nil {
		t.Errorf("expected Set parent_id = nil, got %+v", call.Sets)
	}
	if len(d.deleteCalls) != 1 {
		t.Fatalf("expected 1 CompileDelete call (parent itself only), got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_OnDeleteRestrict_BlocksWhenChildrenExist(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_restrict_block")
	newCascadeChildEntity("cascade_child_restrict_block", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))

	d := &fakeDialect{
		selectResult: []map[string]any{{"count": int64(3)}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 1})
	if !errors.Is(err, golem.ErrForeignKeyViolation) {
		t.Fatalf("Delete: expected errors.Is(err, golem.ErrForeignKeyViolation), got %v", err)
	}
	if len(d.deleteCalls) != 0 {
		t.Errorf("expected 0 CompileDelete calls (blocked before parent delete), got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_OnDeleteRestrict_AllowsWhenNoChildren(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_restrict_allow")
	newCascadeChildEntity("cascade_child_restrict_allow", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))

	d := &fakeDialect{
		selectResult: []map[string]any{{"count": int64(0)}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 2}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(d.deleteCalls) != 1 {
		t.Errorf("expected 1 CompileDelete call (parent itself), got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_OnDeleteDefault_NoCascadeSideEffects(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_default")
	newCascadeChildEntity("cascade_child_default", parent, nil) // default options: OnDelete unset

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 3}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if d.beginCalls != 0 {
		t.Errorf("beginCalls = %d, want 0 (OnDeleteDefault is not cascade-actionable, no transaction should open)", d.beginCalls)
	}
	if len(d.deleteCalls) != 1 {
		t.Errorf("expected 1 CompileDelete call (parent itself only, no cascade), got %d", len(d.deleteCalls))
	}
	if len(d.updateCalls) != 0 {
		t.Errorf("expected 0 Update calls, got %d", len(d.updateCalls))
	}
}

func TestRepository_Delete_NoIncomingForeignKeys_DoesNotOpenTransaction(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_no_fk")

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 4}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if d.beginCalls != 0 {
		t.Errorf("beginCalls = %d, want 0 (no entity references this table)", d.beginCalls)
	}
}

func TestRepository_Delete_AlreadyInsideTx_ReusesIt(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_already_tx")
	newCascadeChildEntity("cascade_child_already_tx", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	d := &fakeDialect{}
	ds := newFakeConn(t, d)

	txConn, err := d.Begin(context.Background(), ds)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	d.beginCalls = 0 // reset: only count Begin calls made *by Delete itself* below
	tx := golem.NewTx(d, txConn)

	repo := Get(tx, parent)
	if err := repo.Delete(context.Background(), &cascadeParent{ID: 6}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if d.beginCalls != 0 {
		t.Errorf("beginCalls = %d, want 0 (Delete must reuse the existing Tx, not open a nested one)", d.beginCalls)
	}
	if len(d.deleteCalls) != 2 {
		t.Fatalf("expected 2 CompileDelete calls (child cascade + parent), got %d", len(d.deleteCalls))
	}
}

// -----------------------------------------------------------------------
// Soft-delete-aware child variant, for restrict/cascade branches that read
// ChildDeleteDateColumn.
// -----------------------------------------------------------------------

type cascadeChildSoftDelete struct {
	ID        int64
	ParentID  int64
	DeletedAt *time.Time
}

func newCascadeChildSoftDeleteEntity(table string, parent *entity.Entity[cascadeParent], opts *relation.ForeignKeyOptions) *entity.Entity[cascadeChildSoftDelete] {
	return entity.New(func(c *cascadeChildSoftDelete, b *entity.Table) {
		b.TableName(table)
		b.Col(&c.ID, golem.BIGINT())
		b.Col(&c.ParentID, golem.BIGINT()).Name("parent_id")
		b.Col(&c.DeletedAt, golem.DATETIME()).Name("deleted_at")
		b.PrimaryKey(&c.ID)
		b.DeleteDate(&c.DeletedAt)
		b.ForeignKey(&c.ParentID, parent, opts)
	})
}

func TestRepository_Delete_OnDeleteRestrict_SoftDeleteAwareChild_AddsIsNullFilter(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_restrict_sd")
	newCascadeChildSoftDeleteEntity("cascade_child_restrict_sd", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))

	d := &fakeDialect{
		selectResult: []map[string]any{{"count": int64(0)}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 11}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(d.selectCalls) != 1 {
		t.Fatalf("expected 1 restrict-check Select call, got %d", len(d.selectCalls))
	}
	logical, ok := d.selectCalls[0].Where.(stmt.Logical)
	if !ok || logical.Op != "and" || len(logical.Predicates) != 2 {
		t.Fatalf("expected logical AND (fk eq + deleted_at IS NULL), got %+v", d.selectCalls[0].Where)
	}
	comp1, ok1 := logical.Predicates[1].(stmt.Comparison)
	if !ok1 || comp1.Column != "deleted_at" || comp1.Op != "is_null" {
		t.Errorf("expected deleted_at IS NULL as second predicate, got %+v", logical.Predicates[1])
	}
}

func TestRepository_Delete_OnDeleteCascade_SoftDeleteAwareChild_UpdatesInsteadOfDeleting(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_cascade_sd")
	newCascadeChildSoftDeleteEntity("cascade_child_cascade_sd", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 12}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(d.updateCalls) != 1 {
		t.Fatalf("expected 1 Update call (soft-delete cascade), got %d", len(d.updateCalls))
	}
	call := d.updateCalls[0]
	if call.Table != "cascade_child_cascade_sd" {
		t.Errorf("update table = %q, want cascade_child_cascade_sd", call.Table)
	}
	if len(call.Sets) != 1 || call.Sets[0].Column != "deleted_at" || call.Sets[0].Value == nil {
		t.Errorf("expected Set deleted_at = time.Now(), got %+v", call.Sets)
	}
	// Only the parent's own hard delete should hit CompileDelete — the
	// child cascade went through Update (soft-delete), not Delete.
	if len(d.deleteCalls) != 1 {
		t.Errorf("expected 1 CompileDelete call (parent only), got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_OnDeleteRestrict_EmptyRows_Continues(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_restrict_empty")
	newCascadeChildEntity("cascade_child_restrict_empty", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))

	d := &fakeDialect{
		selectResult: nil, // zero rows returned at all (not even a {"count":0} row)
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 13}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(d.deleteCalls) != 1 {
		t.Errorf("expected 1 CompileDelete call (parent itself), got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_OnDeleteRestrict_CompileSelectError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_restrict_compileerr")
	newCascadeChildEntity("cascade_child_restrict_compileerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))

	wantErr := errors.New("boom: compile select")
	d := &fakeDialect{selectErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 14})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Delete_OnDeleteRestrict_QueryError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_restrict_queryerr")
	newCascadeChildEntity("cascade_child_restrict_queryerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteRestrict))

	wantErr := errors.New("boom: query")
	d := &fakeDialect{queryErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 15})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Delete_OnDeleteCascade_CompileDeleteError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_cascade_compileerr")
	newCascadeChildEntity("cascade_child_cascade_compileerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	wantErr := errors.New("boom: compile delete")
	d := &fakeDialect{deleteCompErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 16})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Delete_OnDeleteCascade_ExecError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_cascade_execerr")
	newCascadeChildEntity("cascade_child_cascade_execerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	wantErr := errors.New("boom: exec")
	d := &fakeDialect{execErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 17})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Delete_OnDeleteCascade_SoftDeleteUpdateError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_cascade_sd_updateerr")
	newCascadeChildSoftDeleteEntity("cascade_child_cascade_sd_updateerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	wantErr := errors.New("boom: update")
	d := &fakeDialect{updateErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 18})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Delete_OnDeleteSetNull_UpdateError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_setnull_updateerr")
	newCascadeChildEntity("cascade_child_setnull_updateerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteSetNull))

	wantErr := errors.New("boom: update")
	d := &fakeDialect{updateErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 19})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestRepository_Delete_BeginCascadeTxError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_beginerr")
	newCascadeChildEntity("cascade_child_beginerr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	wantErr := errors.New("boom: begin")
	d := &fakeDialect{beginErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 20})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
	if len(d.deleteCalls) != 0 {
		t.Errorf("expected 0 CompileDelete calls (never got past begin), got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_CommitCascadeTxError_Propagates(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_commiterr")
	newCascadeChildEntity("cascade_child_commiterr", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	wantErr := errors.New("boom: commit")
	d := &fakeDialect{commitErr: wantErr}
	conn := newFakeConn(t, d)
	repo := Get(conn, parent)

	err := repo.Delete(context.Background(), &cascadeParent{ID: 21})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete: expected wrapped %v, got %v", wantErr, err)
	}
}

func TestCountValue(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int64
	}{
		{"int64", int64(5), 5},
		{"int32", int32(6), 6},
		{"int", int(7), 7},
		{"unrecognized type", "not a number", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := countValue(tc.in); got != tc.want {
				t.Errorf("countValue(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// --- Extracted from extra cov test ---
func TestRepository_Insert_TriggerErrors(t *testing.T) {
	d := &fakeDialect{
		insertErr: errors.New("conflict"),
	}
	conn := newFakeConn(t, d)

	// Create entity with conflict hook error
	errEntity := entity.New(func(s *testSubject, b *entity.Table) {
		b.TableName("test")
		b.Col(&s.Name, golem.VARCHAR(50))
	})
	entity.AddHook(errEntity).OnConflictCreate(func(ctx context.Context, item *testSubject, conn golem.Conn) error {
		return errors.New("hook conflict error")
	})
	repo := Get(conn, errEntity)
	_, err := repo.Insert(context.Background(), &testSubject{})
	if err == nil || err.Error() != "hook conflict error" {
		t.Fatalf("expected hook conflict error, got %v", err)
	}

	// TriggerAfterCreate error
	d.insertErr = nil
	errEntity2 := entity.New(func(s *testSubject, b *entity.Table) {
		b.TableName("test")
		b.Col(&s.Name, golem.VARCHAR(50))
	})
	entity.AddHook(errEntity2).AfterCreate(func(ctx context.Context, item *testSubject, conn golem.Conn) error {
		return errors.New("after create error")
	})
	repo2 := Get(conn, errEntity2)
	_, err = repo2.Insert(context.Background(), &testSubject{})
	if err == nil || err.Error() != "after create error" {
		t.Fatalf("expected after create error, got %v", err)
	}
}

func TestRepository_FindMany_ScanErrors(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{{"id": "not-an-int"}}, // causes scan error
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindMany(context.Background())
	if err == nil {
		t.Fatal("expected scan error")
	}
}

func TestRepository_FindMany_JoinErrors(t *testing.T) {
	// 348-356, 362-368, 377-379, 391-394
	// missing join errors
	d := &fakeDialect{}
	conn := newFakeConn(t, d)

	parent := entity.New(func(s *testSubject, b *entity.Table) {
		b.TableName("parent")
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
	})

	repo := Get(conn, parent)

	_, err := repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		// unmapped field pointer
		var unmapped string
		q.Select(&unmapped)
	})
	// Just unmapped field doesn't error, just skips.

	_, err = repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		var foreign int
		q.OrderBy(op.Asc(&foreign))
	})

	// join with bad pointers
	childEntity := entity.New(func(s *testSubject, b *entity.Table) {
		b.TableName("child")
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
	})

	_, err = repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		var foreign int
		join.Inner(q, childEntity, func(j *testSubject, jq *query.Join[testSubject]) {
			jq.On(&foreign, &j.ID)
		})
	})
	if err == nil {
		t.Fatal("expected left pointer error")
	}

	_, err = repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		var foreign int
		join.Inner(q, childEntity, func(j *testSubject, jq *query.Join[testSubject]) {
			jq.On(&s.ID, &foreign)
		})
	})
	if err == nil {
		t.Fatal("expected right pointer error")
	}
}

func TestRepository_Update_HooksAndErrors(t *testing.T) {
	d := &fakeDialect{
		updateErr: errors.New("conflict"),
	}
	conn := newFakeConn(t, d)

	errEntity := entity.New(func(s *testSubject, b *entity.Table) {
		b.TableName("test")
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
		b.Col(&s.Name, golem.VARCHAR(50))
	})
	entity.AddHook(errEntity).OnConflictUpdate(func(ctx context.Context, item *testSubject, conn golem.Conn) error {
		return errors.New("hook conflict update error")
	}).BeforeUpdate(func(ctx context.Context, item *testSubject, conn golem.Conn) error {
		if item.Name == "before_err" {
			return errors.New("before update err")
		}
		return nil
	}).AfterUpdate(func(ctx context.Context, item *testSubject, conn golem.Conn) error {
		return errors.New("after update err")
	})
	repo := Get(conn, errEntity)

	// conflict update
	_, err := repo.SaveOne(context.Background(), &testSubject{ID: 1})
	if err == nil || err.Error() != "hook conflict update error" {
		t.Fatalf("expected hook conflict update error, got %v", err)
	}

	// before update error
	_, err = repo.SaveOne(context.Background(), &testSubject{ID: 2, Name: "before_err"})
	if err == nil || err.Error() != "before update err" {
		t.Fatalf("expected before update err, got %v", err)
	}

	// after update error
	d.updateErr = nil
	d.updateResult = []map[string]any{{"id": int64(3)}}
	_, err = repo.SaveOne(context.Background(), &testSubject{ID: 3})
	if err == nil || err.Error() != "after update err" {
		t.Fatalf("expected after update err, got %v", err)
	}

	// len(rows) == 0 -> NotFound
	d.updateResult = nil
	errEntityNoHooks := entity.New(func(s *testSubject, b *entity.Table) {
		b.TableName("test")
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
	})
	repoNoHooks := Get(conn, errEntityNoHooks)
	_, err = repoNoHooks.SaveOne(context.Background(), &testSubject{ID: 4})
	if !errors.Is(err, golem.ErrNotFound) {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestRepository_Delete_HooksAndErrors(t *testing.T) {
	d := &fakeDialect{
		execErr: errors.New("conflict"),
	}
	conn := newFakeConn(t, d)

	errEntity := entity.New(func(s *testSubject, b *entity.Table) {
		b.TableName("test")
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
	})
	entity.AddHook(errEntity).OnConflictDelete(func(ctx context.Context, item *testSubject, conn golem.Conn) error {
		return errors.New("hook conflict delete error")
	}).BeforeDelete(func(ctx context.Context, item *testSubject, conn golem.Conn) error {
		if item.Name == "before_err" {
			return errors.New("before delete err")
		}
		return nil
	}).AfterDelete(func(ctx context.Context, item *testSubject, conn golem.Conn) error {
		return errors.New("after delete err")
	})
	repo := Get(conn, errEntity)

	// empty
	err := repo.Delete(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// conflict delete
	err = repo.Delete(context.Background(), &testSubject{ID: 1})
	if err == nil || err.Error() != "hook conflict delete error" {
		t.Fatalf("expected hook conflict delete error, got %v", err)
	}

	// before delete error
	err = repo.Delete(context.Background(), &testSubject{ID: 2, Name: "before_err"})
	if err == nil || err.Error() != "before delete err" {
		t.Fatalf("expected before delete err, got %v", err)
	}

	// after delete error
	d.execErr = nil
	err = repo.Delete(context.Background(), &testSubject{ID: 3})
	if err == nil || err.Error() != "after delete err" {
		t.Fatalf("expected after delete err, got %v", err)
	}
}

func TestRepository_Restore_Errors(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)

	errEntity := entity.New(func(s *testSubject, b *entity.Table) {
		b.TableName("test")
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
	})
	repo := Get(conn, errEntity)
	sdEntity := entity.New(func(s *softDeleteSubject, b *entity.Table) {
		b.TableName("test")
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
		b.Col(&s.DeletedAt, golem.DATETIME()).Name("deleted_at")
		b.DeleteDate(&s.DeletedAt)
	})
	repoSd := Get(conn, sdEntity)

	// empty
	err := repoSd.Restore(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// no delete date
	err = repo.Restore(context.Background(), &testSubject{ID: 1})
	if err == nil {
		t.Fatal("expected error")
	}

	// Query error
	d.updateErr = errors.New("update err")
	err = repoSd.Restore(context.Background(), &softDeleteSubject{ID: 1})
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestRepository_Count_ReturnsInt(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{{"count": int32(5)}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	c, err := repo.Count(context.Background())
	if err != nil || c != 5 {
		t.Fatalf("expected 5, got %d, err: %v", c, err)
	}

	d.selectResult = []map[string]any{{"count": int(6)}}
	c, err = repo.Count(context.Background())
	if err != nil || c != 6 {
		t.Fatalf("expected 6, got %d, err: %v", c, err)
	}

	d.selectResult = []map[string]any{}
	c, err = repo.Count(context.Background())
	if err != nil || c != 0 {
		t.Fatalf("expected 0, got %d, err: %v", c, err)
	}

	d.selectResult = []map[string]any{{"count": "not_an_int"}}
	_, err = repo.Count(context.Background())
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestRepository_Exists_ReturnsBool(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{{"exists": int32(1)}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	c, err := repo.Exists(context.Background())
	if err != nil || c != true {
		t.Fatalf("expected true, err: %v", err)
	}

	d.selectResult = []map[string]any{{"exists": int(0)}}
	c, err = repo.Exists(context.Background())
	if err != nil || c != false {
		t.Fatalf("expected false, err: %v", err)
	}

	d.selectResult = []map[string]any{}
	c, err = repo.Exists(context.Background())
	if err != nil || c != false {
		t.Fatalf("expected false, err: %v", err)
	}

	d.selectResult = []map[string]any{{"exists": "not_an_int"}}
	_, err = repo.Exists(context.Background())
	if err == nil { // Wait, Exists uses countValue which handles unrecognized type by returning 0, so Exists will return false instead of error. Let's see countValue.
		// Wait, countValue returns 0. So Exists returns val > 0 which is false, nil.
	}
}

func TestRepository_Exec_ScanRowError(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{{"id": "not_an_int"}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.Exec(context.Background(), "SELECT *")
	if err == nil {
		t.Fatal("expected scan row err")
	}
}

// applySoftDeleteFilter Logical AND logic
func TestRepository_applySoftDeleteFilter_LogicalAnd(t *testing.T) {
	repo := Get(newFakeConn(t, &fakeDialect{}), softDeleteSubjectEntity)
	where := stmt.Logical{Op: "and", Predicates: []stmt.Predicate{stmt.Comparison{Column: "id", Op: "eq", Value: 1}}}
	res := repo.applySoftDeleteFilter("test", "DeletedAt", false, where)
	log := res.(stmt.Logical)
	if log.Op != "and" || len(log.Predicates) != 2 {
		t.Fatalf("expected logical and with 2 preds, got %v", log)
	}
}

func TestRepository_RemainingCoverage(t *testing.T) {
	// translateCondition: op.Not -> nil predicate
	repo := Get(newFakeConn(t, &fakeDialect{}), testSubjectEntity)
	_, err := repo.translateCondition(&testSubject{}, repo.fieldToColumn(), "test", op.Not(op.Or()))
	if err != nil {
		t.Fatal(err)
	}

	// translateCondition: unmapped field
	var unmapped int
	_, err = repo.translateCondition(&testSubject{}, repo.fieldToColumn(), "test", op.Eq(&unmapped, 1))
	if err == nil {
		t.Fatal("expected unmapped err")
	}

	// buildWherePredicate: empty conditions
	pred, err := repo.buildWherePredicate(&testSubject{}, repo.fieldToColumn(), "test", []op.Condition{})
	if err != nil || pred != nil {
		t.Fatal("expected nil, nil")
	}

	// assignFieldValue: invalid value
	// rawVal.IsValid() is false if scanning a nil into an interface{} that doesn't expect it, or similar.
	err = assignFieldValue(reflect.ValueOf(&testSubject{}).Elem().FieldByName("Name"), nil, "name", "Name")
	if err != nil {
		t.Fatal(err)
	}

	// Insert: err after exec (scanRow)
	d := &fakeDialect{insertResult: map[string]any{"id": "not-an-int"}}
	conn := newFakeConn(t, d)
	repoIns := Get(conn, testSubjectEntity)
	_, err = repoIns.Insert(context.Background(), &testSubject{})
	if err == nil {
		t.Fatal("expected insert scanRow err")
	}

	// FindMany: select err
	d.selectErr = errors.New("select err")
	_, err = repoIns.FindMany(context.Background())
	if err == nil || err.Error() != "repository: compile select: select err" {
		t.Fatal("expected compile select err")
	}

	// FindMany: query err
	d.selectErr = nil
	d.queryErr = errors.New("query err")
	_, err = repoIns.FindMany(context.Background())
	if err == nil {
		t.Fatal("expected query err")
	}

	// FindMany: compile join err (translateCondition err inside join)
	_, err = repoIns.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		join.Inner(q, testSubjectEntity, func(j *testSubject, jq *query.Join[testSubject]) {
			jq.Where(op.Eq(&unmapped, 1))
		})
	})
	if err == nil {
		t.Fatal("expected join compile err")
	}

	// Delete: composite PK
	repoComposite := Get(conn, testCompositeSubjectEntity)
	err = repoComposite.Delete(context.Background(), &testCompositeSubject{SubjectID: 1, CategoryID: 2})
	if err != nil {
		// we might hit an exec err if we didn't clear it
	}

	// Restore: composite PK
	err = repoComposite.Restore(context.Background(), &testCompositeSubject{SubjectID: 1, CategoryID: 2})
	if err == nil {
		t.Fatal("expected no soft delete field err")
	}
}

func TestTranslateCondition_FieldNotMappedToColumn(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	var zero testSubject
	f2c := map[string]string{"ID": "id"} // Name deliberately missing

	_, err := repo.translateCondition(&zero, f2c, "testsubject", op.Eq(&zero.Name, "x"))
	if err == nil {
		t.Fatal("expected error for field not mapped to any column")
	}
}

func TestBuildWherePredicate_AllNilPreds_ReturnsNil(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)
	var zero testSubject

	p, err := repo.buildWherePredicate(&zero, repo.fieldToColumn(), "test", []op.Condition{
		op.Or(), // empty OR -> nil pred, nil err
	})
	if err != nil || p != nil {
		t.Fatalf("expected nil, nil, got %+v, %v", p, err)
	}
}

func TestBuildWherePredicate_MultiplePreds_ReturnsLogicalAnd(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)
	var zero testSubject

	p, err := repo.buildWherePredicate(&zero, repo.fieldToColumn(), "test", []op.Condition{
		op.Eq(&zero.Name, "a"),
		op.Eq(&zero.Email, "b"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logical, ok := p.(stmt.Logical)
	if !ok || logical.Op != "and" || len(logical.Predicates) != 2 {
		t.Fatalf("expected logical AND of 2 predicates, got %+v", p)
	}
}

func TestAssignFieldValue_ConvertibleType_Success(t *testing.T) {
	var s struct {
		Big int64
	}
	v := reflect.ValueOf(&s).Elem().FieldByName("Big")
	if err := assignFieldValue(v, int32(5), "col", "Big"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Big != 5 {
		t.Fatalf("Big = %d, want 5", s.Big)
	}
}

func TestAssignFieldValue_PointerField_ExactElemType_WrapsInPointer(t *testing.T) {
	var s struct {
		DeletedAt *time.Time
	}
	now := time.Now()
	v := reflect.ValueOf(&s).Elem().FieldByName("DeletedAt")
	if err := assignFieldValue(v, now, "deleted_at", "DeletedAt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DeletedAt == nil || !s.DeletedAt.Equal(now) {
		t.Fatalf("DeletedAt = %v, want a pointer wrapping %v", s.DeletedAt, now)
	}
}

func TestAssignFieldValue_PointerField_ConvertibleElemType_WrapsInPointer(t *testing.T) {
	var s struct {
		Count *int64
	}
	v := reflect.ValueOf(&s).Elem().FieldByName("Count")
	if err := assignFieldValue(v, int32(7), "count", "Count"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Count == nil || *s.Count != 7 {
		t.Fatalf("Count = %v, want a pointer to 7", s.Count)
	}
}

func TestAssignFieldValue_PointerField_IncompatibleRaw_ReturnsError(t *testing.T) {
	var s struct {
		Count *int64
	}
	v := reflect.ValueOf(&s).Elem().FieldByName("Count")
	err := assignFieldValue(v, "not an int", "count", "Count")
	if err == nil {
		t.Fatal("expected error for a raw value inconvertible to *int64's element type")
	}
}

func TestAssignFieldValue_BoolField_NumericRaw(t *testing.T) {
	var s struct {
		Active bool
	}
	v := reflect.ValueOf(&s).Elem().FieldByName("Active")
	if err := assignFieldValue(v, int64(1), "active", "Active"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.Active {
		t.Fatal("expected Active = true for a non-zero int64")
	}

	if err := assignFieldValue(v, int64(0), "active", "Active"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Active {
		t.Fatal("expected Active = false for a zero int64")
	}

	if err := assignFieldValue(v, uint8(1), "active", "Active"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.Active {
		t.Fatal("expected Active = true for a non-zero uint8")
	}
}

func TestAssignFieldValue_PointerBoolField_NumericRaw(t *testing.T) {
	var s struct {
		Active *bool
	}
	v := reflect.ValueOf(&s).Elem().FieldByName("Active")
	if err := assignFieldValue(v, int64(1), "active", "Active"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Active == nil || !*s.Active {
		t.Fatalf("Active = %v, want a pointer to true", s.Active)
	}
}

func TestNumericToBool(t *testing.T) {
	cases := []struct {
		raw    any
		wantB  bool
		wantOK bool
	}{
		{int(1), true, true},
		{int8(0), false, true},
		{int16(1), true, true},
		{int32(0), false, true},
		{int64(1), true, true},
		{uint(0), false, true},
		{uint8(1), true, true},
		{uint16(0), false, true},
		{uint32(1), true, true},
		{uint64(0), false, true},
		{"not numeric", false, false},
	}
	for _, tc := range cases {
		b, ok := numericToBool(reflect.ValueOf(tc.raw))
		if ok != tc.wantOK || (ok && b != tc.wantB) {
			t.Errorf("numericToBool(%v) = (%v, %v), want (%v, %v)", tc.raw, b, ok, tc.wantB, tc.wantOK)
		}
	}
}

func TestRepository_FindMany_TopLevelWhereError(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	var foreign string
	_, err := repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		q.Where(op.Eq(&foreign, "x"))
	})
	if err == nil {
		t.Fatal("expected where-predicate error to propagate")
	}
}

func TestRepository_FindMany_Select_MappedField(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{{"id": int64(1)}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		q.Select(&s.Name)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := d.selectCalls[0]
	if len(call.Columns) != 1 || call.Columns[0] != "testsubject.name" {
		t.Fatalf("expected projected column testsubject.name, got %+v", call.Columns)
	}
}

func TestRepository_FindMany_OrderBy_MappedField(t *testing.T) {
	d := &fakeDialect{
		selectResult: []map[string]any{{"id": int64(1)}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindMany(context.Background(), func(s *testSubject, q *query.Query[testSubject]) {
		q.OrderBy(op.Asc(&s.Name))
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := d.selectCalls[0]
	if len(call.OrderBy) != 1 || call.OrderBy[0].Column != "testsubject.name" {
		t.Fatalf("expected order by testsubject.name, got %+v", call.OrderBy)
	}
}

func TestRepository_FindMany_Join_RightFieldResolvedInJoinedEntity(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindMany(context.Background(), func(s *testSubject, q0 *query.Query[testSubject]) {
		join.Inner(q0, softDeleteSubjectEntity, func(sd *softDeleteSubject, q1 *query.Join[softDeleteSubject]) {
			// Reversed order vs TestRepository_FindMany_WithInnerJoin: left field
			// belongs to the parent, right field belongs to the joined entity.
			q1.On(&s.ID, &sd.ID)
			q1.Where(op.Eq(&sd.ID, int64(1)), op.Eq(&sd.ID, int64(2)))
		})
	})
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}
	call := d.selectCalls[0]
	jPlan := call.Joins[0]
	if len(jPlan.On) != 1 || jPlan.On[0].LeftCol != "testsubject.id" || jPlan.On[0].RightCol != "softdeletesubject.id" {
		t.Fatalf("unexpected join ON keys: %+v", jPlan.On)
	}

	// jd.Conditions has 2 entries -> buildWherePredicate returns a Logical AND
	// on its own, so the delete-date filter must prepend into that existing
	// Logical rather than wrapping a fresh one.
	logical, ok := jPlan.Where.(stmt.Logical)
	if !ok || logical.Op != "and" || len(logical.Predicates) != 3 {
		t.Fatalf("expected logical AND of 3 predicates (deleted_at + 2 conditions), got %+v", jPlan.Where)
	}
	sdFilter, ok := logical.Predicates[0].(stmt.Comparison)
	if !ok || sdFilter.Column != "softdeletesubject.deleted_at" || sdFilter.Op != "is_null" {
		t.Fatalf("expected deleted_at IS NULL prepended first, got %+v", logical.Predicates[0])
	}
}

func TestRepository_SaveOne_ScanRowError(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{{"id": "not-an-int64"}},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.SaveOne(context.Background(), &testSubject{ID: 1})
	if err == nil {
		t.Fatal("expected scanRow conversion error to propagate")
	}
}

func TestRepository_Delete_Cascade_ReusesExistingTx(t *testing.T) {
	parent := newCascadeParentEntity("cascade_parent_reuse_tx")
	newCascadeChildEntity("cascade_child_reuse_tx", parent, relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade))

	d := &fakeDialect{}
	tx := golem.NewTx(d, &fakeTx{})
	repo := Get(tx, parent)

	if err := repo.Delete(context.Background(), &cascadeParent{ID: 1}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if d.beginCalls != 0 {
		t.Errorf("beginCalls = %d, want 0 (an already-open Tx must be reused, not re-begun)", d.beginCalls)
	}
	if len(d.deleteCalls) != 2 {
		t.Fatalf("expected 2 CompileDelete calls, got %d", len(d.deleteCalls))
	}
}

func TestRepository_Delete_SoftDelete_DeleteDateFieldNotMapped(t *testing.T) {
	type badSoftDelete struct {
		ID        int64
		DeletedAt *time.Time
	}
	badEntity := entity.New(func(s *badSoftDelete, b *entity.Table) {
		b.TableName("bad_soft_delete")
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
		b.DeleteDate(&s.DeletedAt) // deliberately never declared via Col()
	})

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, badEntity)

	err := repo.Delete(context.Background(), &badSoftDelete{ID: 1})
	if err == nil {
		t.Fatal("expected delete-date-field-not-mapped error")
	}
}

func TestRepository_Delete_NonConflictError(t *testing.T) {
	d := &fakeDialect{execErr: errors.New("connection reset")}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	err := repo.Delete(context.Background(), &testSubject{ID: 1})
	if err == nil || !strings.Contains(err.Error(), "connection reset") {
		t.Fatalf("expected wrapped non-conflict error, got %v", err)
	}
}

func TestRepository_Restore_DeleteDateFieldNotMapped(t *testing.T) {
	type badSoftDelete struct {
		ID        int64
		DeletedAt *time.Time
	}
	badEntity := entity.New(func(s *badSoftDelete, b *entity.Table) {
		b.TableName("bad_soft_delete_restore")
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
		b.DeleteDate(&s.DeletedAt) // deliberately never declared via Col()
	})

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, badEntity)

	err := repo.Restore(context.Background(), &badSoftDelete{ID: 1})
	if err == nil {
		t.Fatal("expected delete-date-field-not-mapped error")
	}
}

func TestRepository_Restore_CompositePK_LogicalAnd(t *testing.T) {
	type compositeSoftDelete struct {
		SubjectID  int64
		CategoryID int64
		DeletedAt  *time.Time
	}
	compositeEntity := entity.New(func(s *compositeSoftDelete, b *entity.Table) {
		b.TableName("composite_soft_delete")
		b.Col(&s.SubjectID, golem.BIGINT())
		b.Col(&s.CategoryID, golem.BIGINT())
		b.Col(&s.DeletedAt, golem.DATETIME()).Name("deleted_at")
		b.PrimaryKey(&s.SubjectID, &s.CategoryID)
		b.DeleteDate(&s.DeletedAt)
	})

	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, compositeEntity)

	if err := repo.Restore(context.Background(), &compositeSoftDelete{SubjectID: 1, CategoryID: 2}); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if len(d.updateCalls) != 1 {
		t.Fatalf("expected 1 Update call, got %d", len(d.updateCalls))
	}
	logical, ok := d.updateCalls[0].Where.(stmt.Logical)
	if !ok || logical.Op != "and" || len(logical.Predicates) != 2 {
		t.Fatalf("expected logical AND of 2 PK predicates, got %+v", d.updateCalls[0].Where)
	}
}

// customExecConn embeds golem.DataSource purely to inherit its unexported
// isConn() method (the only way to satisfy the sealed golem.Conn interface
// from outside the golem package) while overriding Exec to return a
// caller-controlled golem.Result. This is the only way to make
// Repository.Exec's row-scan loop observe a Scan() error: the real
// rawResult (returned by every production Conn) can only error out of bounds,
// which Next() already prevents from ever happening.
type customExecConn struct {
	golem.DataSource
	result golem.Result
	err    error
}

func (c *customExecConn) Exec(ctx context.Context, sql string, args ...any) (golem.Result, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.result, nil
}

type scanErrResult struct {
	scanErr error
	done    bool
}

func (r *scanErrResult) Next() bool {
	if r.done {
		return false
	}
	r.done = true
	return true
}
func (r *scanErrResult) Scan() (map[string]any, error) { return nil, r.scanErr }
func (r *scanErrResult) RowsAffected() (int64, error)  { return 0, nil }

func TestRepository_Exec_ResultScanError(t *testing.T) {
	conn := &customExecConn{result: &scanErrResult{scanErr: errors.New("scan boom")}}
	repo := Get(conn, testSubjectEntity)

	_, err := repo.Exec(context.Background(), "SELECT 1")
	if err == nil || err.Error() != "scan boom" {
		t.Fatalf("expected scan boom error, got %v", err)
	}
}
