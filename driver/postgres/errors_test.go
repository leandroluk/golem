package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/leandroluk/golem"
)

func TestMapError_Nil_ReturnsNil(t *testing.T) {
	if err := mapError(nil); err != nil {
		t.Fatalf("mapError(nil) = %v, want nil", err)
	}
}

func TestMapError_UniqueViolation_WrapsErrDuplicateKey(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}

	got := mapError(pgErr)

	if !errors.Is(got, golem.ErrDuplicateKey) {
		t.Fatalf("mapError(23505) = %v, want wrapped golem.ErrDuplicateKey", got)
	}
	var asPgErr *pgconn.PgError
	if !errors.As(got, &asPgErr) {
		t.Fatalf("mapError(23505) = %v, want original *pgconn.PgError reachable via errors.As", got)
	}
	if asPgErr.Code != "23505" {
		t.Fatalf("unwrapped PgError.Code = %q, want 23505", asPgErr.Code)
	}
}

func TestMapError_ForeignKeyViolation_WrapsErrForeignKeyViolation(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23503", Message: "violates foreign key constraint"}

	got := mapError(pgErr)

	if !errors.Is(got, golem.ErrForeignKeyViolation) {
		t.Fatalf("mapError(23503) = %v, want wrapped golem.ErrForeignKeyViolation", got)
	}
	var asPgErr *pgconn.PgError
	if !errors.As(got, &asPgErr) {
		t.Fatalf("mapError(23503) = %v, want original *pgconn.PgError reachable via errors.As", got)
	}
}

func TestMapError_UnmappedPgCode_PassesThroughUnchanged(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "42601", Message: "syntax error"}

	got := mapError(pgErr)

	if got != error(pgErr) {
		t.Fatalf("mapError(42601) = %v, want unchanged original error", got)
	}
	if errors.Is(got, golem.ErrDuplicateKey) || errors.Is(got, golem.ErrForeignKeyViolation) {
		t.Fatalf("mapError(42601) should not match any sentinel, got %v", got)
	}
}

func TestMapError_NonPgError_PassesThroughUnchanged(t *testing.T) {
	plain := errors.New("connection refused")

	got := mapError(plain)

	if got != plain {
		t.Fatalf("mapError(plain) = %v, want unchanged original error", got)
	}
}
