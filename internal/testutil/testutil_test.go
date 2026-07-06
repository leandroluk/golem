package testutil

import (
	"errors"
	"testing"
)

type mockTB struct {
	testing.TB
	fatalfCalled bool
	errorfCalled bool
	helperCalled bool
}

func (m *mockTB) Fatalf(format string, args ...any) {
	m.fatalfCalled = true
}

func (m *mockTB) Errorf(format string, args ...any) {
	m.errorfCalled = true
}

func (m *mockTB) Helper() {
	m.helperCalled = true
}

func TestFatalIfError(t *testing.T) {
	m := &mockTB{}
	FatalIfError(m, nil, "should not fail")
	if m.fatalfCalled {
		t.Error("expected Fatalf not to be called")
	}

	m = &mockTB{}
	FatalIfError(m, errors.New("err"), "should fail")
	if !m.fatalfCalled {
		t.Error("expected Fatalf to be called")
	}
	if !m.helperCalled {
		t.Error("expected Helper to be called")
	}
}

func TestFatalIf(t *testing.T) {
	m := &mockTB{}
	FatalIf(m, false, "should not fail")
	if m.fatalfCalled {
		t.Error("expected Fatalf not to be called")
	}

	m = &mockTB{}
	FatalIf(m, true, "should fail")
	if !m.fatalfCalled {
		t.Error("expected Fatalf to be called")
	}
	if !m.helperCalled {
		t.Error("expected Helper to be called")
	}
}

func TestErrorIf(t *testing.T) {
	m := &mockTB{}
	ErrorIf(m, false, "should not fail")
	if m.errorfCalled {
		t.Error("expected Errorf not to be called")
	}

	m = &mockTB{}
	ErrorIf(m, true, "should fail")
	if !m.errorfCalled {
		t.Error("expected Errorf to be called")
	}
	if !m.helperCalled {
		t.Error("expected Helper to be called")
	}
}
