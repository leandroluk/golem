package must

import (
	"errors"
	"testing"
)

func TestValue(t *testing.T) {
	// Should not panic
	val := Value("success", nil)
	if val != "success" {
		t.Errorf("Expected 'success', got %v", val)
	}

	// Should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic, got nil")
		}
	}()
	Value("fail", errors.New("error"))
}

func TestExec(t *testing.T) {
	// Should not panic
	Exec(nil)

	// Should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic, got nil")
		}
	}()
	Exec(errors.New("error"))
}

func TestRecover(t *testing.T) {
	t.Run("Recover from error", func(t *testing.T) {
		var err error
		func() {
			defer Recover(&err)
			panic(errors.New("recovered error"))
		}()
		if err == nil || err.Error() != "recovered error" {
			t.Errorf("Expected 'recovered error', got %v", err)
		}
	})

	t.Run("Recover from non-error", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil || r != "string panic" {
				t.Errorf("Expected to repanic 'string panic', got %v", r)
			}
		}()
		var err error
		func() {
			defer Recover(&err)
			panic("string panic")
		}()
	})

	t.Run("No panic", func(t *testing.T) {
		var err error
		func() {
			defer Recover(&err)
		}()
		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
	})
}
