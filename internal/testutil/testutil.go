package testutil

import "testing"

// FatalIfError fails the test if err is not nil.
func FatalIfError(t testing.TB, err error, msg string, args ...any) {
	t.Helper()
	if err != nil {
		t.Fatalf(msg+": %v", append(args, err)...)
	}
}

// FatalIf fails the test if cond is true.
func FatalIf(t testing.TB, cond bool, msg string, args ...any) {
	t.Helper()
	if cond {
		t.Fatalf(msg, args...)
	}
}

// ErrorIf fails the test non-fatally if cond is true.
func ErrorIf(t testing.TB, cond bool, msg string, args ...any) {
	t.Helper()
	if cond {
		t.Errorf(msg, args...)
	}
}
