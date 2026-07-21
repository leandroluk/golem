package dialecttest

import (
	"context"
	"testing"
	"time"

	"github.com/leandroluk/golem"
	"github.com/leandroluk/golem/internal/testutil"
	"github.com/leandroluk/golem/internal/op"
	"github.com/leandroluk/golem/internal/query"
	"github.com/leandroluk/golem/internal/repository"
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
	testutil.FatalIfError(t, err, "Insert")

	found, err := repo.FindOne(ctx, func(w *Widget, q *query.Query[Widget]) {
		q.Where(op.Eq(&w.ID, created.ID))
	})
	testutil.FatalIfError(t, err, "FindOne")

	testutil.ErrorIf(t, found.Name != in.Name, "Name = %q, want %q", found.Name, in.Name)
	testutil.ErrorIf(t, found.Bio != in.Bio, "Bio = %q, want %q", found.Bio, in.Bio)
	testutil.ErrorIf(t, found.Score != in.Score, "Score = %d, want %d", found.Score, in.Score)
	testutil.ErrorIf(t, found.Price != in.Price, "Price = %v, want %v", found.Price, in.Price)
	testutil.ErrorIf(t, found.Active != in.Active, "Active = %v, want %v", found.Active, in.Active)
	testutil.ErrorIf(t, found.Born.Year() != born.Year() || found.Born.Month() != born.Month() || found.Born.Day() != born.Day(), "Born = %v, want date %v", found.Born, born)
	testutil.ErrorIf(t, found.Seen.Year() != seen.Year() || found.Seen.Month() != seen.Month() || found.Seen.Day() != seen.Day(), "Seen = %v, want same date as %v", found.Seen, seen)
	testutil.ErrorIf(t, found.Duration.Hour() != duration.Hour() || found.Duration.Minute() != duration.Minute(), "Duration = %v, want time-of-day %v", found.Duration, duration)
	testutil.ErrorIf(t, len(found.Data) != len(in.Data), "Data = %v, want %v", found.Data, in.Data)
	testutil.ErrorIf(t, found.UID == "", "UID: expected non-empty round-tripped value")
	testutil.ErrorIf(t, found.Meta == "", "Meta: expected non-empty round-tripped value")
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
	testutil.FatalIfError(t, err, "Insert")
	testutil.FatalIf(t, w.ID == 0, "Insert: expected non-zero ID")

	inserted, err := repo.InsertMany(ctx,
		&Widget{Name: "crud-2", Category: "tools", Score: 2},
		&Widget{Name: "crud-3", Category: "tools", Score: 3},
	)
	testutil.FatalIfError(t, err, "InsertMany")
	testutil.FatalIf(t, len(inserted) != 2, "InsertMany: expected 2 rows, got %d", len(inserted))

	w.Score = 100
	saved, err := repo.SaveOne(ctx, &w)
	testutil.FatalIfError(t, err, "SaveOne")
	testutil.ErrorIf(t, saved.Score != 100, "SaveOne: Score = %d, want 100", saved.Score)

	updated, err := repo.Update(ctx, func(wg *Widget, u *query.Update[Widget]) {
		u.Where(op.Eq(&wg.Category, "tools"))
		u.Set(&wg.Category, "hardware")
	})
	testutil.FatalIfError(t, err, "Update")
	testutil.ErrorIf(t, len(updated) != 3, "Update: expected 3 rows matched, got %d", len(updated))

	found, err := repo.FindOne(ctx, func(wg *Widget, q *query.Query[Widget]) {
		q.Where(op.Eq(&wg.ID, w.ID))
	})
	testutil.FatalIfError(t, err, "FindOne")
	testutil.ErrorIf(t, found.Category != "hardware", "FindOne: Category = %q, want hardware", found.Category)

	many, err := repo.FindMany(ctx, func(wg *Widget, q *query.Query[Widget]) {
		q.Where(op.Eq(&wg.Category, "hardware"))
		q.OrderBy(op.Asc(&wg.Score))
		q.Limit(10)
		q.Offset(0)
	})
	testutil.FatalIfError(t, err, "FindMany")
	testutil.ErrorIf(t, len(many) != 3, "FindMany: expected 3 rows, got %d", len(many))

	count, err := repo.Count(ctx, func(wg *Widget, c *query.Count[Widget]) {
		c.Where(op.Eq(&wg.Category, "hardware"))
	})
	testutil.FatalIfError(t, err, "Count")
	testutil.ErrorIf(t, count != 3, "Count = %d, want 3", count)

	exists, err := repo.Exists(ctx, func(wg *Widget, c *query.Count[Widget]) {
		c.Where(op.Eq(&wg.ID, w.ID))
	})
	testutil.FatalIfError(t, err, "Exists")
	testutil.ErrorIf(t, !exists, "Exists: expected true")

	err = repo.Delete(ctx, &found)
	testutil.FatalIfError(t, err, "Delete")
	stillExists, err := repo.Exists(ctx, func(wg *Widget, c *query.Count[Widget]) {
		c.Where(op.Eq(&wg.ID, w.ID))
	})
	testutil.FatalIfError(t, err, "Exists after delete")
	testutil.ErrorIf(t, stillExists, "Exists after Delete: expected false (Widget has no DeleteDate, Delete is a hard delete)")
}
