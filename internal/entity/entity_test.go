package entity

import (
	"context"
	"reflect"
	"testing"
	"time"

	golem "github.com/leandroluk/golem/internal/core"
)

type Widget struct {
	ID int64
}

func TestNew_DefaultTableName_LowercasedStructName(t *testing.T) {
	e := New(func(w *Widget, b *Table) {
		b.Col(&w.ID, golem.BIGINT())
	})

	got := e.Describe().TableName
	if got != "widget" {
		t.Fatalf("TableName = %q, want %q", got, "widget")
	}
}

type User struct {
	ID   int64
	Name string
}

func TestNew_TableName_Override(t *testing.T) {
	e := New(func(u *User, b *Table) {
		b.TableName("users")
		b.Col(&u.ID, golem.BIGINT())
	})

	got := e.Describe().TableName
	if got != "users" {
		t.Fatalf("TableName = %q, want %q", got, "users")
	}
}

func TestNew_SchemaName_Override(t *testing.T) {
	e := New(func(u *User, b *Table) {
		b.SchemaName("public")
		b.Col(&u.ID, golem.BIGINT())
	})

	got := e.Describe().SchemaName
	if got != "public" {
		t.Fatalf("SchemaName = %q, want %q", got, "public")
	}
}

func TestNew_Col_DefaultColumnName_LowercasedFieldName(t *testing.T) {
	e := New(func(u *User, b *Table) {
		b.Col(&u.ID, golem.BIGINT())
	})

	cols := e.Describe().Columns
	if len(cols) != 1 {
		t.Fatalf("len(Columns) = %d, want 1", len(cols))
	}
	if cols[0].FieldName != "ID" {
		t.Fatalf("Columns[0].FieldName = %q, want %q", cols[0].FieldName, "ID")
	}
	if cols[0].Name != "id" {
		t.Fatalf("Columns[0].Name = %q, want %q", cols[0].Name, "id")
	}
}

func TestNew_Col_NameOverride(t *testing.T) {
	e := New(func(u *User, b *Table) {
		b.Col(&u.Name, golem.VARCHAR(50)).Name("full_name")
	})

	cols := e.Describe().Columns
	if len(cols) != 1 {
		t.Fatalf("len(Columns) = %d, want 1", len(cols))
	}
	if cols[0].FieldName != "Name" {
		t.Fatalf("Columns[0].FieldName = %q, want %q", cols[0].FieldName, "Name")
	}
	if cols[0].Name != "full_name" {
		t.Fatalf("Columns[0].Name = %q, want %q", cols[0].Name, "full_name")
	}
}

type TwoInts struct {
	A int64
	B int64
}

func TestNew_Col_DistinguishesSameTypeFields(t *testing.T) {
	e := New(func(v *TwoInts, b *Table) {
		b.Col(&v.A, golem.BIGINT())
		b.Col(&v.B, golem.BIGINT())
	})

	cols := e.Describe().Columns
	if len(cols) != 2 {
		t.Fatalf("len(Columns) = %d, want 2", len(cols))
	}

	var gotA, gotB bool
	for _, c := range cols {
		switch c.FieldName {
		case "A":
			gotA = true
			if c.Name != "a" {
				t.Fatalf("column for A has Name = %q, want %q", c.Name, "a")
			}
		case "B":
			gotB = true
			if c.Name != "b" {
				t.Fatalf("column for B has Name = %q, want %q", c.Name, "b")
			}
		default:
			t.Fatalf("unexpected FieldName %q", c.FieldName)
		}
	}
	if !gotA || !gotB {
		t.Fatalf("expected distinct entries for both A and B, got %+v", cols)
	}
}

func TestNew_Col_RecordsColumnType(t *testing.T) {
	e := New(func(u *User, b *Table) {
		b.Col(&u.ID, golem.BIGINT())
	})

	cols := e.Describe().Columns
	if len(cols) != 1 {
		t.Fatalf("len(Columns) = %d, want 1", len(cols))
	}
	if !reflect.DeepEqual(cols[0].Type, golem.BIGINT()) {
		t.Fatalf("Columns[0].Type = %+v, want %+v", cols[0].Type, golem.BIGINT())
	}
}

func TestNew_Col_Nullable(t *testing.T) {
	e := New(func(u *User, b *Table) {
		b.Col(&u.Name, golem.TEXT()).Nullable()
	})

	cols := e.Describe().Columns
	if len(cols) != 1 {
		t.Fatalf("len(Columns) = %d, want 1", len(cols))
	}
	if !cols[0].Nullable {
		t.Fatal("Columns[0].Nullable = false, want true")
	}
}

func TestNew_Col_Default(t *testing.T) {
	e := New(func(u *User, b *Table) {
		b.Col(&u.Name, golem.TEXT()).Default("anon")
	})

	cols := e.Describe().Columns
	if len(cols) != 1 {
		t.Fatalf("len(Columns) = %d, want 1", len(cols))
	}
	if !cols[0].HasDefault {
		t.Fatal("Columns[0].HasDefault = false, want true")
	}
	if cols[0].Default != "anon" {
		t.Fatalf("Columns[0].Default = %v, want %q", cols[0].Default, "anon")
	}
}

func TestNew_Col_DefaultFunc(t *testing.T) {
	fn := func() (any, error) { return "computed", nil }
	e := New(func(u *User, b *Table) {
		b.Col(&u.Name, golem.TEXT()).DefaultFunc(fn)
	})

	cols := e.Describe().Columns
	if len(cols) != 1 {
		t.Fatalf("len(Columns) = %d, want 1", len(cols))
	}
	if cols[0].DefaultFunc == nil {
		t.Fatal("Columns[0].DefaultFunc = nil, want non-nil")
	}
	val, err := cols[0].DefaultFunc()
	if err != nil {
		t.Fatalf("DefaultFunc() returned error: %v", err)
	}
	if val != "computed" {
		t.Fatalf("DefaultFunc()() = %v, want %q", val, "computed")
	}
}

func TestNew_PrimaryKey_Single(t *testing.T) {
	e := New(func(u *User, b *Table) {
		b.Col(&u.ID, golem.BIGINT())
		b.PrimaryKey(&u.ID)
	})

	got := e.Describe().PrimaryKey
	want := []string{"id"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PrimaryKey = %v, want %v", got, want)
	}
}

func TestNew_PrimaryKey_Composite_PreservesOrder(t *testing.T) {
	e := New(func(v *TwoInts, b *Table) {
		b.Col(&v.A, golem.BIGINT())
		b.Col(&v.B, golem.BIGINT())
		b.PrimaryKey(&v.A, &v.B)
	})

	got := e.Describe().PrimaryKey
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PrimaryKey = %v, want %v", got, want)
	}
}

type PostEntityUnderTest struct {
	ID      int64
	OwnerID int64
}

func TestNew_ForeignKey_DoesNotPanicAndResolvesField(t *testing.T) {
	userEntity := New(func(u *User, b *Table) {
		b.Col(&u.ID, golem.BIGINT())
		b.PrimaryKey(&u.ID)
	})

	var e *Entity[PostEntityUnderTest]
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ForeignKey panicked unexpectedly: %v", r)
			}
		}()
		e = New(func(p *PostEntityUnderTest, b *Table) {
			b.Col(&p.ID, golem.BIGINT())
			b.Col(&p.OwnerID, golem.BIGINT())
			b.PrimaryKey(&p.ID)
			b.ForeignKey(&p.OwnerID, userEntity)
		})
	}()

	fks := e.Describe().ForeignKeys
	if len(fks) != 1 {
		t.Fatalf("len(ForeignKeys) = %d, want 1", len(fks))
	}
	if fks[0].FieldName != "OwnerID" {
		t.Fatalf("ForeignKeys[0].FieldName = %q, want %q", fks[0].FieldName, "OwnerID")
	}
}

func TestResolveField_ReturnsErrorForForeignPointer(t *testing.T) {
	var u User
	var other int64

	_, err := ResolveField(&u, &other)
	if err == nil {
		t.Fatal("expected error when fieldPtr does not belong to zero's type, got nil")
	}
}

func TestResolveField_OffsetBased_DistinguishesSameTypeFields(t *testing.T) {
	var v TwoInts

	nameA, err := ResolveField(&v, &v.A)
	if err != nil {
		t.Fatalf("ResolveField(&v.A) error = %v", err)
	}
	if nameA != "A" {
		t.Fatalf("ResolveField(&v.A) = %q, want %q", nameA, "A")
	}

	nameB, err := ResolveField(&v, &v.B)
	if err != nil {
		t.Fatalf("ResolveField(&v.B) error = %v", err)
	}
	if nameB != "B" {
		t.Fatalf("ResolveField(&v.B) = %q, want %q", nameB, "B")
	}
}

// -----------------------------------------------------------------------
// Unique
// -----------------------------------------------------------------------

type UniqueSubject struct {
	Email  string
	UserID int64
	OrgID  int64
}

func TestNew_Unique_Single_RecordsColumnName(t *testing.T) {
	e := New(func(s *UniqueSubject, b *Table) {
		b.Col(&s.Email, golem.TEXT())
		b.Unique(&s.Email)
	})

	uniques := e.Describe().Uniques
	if len(uniques) != 1 {
		t.Fatalf("len(Uniques) = %d, want 1", len(uniques))
	}
	if len(uniques[0]) != 1 || uniques[0][0] != "email" {
		t.Fatalf("Uniques[0] = %v, want [email]", uniques[0])
	}
}

func TestNew_Unique_Composite_PreservesOrder(t *testing.T) {
	e := New(func(s *UniqueSubject, b *Table) {
		b.Col(&s.UserID, golem.BIGINT())
		b.Col(&s.OrgID, golem.BIGINT())
		b.Unique(&s.UserID, &s.OrgID)
	})

	uniques := e.Describe().Uniques
	if len(uniques) != 1 {
		t.Fatalf("len(Uniques) = %d, want 1", len(uniques))
	}
	want := []string{"userid", "orgid"}
	if !reflect.DeepEqual(uniques[0], want) {
		t.Fatalf("Uniques[0] = %v, want %v", uniques[0], want)
	}
}

func TestNew_Unique_MultipleConstraints(t *testing.T) {
	e := New(func(s *UniqueSubject, b *Table) {
		b.Col(&s.Email, golem.TEXT())
		b.Col(&s.UserID, golem.BIGINT())
		b.Unique(&s.Email)
		b.Unique(&s.UserID)
	})

	uniques := e.Describe().Uniques
	if len(uniques) != 2 {
		t.Fatalf("len(Uniques) = %d, want 2", len(uniques))
	}
}

func TestNew_Unique_NoUniques_IsNil(t *testing.T) {
	e := New(func(u *User, b *Table) {
		b.Col(&u.ID, golem.BIGINT())
	})

	if e.Describe().Uniques != nil {
		t.Fatalf("Uniques = %v, want nil when no Unique() declared", e.Describe().Uniques)
	}
}

// -----------------------------------------------------------------------
// Index
// -----------------------------------------------------------------------

func TestNew_Index_DefaultNameAndNotUnique(t *testing.T) {
	e := New(func(s *UniqueSubject, b *Table) {
		b.Col(&s.Email, golem.TEXT())
		b.Index(&s.Email)
	})

	indexes := e.Describe().Indexes
	if len(indexes) != 1 {
		t.Fatalf("len(Indexes) = %d, want 1", len(indexes))
	}
	if indexes[0].Name != "" {
		t.Fatalf("Indexes[0].Name = %q, want empty", indexes[0].Name)
	}
	if indexes[0].Unique {
		t.Fatal("Indexes[0].Unique = true, want false")
	}
	if len(indexes[0].Columns) != 1 || indexes[0].Columns[0] != "email" {
		t.Fatalf("Indexes[0].Columns = %v, want [email]", indexes[0].Columns)
	}
}

func TestNew_Index_WithNameAndUnique(t *testing.T) {
	e := New(func(s *UniqueSubject, b *Table) {
		b.Col(&s.Email, golem.TEXT())
		b.Index(&s.Email).Name("idx_email").Unique()
	})

	indexes := e.Describe().Indexes
	if len(indexes) != 1 {
		t.Fatalf("len(Indexes) = %d, want 1", len(indexes))
	}
	if indexes[0].Name != "idx_email" {
		t.Fatalf("Indexes[0].Name = %q, want %q", indexes[0].Name, "idx_email")
	}
	if !indexes[0].Unique {
		t.Fatal("Indexes[0].Unique = false, want true")
	}
}

func TestNew_Index_Composite_PreservesOrder(t *testing.T) {
	e := New(func(s *UniqueSubject, b *Table) {
		b.Col(&s.UserID, golem.BIGINT())
		b.Col(&s.OrgID, golem.BIGINT())
		b.Index(&s.UserID, &s.OrgID)
	})

	indexes := e.Describe().Indexes
	if len(indexes) != 1 {
		t.Fatalf("len(Indexes) = %d, want 1", len(indexes))
	}
	want := []string{"userid", "orgid"}
	if !reflect.DeepEqual(indexes[0].Columns, want) {
		t.Fatalf("Indexes[0].Columns = %v, want %v", indexes[0].Columns, want)
	}
}

func TestNew_Index_NoIndexes_IsNil(t *testing.T) {
	e := New(func(u *User, b *Table) {
		b.Col(&u.ID, golem.BIGINT())
	})

	if e.Describe().Indexes != nil {
		t.Fatalf("Indexes = %v, want nil when no Index() declared", e.Describe().Indexes)
	}
}

// -----------------------------------------------------------------------
// CreateDate / UpdateDate / DeleteDate
// -----------------------------------------------------------------------

type TimestampedSubject struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func TestNew_CreateDate_RecordsFieldName(t *testing.T) {
	e := New(func(s *TimestampedSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.Col(&s.CreatedAt, golem.DATETIME())
		b.CreateDate(&s.CreatedAt)
	})

	if got := e.Describe().CreateDateField; got != "CreatedAt" {
		t.Fatalf("CreateDateField = %q, want %q", got, "CreatedAt")
	}
}

func TestNew_UpdateDate_RecordsFieldName(t *testing.T) {
	e := New(func(s *TimestampedSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.Col(&s.UpdatedAt, golem.DATETIME())
		b.UpdateDate(&s.UpdatedAt)
	})

	if got := e.Describe().UpdateDateField; got != "UpdatedAt" {
		t.Fatalf("UpdateDateField = %q, want %q", got, "UpdatedAt")
	}
}

func TestNew_DeleteDate_RecordsFieldName(t *testing.T) {
	e := New(func(s *TimestampedSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.Col(&s.DeletedAt, golem.DATETIME())
		b.DeleteDate(&s.DeletedAt)
	})

	if got := e.Describe().DeleteDateField; got != "DeletedAt" {
		t.Fatalf("DeleteDateField = %q, want %q", got, "DeletedAt")
	}
}

func TestNew_NoTimestamps_FieldsAreEmpty(t *testing.T) {
	e := New(func(u *User, b *Table) {
		b.Col(&u.ID, golem.BIGINT())
	})

	meta := e.Describe()
	if meta.CreateDateField != "" {
		t.Fatalf("CreateDateField = %q, want empty", meta.CreateDateField)
	}
	if meta.UpdateDateField != "" {
		t.Fatalf("UpdateDateField = %q, want empty", meta.UpdateDateField)
	}
	if meta.DeleteDateField != "" {
		t.Fatalf("DeleteDateField = %q, want empty", meta.DeleteDateField)
	}
}

type triggerTestSubject struct {
	ID int64
}

func TestEntity_Triggers_NoHookRegistered_ReturnNil(t *testing.T) {
	e := New(func(s *triggerTestSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
	})
	ctx := context.Background()
	var conn golem.Conn
	item := &triggerTestSubject{ID: 1}

	triggers := map[string]func() error{
		"BeforeCreate":     func() error { return e.TriggerBeforeCreate(ctx, item, conn) },
		"AfterCreate":      func() error { return e.TriggerAfterCreate(ctx, item, conn) },
		"OnConflictCreate": func() error { return e.TriggerOnConflictCreate(ctx, item, conn) },
		"BeforeUpdate":     func() error { return e.TriggerBeforeUpdate(ctx, item, conn) },
		"AfterUpdate":      func() error { return e.TriggerAfterUpdate(ctx, item, conn) },
		"OnConflictUpdate": func() error { return e.TriggerOnConflictUpdate(ctx, item, conn) },
		"BeforeDelete":     func() error { return e.TriggerBeforeDelete(ctx, item, conn) },
		"AfterDelete":      func() error { return e.TriggerAfterDelete(ctx, item, conn) },
		"OnConflictDelete": func() error { return e.TriggerOnConflictDelete(ctx, item, conn) },
	}
	for name, fn := range triggers {
		if err := fn(); err != nil {
			t.Errorf("Trigger%s with no hook registered: expected nil, got %v", name, err)
		}
	}
}

func TestEntity_Triggers_HookRegistered_RunsIt(t *testing.T) {
	e := New(func(s *triggerTestSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
	})
	ctx := context.Background()
	var conn golem.Conn
	item := &triggerTestSubject{ID: 1}

	var calls []string
	record := func(name string) func(context.Context, *triggerTestSubject, golem.Conn) error {
		return func(context.Context, *triggerTestSubject, golem.Conn) error {
			calls = append(calls, name)
			return nil
		}
	}

	AddHook(e).
		BeforeCreate(record("BeforeCreate")).
		AfterCreate(record("AfterCreate")).
		OnConflictCreate(record("OnConflictCreate")).
		BeforeUpdate(record("BeforeUpdate")).
		AfterUpdate(record("AfterUpdate")).
		OnConflictUpdate(record("OnConflictUpdate")).
		BeforeDelete(record("BeforeDelete")).
		AfterDelete(record("AfterDelete")).
		OnConflictDelete(record("OnConflictDelete"))

	triggers := []struct {
		name string
		fn   func() error
	}{
		{"BeforeCreate", func() error { return e.TriggerBeforeCreate(ctx, item, conn) }},
		{"AfterCreate", func() error { return e.TriggerAfterCreate(ctx, item, conn) }},
		{"OnConflictCreate", func() error { return e.TriggerOnConflictCreate(ctx, item, conn) }},
		{"BeforeUpdate", func() error { return e.TriggerBeforeUpdate(ctx, item, conn) }},
		{"AfterUpdate", func() error { return e.TriggerAfterUpdate(ctx, item, conn) }},
		{"OnConflictUpdate", func() error { return e.TriggerOnConflictUpdate(ctx, item, conn) }},
		{"BeforeDelete", func() error { return e.TriggerBeforeDelete(ctx, item, conn) }},
		{"AfterDelete", func() error { return e.TriggerAfterDelete(ctx, item, conn) }},
		{"OnConflictDelete", func() error { return e.TriggerOnConflictDelete(ctx, item, conn) }},
	}
	for _, tc := range triggers {
		calls = nil
		if err := tc.fn(); err != nil {
			t.Errorf("Trigger%s: unexpected error %v", tc.name, err)
		}
		if len(calls) != 1 || calls[0] != tc.name {
			t.Errorf("Trigger%s: expected hook %q to run, got calls=%v", tc.name, tc.name, calls)
		}
	}
}

// Mixin types mirroring the value-embed composition pattern (Indexable /
// Creatable / Props structs embedded by value into a domain entity).
type MixinIndexable struct {
	ID string
}

type MixinProps struct {
	FirstName string
	LastName  string
}

type WidgetWithMixins struct {
	MixinIndexable
	MixinProps
}

// Regression test: fields promoted through a value-embedded anonymous
// struct must resolve to their own name and offset, not the embed's. Before
// fieldByOffset gained recursion, any field past the embed's own first
// field (e.g. LastName, not at the same offset as the MixinProps field
// itself) failed to resolve at all.
func TestNew_ResolvesFieldsPromotedThroughValueEmbeds(t *testing.T) {
	e := New(func(w *WidgetWithMixins, b *Table) {
		b.Col(&w.ID, golem.TEXT()).Name("id")
		b.Col(&w.FirstName, golem.TEXT()).Name("first_name")
		b.Col(&w.LastName, golem.TEXT()).Name("last_name")
		b.PrimaryKey(&w.ID)
	})

	meta := e.Describe()
	byName := make(map[string]ColumnMeta, len(meta.Columns))
	for _, c := range meta.Columns {
		byName[c.FieldName] = c
	}

	wantType := reflect.TypeOf("")
	for _, fieldName := range []string{"ID", "FirstName", "LastName"} {
		col, ok := byName[fieldName]
		if !ok {
			t.Fatalf("column for field %q not resolved (columns: %+v)", fieldName, meta.Columns)
		}
		if col.GoType != wantType {
			t.Errorf("field %q: GoType = %v, want %v", fieldName, col.GoType, wantType)
		}
	}

	var zero WidgetWithMixins
	base := reflect.ValueOf(&zero).Elem()
	wantOffset := func(fieldPtr any) uintptr {
		return reflect.ValueOf(fieldPtr).Pointer() - base.UnsafeAddr()
	}
	if got, want := byName["ID"].Offset, wantOffset(&zero.ID); got != want {
		t.Errorf("ID offset = %d, want %d", got, want)
	}
	if got, want := byName["FirstName"].Offset, wantOffset(&zero.FirstName); got != want {
		t.Errorf("FirstName offset = %d, want %d", got, want)
	}
	if got, want := byName["LastName"].Offset, wantOffset(&zero.LastName); got != want {
		t.Errorf("LastName offset = %d, want %d", got, want)
	}

	if len(meta.PrimaryKey) != 1 || meta.PrimaryKey[0] != "id" {
		t.Errorf("PrimaryKey = %v, want [id]", meta.PrimaryKey)
	}
}
