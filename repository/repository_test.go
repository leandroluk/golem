package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/entity"
)

// -----------------------------------------------------------------------
// Test-only entity: a minimal single-PK subject with a DB-generated ID.
// -----------------------------------------------------------------------

type testSubject struct {
	ID    int64
	Name  string
	Email string
}

var testSubjectEntity = entity.New(func(s *testSubject, b *entity.Builder) {
	b.TableName("testsubject")
	b.Col(&s.ID, golem.BIGINT())
	b.Col(&s.Name, golem.VARCHAR(50))
	b.Col(&s.Email, golem.TEXT())
	b.PrimaryKey(&s.ID)
})

// -----------------------------------------------------------------------
// Composite-PK test entity (2 PK columns), to exercise the FindByID guard.
// -----------------------------------------------------------------------

type testCompositeSubject struct {
	SubjectID  int64
	CategoryID int64
}

var testCompositeSubjectEntity = entity.New(func(s *testCompositeSubject, b *entity.Builder) {
	b.TableName("testcompositesubject")
	b.Col(&s.SubjectID, golem.BIGINT())
	b.Col(&s.CategoryID, golem.BIGINT())
	b.PrimaryKey(&s.SubjectID, &s.CategoryID)
})

// -----------------------------------------------------------------------
// Fakes: golem.Connector + golem.Dialect are fully exported interfaces, so
// they can be faked from outside package golem. golem.Conn cannot be
// implemented directly here (isConn is unexported) — instead we obtain a
// real golem.Conn by wiring our fake Connector/Dialect through a real
// golem.DataSource, which already implements Conn.
// -----------------------------------------------------------------------

type insertCall struct {
	table   string
	columns []string
	values  []driver.Value
}

type findByIDCall struct {
	table    string
	pkColumn string
	id       driver.Value
}

type fakeDialect struct {
	insertCalls   []insertCall
	insertResult  map[string]any
	insertErr     error
	findByIDCalls []findByIDCall
	findByIDRow   map[string]any
	findByIDFound bool
	findByIDErr   error
}

func (d *fakeDialect) Bind(t golem.ColumnType, value any) (driver.Value, error) {
	return value, nil
}

func (d *fakeDialect) Scan(t golem.ColumnType, raw any, dest any) error {
	return nil
}

func (d *fakeDialect) Insert(ctx context.Context, conn golem.Conn, table string, columns []string, values []driver.Value) (map[string]any, error) {
	// copy columns/values defensively so later mutation by the caller (there
	// is none today, but tests should not rely on that) can't retroactively
	// change what was recorded.
	colsCopy := append([]string(nil), columns...)
	valsCopy := append([]driver.Value(nil), values...)
	d.insertCalls = append(d.insertCalls, insertCall{table: table, columns: colsCopy, values: valsCopy})
	if d.insertErr != nil {
		return nil, d.insertErr
	}
	return d.insertResult, nil
}

func (d *fakeDialect) FindByID(ctx context.Context, conn golem.Conn, table string, pkColumn string, id driver.Value) (map[string]any, bool, error) {
	d.findByIDCalls = append(d.findByIDCalls, findByIDCall{table: table, pkColumn: pkColumn, id: id})
	if d.findByIDErr != nil {
		return nil, false, d.findByIDErr
	}
	return d.findByIDRow, d.findByIDFound, nil
}

var _ golem.Dialect = (*fakeDialect)(nil)

// anyDialectConnector wires any golem.Dialect into a golem.DataSource via
// golem.NewDataSource(golem.WithConnector(...)).
type anyDialectConnector struct {
	dialect golem.Dialect
}

func (c *anyDialectConnector) Connect() (golem.Dialect, error) { return c.dialect, nil }
func (c *anyDialectConnector) Close() error                    { return nil }

var _ golem.Connector = (*anyDialectConnector)(nil)

// newConnWithDialect builds a real golem.Conn (a connected *golem.DataSource)
// backed by the given golem.Dialect. This is how a golem.Conn is faked from
// outside package golem: Conn's isConn() method is unexported, so it cannot
// be implemented directly here — but Connector and Dialect are fully
// exported, and a connected *golem.DataSource already implements Conn.
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

// newFakeConn is a convenience wrapper for the common case of a *fakeDialect.
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

	in := &testSubject{Name: "Ada", Email: "ada@example.com"} // ID unset — DB-generated
	got, err := repo.Insert(context.Background(), in)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if len(d.insertCalls) != 1 {
		t.Fatalf("expected 1 Insert call, got %d", len(d.insertCalls))
	}
	call := d.insertCalls[0]
	if call.table != "testsubject" {
		t.Errorf("table = %q, want %q", call.table, "testsubject")
	}
	wantCols := []string{"id", "name", "email"}
	if len(call.columns) != len(wantCols) {
		t.Fatalf("columns = %v, want %v", call.columns, wantCols)
	}
	for i, c := range wantCols {
		if call.columns[i] != c {
			t.Errorf("columns[%d] = %q, want %q", i, call.columns[i], c)
		}
	}
	// values[1] (name) and values[2] (email) should reflect the input struct;
	// values[0] (id) should be the input's zero value (0) since it was unset.
	if call.values[0] != int64(0) {
		t.Errorf("values[0] (id) = %v, want 0", call.values[0])
	}
	if call.values[1] != "Ada" {
		t.Errorf("values[1] (name) = %v, want Ada", call.values[1])
	}
	if call.values[2] != "ada@example.com" {
		t.Errorf("values[2] (email) = %v, want ada@example.com", call.values[2])
	}

	// scanned back from the fake's returned row, including the DB-generated
	// PK that wasn't in the original input.
	if got.ID != 42 {
		t.Errorf("got.ID = %d, want 42", got.ID)
	}
	if got.Name != "Ada" {
		t.Errorf("got.Name = %q, want Ada", got.Name)
	}
	if got.Email != "ada@example.com" {
		t.Errorf("got.Email = %q, want ada@example.com", got.Email)
	}
}

func TestRepository_InsertMany_PreservesOrderAndCallsTwice(t *testing.T) {
	// Use a stateful wrapper dialect so each successive Insert call returns a
	// distinct row (first -> id 1, second -> id 2).
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
	if results[0].ID != 1 || results[0].Name != "Ada" {
		t.Errorf("results[0] = %+v, want ID=1 Name=Ada", results[0])
	}
	if results[1].ID != 2 || results[1].Name != "Bob" {
		t.Errorf("results[1] = %+v, want ID=2 Name=Bob", results[1])
	}
	if wrapped.calls != 2 {
		t.Errorf("expected fake Insert called 2 times, got %d", wrapped.calls)
	}
}

// sequencedInsertDialect wraps a *fakeDialect but returns rows[calls-1] on
// each successive Insert call, instead of a single fixed insertResult.
type sequencedInsertDialect struct {
	*fakeDialect
	rows  []map[string]any
	calls int
}

func (s *sequencedInsertDialect) Insert(ctx context.Context, conn golem.Conn, table string, columns []string, values []driver.Value) (map[string]any, error) {
	row := s.rows[s.calls]
	s.calls++
	// still record on the embedded fake for table/columns/values assertions
	// if a test wants them (not used by InsertMany's test, but kept for
	// consistency / future reuse).
	colsCopy := append([]string(nil), columns...)
	valsCopy := append([]driver.Value(nil), values...)
	s.fakeDialect.insertCalls = append(s.fakeDialect.insertCalls, insertCall{table: table, columns: colsCopy, values: valsCopy})
	return row, nil
}

var _ golem.Dialect = (*sequencedInsertDialect)(nil)

// -----------------------------------------------------------------------
// FindByID
// -----------------------------------------------------------------------

func TestRepository_FindByID_Found_ScansRowCorrectly(t *testing.T) {
	d := &fakeDialect{
		findByIDFound: true,
		findByIDRow: map[string]any{
			"id":    int64(7),
			"name":  "Grace",
			"email": "grace@example.com",
		},
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	got, err := repo.FindByID(context.Background(), int64(7))
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if len(d.findByIDCalls) != 1 {
		t.Fatalf("expected 1 FindByID call, got %d", len(d.findByIDCalls))
	}
	call := d.findByIDCalls[0]
	if call.table != "testsubject" {
		t.Errorf("table = %q, want %q", call.table, "testsubject")
	}
	if call.pkColumn != "id" {
		t.Errorf("pkColumn = %q, want %q", call.pkColumn, "id")
	}
	if call.id != int64(7) {
		t.Errorf("id = %v, want 7", call.id)
	}

	if got.ID != 7 || got.Name != "Grace" || got.Email != "grace@example.com" {
		t.Errorf("got = %+v, want {7 Grace grace@example.com}", got)
	}
}

func TestRepository_FindByID_NotFound_ReturnsErrNotFound(t *testing.T) {
	d := &fakeDialect{
		findByIDFound: false,
	}
	conn := newFakeConn(t, d)
	repo := Get(conn, testSubjectEntity)

	_, err := repo.FindByID(context.Background(), int64(999))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, golem.ErrNotFound) {
		t.Errorf("expected errors.Is(err, golem.ErrNotFound), got %v", err)
	}
}

func TestRepository_FindByID_CompositePK_ReturnsNonNotFoundErrorWithoutCallingDialect(t *testing.T) {
	d := &fakeDialect{}
	conn := newFakeConn(t, d)
	repo := Get(conn, testCompositeSubjectEntity)

	_, err := repo.FindByID(context.Background(), int64(1))
	if err == nil {
		t.Fatal("expected an error for composite PK, got nil")
	}
	if errors.Is(err, golem.ErrNotFound) {
		t.Errorf("expected a plain descriptive error, not golem.ErrNotFound; got %v", err)
	}
	if len(d.findByIDCalls) != 0 {
		t.Errorf("expected fake Dialect.FindByID to never be called, got %d calls", len(d.findByIDCalls))
	}
}
