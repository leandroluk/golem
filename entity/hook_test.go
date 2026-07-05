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

	fn := func(ctx context.Context, s *HookTestSubject, conn golem.Conn) error { return nil }

	AddHook(e).
		BeforeCreate(fn).
		AfterCreate(fn).
		OnConflictCreate(fn).
		BeforeUpdate(fn).
		AfterUpdate(fn).
		OnConflictUpdate(fn).
		BeforeDelete(fn).
		AfterDelete(fn).
		OnConflictDelete(fn)

	if e.hooks.BeforeCreate == nil {
		t.Fatal("BeforeCreate not registered")
	}
	if e.hooks.AfterCreate == nil {
		t.Fatal("AfterCreate not registered")
	}
	if e.hooks.OnConflictCreate == nil {
		t.Fatal("OnConflictCreate not registered")
	}
	if e.hooks.BeforeUpdate == nil {
		t.Fatal("BeforeUpdate not registered")
	}
	if e.hooks.AfterUpdate == nil {
		t.Fatal("AfterUpdate not registered")
	}
	if e.hooks.OnConflictUpdate == nil {
		t.Fatal("OnConflictUpdate not registered")
	}
	if e.hooks.BeforeDelete == nil {
		t.Fatal("BeforeDelete not registered")
	}
	if e.hooks.AfterDelete == nil {
		t.Fatal("AfterDelete not registered")
	}
	if e.hooks.OnConflictDelete == nil {
		t.Fatal("OnConflictDelete not registered")
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

func TestAddHook_PanicsOnDuplicateRegistration_AllSlots(t *testing.T) {
	fn := func(ctx context.Context, s *HookTestSubject, conn golem.Conn) error { return nil }

	cases := []struct {
		name     string
		register func(hb *HookBuilder[HookTestSubject])
	}{
		{"AfterCreate", func(hb *HookBuilder[HookTestSubject]) { hb.AfterCreate(fn) }},
		{"OnConflictCreate", func(hb *HookBuilder[HookTestSubject]) { hb.OnConflictCreate(fn) }},
		{"BeforeUpdate", func(hb *HookBuilder[HookTestSubject]) { hb.BeforeUpdate(fn) }},
		{"AfterUpdate", func(hb *HookBuilder[HookTestSubject]) { hb.AfterUpdate(fn) }},
		{"OnConflictUpdate", func(hb *HookBuilder[HookTestSubject]) { hb.OnConflictUpdate(fn) }},
		{"BeforeDelete", func(hb *HookBuilder[HookTestSubject]) { hb.BeforeDelete(fn) }},
		{"AfterDelete", func(hb *HookBuilder[HookTestSubject]) { hb.AfterDelete(fn) }},
		{"OnConflictDelete", func(hb *HookBuilder[HookTestSubject]) { hb.OnConflictDelete(fn) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := New(func(s *HookTestSubject, b *Table) {
				b.Col(&s.ID, golem.BIGINT())
			})
			hb := AddHook(e)
			tc.register(hb)

			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected panic on duplicate %s registration, got none", tc.name)
				}
				want := "entity: hook slot '" + tc.name + "' already registered"
				if got := r.(string); got != want {
					t.Fatalf("panic = %q, want %q", got, want)
				}
			}()
			tc.register(hb)
		})
	}
}
