package postgres

import "testing"

func TestBuildInsertSQL_QuotesIdentifiersAndOrdersPlaceholders(t *testing.T) {
	got := buildInsertSQL("users", []string{"name", "email"})
	want := `INSERT INTO "users" ("name","email") VALUES ($1,$2) RETURNING *`

	if got != want {
		t.Fatalf("buildInsertSQL() = %q, want %q", got, want)
	}
}

func TestBuildInsertSQL_SingleColumn(t *testing.T) {
	got := buildInsertSQL("posts", []string{"title"})
	want := `INSERT INTO "posts" ("title") VALUES ($1) RETURNING *`

	if got != want {
		t.Fatalf("buildInsertSQL() = %q, want %q", got, want)
	}
}

func TestBuildSelectSQL_NoWhereColumns(t *testing.T) {
	got := buildSelectSQL("users", nil)
	want := `SELECT * FROM "users"`

	if got != want {
		t.Fatalf("buildSelectSQL() = %q, want %q", got, want)
	}
}

func TestBuildSelectSQL_WithWhereColumns(t *testing.T) {
	got := buildSelectSQL("users", []string{"name", "email"})
	want := `SELECT * FROM "users" WHERE "name"=$1 AND "email"=$2`

	if got != want {
		t.Fatalf("buildSelectSQL() = %q, want %q", got, want)
	}
}

func TestBuildUpdateSQL_SingleSetAndWhereColumn(t *testing.T) {
	got := buildUpdateSQL("users", []string{"name"}, []string{"id"})
	want := `UPDATE "users" SET "name"=$1 WHERE "id"=$2 RETURNING *`

	if got != want {
		t.Fatalf("buildUpdateSQL() = %q, want %q", got, want)
	}
}

func TestBuildUpdateSQL_MultipleSetAndWhereColumns(t *testing.T) {
	got := buildUpdateSQL("users", []string{"name", "email"}, []string{"id", "org_id"})
	want := `UPDATE "users" SET "name"=$1,"email"=$2 WHERE "id"=$3 AND "org_id"=$4 RETURNING *`

	if got != want {
		t.Fatalf("buildUpdateSQL() = %q, want %q", got, want)
	}
}

func TestBuildUpdateSQL_NoWhereColumns(t *testing.T) {
	got := buildUpdateSQL("users", []string{"name"}, nil)
	want := `UPDATE "users" SET "name"=$1 RETURNING *`

	if got != want {
		t.Fatalf("buildUpdateSQL() = %q, want %q", got, want)
	}
}
