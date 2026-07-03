package entity

import (
	"context"

	"github.com/leandroluk/golem"
)

// HookBuilder provides a fluent chainable API to register transaction-aware
// lifecycle hooks on an Entity[T].
type HookBuilder[T any] struct {
	entity *Entity[T]
}

// AddHook starts a fluent hook registration chain for the given entity.
func AddHook[T any](e *Entity[T]) *HookBuilder[T] {
	return &HookBuilder[T]{entity: e}
}

// BeforeCreate registers a hook that runs before an INSERT statement.
func (hb *HookBuilder[T]) BeforeCreate(fn func(context.Context, *T, golem.Conn) error) *HookBuilder[T] {
	if hb.entity.hooks.BeforeCreate != nil {
		panic("entity: hook slot 'BeforeCreate' already registered")
	}
	hb.entity.hooks.BeforeCreate = fn
	return hb
}

// AfterCreate registers a hook that runs after a successful INSERT statement.
func (hb *HookBuilder[T]) AfterCreate(fn func(context.Context, *T, golem.Conn) error) *HookBuilder[T] {
	if hb.entity.hooks.AfterCreate != nil {
		panic("entity: hook slot 'AfterCreate' already registered")
	}
	hb.entity.hooks.AfterCreate = fn
	return hb
}

// OnConflictCreate registers a hook that runs if an INSERT statement returns a constraint conflict.
func (hb *HookBuilder[T]) OnConflictCreate(fn func(context.Context, *T, golem.Conn) error) *HookBuilder[T] {
	if hb.entity.hooks.OnConflictCreate != nil {
		panic("entity: hook slot 'OnConflictCreate' already registered")
	}
	hb.entity.hooks.OnConflictCreate = fn
	return hb
}

// BeforeUpdate registers a hook that runs before a Save/UPDATE operation on an instance.
func (hb *HookBuilder[T]) BeforeUpdate(fn func(context.Context, *T, golem.Conn) error) *HookBuilder[T] {
	if hb.entity.hooks.BeforeUpdate != nil {
		panic("entity: hook slot 'BeforeUpdate' already registered")
	}
	hb.entity.hooks.BeforeUpdate = fn
	return hb
}

// AfterUpdate registers a hook that runs after a successful UPDATE statement.
func (hb *HookBuilder[T]) AfterUpdate(fn func(context.Context, *T, golem.Conn) error) *HookBuilder[T] {
	if hb.entity.hooks.AfterUpdate != nil {
		panic("entity: hook slot 'AfterUpdate' already registered")
	}
	hb.entity.hooks.AfterUpdate = fn
	return hb
}

// OnConflictUpdate registers a hook that runs if an UPDATE statement returns a constraint conflict.
func (hb *HookBuilder[T]) OnConflictUpdate(fn func(context.Context, *T, golem.Conn) error) *HookBuilder[T] {
	if hb.entity.hooks.OnConflictUpdate != nil {
		panic("entity: hook slot 'OnConflictUpdate' already registered")
	}
	hb.entity.hooks.OnConflictUpdate = fn
	return hb
}

// BeforeDelete registers a hook that runs before a DELETE statement.
func (hb *HookBuilder[T]) BeforeDelete(fn func(context.Context, *T, golem.Conn) error) *HookBuilder[T] {
	if hb.entity.hooks.BeforeDelete != nil {
		panic("entity: hook slot 'BeforeDelete' already registered")
	}
	hb.entity.hooks.BeforeDelete = fn
	return hb
}

// AfterDelete registers a hook that runs after a successful DELETE statement.
func (hb *HookBuilder[T]) AfterDelete(fn func(context.Context, *T, golem.Conn) error) *HookBuilder[T] {
	if hb.entity.hooks.AfterDelete != nil {
		panic("entity: hook slot 'AfterDelete' already registered")
	}
	hb.entity.hooks.AfterDelete = fn
	return hb
}

// OnConflictDelete registers a hook that runs if a DELETE statement returns a constraint conflict.
func (hb *HookBuilder[T]) OnConflictDelete(fn func(context.Context, *T, golem.Conn) error) *HookBuilder[T] {
	if hb.entity.hooks.OnConflictDelete != nil {
		panic("entity: hook slot 'OnConflictDelete' already registered")
	}
	hb.entity.hooks.OnConflictDelete = fn
	return hb
}
