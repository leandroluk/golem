package main

import (
	"reflect"
	"testing"
)

func TestUserEntity_Describe(t *testing.T) {
	meta := UserEntity.Describe()

	if meta.TableName != "users" {
		t.Fatalf("TableName = %q, want %q", meta.TableName, "users")
	}

	wantCols := map[string]string{
		"ID":    "id",
		"Name":  "name",
		"Email": "email",
	}
	if len(meta.Columns) != len(wantCols) {
		t.Fatalf("len(Columns) = %d, want %d", len(meta.Columns), len(wantCols))
	}
	for _, c := range meta.Columns {
		want, ok := wantCols[c.FieldName]
		if !ok {
			t.Fatalf("unexpected FieldName %q", c.FieldName)
		}
		if c.Name != want {
			t.Fatalf("column %s.Name = %q, want %q", c.FieldName, c.Name, want)
		}
	}

	wantPK := []string{"id"}
	if !reflect.DeepEqual(meta.PrimaryKey, wantPK) {
		t.Fatalf("PrimaryKey = %v, want %v", meta.PrimaryKey, wantPK)
	}
}

func TestPostEntity_Describe(t *testing.T) {
	meta := PostEntity.Describe()

	if meta.TableName != "post" {
		t.Fatalf("TableName = %q, want %q", meta.TableName, "post")
	}

	wantCols := map[string]string{
		"ID":          "id",
		"OwnerUserID": "owner_user_id",
		"Title":       "title",
		"Content":     "content",
	}
	if len(meta.Columns) != len(wantCols) {
		t.Fatalf("len(Columns) = %d, want %d", len(meta.Columns), len(wantCols))
	}
	for _, c := range meta.Columns {
		want, ok := wantCols[c.FieldName]
		if !ok {
			t.Fatalf("unexpected FieldName %q", c.FieldName)
		}
		if c.Name != want {
			t.Fatalf("column %s.Name = %q, want %q", c.FieldName, c.Name, want)
		}
	}

	wantPK := []string{"id"}
	if !reflect.DeepEqual(meta.PrimaryKey, wantPK) {
		t.Fatalf("PrimaryKey = %v, want %v", meta.PrimaryKey, wantPK)
	}

	if len(meta.ForeignKeys) != 1 {
		t.Fatalf("len(ForeignKeys) = %d, want 1", len(meta.ForeignKeys))
	}
	if meta.ForeignKeys[0].FieldName != "OwnerUserID" {
		t.Fatalf("ForeignKeys[0].FieldName = %q, want %q", meta.ForeignKeys[0].FieldName, "OwnerUserID")
	}
}

func TestCategoryEntity_Describe(t *testing.T) {
	meta := CategoryEntity.Describe()

	if meta.TableName != "category" {
		t.Fatalf("TableName = %q, want %q", meta.TableName, "category")
	}

	wantCols := map[string]string{
		"ID":   "id",
		"Name": "name",
	}
	if len(meta.Columns) != len(wantCols) {
		t.Fatalf("len(Columns) = %d, want %d", len(meta.Columns), len(wantCols))
	}
	for _, c := range meta.Columns {
		want, ok := wantCols[c.FieldName]
		if !ok {
			t.Fatalf("unexpected FieldName %q", c.FieldName)
		}
		if c.Name != want {
			t.Fatalf("column %s.Name = %q, want %q", c.FieldName, c.Name, want)
		}
	}

	wantPK := []string{"id"}
	if !reflect.DeepEqual(meta.PrimaryKey, wantPK) {
		t.Fatalf("PrimaryKey = %v, want %v", meta.PrimaryKey, wantPK)
	}
}

func TestPostToCategoryEntity_Describe(t *testing.T) {
	meta := PostToCategoryEntity.Describe()

	if meta.TableName != "post_to_category" {
		t.Fatalf("TableName = %q, want %q", meta.TableName, "post_to_category")
	}

	wantCols := map[string]string{
		"PostID":     "post_id",
		"CategoryID": "category_id",
	}
	if len(meta.Columns) != len(wantCols) {
		t.Fatalf("len(Columns) = %d, want %d", len(meta.Columns), len(wantCols))
	}
	for _, c := range meta.Columns {
		want, ok := wantCols[c.FieldName]
		if !ok {
			t.Fatalf("unexpected FieldName %q", c.FieldName)
		}
		if c.Name != want {
			t.Fatalf("column %s.Name = %q, want %q", c.FieldName, c.Name, want)
		}
	}

	wantPK := []string{"post_id", "category_id"}
	if !reflect.DeepEqual(meta.PrimaryKey, wantPK) {
		t.Fatalf("PrimaryKey = %v, want %v", meta.PrimaryKey, wantPK)
	}

	if len(meta.ForeignKeys) != 2 {
		t.Fatalf("len(ForeignKeys) = %d, want 2", len(meta.ForeignKeys))
	}
	var gotPost, gotCategory bool
	for _, fk := range meta.ForeignKeys {
		switch fk.FieldName {
		case "PostID":
			gotPost = true
		case "CategoryID":
			gotCategory = true
		default:
			t.Fatalf("unexpected ForeignKey FieldName %q", fk.FieldName)
		}
	}
	if !gotPost || !gotCategory {
		t.Fatalf("expected ForeignKeys for both PostID and CategoryID, got %+v", meta.ForeignKeys)
	}
}

