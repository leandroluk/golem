package scanner_test

import (
	"reflect"
	"testing"

	golem "github.com/leandroluk/golem/internal/core"
	"github.com/leandroluk/golem/internal/entity"
	"github.com/leandroluk/golem/internal/scanner"
)

type User struct {
	ID     int64
	Name   string
	Active bool
	Score  float64
}

func setupUserMeta() entity.EntityMeta {
	rUser := reflect.TypeFor[User]()
	return entity.EntityMeta{
		Columns: []entity.ColumnMeta{
			{Name: "id", FieldName: "ID", GoType: rUser.Field(0).Type, Offset: rUser.Field(0).Offset},
			{Name: "name", FieldName: "Name", GoType: rUser.Field(1).Type, Offset: rUser.Field(1).Offset},
			{Name: "active", FieldName: "Active", GoType: rUser.Field(2).Type, Offset: rUser.Field(2).Offset},
			{Name: "score", FieldName: "Score", GoType: rUser.Field(3).Type, Offset: rUser.Field(3).Offset},
		},
	}
}

func BenchmarkScanFromMap_AllPrimitives(b *testing.B) {
	meta := setupUserMeta()
	plan := scanner.Compile(meta, golem.DefaultParser)
	row := map[string]any{
		"id":     int64(100),
		"name":   "Test User",
		"active": true,
		"score":  float64(9.99),
	}

	b.ReportAllocs()

	for b.Loop() {
		_, err := scanner.ScanFromMap[User](plan, row)
		if err != nil {
			b.Fatal(err)
		}
	}
}
