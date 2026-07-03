package entity

import (
	"testing"
	"time"

	"github.com/leandroluk/golem"
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

