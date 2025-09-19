// Package core provides the fundamental building blocks of the golem ORM.
// It defines abstractions for queries, models, schema handling, events, and drivers.
package core

import "sync"

// Event represents a lifecycle event that can be emitted by the ORM.
//
// Events are triggered during insert, update, delete, and find operations.
// They allow users to register custom handlers to observe or react to changes
// in the persistence layer.
type Event string

const (
	// EventInsert is emitted after an entity/document is inserted.
	EventInsert Event = "insert"
	// EventUpdate is emitted after an entity/document is updated.
	EventUpdate Event = "update"
	// EventDelete is emitted after an entity/document is deleted.
	EventDelete Event = "delete"
	// EventFind is emitted after an entity/document is retrieved.
	EventFind Event = "find"
)

// EventHandler defines the callback signature for event listeners.
// The payload argument varies depending on the event type (InsertPayload,
// UpdatePayload, DeletePayload, FindOnePayload, FindManyPayload).
type EventHandler func(payload any)

// EventDispatcher manages a list of event handlers and dispatches them
// when the corresponding events are emitted.
type EventDispatcher struct {
	mutex       sync.RWMutex
	handlerList map[Event][]EventHandler
}

// globalDispatcher is the shared event dispatcher used by the ORM.
//
// It provides a global subscription and emission mechanism for events.
var globalDispatcher = &EventDispatcher{
	handlerList: make(map[Event][]EventHandler),
}

// On registers an EventHandler for a specific Event.
//
// Example:
//
//	On(core.EventInsert, func(payload any) {
//	    if p, ok := payload.(core.InsertPayload[User]); ok {
//	        log.Printf("User inserted: %+v", p.Doc)
//	    }
//	})
func On(event Event, handler EventHandler) {
	globalDispatcher.mutex.Lock()
	defer globalDispatcher.mutex.Unlock()
	globalDispatcher.handlerList[event] = append(globalDispatcher.handlerList[event], handler)
}

// Emit triggers all registered handlers for the given Event.
//
// Handlers are executed asynchronously in separate goroutines.
// The payload type depends on the event being emitted.
func Emit(event Event, payload any) {
	globalDispatcher.mutex.RLock()
	defer globalDispatcher.mutex.RUnlock()
	if hs, ok := globalDispatcher.handlerList[event]; ok {
		for _, h := range hs {
			go h(payload)
		}
	}
}

// InsertPayload represents the payload passed to EventInsert handlers.
//
// It contains the schema and the inserted document.
type InsertPayload[T any] struct {
	Schema *SchemaCore
	Doc    *T
}

// UpdatePayload represents the payload passed to EventUpdate handlers.
//
// It contains the schema, the condition used for the update, and the applied changes.
type UpdatePayload struct {
	Schema    *SchemaCore
	Condition *Condition
	Changes   Changes
}

// DeletePayload represents the payload passed to EventDelete handlers.
//
// It contains the schema and the condition that matched the deleted records.
type DeletePayload struct {
	Schema    *SchemaCore
	Condition *Condition
}

// FindOnePayload represents the payload passed to EventFind handlers
// when a single entity/document is retrieved.
type FindOnePayload[T any] struct {
	Schema *SchemaCore
	Where  *Where
	Doc    *T
}

// FindManyPayload represents the payload passed to EventFind handlers
// when multiple entities/documents are retrieved.
type FindManyPayload[T any] struct {
	Schema  *SchemaCore
	Where   *Where
	DocList []T
}
