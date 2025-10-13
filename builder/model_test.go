package builder_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/leandroluk/golem/builder"
	"github.com/leandroluk/golem/core"
	"github.com/stretchr/testify/assert"
)

//
// ---- Fakes ----
//

//region fakeRows

type fakeRows struct {
	cols   []string
	values [][]any
	idx    int
	err    error
}

func (r *fakeRows) Next() bool {
	return r.idx < len(r.values)
}
func (r *fakeRows) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	row := r.values[r.idx]
	for i := range dest {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(row[i]))
	}
	r.idx++
	return nil
}
func (r *fakeRows) Close() error {
	return nil
}
func (r *fakeRows) Columns() []string {
	return r.cols
}

//endregion

//region fakeResult

type fakeResult struct{}

func (fakeResult) RowsAffected() (int64, error) {
	return 1, nil
}

//endregion

//region fakeDriver

type fakeDriver struct {
	queryFn func(string, ...any) (core.Rows, error)
	execFn  func(string, ...any) (core.Result, error)
}

func (d *fakeDriver) Connect(ctx context.Context) error {
	return nil
}
func (d *fakeDriver) Close() error {
	return nil
}
func (d *fakeDriver) Ping(ctx context.Context) error {
	return nil
}
func (d *fakeDriver) Dialect() core.Dialect {
	return fakeDialect{}
}
func (d *fakeDriver) Exec(ctx context.Context, stmt string, args ...any) (core.Result, error) {
	if d.execFn != nil {
		return d.execFn(stmt, args...)
	}
	return fakeResult{}, nil
}
func (d *fakeDriver) Query(ctx context.Context, stmt string, args ...any) (core.Rows, error) {
	if d.queryFn != nil {
		return d.queryFn(stmt, args...)
	}
	return &fakeRows{}, nil
}
func (d *fakeDriver) Begin(ctx context.Context) (core.Tx[string], error) {
	return nil, nil
}

//endregion

//region fakeDialect

type fakeDialect struct{}

func (fakeDialect) ToValue(f *core.FieldMeta, v any) any {
	return v
}
func (fakeDialect) FromValue(f *core.FieldMeta, v any) (any, error) {
	return v, nil
}

//endregion

//
// ---- Tests ----
//

func TestReturningSQL_InsertModes(t *testing.T) {
	type User struct {
		ID int `db:"id"`
	}
	s := core.NewSchema(func(u *User, s *core.Schema) *core.Schema {
		// PK com coluna "id"
		return s.Field(&u.ID, core.Primary())
	})

	// ReturningNone → não deve ter "RETURNING"
	var capturedNone string
	cliNone := core.NewClient(&fakeDriver{
		execFn: func(stmt string, _ ...any) (core.Result, error) {
			capturedNone = stmt
			return fakeResult{}, nil
		},
	})
	mNone := builder.NewModel[User](s, cliNone) // default é ReturningNone
	_ = mNone.Insert(context.Background(), &User{})
	assert.NotContains(t, strings.ToUpper(capturedNone), "RETURNING")

	// ReturningPK → deve ter `RETURNING "id"`
	var capturedPK string
	cliPK := core.NewClient(&fakeDriver{
		queryFn: func(stmt string, _ ...any) (core.Rows, error) {
			capturedPK = stmt
			// devolve uma linha com id só pra não quebrar o fluxo
			return &fakeRows{cols: []string{"id"}, values: [][]any{{123}}}, nil
		},
	})
	mPK := builder.NewModel[User](s, cliPK).WithReturning(builder.ReturningPK)
	_ = mPK.Insert(context.Background(), &User{})
	assert.Contains(t, capturedPK, `RETURNING "id"`)

	// ReturningAll → deve ter `RETURNING *`
	var capturedAll string
	cliAll := core.NewClient(&fakeDriver{
		queryFn: func(stmt string, _ ...any) (core.Rows, error) {
			capturedAll = stmt
			return &fakeRows{cols: []string{"id"}, values: [][]any{{1}}}, nil
		},
	})
	mAll := builder.NewModel[User](s, cliAll).WithReturning(builder.ReturningAll)
	_ = mAll.Insert(context.Background(), &User{})
	assert.Contains(t, strings.ToUpper(capturedAll), "RETURNING *")
}

func TestSaveInsertAndUpdate(t *testing.T) {
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	s := core.NewSchema(func(u *User, s *core.Schema) *core.Schema {
		return s.Field(&u.ID, core.Primary()).Field(&u.Name)
	})

	cli := core.NewClient(&fakeDriver{
		execFn: func(_ string, _ ...any) (core.Result, error) {
			return fakeResult{}, nil
		},
	})
	m := builder.NewModel[User](s, cli)

	ctx := context.Background()

	// Insert (PK vazio)
	u1 := &User{Name: "a"}
	assert.NoError(t, m.Save(ctx, u1))

	// Update (PK setado)
	u2 := &User{ID: 10, Name: "b"}
	assert.NoError(t, m.Save(ctx, u2))
}

func TestInsertReturningModes_DataPaths(t *testing.T) {
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	s := core.NewSchema(func(u *User, s *core.Schema) *core.Schema {
		return s.Field(&u.ID, core.Primary()).Field(&u.Name)
	})

	// ReturningNone
	cli1 := core.NewClient(&fakeDriver{})
	m1 := builder.NewModel[User](s, cli1)
	assert.NoError(t, m1.Insert(context.Background(), &User{}))

	// ReturningPK (preenche ID)
	cli2 := core.NewClient(&fakeDriver{
		queryFn: func(_ string, _ ...any) (core.Rows, error) {
			return &fakeRows{cols: []string{"id"}, values: [][]any{{123}}}, nil
		},
	})
	m2 := builder.NewModel[User](s, cli2).WithReturning(builder.ReturningPK)
	u := &User{Name: "x"}
	assert.NoError(t, m2.Insert(context.Background(), u))
	assert.Equal(t, 123, u.ID)

	// ReturningAll (preenche todos os campos)
	cli3 := core.NewClient(&fakeDriver{
		queryFn: func(_ string, _ ...any) (core.Rows, error) {
			return &fakeRows{cols: []string{"id", "name"}, values: [][]any{{42, "y"}}}, nil
		},
	})
	m3 := builder.NewModel[User](s, cli3).WithReturning(builder.ReturningAll)
	u2 := &User{}
	assert.NoError(t, m3.Insert(context.Background(), u2))
	assert.Equal(t, 42, u2.ID)
	assert.Equal(t, "y", u2.Name)
}

func TestUpdateBranches(t *testing.T) {
	type User struct {
		ID        int       `db:"id"`
		Name      string    `db:"name"`
		UpdatedAt time.Time `db:"updated_at"` // coluna em snake_case para cobrir branch
	}
	s := core.NewSchema(func(u *User, s *core.Schema) *core.Schema {
		return s.Field(&u.ID, core.Primary()).
			Field(&u.Name).
			Field(&u.UpdatedAt, core.UpdatedAt())
	})

	// no SET → erro "update sem SET"
	cli := core.NewClient(&fakeDriver{})
	m := builder.NewModel[User](s, cli)
	_, err := m.Update(context.Background(), func(*User, *core.Update, *core.Query) {})
	assert.EqualError(t, err, "update sem SET")

	// ReturningAll
	cliAll := core.NewClient(&fakeDriver{
		queryFn: func(_ string, _ ...any) (core.Rows, error) {
			return &fakeRows{
				cols:   []string{"id", "name", "updated_at"},
				values: [][]any{{1, "a", time.Now()}},
			}, nil
		},
	})
	mAll := builder.NewModel[User](s, cliAll).WithReturning(builder.ReturningAll)
	res, err := mAll.Update(context.Background(), func(u *User, upd *core.Update, q *core.Query) {
		upd.Set(&u.Name, "changed")
	})
	assert.NoError(t, err)
	assert.Len(t, res, 1)

	// ReturningPK
	cliPK := core.NewClient(&fakeDriver{
		queryFn: func(_ string, _ ...any) (core.Rows, error) {
			return &fakeRows{cols: []string{"id"}, values: [][]any{{99}}}, nil
		},
	})
	mPK := builder.NewModel[User](s, cliPK).WithReturning(builder.ReturningPK)
	res2, err := mPK.Update(context.Background(), func(u *User, upd *core.Update, q *core.Query) {
		upd.Set(&u.Name, "changed")
	})
	assert.NoError(t, err)
	assert.Equal(t, 99, res2[0].ID)

	// Default (sem returning)
	cliDef := core.NewClient(&fakeDriver{
		execFn: func(_ string, _ ...any) (core.Result, error) {
			return fakeResult{}, nil
		},
	})
	mDef := builder.NewModel[User](s, cliDef)
	_, err = mDef.Update(context.Background(), func(u *User, upd *core.Update, q *core.Query) {
		upd.Set(&u.Name, "changed")
	})
	assert.NoError(t, err)
}

func TestDeleteBranches(t *testing.T) {
	type User struct{ ID int }
	s := core.NewSchema(func(u *User, s *core.Schema) *core.Schema {
		return s.Field(&u.ID, core.Primary())
	})

	// ReturningAll
	cliAll := core.NewClient(&fakeDriver{
		queryFn: func(_ string, _ ...any) (core.Rows, error) {
			return &fakeRows{cols: []string{"id"}, values: [][]any{{1}}}, nil
		},
	})
	mAll := builder.NewModel[User](s, cliAll).WithReturning(builder.ReturningAll)
	res, err := mAll.Delete(context.Background(), func(*User, *core.Query) {})
	assert.NoError(t, err)
	assert.Equal(t, 1, res[0].ID)

	// ReturningPK
	cliPK := core.NewClient(&fakeDriver{
		queryFn: func(_ string, _ ...any) (core.Rows, error) {
			return &fakeRows{cols: []string{"id"}, values: [][]any{{2}}}, nil
		},
	})
	mPK := builder.NewModel[User](s, cliPK).WithReturning(builder.ReturningPK)
	res2, err := mPK.Delete(context.Background(), func(*User, *core.Query) {})
	assert.NoError(t, err)
	assert.Equal(t, 2, res2[0].ID)

	// Default (sem returning)
	cliDef := core.NewClient(&fakeDriver{
		execFn: func(_ string, _ ...any) (core.Result, error) {
			return fakeResult{}, nil
		},
	})
	mDef := builder.NewModel[User](s, cliDef)
	_, err = mDef.Delete(context.Background(), func(*User, *core.Query) {})
	assert.NoError(t, err)
}

func TestCountFindOneFindMany(t *testing.T) {
	type User struct{ ID int }
	s := core.NewSchema(func(u *User, s *core.Schema) *core.Schema {
		return s.Field(&u.ID, core.Primary())
	})

	// Count
	cliCount := core.NewClient(&fakeDriver{
		queryFn: func(_ string, _ ...any) (core.Rows, error) {
			return &fakeRows{cols: []string{"count"}, values: [][]any{{int64(7)}}}, nil
		},
	})
	m := builder.NewModel[User](s, cliCount)
	n, err := m.Count(context.Background(), func(*User, *core.Query) {})
	assert.NoError(t, err)
	assert.Equal(t, int64(7), n)

	// FindOne success
	cliFind := core.NewClient(&fakeDriver{
		queryFn: func(_ string, _ ...any) (core.Rows, error) {
			return &fakeRows{cols: []string{"id"}, values: [][]any{{123}}}, nil
		},
	})
	mFind := builder.NewModel[User](s, cliFind)
	u, err := mFind.FindOne(context.Background(), func(*User, *core.Query) {})
	assert.NoError(t, err)
	assert.Equal(t, 123, u.ID)

	// FindOne empty → erro
	cliEmpty := core.NewClient(&fakeDriver{
		queryFn: func(_ string, _ ...any) (core.Rows, error) {
			return &fakeRows{cols: []string{"id"}, values: [][]any{}}, nil
		},
	})
	mEmpty := builder.NewModel[User](s, cliEmpty)
	_, err = mEmpty.FindOne(context.Background(), func(*User, *core.Query) {})
	assert.Error(t, err)

	// FindMany
	cliMany := core.NewClient(&fakeDriver{
		queryFn: func(_ string, _ ...any) (core.Rows, error) {
			return &fakeRows{cols: []string{"id"}, values: [][]any{{1}, {2}}}, nil
		},
	})
	mMany := builder.NewModel[User](s, cliMany)
	users, err := mMany.FindMany(context.Background(), func(*User, *core.Query) {})
	assert.NoError(t, err)
	assert.Len(t, users, 2)
}
