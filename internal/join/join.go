// Package join provides generic builder functions (Inner, Left, Right, Full)
// to declare SQL JOIN clauses within query.Query callbacks.
package join

import (
	"github.com/leandroluk/golem/internal/entity"
	"github.com/leandroluk/golem/internal/query"
)

// Inner registers an INNER JOIN on the parent query.
func Inner[T, J any](q *query.Query[T], target *entity.Entity[J], fn func(j *J, q1 *query.Join[J])) {
	var zero J
	jBuilder := query.NewJoin[J]()
	fn(&zero, jBuilder)

	meta := target.Describe()
	f2c := make(map[string]string, len(meta.Columns))
	for _, col := range meta.Columns {
		f2c[col.FieldName] = col.Name
	}

	q.AddJoinData(query.JoinData{
		Type:            "inner",
		TableName:       meta.TableName,
		Zero:            &zero,
		OnFieldPairs:    jBuilder.Ons(),
		Conditions:      jBuilder.Conditions(),
		WithDeleted:     jBuilder.IsWithDeleted(),
		DeleteDateField: meta.DeleteDateField,
		FieldToColumn:   f2c,
	})
}

// Left registers a LEFT JOIN on the parent query.
func Left[T, J any](q *query.Query[T], target *entity.Entity[J], fn func(j *J, q1 *query.Join[J])) {
	var zero J
	jBuilder := query.NewJoin[J]()
	fn(&zero, jBuilder)

	meta := target.Describe()
	f2c := make(map[string]string, len(meta.Columns))
	for _, col := range meta.Columns {
		f2c[col.FieldName] = col.Name
	}

	q.AddJoinData(query.JoinData{
		Type:            "left",
		TableName:       meta.TableName,
		Zero:            &zero,
		OnFieldPairs:    jBuilder.Ons(),
		Conditions:      jBuilder.Conditions(),
		WithDeleted:     jBuilder.IsWithDeleted(),
		DeleteDateField: meta.DeleteDateField,
		FieldToColumn:   f2c,
	})
}

// Right registers a RIGHT JOIN on the parent query.
func Right[T, J any](q *query.Query[T], target *entity.Entity[J], fn func(j *J, q1 *query.Join[J])) {
	var zero J
	jBuilder := query.NewJoin[J]()
	fn(&zero, jBuilder)

	meta := target.Describe()
	f2c := make(map[string]string, len(meta.Columns))
	for _, col := range meta.Columns {
		f2c[col.FieldName] = col.Name
	}

	q.AddJoinData(query.JoinData{
		Type:            "right",
		TableName:       meta.TableName,
		Zero:            &zero,
		OnFieldPairs:    jBuilder.Ons(),
		Conditions:      jBuilder.Conditions(),
		WithDeleted:     jBuilder.IsWithDeleted(),
		DeleteDateField: meta.DeleteDateField,
		FieldToColumn:   f2c,
	})
}

// Full registers a FULL JOIN on the parent query.
func Full[T, J any](q *query.Query[T], target *entity.Entity[J], fn func(j *J, q1 *query.Join[J])) {
	var zero J
	jBuilder := query.NewJoin[J]()
	fn(&zero, jBuilder)

	meta := target.Describe()
	f2c := make(map[string]string, len(meta.Columns))
	for _, col := range meta.Columns {
		f2c[col.FieldName] = col.Name
	}

	q.AddJoinData(query.JoinData{
		Type:            "full",
		TableName:       meta.TableName,
		Zero:            &zero,
		OnFieldPairs:    jBuilder.Ons(),
		Conditions:      jBuilder.Conditions(),
		WithDeleted:     jBuilder.IsWithDeleted(),
		DeleteDateField: meta.DeleteDateField,
		FieldToColumn:   f2c,
	})
}
