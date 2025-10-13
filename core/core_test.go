package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/leandroluk/golem/core"
	"github.com/stretchr/testify/assert"
)

// -------------------- fakes --------------------

type fakeResult struct{ n int64 }

func (r fakeResult) RowsAffected() (int64, error) {
	return r.n, nil
}

type fakeRows struct {
	cols   []string
	data   [][]any
	i      int
	closed bool
}

func (r *fakeRows) Next() bool {
	return r.i < len(r.data)
}
func (r *fakeRows) Scan(dest ...any) error {
	if r.i >= len(r.data) {
		return errors.New("no more rows")
	}
	for j := range dest {
		reflect.ValueOf(dest[j]).Elem().Set(reflect.ValueOf(r.data[r.i][j]))
	}
	r.i++
	return nil
}
func (r *fakeRows) Close() error {
	r.closed = true
	return nil
}
func (r *fakeRows) Columns() []string {
	return r.cols
}

type fakeTx struct {
	committed, rolled bool
}

func (t *fakeTx) Exec(ctx context.Context, stmt string, args ...any) (core.Result, error) {
	return fakeResult{n: 1}, nil
}
func (t *fakeTx) Query(ctx context.Context, stmt string, args ...any) (core.Rows, error) {
	return &fakeRows{}, nil
}
func (t *fakeTx) Commit() error {
	t.committed = true
	return nil
}
func (t *fakeTx) Rollback() error {
	t.rolled = true
	return nil
}

type fakeDriver struct {
	tx *fakeTx
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
func (d *fakeDriver) Exec(ctx context.Context, stmt string, args ...any) (core.Result, error) {
	return fakeResult{n: 1}, nil
}
func (d *fakeDriver) Query(ctx context.Context, stmt string, args ...any) (core.Rows, error) {
	return &fakeRows{}, nil
}
func (d *fakeDriver) Begin(ctx context.Context) (core.Tx[string], error) {
	d.tx = &fakeTx{}
	return d.tx, nil
}
func (d *fakeDriver) Dialect() core.Dialect {
	return fakeDialect{}
}

type fakeDialect struct{}

func (d fakeDialect) ToValue(f *core.FieldMeta, v any) any {
	return v
}
func (d fakeDialect) FromValue(f *core.FieldMeta, raw any) (any, error) {
	return raw, nil
}

// -------------------- tests --------------------

func TestClientLifecycleAndTransaction(t *testing.T) {
	fakeDriver := &fakeDriver{}
	client := core.NewClient(fakeDriver)

	assert.NoError(t, client.Connect(context.Background()))
	assert.NoError(t, client.Ping(context.Background()))
	assert.NoError(t, client.Close())

	// commit
	err := client.Transaction(context.Background(), func(tx core.Tx[string]) error { return nil })
	assert.NoError(t, err)
	assert.True(t, fakeDriver.tx.committed)

	// rollback
	err = client.Transaction(context.Background(), func(tx core.Tx[string]) error { return errors.New("fail") })
	assert.Error(t, err)
	assert.True(t, fakeDriver.tx.rolled)
}

func TestQueryAndUpdateNormalization(t *testing.T) {
	type User struct {
		ID   int    `test:"id"`
		Name string `test:"name"`
	}
	schema := core.NewSchema(func(u *User, s *core.Schema) *core.Schema {
		return s.Field(&u.ID, core.Primary()).Field(&u.Name)
	}).WithTable("test")

	user := User{}
	query := core.NewQuery(schema, &user)
	query.Eq(&user.ID, 1).
		Neq("id", 2).
		Like("name", "%a%").
		NotLike("name", "%b%").
		Gt("id", 1).NotGt("id", 1).
		Gte("id", 1).NotGte("id", 1).
		Lt("id", 1).NotLt("id", 1).
		Lte("id", 1).NotLte("id", 1).
		In("id", 1, 2).NotIn("id", 3, 4).
		IsNull("id").IsNotNull("id").
		Limit(10).Skip(2).
		And(core.NewQuery(schema, nil)).
		Or(core.NewQuery(schema, nil)).
		OrderBy("id", core.Asc).
		Join(core.Inner, schema, func(l, r *core.Schema, q *core.Query) {})

	assert.True(t, len(query.Conds) > 0)

	// Update
	update := core.NewUpdate(schema, &user)
	update.Set("name", "X")
	assert.Equal(t, core.SetO, update.Sets[0].Op)
}

func TestAsJson(t *testing.T) {
	rows := &fakeRows{
		cols: []string{"id", "name"},
		data: [][]any{
			{[]byte("1"), "Alice"},
			{[]byte("2"), "Bob"},
		},
	}
	res, err := core.AsJson(rows)
	assert.NoError(t, err)
	assert.Len(t, res, 2)

	var obj map[string]any
	err = json.Unmarshal(res[0], &obj)
	assert.NoError(t, err)
	assert.Equal(t, "1", obj["id"])
	assert.Equal(t, "Alice", obj["name"])
	assert.True(t, rows.closed)
}

func TestAsType(t *testing.T) {
	type Row struct {
		ID   int
		Name string
	}
	rows := &fakeRows{
		cols: []string{"id", "name"},
		data: [][]any{
			{1, "Alice"},
			{2, "Bob"},
		},
	}
	list, err := core.AsType[Row](rows)
	assert.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, "Alice", list[0].Name)
	assert.True(t, rows.closed)
}

func TestSchemaFieldsAndHooks(t *testing.T) {
	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	s := core.NewSchema(func(u *User, s *core.Schema) *core.Schema {
		return s.Field(&u.ID, core.Primary(), core.Unique()).Field(&u.Name, core.Nullable())
	})
	assert.NotNil(t, s.FindPrimaryKey())
	assert.NotNil(t, s.FieldByName("ID"))
	assert.NotNil(t, s.FieldByColumn("id"))
	assert.NotNil(t, s.FieldByIndex(s.Fields[0].Index))

	// AddrIndex
	u := User{}
	idx := s.AddrIndex(&u)
	assert.NotNil(t, idx)

	// Hook
	called := false
	s.Hook("AfterSelect", func(any) error { called = true; return nil })
	_ = s.Hooks["AfterSelect"](&u)
	assert.True(t, called)

	// ResolveField
	assert.Equal(t, "ID", s.Fields[0].Name)
	assert.Equal(t, "id", s.Fields[0].Column)
}

func TestKeyIndex(t *testing.T) {
	assert.Equal(t, "", core.KeyIndex(nil))
	assert.Equal(t, "1,2,3", core.KeyIndex([]int{1, 2, 3}))
}

func TestBuildFieldCache(t *testing.T) {
	type User struct {
		ID   int
		Name string
	}
	s := core.NewSchema(func(u *User, s *core.Schema) *core.Schema {
		return s.Field(&u.ID, core.Primary(),
			core.Getter(func(v any) any { return v.(int) + 1 })).
			Field(&u.Name,
				core.Setter(func(v any) any { return v.(string) + "!" }))
	})

	cache := core.BuildFieldCache(s, reflect.TypeOf(User{}))
	assert.Len(t, cache, 2)
	assert.NotNil(t, cache[0].Getter)
	assert.NotNil(t, cache[1].Setter)

	// chamar de novo pega do cache
	cache2 := core.BuildFieldCache(s, reflect.TypeOf(User{}))
	assert.Equal(t, cache, cache2)
}

func TestFieldOptions(t *testing.T) {
	assert.Equal(t, core.DefaultO, core.Default(123).Token)
	assert.Equal(t, core.NullableO, core.Nullable().Token)
	assert.Equal(t, core.UniqueO, core.Unique().Token)
	assert.Equal(t, core.GetterO, core.Getter(func(v int) int { return v }).Token)
	assert.Equal(t, core.SetterO, core.Setter(func(v int) int { return v }).Token)
	assert.Equal(t, core.EnumO, core.Enum(1, 2).Token)
	assert.Equal(t, core.ReferenceO, core.Reference[int]().Token)
	assert.Equal(t, core.PrimaryO, core.Primary().Token)
	assert.Equal(t, core.CreatedAtO, core.CreatedAt().Token)
	assert.Equal(t, core.UpdatedAtO, core.UpdatedAt().Token)
	assert.Equal(t, core.DeletedAtO, core.DeletedAt().Token)
}
