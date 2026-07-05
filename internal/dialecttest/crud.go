package dialecttest

import (
	"context"
	"testing"
	"time"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/op"
	"github.com/leandroluk/golem/query"
	"github.com/leandroluk/golem/repository"
)

// runBindScanRoundTrip proves every golem.ColumnType kind used by Widget
// binds and scans without error and comes back recognizable. Timestamp
// fields are compared loosely (date/time-of-day components, not full
// precision) since dialects vary in stored precision — the point is "no
// error, value survives," not byte-for-byte round-tripping.
func runBindScanRoundTrip(t *testing.T, ctx context.Context, ds *golem.DataSource, _ Schema) {
	repo := repository.Get(ds, widgetEntity)

	born := time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC)
	seen := time.Date(2020, 6, 15, 10, 30, 0, 0, time.UTC)
	// A fixed non-zero reference date: some drivers (go-sql-driver/mysql)
	// reject year 0/1 outright when binding a time.Time for a TIME column,
	// even though only the time-of-day part is ever stored.
	duration := time.Date(2000, 1, 1, 8, 15, 0, 0, time.UTC)

	in := &Widget{
		Name:     "widget-1",
		Bio:      "a longer text field",
		Category: "gadgets",
		Score:    42,
		Price:    19.99,
		Ratio:    0.5,
		Active:   true,
		Code:     "ABCDEFGH",
		Born:     born,
		Seen:     seen,
		Duration: duration,
		Data:     []byte{0x01, 0x02, 0x03},
		UID:      "123e4567-e89b-12d3-a456-426614174000",
		Meta:     `{"key":"value"}`,
	}

	created, err := repo.Insert(ctx, in)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	found, err := repo.FindOne(ctx, func(w *Widget, q *query.Query[Widget]) {
		q.Where(op.Eq(&w.ID, created.ID))
	})
	if err != nil {
		t.Fatalf("FindOne: %v", err)
	}

	if found.Name != in.Name {
		t.Errorf("Name = %q, want %q", found.Name, in.Name)
	}
	if found.Bio != in.Bio {
		t.Errorf("Bio = %q, want %q", found.Bio, in.Bio)
	}
	if found.Score != in.Score {
		t.Errorf("Score = %d, want %d", found.Score, in.Score)
	}
	if found.Price != in.Price {
		t.Errorf("Price = %v, want %v", found.Price, in.Price)
	}
	if found.Active != in.Active {
		t.Errorf("Active = %v, want %v", found.Active, in.Active)
	}
	if found.Born.Year() != born.Year() || found.Born.Month() != born.Month() || found.Born.Day() != born.Day() {
		t.Errorf("Born = %v, want date %v", found.Born, born)
	}
	if found.Seen.Year() != seen.Year() || found.Seen.Month() != seen.Month() || found.Seen.Day() != seen.Day() {
		t.Errorf("Seen = %v, want same date as %v", found.Seen, seen)
	}
	if found.Duration.Hour() != duration.Hour() || found.Duration.Minute() != duration.Minute() {
		t.Errorf("Duration = %v, want time-of-day %v", found.Duration, duration)
	}
	if len(found.Data) != len(in.Data) {
		t.Errorf("Data = %v, want %v", found.Data, in.Data)
	}
	if found.UID == "" {
		t.Error("UID: expected non-empty round-tripped value")
	}
	if found.Meta == "" {
		t.Error("Meta: expected non-empty round-tripped value")
	}
}

// runCRUD exercises Repository[T]'s full CRUD surface against Widget (a
// hard-delete entity — no DeleteDate).
func runCRUD(t *testing.T, ctx context.Context, ds *golem.DataSource, _ Schema) {
	repo := repository.Get(ds, widgetEntity)

	// Every field is populated, even ones this group isn't specifically
	// testing (BindScanRoundTrip covers that): SaveOne below re-persists
	// every column including zero values (unlike Insert), and several
	// column kinds reject Go's zero value outright where NULL would
	// otherwise be fine — "" isn't valid UUID/JSON syntax, and Go's zero
	// time.Time (year 1) is before MySQL's minimum representable DATE/
	// DATETIME/TIME year.
	w, err := repo.Insert(ctx, &Widget{
		Name: "crud-1", Category: "tools", Score: 1,
		Born:     time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		Seen:     time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC),
		Duration: time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC),
		Data:     []byte{0x01},
		UID:      "00000000-0000-0000-0000-000000000001",
		Meta:     "{}",
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if w.ID == 0 {
		t.Fatal("Insert: expected non-zero ID")
	}

	inserted, err := repo.InsertMany(ctx,
		&Widget{Name: "crud-2", Category: "tools", Score: 2},
		&Widget{Name: "crud-3", Category: "tools", Score: 3},
	)
	if err != nil {
		t.Fatalf("InsertMany: %v", err)
	}
	if len(inserted) != 2 {
		t.Fatalf("InsertMany: expected 2 rows, got %d", len(inserted))
	}

	w.Score = 100
	saved, err := repo.SaveOne(ctx, &w)
	if err != nil {
		t.Fatalf("SaveOne: %v", err)
	}
	if saved.Score != 100 {
		t.Errorf("SaveOne: Score = %d, want 100", saved.Score)
	}

	updated, err := repo.Update(ctx, func(wg *Widget, u *query.Update[Widget]) {
		u.Where(op.Eq(&wg.Category, "tools"))
		u.Set(&wg.Category, "hardware")
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(updated) != 3 {
		t.Errorf("Update: expected 3 rows matched, got %d", len(updated))
	}

	found, err := repo.FindOne(ctx, func(wg *Widget, q *query.Query[Widget]) {
		q.Where(op.Eq(&wg.ID, w.ID))
	})
	if err != nil {
		t.Fatalf("FindOne: %v", err)
	}
	if found.Category != "hardware" {
		t.Errorf("FindOne: Category = %q, want hardware", found.Category)
	}

	many, err := repo.FindMany(ctx, func(wg *Widget, q *query.Query[Widget]) {
		q.Where(op.Eq(&wg.Category, "hardware"))
		q.OrderBy(op.Asc(&wg.Score))
		q.Limit(10)
		q.Offset(0)
	})
	if err != nil {
		t.Fatalf("FindMany: %v", err)
	}
	if len(many) != 3 {
		t.Errorf("FindMany: expected 3 rows, got %d", len(many))
	}

	count, err := repo.Count(ctx, func(wg *Widget, c *query.Count[Widget]) {
		c.Where(op.Eq(&wg.Category, "hardware"))
	})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 3 {
		t.Errorf("Count = %d, want 3", count)
	}

	exists, err := repo.Exists(ctx, func(wg *Widget, c *query.Count[Widget]) {
		c.Where(op.Eq(&wg.ID, w.ID))
	})
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("Exists: expected true")
	}

	if err := repo.Delete(ctx, &found); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	stillExists, err := repo.Exists(ctx, func(wg *Widget, c *query.Count[Widget]) {
		c.Where(op.Eq(&wg.ID, w.ID))
	})
	if err != nil {
		t.Fatalf("Exists after delete: %v", err)
	}
	if stillExists {
		t.Error("Exists after Delete: expected false (Widget has no DeleteDate, Delete is a hard delete)")
	}
}
