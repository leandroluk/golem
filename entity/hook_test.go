package entity

import (
	"context"
	"testing"

	"github.com/leandroluk/golem"
)

type HookTestSubject struct {
	ID int64
}

func TestAddHook_RegistersFluentHooks(t *testing.T) {
	e := New(func(s *HookTestSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
	})

	fnBefore := func(ctx context.Context, s *HookTestSubject, conn golem.Conn) error { return nil }
	fnAfter := func(ctx context.Context, s *HookTestSubject, conn golem.Conn) error { return nil }
	fnConflict := func(ctx context.Context, s *HookTestSubject, conn golem.Conn) error { return nil }

	AddHook(e).
		BeforeCreate(fnBefore).
		AfterCreate(fnAfter).
		OnConflictCreate(fnConflict)

	if e.hooks.BeforeCreate == nil {
		t.Fatal("BeforeCreate not registered")
	}
	if e.hooks.AfterCreate == nil {
		t.Fatal("AfterCreate not registered")
	}
	if e.hooks.OnConflictCreate == nil {
		t.Fatal("OnConflictCreate not registered")
	}
}

func TestAddHook_PanicsOnDuplicateRegistration(t *testing.T) {
	e := New(func(s *HookTestSubject, b *Table) {
		b.Col(&s.ID, golem.BIGINT())
	})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate hook registration, got none")
		} else {
			want := "entity: hook slot 'BeforeCreate' already registered"
			if got := r.(string); got != want {
				t.Fatalf("panic = %q, want %q", got, want)
			}
		}
	}()

	fn := func(ctx context.Context, s *HookTestSubject, conn golem.Conn) error { return nil }
	AddHook(e).
		BeforeCreate(fn).
		BeforeCreate(fn)
}
