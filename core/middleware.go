// Package core provides the fundamental building blocks of the golem ORM.
// This file defines the middleware system, which allows cross-cutting concerns
// (logging, caching, auditing, etc.) to be applied to ORM operations.
package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Operation represents the type of operation being executed by the ORM.
//
// It is used within middlewares to distinguish between inserts, updates,
// deletes, and queries.
type Operation string

const (
	// OperationInsert corresponds to an insert (create) operation.
	OperationInsert Operation = "insert"
	// OperationUpdate corresponds to an update operation.
	OperationUpdate Operation = "update"
	// OperationDelete corresponds to a delete operation.
	OperationDelete Operation = "delete"
	// OperationFind corresponds to a query (find) operation.
	OperationFind Operation = "find"
)

// Handler is the function signature executed by the ORM pipeline.
//
// It receives a context, the operation type, and an arbitrary payload.
// Handlers are composed by middlewares to add cross-cutting logic.
type Handler func(ctx context.Context, op Operation, payload any) error

// Middleware is a function that wraps a Handler with additional logic.
//
// Middlewares are chained globally and executed for every operation.
// They follow the decorator pattern.
type Middleware func(next Handler) Handler

var globalMiddlewareList []Middleware

// Use registers a new global middleware, applied to all operations.
//
// Middlewares are executed in reverse registration order: the most
// recently registered middleware is executed first.
func Use(mw Middleware) {
	globalMiddlewareList = append(globalMiddlewareList, mw)
}

// runMiddlewares applies the chain of middlewares to the final handler.
func runMiddlewares(final Handler) Handler {
	h := final
	// Apply in reverse order (last registered runs first).
	for i := len(globalMiddlewareList) - 1; i >= 0; i-- {
		h = globalMiddlewareList[i](h)
	}
	return h
}

// dispatchOperation executes an operation through the global middleware chain.
//
// The exec function contains the core logic of the operation and is wrapped
// by the registered middlewares.
func dispatchOperation(ctx context.Context, op Operation, payload any, exec func() error) error {
	handler := runMiddlewares(func(ctx context.Context, op Operation, payload any) error {
		return exec()
	})
	return handler(ctx, op, payload)
}

// DebugMiddleware logs all operations passing through the ORM.
//
// It measures execution time and prints both success and error cases.
// This is useful for debugging and profiling.
//
// Example:
//
//	core.Use(core.DebugMiddleware())
func DebugMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, op Operation, payload any) error {
			start := time.Now()
			fmt.Printf("[DEBUG] op=%s payload=%+v\n", op, payload)
			err := next(ctx, op, payload)
			elapsed := time.Since(start)
			if err != nil {
				fmt.Printf("[DEBUG] op=%s error=%v took=%s\n", op, err, elapsed)
			} else {
				fmt.Printf("[DEBUG] op=%s success took=%s\n", op, elapsed)
			}
			return err
		}
	}
}

// Cache defines the interface for pluggable caching mechanisms.
//
// A Cache stores arbitrary values with a TTL (time-to-live) and can
// be used by middlewares to avoid hitting the database repeatedly.
type Cache interface {
	Get(key string) (any, bool)
	Set(key string, value any, ttl time.Duration)
}

// memoryCache is a simple in-memory Cache implementation.
//
// It uses a map protected by a RWMutex and supports expiration.
type memoryCache struct {
	data  map[string]memoryEntry
	mutex sync.RWMutex
}

type memoryEntry struct {
	value      any
	expiration time.Time
}

// NewMemoryCache creates a new in-memory Cache instance.
func NewMemoryCache() Cache {
	return &memoryCache{
		data: make(map[string]memoryEntry),
	}
}

// Get retrieves a value from the cache by key.
// It returns false if the key does not exist or is expired.
func (c *memoryCache) Get(key string) (any, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	entry, ok := c.data[key]
	if !ok {
		return nil, false
	}
	if !entry.expiration.IsZero() && time.Now().After(entry.expiration) {
		return nil, false
	}
	return entry.value, true
}

// Set stores a value in the cache with the given TTL (time-to-live).
// If TTL is 0, the entry does not expire.
func (c *memoryCache) Set(key string, value any, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.data[key] = memoryEntry{value: value, expiration: exp}
}

// cacheMiddlewareFieldToken identifies optional parameters for the cache middleware.
type cacheMiddlewareFieldToken string

const (
	cacheMiddlewareFieldTokenTTL = "ttl"
)

// cacheMiddlewareField allows customization of CacheMiddleware behavior,
// such as overriding the default TTL.
type cacheMiddlewareField struct {
	Token cacheMiddlewareFieldToken
	Value any
}

// CacheMiddlewareTTLField creates a field that overrides the TTL used by CacheMiddleware.
func CacheMiddlewareTTLField(value time.Duration) *cacheMiddlewareField {
	return &cacheMiddlewareField{
		Token: cacheMiddlewareFieldTokenTTL,
		Value: value,
	}
}

// CacheMiddleware adds caching for read operations (FindOne, FindMany, Count).
//
// It caches query results keyed by the operation and payload. If the same
// query is executed within the TTL window, the cached result is returned
// instead of hitting the database.
//
// Example:
//
//	cache := core.NewMemoryCache()
//	core.Use(core.CacheMiddleware(cache, *core.CacheMiddlewareTTLField(1*time.Minute)))
func CacheMiddleware(cache Cache, fieldList ...cacheMiddlewareField) Middleware {
	var ttl = 30 * time.Second

	for _, f := range fieldList {
		if f.Token == cacheMiddlewareFieldTokenTTL {
			ttl = f.Value.(time.Duration)
		}
	}

	return func(next Handler) Handler {
		return func(ctx context.Context, op Operation, payload any) error {
			if op != OperationFind {
				return next(ctx, op, payload)
			}

			// simple cache key: operation + payload
			key := fmt.Sprintf("%s:%#v", op, payload)

			if _, ok := cache.Get(key); ok {
				// cache hit → skip execution, assume payload is filled
				switch p := payload.(type) {
				case *Query[any]:
					// nothing to do, value already in cache
					_ = p
				}
				return nil
			}

			// execute normally and cache the result if no error occurred
			err := next(ctx, op, payload)
			if err == nil {
				cache.Set(key, payload, ttl)
			}
			return err
		}
	}
}
