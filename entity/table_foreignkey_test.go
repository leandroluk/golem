package entity

import (
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/relation"
)

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
