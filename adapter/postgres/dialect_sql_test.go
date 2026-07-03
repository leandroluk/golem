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

func TestBuildFindByIDSQL_QuotesIdentifiers(t *testing.T) {
	got := buildFindByIDSQL("users", "id")
	want := `SELECT * FROM "users" WHERE "id" = $1`

	if got != want {
		t.Fatalf("buildFindByIDSQL() = %q, want %q", got, want)
	}
}
