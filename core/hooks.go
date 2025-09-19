// Package core provides the fundamental building blocks of the golem ORM.
// This file defines lifecycle hooks that allow custom logic to be executed
// before or after persistence operations such as insert, update, delete, and find.
package core

// PreHook represents a lifecycle hook that runs before a persistence operation.
//
// Hooks are identified by string tokens (e.g., "pre:insert") and can be
// registered per entity schema. They allow validation, transformation,
// or side effects to be applied before the operation is executed.
type PreHook string

// PostHook represents a lifecycle hook that runs after a persistence operation.
//
// Hooks are identified by string tokens (e.g., "post:update") and can be
// registered per entity schema. They allow actions such as logging,
// cache invalidation, or event publishing after the operation succeeds.
type PostHook string

const (
	// PreInsert is executed before an entity/document is inserted.
	PreInsert PreHook = "pre:insert"
	// PreUpdate is executed before an entity/document is updated.
	PreUpdate PreHook = "pre:update"
	// PreDelete is executed before an entity/document is deleted.
	PreDelete PreHook = "pre:delete"
	// PreFind is executed before a query (find operation) is performed.
	PreFind PreHook = "pre:find"

	// PostInsert is executed after an entity/document is inserted.
	PostInsert PostHook = "post:insert"
	// PostUpdate is executed after an entity/document is updated.
	PostUpdate PostHook = "post:update"
	// PostDelete is executed after an entity/document is deleted.
	PostDelete PostHook = "post:delete"
	// PostFind is executed after a query (find operation) has been executed.
	PostFind PostHook = "post:find"
)
