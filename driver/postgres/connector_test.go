package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func TestConnector_Connect_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close()

	orig := newPgxPool
	defer func() { newPgxPool = orig }()
	newPgxPool = func(ctx context.Context, dsn string) (pgxPoolIface, error) {
		return mock, nil
	}

	mock.ExpectPing()

	opts := &Options{Logging: true, DSN: "postgres://user:pass@localhost:5432/db"}
	c := &connector{opts: opts}

	d, err := c.Connect()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if d == nil {
		t.Fatal("expected dialect, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}

	c.Close()
}

func TestConnector_Connect_ErrorResolveDSN(t *testing.T) {
	opts := &Options{DSN: "::::"} // invalid
	c := &connector{opts: opts}
	_, err := c.Connect()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConnector_Connect_ErrorNewPool(t *testing.T) {
	orig := newPgxPool
	defer func() { newPgxPool = orig }()
	newPgxPool = func(ctx context.Context, dsn string) (pgxPoolIface, error) {
		return nil, errors.New("new pool error")
	}

	opts := &Options{DSN: "postgres://user:pass@localhost:5432/db"}
	c := &connector{opts: opts}
	_, err := c.Connect()
	if err == nil || err.Error() != "postgres: connect: new pool error" {
		t.Fatalf("expected new pool error, got %v", err)
	}
}

func TestConnector_Connect_ErrorPing(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close()

	orig := newPgxPool
	defer func() { newPgxPool = orig }()
	newPgxPool = func(ctx context.Context, dsn string) (pgxPoolIface, error) {
		return mock, nil
	}

	mock.ExpectPing().WillReturnError(errors.New("ping error"))

	opts := &Options{Logging: true, DSN: "postgres://user:pass@localhost:5432/db"}
	c := &connector{opts: opts}
	_, err = c.Connect()
	if err == nil || err.Error() != "postgres: connect: ping error" {
		t.Fatalf("expected ping error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
