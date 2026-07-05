package entity

import (
	"testing"
	"time"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/relation"
)

type TableTestSubject struct {
	ID   int64
	Name string
}

func TestTable_Col_ReturnsColumnForChaining(t *testing.T) {
	var cb *Column
	New(func(s *TableTestSubject, b *Table) {
		cb = b.Col(&s.Name, golem.VARCHAR(50))
	})

	if cb == nil {
		t.Fatal("Col() returned nil *Column")
	}
	cb.Name("renamed")
	if got := cb.ResolvedName(); got != "renamed" {
		t.Fatalf("ResolvedName() = %q, want %q", got, "renamed")
	}
}

func TestTable_Col_PanicsOnForeignFieldPointer(t *testing.T) {
	var other int64

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when fieldPtr does not belong to T, got none")
		}
	}()

	New(func(s *TableTestSubject, b *Table) {
		b.Col(&other, golem.BIGINT())
	})
}

func TestTable_PrimaryKey_PanicsOnForeignFieldPointer(t *testing.T) {
	var other int64

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when fieldPtr does not belong to T, got none")
		}
	}()

	New(func(s *TableTestSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&other)
	})
}

func TestTable_ForeignKey_PanicsOnForeignFieldPointer(t *testing.T) {
	var other int64
	target := New(func(s *TableTestSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
	})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when fieldPtr does not belong to T, got none")
		}
	}()

	New(func(s *TableTestSubject, b *Table) {
		b.ForeignKey(&other, target)
	})
}

func TestTable_TableName_And_SchemaName_AreIndependentlyOverridable(t *testing.T) {
	e := New(func(s *TableTestSubject, b *Table) {
		b.TableName("subjects")
		b.SchemaName("app")
		b.Col(&s.ID, golem.BIGINT())
	})

	meta := e.Describe()
	if meta.TableName != "subjects" {
		t.Fatalf("TableName = %q, want %q", meta.TableName, "subjects")
	}
	if meta.SchemaName != "app" {
		t.Fatalf("SchemaName = %q, want %q", meta.SchemaName, "app")
	}
}

func TestTable_Unique_PanicsOnForeignFieldPointer(t *testing.T) {
	var other int64

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when fieldPtr does not belong to T, got none")
		}
	}()

	New(func(s *TableTestSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.Unique(&other)
	})
}

func TestTable_Index_PanicsOnForeignFieldPointer(t *testing.T) {
	var other int64

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when fieldPtr does not belong to T, got none")
		}
	}()

	New(func(s *TableTestSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.Index(&other)
	})
}

type TimestampedTableSubject struct {
	ID        int64
	CreatedAt time.Time
}

func TestTable_CreateDate_PanicsOnForeignFieldPointer(t *testing.T) {
	var other time.Time

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when fieldPtr does not belong to T, got none")
		}
	}()

	New(func(s *TimestampedTableSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.CreateDate(&other)
	})
}

func TestTable_UpdateDate_PanicsOnForeignFieldPointer(t *testing.T) {
	var other time.Time

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when fieldPtr does not belong to T, got none")
		}
	}()

	New(func(s *TimestampedTableSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.UpdateDate(&other)
	})
}

func TestTable_DeleteDate_PanicsOnForeignFieldPointer(t *testing.T) {
	var other time.Time

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when fieldPtr does not belong to T, got none")
		}
	}()

	New(func(s *TimestampedTableSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.DeleteDate(&other)
	})
}

// -----------------------------------------------------------------------
// Uncol'd-field fallback: PrimaryKey/ForeignKey/Unique referencing a field
// that was never declared via Col() fall back to strings.ToLower(fieldName)
// for the column name, instead of the (absent) resolved Col name.
// -----------------------------------------------------------------------

func TestTable_PrimaryKey_UncolledField_FallsBackToLowercasedFieldName(t *testing.T) {
	e := New(func(s *TableTestSubject, b *Table) {
		b.PrimaryKey(&s.ID) // ID never declared via Col
	})
	if got := e.Describe().PrimaryKey; len(got) != 1 || got[0] != "id" {
		t.Fatalf("PrimaryKey = %v, want [id]", got)
	}
}

func TestTable_Unique_UncolledField_FallsBackToLowercasedFieldName(t *testing.T) {
	e := New(func(s *TableTestSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
		b.Unique(&s.Name) // Name never declared via Col
	})
	uniques := e.Describe().Uniques
	if len(uniques) != 1 || len(uniques[0]) != 1 || uniques[0][0] != "name" {
		t.Fatalf("Uniques = %v, want [[name]]", uniques)
	}
}

func TestTable_Index_UncolledField_FallsBackToLowercasedFieldName(t *testing.T) {
	e := New(func(s *TableTestSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
		b.Index(&s.Name) // Name never declared via Col
	})
	indexes := e.Describe().Indexes
	if len(indexes) != 1 || len(indexes[0].Columns) != 1 || indexes[0].Columns[0] != "name" {
		t.Fatalf("Indexes = %+v, want Columns=[name]", indexes)
	}
}

func TestTable_ForeignKey_UncolledField_FallsBackToLowercasedFieldName(t *testing.T) {
	target := New(func(s *TableTestSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
	})
	e := New(func(s *TableTestSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
		b.ForeignKey(&s.Name, target) // Name never declared via Col
	})
	fks := e.Describe().ForeignKeys
	if len(fks) != 1 || fks[0].ColumnName != "name" {
		t.Fatalf("ForeignKeys = %+v, want ColumnName=name", fks)
	}
}


// -----------------------------------------------------------------------
// ForeignKey target resolution (moved here from table_foreignkey_test.go —
// all tests for table.go's methods live in table_test.go)
// -----------------------------------------------------------------------

type fkTestParent struct {
	ID int64
}

type fkTestChild struct {
	ID       int64
	ParentID int64
}

type fkTestCompositeParent struct {
	A int64
	B int64
}

func TestTable_ForeignKey_ResolvesTargetTableAndColumn(t *testing.T) {
	parent := New(func(p *fkTestParent, b *Table) {
		b.TableName("fk_parents")
		b.Col(&p.ID, golem.BIGINT())
		b.PrimaryKey(&p.ID)
	})

	child := New(func(c *fkTestChild, b *Table) {
		b.TableName("fk_children")
		b.Col(&c.ID, golem.BIGINT())
		b.Col(&c.ParentID, golem.BIGINT()).Name("parent_id")
		b.PrimaryKey(&c.ID)
		b.ForeignKey(&c.ParentID, parent)
	})

	fks := child.Describe().ForeignKeys
	if len(fks) != 1 {
		t.Fatalf("len(ForeignKeys) = %d, want 1", len(fks))
	}
	fk := fks[0]
	if fk.FieldName != "ParentID" {
		t.Errorf("FieldName = %q, want ParentID", fk.FieldName)
	}
	if fk.ColumnName != "parent_id" {
		t.Errorf("ColumnName = %q, want parent_id", fk.ColumnName)
	}
	if fk.TargetTableName != "fk_parents" {
		t.Errorf("TargetTableName = %q, want fk_parents", fk.TargetTableName)
	}
	if fk.TargetPrimaryKey != "id" {
		t.Errorf("TargetPrimaryKey = %q, want id", fk.TargetPrimaryKey)
	}
	if fk.Options == nil {
		t.Fatal("Options is nil, want a default *relation.ForeignKeyOptions")
	}
	if fk.Options.ResolvedOnDelete() != relation.OnDeleteDefault {
		t.Errorf("default Options.OnDelete = %q, want %q", fk.Options.ResolvedOnDelete(), relation.OnDeleteDefault)
	}
}

func TestTable_ForeignKey_WithExplicitOptions(t *testing.T) {
	parent := New(func(p *fkTestParent, b *Table) {
		b.TableName("fk_parents2")
		b.Col(&p.ID, golem.BIGINT())
		b.PrimaryKey(&p.ID)
	})

	opts := relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade)

	child := New(func(c *fkTestChild, b *Table) {
		b.TableName("fk_children2")
		b.Col(&c.ID, golem.BIGINT())
		b.Col(&c.ParentID, golem.BIGINT())
		b.PrimaryKey(&c.ID)
		b.ForeignKey(&c.ParentID, parent, opts)
	})

	fk := child.Describe().ForeignKeys[0]
	if fk.Options != opts {
		t.Error("expected the exact *relation.ForeignKeyOptions instance passed in to be stored")
	}
	if fk.Options.ResolvedOnDelete() != relation.OnDeleteCascade {
		t.Errorf("OnDelete = %q, want cascade", fk.Options.ResolvedOnDelete())
	}
}

func TestTable_ForeignKey_PanicsOnNonDescriberTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for a target that doesn't implement Describe() EntityMeta, got none")
		}
	}()

	New(func(c *fkTestChild, b *Table) {
		b.Col(&c.ID, golem.BIGINT())
		b.PrimaryKey(&c.ID)
		b.ForeignKey(&c.ParentID, "not an entity")
	})
}

func TestTable_ForeignKey_PanicsOnCompositePrimaryKeyTarget(t *testing.T) {
	compositeParent := New(func(p *fkTestCompositeParent, b *Table) {
		b.TableName("fk_composite_parents")
		b.Col(&p.A, golem.BIGINT())
		b.Col(&p.B, golem.BIGINT())
		b.PrimaryKey(&p.A, &p.B)
	})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for a composite-PK ForeignKey target, got none")
		}
	}()

	New(func(c *fkTestChild, b *Table) {
		b.Col(&c.ID, golem.BIGINT())
		b.PrimaryKey(&c.ID)
		b.ForeignKey(&c.ParentID, compositeParent)
	})
}
