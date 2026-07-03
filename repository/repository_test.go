package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
	"github.com/leandroluk/golem/internal/stmt"
	"github.com/leandroluk/golem/join"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
)

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

	deleteCalls []*stmt.Delete
	execCalls   []execCall
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
	return "mock-sql-delete", nil, nil
}

func (d *fakeDialect) Query(ctx context.Context, conn golem.Conn, sql string, args []any) ([]map[string]any, error) {
	return d.selectResult, nil
}

func (d *fakeDialect) Exec(ctx context.Context, conn golem.Conn, sql string, args []any) (int64, error) {
	d.execCalls = append(d.execCalls, execCall{sql: sql, args: args})
	return 0, nil
}

func (d *fakeDialect) IsConflict(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "conflict")
}

type fakeTx struct {
	committed  bool
	rolledBack bool
}

func (t *fakeTx) Commit(ctx context.Context) error {
	t.committed = true
	return nil
}

func (t *fakeTx) Rollback(ctx context.Context) error {
	t.rolledBack = true
	return nil
}

func (d *fakeDialect) Begin(ctx context.Context, conn golem.Conn) (golem.TxConn, error) {
	return &fakeTx{}, nil
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
	ds, err := golem.NewDataSource(golem.WithConnector(&anyDialectConnector{dialect: d}))
	if err != nil {
		t.Fatalf("golem.NewDataSource: %v", err)
	}
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
// UpdateOne / UpdateMany
// -----------------------------------------------------------------------

func TestRepository_UpdateOne_Found_PassesWhereAndSetColumns(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(7), "name": "Grace", "email": "new@example.com"},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	got, err := repo.UpdateOne(context.Background(), func(t *testSubject, u *query.Update[testSubject]) {
		u.Where(op.Eq(&t.ID, int64(7)))
		u.Set(&t.Email, "new@example.com")
	})
	if err != nil {
		t.Fatalf("UpdateOne: %v", err)
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
	if got.Email != "new@example.com" {
		t.Errorf("got.Email = %q", got.Email)
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

func TestRepository_UpdateOne_AppendsSoftDeleteFilterByDefault(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(7)},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, softDeleteSubjectEntity)

	_, err := repo.UpdateOne(context.Background(), func(t *softDeleteSubject, u *query.Update[softDeleteSubject]) {
		u.Where(op.Eq(&t.ID, int64(7)))
		u.Set(&t.ID, 7)
	})
	if err != nil {
		t.Fatalf("UpdateOne: %v", err)
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

func TestRepository_UpdateOne_WithDeleted_SkipsSoftDeleteFilter(t *testing.T) {
	d := &fakeDialect{
		updateResult: []map[string]any{
			{"id": int64(7)},
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, softDeleteSubjectEntity)

	_, err := repo.UpdateOne(context.Background(), func(t *softDeleteSubject, u *query.Update[softDeleteSubject]) {
		u.Where(op.Eq(&t.ID, int64(7)))
		u.Set(&t.ID, 7)
		u.WithDeleted()
	})
	if err != nil {
		t.Fatalf("UpdateOne: %v", err)
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
	ds, err := golem.NewDataSource(golem.WithConnector(connector))
	if err != nil {
		t.Fatalf("NewDataSource: %v", err)
	}
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
	ds, err := golem.NewDataSource(golem.WithConnector(connector))
	if err != nil {
		t.Fatalf("NewDataSource: %v", err)
	}
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
	ds, err := golem.NewDataSource(golem.WithConnector(connector))
	if err != nil {
		t.Fatalf("NewDataSource: %v", err)
	}
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






