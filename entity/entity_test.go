package entity

import (
	"reflect"
	"testing"

	"github.com/leandroluk/golem"
)

// Widget is a struct whose name is used to assert the default table-name
// behavior (lowercased struct name) independent of the "user"/"post"-style
// domain structs used elsewhere in these tests.
type Widget struct {
	ID int64
}

func TestNew_DefaultTableName_LowercasedStructName(t *testing.T) {
	e := New(func(w *Widget, b *Builder) {
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
	e := New(func(u *User, b *Builder) {
		b.TableName("users")
		b.Col(&u.ID, golem.BIGINT())
	})

	got := e.Describe().TableName
	if got != "users" {
		t.Fatalf("TableName = %q, want %q", got, "users")
	}
}

func TestNew_SchemaName_Override(t *testing.T) {
	e := New(func(u *User, b *Builder) {
		b.SchemaName("public")
		b.Col(&u.ID, golem.BIGINT())
	})

	got := e.Describe().SchemaName
	if got != "public" {
		t.Fatalf("SchemaName = %q, want %q", got, "public")
	}
}

func TestNew_Col_DefaultColumnName_LowercasedFieldName(t *testing.T) {
	e := New(func(u *User, b *Builder) {
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
	e := New(func(u *User, b *Builder) {
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

// TwoInts is the struct required by spec.md AC-2: two fields of the *same*
// Go type (int64), which can only be distinguished by memory offset, not by
// type or declaration order.
type TwoInts struct {
	A int64
	B int64
}

func TestNew_Col_DistinguishesSameTypeFields(t *testing.T) {
	e := New(func(v *TwoInts, b *Builder) {
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
	e := New(func(u *User, b *Builder) {
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

func TestNew_PrimaryKey_Single(t *testing.T) {
	e := New(func(u *User, b *Builder) {
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
	e := New(func(v *TwoInts, b *Builder) {
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

// PostEntityUnderTest mimics a "Post" style entity referencing a "User"
// style entity, per AC-3 (ForeignKey recording).
type PostEntityUnderTest struct {
	ID      int64
	OwnerID int64
}

func TestNew_ForeignKey_DoesNotPanicAndResolvesField(t *testing.T) {
	userEntity := New(func(u *User, b *Builder) {
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
		e = New(func(p *PostEntityUnderTest, b *Builder) {
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

	_, err := resolveField(&u, &other)
	if err == nil {
		t.Fatal("expected error when fieldPtr does not belong to zero's type, got nil")
	}
}

func TestResolveField_OffsetBased_DistinguishesSameTypeFields(t *testing.T) {
	var v TwoInts

	nameA, err := resolveField(&v, &v.A)
	if err != nil {
		t.Fatalf("resolveField(&v.A) error = %v", err)
	}
	if nameA != "A" {
		t.Fatalf("resolveField(&v.A) = %q, want %q", nameA, "A")
	}

	nameB, err := resolveField(&v, &v.B)
	if err != nil {
		t.Fatalf("resolveField(&v.B) error = %v", err)
	}
	if nameB != "B" {
		t.Fatalf("resolveField(&v.B) = %q, want %q", nameB, "B")
	}
}
