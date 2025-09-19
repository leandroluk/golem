// Package core provides the fundamental building blocks of the golem ORM.
// This file defines the Model[T], which represents the entry point for working
// with a specific schema (entity). A Model handles persistence, queries,
// relations, hooks, soft-deletes, and event emission.
package core

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

// Model represents a repository-like abstraction for a schema T.
//
// It wraps a SchemaMeta[T] and a Driver, exposing high-level operations such as
// Create, Update, Delete, FindOne, FindMany, and Count. Models are generic
// and type-safe, ensuring that all operations are tied to a specific entity type.
type Model[T any] struct {
	schema *SchemaMeta[T]
	driver Driver
}

// NewModel creates a new Model instance bound to a schema and driver.
//
// Example:
//
//	userModel := core.NewModel(userSchema, postgresDriver)
func NewModel[T any](schema *SchemaMeta[T], driver Driver) *Model[T] {
	return &Model[T]{schema: schema, driver: driver}
}

// LoadRelation explicitly loads one or more relations into a given document.
//
// It accepts pointers to struct fields that represent relations and resolves
// their values from the database.
//
// Example:
//
//	var user User
//	_ = userModel.LoadRelation(ctx, &user, &user.Profile, &user.Roles)
func (m *Model[T]) LoadRelation(ctx context.Context, doc *T, fieldPtrs ...any) error {
	value := reflect.ValueOf(doc).Elem()

	for _, ptr := range fieldPtrs {
		rv := reflect.ValueOf(ptr)
		if rv.Kind() != reflect.Pointer {
			return fmt.Errorf("LoadRelation: argument must be a pointer to a field")
		}

		// discover the struct field name that matches the pointer
		fieldName := ""
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			if field.Addr().Interface() == ptr {
				fieldName = value.Type().Field(i).Name
				break
			}
		}
		if fieldName == "" {
			return fmt.Errorf("LoadRelation: field not found for pointer %v", ptr)
		}

		if err := m.loadRelationList(ctx, doc, []string{fieldName}); err != nil {
			return err
		}
	}
	return nil
}

// withSoftDelete applies soft-delete filtering rules to a query.
// It automatically excludes deleted records unless WithDeleted or OnlyDeleted
// flags are set in the query options.
func (m *Model[T]) withSoftDelete(where *Where) *Where {
	if where == nil || m.schema.deletedAtField == nil {
		return where
	}
	eff := *where // shallow copy
	col := m.schema.deletedAtField.DatabaseColumnName

	if where.OnlyDeleted {
		eff.Condition = foldConditionsAnd(
			where.Condition,
			(&Condition{FieldName: col}).Nil().Not(),
		)
		return &eff
	}
	if !where.WithDeleted {
		eff.Condition = foldConditionsAnd(
			where.Condition,
			(&Condition{FieldName: col}).Nil(),
		)
	}
	return &eff
}

// loadRelationList resolves and loads the specified relations into a document.
//
// It supports OneToOne, OneToMany, and ManyToMany relations by automatically
// issuing additional queries against the related models.
func (m *Model[T]) loadRelationList(ctx context.Context, doc *T, nameList []string) error {
	value := reflect.ValueOf(doc).Elem()

	for _, relationName := range nameList {
		relation := m.schema.findRelation(relationName)
		if relation == nil {
			continue
		}

		field := value.FieldByName(relation.FieldName)
		if !field.IsValid() || !field.CanSet() {
			continue
		}

		switch relation.Kind {
		case OneToOne:
			localVal := value.FieldByName(relation.LocalKey).Interface()
			qb := NewQuery(relation.RefSchema)
			qb.Filter(func(_ Filter[any]) []*Condition {
				return []*Condition{
					(&Condition{FieldName: relation.ForeignKey}).Eq(localVal),
				}
			})
			model := NewModel(relation.RefSchema, m.driver)
			result, err := model.findOneInternal(ctx, qb)
			if err != nil {
				return err
			}
			if result != nil {
				field.Set(reflect.ValueOf(result))
			}

		case OneToMany:
			localVal := value.FieldByName(relation.LocalKey).Interface()
			qb := NewQuery(relation.RefSchema)
			qb.Filter(func(_ Filter[any]) []*Condition {
				return []*Condition{
					(&Condition{FieldName: relation.ForeignKey}).Eq(localVal),
				}
			})
			model := NewModel(relation.RefSchema, m.driver)
			results, err := model.findManyInternal(ctx, qb)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(results))

		case ManyToMany:
			localVal := value.FieldByName(relation.LocalKey).Interface()

			// 1) fetch join table rows where JoinLocalKey = localVal
			joinQuery := &Where{
				Condition: (&Condition{FieldName: relation.JoinLocalKey}).Eq(localVal),
			}
			rawJoin, err := m.driver.FindMany(ctx, &SchemaCore{Collection: relation.JoinTable}, joinQuery)
			if err != nil {
				return err
			}
			joinRows, _ := rawJoin.([]map[string]any)

			// 2) extract foreign IDs
			foreignIDs := make([]any, 0, len(joinRows))
			for _, jr := range joinRows {
				foreignIDs = append(foreignIDs, jr[relation.JoinForeignKey])
			}

			// 3) fetch related entities by IN condition
			if len(foreignIDs) > 0 {
				qb := NewQuery(relation.RefSchema)
				qb.Filter(func(_ Filter[any]) []*Condition {
					return []*Condition{
						(&Condition{FieldName: relation.ForeignKey}).In(foreignIDs...),
					}
				})
				model := NewModel(relation.RefSchema, m.driver)
				results, err := model.findManyInternal(ctx, qb)
				if err != nil {
					return err
				}
				field.Set(reflect.ValueOf(results))
			}
		}
	}

	return nil
}

// WithTenant creates a new Model[T] instance bound to a different database.
//
// It clones the schema and replaces only the Database name in SchemaCore.
// This is useful for multi-tenant or sharded architectures.
func (m *Model[T]) WithTenant(database string) *Model[T] {
	cloneSchema := *m.schema
	cloneCore := cloneSchema.SchemaCore
	cloneCore.Database = database
	cloneSchema.SchemaCore = cloneCore

	return &Model[T]{schema: &cloneSchema, driver: m.driver}
}

// runPre executes all registered PreHooks for the given operation.
func (m *Model[T]) runPre(hook PreHook, doc *T) error {
	if fnList, ok := m.schema.PreHookList[hook]; ok {
		for _, fn := range fnList {
			if err := fn(doc); err != nil {
				return err
			}
		}
	}
	return nil
}

// runPost executes all registered PostHooks for the given operation.
func (m *Model[T]) runPost(hook PostHook, doc *T) error {
	if fnList, ok := m.schema.PostHookList[hook]; ok {
		for _, fn := range fnList {
			if err := fn(doc); err != nil {
				return err
			}
		}
	}
	return nil
}

// Create inserts a new entity into the database.
//
// It automatically sets createdAt and updatedAt fields (if defined in the schema),
// executes PreInsert hooks, performs the insert via the driver, executes PostInsert hooks,
// and emits an EventInsert.
func (m *Model[T]) Create(ctx context.Context, doc *T) error {
	return dispatchOperation(ctx, OperationInsert, doc, func() error {
		now := time.Now()
		val := reflect.ValueOf(doc).Elem()

		if m.schema.createdAtField != nil {
			f := val.FieldByName(m.schema.createdAtField.StructFieldName)
			setTimeField(f, now)
		}
		if m.schema.updatedAtField != nil {
			f := val.FieldByName(m.schema.updatedAtField.StructFieldName)
			setTimeField(f, now)
		}

		if err := m.runPre(PreInsert, doc); err != nil {
			return err
		}
		if err := m.driver.Insert(ctx, &m.schema.SchemaCore, doc); err != nil {
			return err
		}
		if err := m.runPost(PostInsert, doc); err != nil {
			return err
		}
		Emit(EventInsert, InsertPayload[T]{Schema: &m.schema.SchemaCore, Doc: doc})
		return nil
	})
}

// findOneInternal is the internal implementation of FindOne queries.
//
// It executes PreFind hooks, applies soft-delete rules, executes the driver query,
// maps the result to the struct, loads relations if requested, executes PostFind hooks,
// and emits an EventFind.
func (m *Model[T]) findOneInternal(ctx context.Context, qb *Query[T], relationList ...string) (*T, error) {
	var zero T
	_ = m.runPre(PreFind, &zero)

	where := m.withSoftDelete(qb.where)

	var result *T
	err := dispatchOperation(ctx, OperationFind, qb, func() error {
		raw, err := m.driver.FindOne(ctx, &m.schema.SchemaCore, where)
		if err != nil || raw == nil {
			return err
		}
		row, ok := raw.(map[string]any)
		if !ok {
			return nil
		}
		value := new(T)
		if err := mapToStruct(row, value); err != nil {
			return err
		}
		if err := m.loadRelationList(ctx, value, relationList); err != nil {
			return err
		}
		_ = m.runPost(PostFind, value)
		Emit(EventFind, FindOnePayload[T]{Schema: &m.schema.SchemaCore, Where: where, Doc: value})
		result = value
		return nil
	})
	return result, err
}

// findManyInternal is the internal implementation of FindMany queries.
//
// It executes PreFind hooks, applies soft-delete rules, executes the driver query,
// maps the results to structs, loads relations if requested, executes PostFind hooks,
// and emits an EventFind.
func (m *Model[T]) findManyInternal(ctx context.Context, qb *Query[T], relationList ...string) ([]T, error) {
	var zero T
	_ = m.runPre(PreFind, &zero)

	where := m.withSoftDelete(qb.where)

	var results []T
	err := dispatchOperation(ctx, OperationFind, qb, func() error {
		raw, err := m.driver.FindMany(ctx, &m.schema.SchemaCore, where)
		if err != nil || raw == nil {
			return err
		}
		rows, ok := raw.([]map[string]any)
		if !ok {
			return nil
		}
		for _, row := range rows {
			value := new(T)
			if err := mapToStruct(row, value); err != nil {
				return err
			}
			if err := m.loadRelationList(ctx, value, relationList); err != nil {
				return err
			}
			_ = m.runPost(PostFind, value)
			results = append(results, *value)
		}
		Emit(EventFind, FindManyPayload[T]{Schema: &m.schema.SchemaCore, Where: where, DocList: results})
		return nil
	})
	return results, err
}

// Update applies changes to entities matching a condition.
//
// It automatically updates the updatedAt field (if defined in the schema),
// performs the update via the driver, and emits an EventUpdate.
func (m *Model[T]) Update(ctx context.Context, condition *Condition, changes Changes) error {
	return dispatchOperation(ctx, OperationUpdate, changes, func() error {
		if m.schema.updatedAtField != nil {
			changes[m.schema.updatedAtField.DatabaseColumnName] = time.Now()
		}
		if err := m.driver.Update(ctx, &m.schema.SchemaCore, condition, changes); err != nil {
			return err
		}
		Emit(EventUpdate, UpdatePayload{Schema: &m.schema.SchemaCore, Condition: condition, Changes: changes})
		return nil
	})
}

// Delete removes entities matching a condition.
//
// If soft-delete is enabled (deletedAt field exists), it sets the deletedAt timestamp
// instead of physically removing the record. Otherwise, it delegates to the driver’s Delete.
// An EventUpdate or EventDelete is emitted depending on the strategy used.
func (m *Model[T]) Delete(ctx context.Context, condition *Condition) error {
	return dispatchOperation(ctx, OperationDelete, condition, func() error {
		if m.schema.deletedAtField != nil {
			changes := Changes{m.schema.deletedAtField.DatabaseColumnName: time.Now()}
			if err := m.driver.Update(ctx, &m.schema.SchemaCore, condition, changes); err != nil {
				return err
			}
			Emit(EventUpdate, UpdatePayload{Schema: &m.schema.SchemaCore, Condition: condition, Changes: changes})
			return nil
		}
		if err := m.driver.Delete(ctx, &m.schema.SchemaCore, condition); err != nil {
			return err
		}
		Emit(EventDelete, DeletePayload{Schema: &m.schema.SchemaCore, Condition: condition})
		return nil
	})
}

// Count returns the number of entities matching the query.
//
// It applies soft-delete rules automatically and delegates counting to the driver.
func (m *Model[T]) Count(ctx context.Context, qb *Query[T]) (int64, error) {
	where := m.withSoftDelete(qb.where)
	var count int64
	err := dispatchOperation(ctx, OperationFind, qb, func() error {
		var err error
		count, err = m.driver.Count(ctx, &m.schema.SchemaCore, where.Condition)
		return err
	})
	return count, err
}
