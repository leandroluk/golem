package entity

import (
	"testing"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/column"
)

type BuilderTestSubject struct {
	ID   int64
	Name string
}

func TestBuilder_Col_ReturnsColumnBuilderForChaining(t *testing.T) {
	var cb *column.Builder
	New(func(s *BuilderTestSubject, b *Builder) {
		cb = b.Col(&s.Name, golem.VARCHAR(50))
	})

	if cb == nil {
		t.Fatal("Col() returned nil *column.Builder")
	}
	// Name() must be chainable and affect the final resolved column name;
	// covered end-to-end in entity_test.go's NameOverride test. Here we just
	// confirm the returned builder responds to Name()/ResolvedName().
	cb.Name("renamed")
	if got := cb.ResolvedName(); got != "renamed" {
		t.Fatalf("ResolvedName() = %q, want %q", got, "renamed")
	}
}

func TestBuilder_Col_PanicsOnForeignFieldPointer(t *testing.T) {
	var other int64

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when fieldPtr does not belong to T, got none")
		}
	}()

	New(func(s *BuilderTestSubject, b *Builder) {
		b.Col(&other, golem.BIGINT())
	})
}

func TestBuilder_PrimaryKey_PanicsOnForeignFieldPointer(t *testing.T) {
	var other int64

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when fieldPtr does not belong to T, got none")
		}
	}()

	New(func(s *BuilderTestSubject, b *Builder) {
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&other)
	})
}

func TestBuilder_ForeignKey_PanicsOnForeignFieldPointer(t *testing.T) {
	var other int64
	target := New(func(s *BuilderTestSubject, b *Builder) {
		b.Col(&s.ID, golem.BIGINT())
		b.PrimaryKey(&s.ID)
	})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when fieldPtr does not belong to T, got none")
		}
	}()

	New(func(s *BuilderTestSubject, b *Builder) {
		b.ForeignKey(&other, target)
	})
}

func TestBuilder_TableName_And_SchemaName_AreIndependentlyOverridable(t *testing.T) {
	e := New(func(s *BuilderTestSubject, b *Builder) {
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
