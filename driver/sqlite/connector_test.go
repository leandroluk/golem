package sqlite

import (
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestConnector_Connect_Success(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	mock.ExpectPing()

	orig := newSQLDB
	defer func() { newSQLDB = orig }()
	newSQLDB = func(dsn string) (sqlIface, error) { return db, nil }

	opts := &Options{Logging: true, Path: ":memory:"}
	c := &connector{opts: opts}

	d, err := c.Connect()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if d == nil {
		t.Fatal("expected dialect, got nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}

	c.Close()
}

func TestConnector_Connect_ErrorNewSQLDB(t *testing.T) {
	orig := newSQLDB
	defer func() { newSQLDB = orig }()
	newSQLDB = func(dsn string) (sqlIface, error) {
		return nil, errors.New("new db error")
	}

	opts := &Options{Path: ":memory:"}
	c := &connector{opts: opts}
	_, err := c.Connect()
	if err == nil || err.Error() != "sqlite: connect: new db error" {
		t.Fatalf("expected new db error, got %v", err)
	}
}

func TestConnector_Connect_ErrorPing(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()
	mock.ExpectPing().WillReturnError(errors.New("ping error"))

	orig := newSQLDB
	defer func() { newSQLDB = orig }()
	newSQLDB = func(dsn string) (sqlIface, error) { return db, nil }

	opts := &Options{Logging: true, Path: ":memory:"}
	c := &connector{opts: opts}
	_, err = c.Connect()
	if err == nil || err.Error() != "sqlite: connect: ping error" {
		t.Fatalf("expected ping error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestNewSQLDB_OpensLazily(t *testing.T) {
	// sql.Open never dials (lazy) — this is a pure unit test, no real I/O.
	db, err := newSQLDB(resolveDSN(&Options{}))
	if err != nil {
		t.Fatalf("newSQLDB: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil db")
	}
	db.Close()
}

func TestConnector_Close_NeverConnected_NoOp(t *testing.T) {
	c := &connector{opts: &Options{}}
	if err := c.Close(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
