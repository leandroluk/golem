package entity

import (
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/relation"
)

type registryTestParent struct {
	ID int64
}

type registryTestChild struct {
	ID        int64
	ParentID  int64
	DeletedAt *int64
}

func TestForeignKeysReferencing_RegistersOnEntityNew(t *testing.T) {
	parent := New(func(p *registryTestParent, b *Table) {
		b.TableName("registry_parents_1")
		b.Col(&p.ID, golem.BIGINT())
		b.PrimaryKey(&p.ID)
	})

	opts := relation.NewForeignKeyOptions().OnDelete(relation.OnDeleteCascade)

	New(func(c *registryTestChild, b *Table) {
		b.TableName("registry_children_1")
		b.Col(&c.ID, golem.BIGINT())
		b.Col(&c.ParentID, golem.BIGINT()).Name("parent_id")
		b.PrimaryKey(&c.ID)
		b.ForeignKey(&c.ParentID, parent, opts)
	})

	regs := ForeignKeysReferencing("registry_parents_1")
	if len(regs) != 1 {
		t.Fatalf("len(regs) = %d, want 1", len(regs))
	}
	reg := regs[0]
	if reg.ChildTableName != "registry_children_1" {
		t.Errorf("ChildTableName = %q, want registry_children_1", reg.ChildTableName)
	}
	if reg.ChildColumn != "parent_id" {
		t.Errorf("ChildColumn = %q, want parent_id", reg.ChildColumn)
	}
	if reg.TargetTableName != "registry_parents_1" {
		t.Errorf("TargetTableName = %q, want registry_parents_1", reg.TargetTableName)
	}
	if reg.Options != opts {
		t.Error("expected the exact *relation.ForeignKeyOptions instance registered")
	}
}

func TestForeignKeysReferencing_CapturesChildDeleteDateColumn(t *testing.T) {
	parent := New(func(p *registryTestParent, b *Table) {
		b.TableName("registry_parents_2")
		b.Col(&p.ID, golem.BIGINT())
		b.PrimaryKey(&p.ID)
	})

	New(func(c *registryTestChild, b *Table) {
		b.TableName("registry_children_2")
		b.Col(&c.ID, golem.BIGINT())
		b.Col(&c.ParentID, golem.BIGINT())
		b.Col(&c.DeletedAt, golem.DATETIME()).Name("deleted_at")
		b.PrimaryKey(&c.ID)
		b.DeleteDate(&c.DeletedAt)
		b.ForeignKey(&c.ParentID, parent)
	})

	regs := ForeignKeysReferencing("registry_parents_2")
	if len(regs) != 1 {
		t.Fatalf("len(regs) = %d, want 1", len(regs))
	}
	if regs[0].ChildDeleteDateColumn != "deleted_at" {
		t.Errorf("ChildDeleteDateColumn = %q, want deleted_at", regs[0].ChildDeleteDateColumn)
	}
}

func TestForeignKeysReferencing_NoMatch_ReturnsEmpty(t *testing.T) {
	regs := ForeignKeysReferencing("no_such_table_registered_anywhere")
	if len(regs) != 0 {
		t.Errorf("len(regs) = %d, want 0", len(regs))
	}
}

func TestForeignKeysReferencing_ReturnsACopy(t *testing.T) {
	parent := New(func(p *registryTestParent, b *Table) {
		b.TableName("registry_parents_3")
		b.Col(&p.ID, golem.BIGINT())
		b.PrimaryKey(&p.ID)
	})
	New(func(c *registryTestChild, b *Table) {
		b.TableName("registry_children_3")
		b.Col(&c.ID, golem.BIGINT())
		b.Col(&c.ParentID, golem.BIGINT())
		b.PrimaryKey(&c.ID)
		b.ForeignKey(&c.ParentID, parent)
	})

	regs := ForeignKeysReferencing("registry_parents_3")
	regs[0].ChildTableName = "mutated"

	regsAgain := ForeignKeysReferencing("registry_parents_3")
	if regsAgain[0].ChildTableName != "registry_children_3" {
		t.Error("mutating a returned slice's element leaked back into the registry — ForeignKeysReferencing must return a copy")
	}
}
