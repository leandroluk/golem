package golem

import (
	"testing"
)

func TestLogLevel_String_KnownLevels(t *testing.T) {
	cases := []struct {
		level LogLevel
		want  string
	}{
		{LogLevelDebug, "debug"},
		{LogLevelInfo, "info"},
		{LogLevelWarn, "warn"},
		{LogLevelError, "error"},
	}

	for _, c := range cases {
		got := c.level.String()
		if got != c.want {
			t.Errorf("LogLevel(%d).String() = %q, want %q", int(c.level), got, c.want)
		}
	}
}

func TestLogLevel_String_OutOfRange_DoesNotPanicAndReturnsFallback(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("LogLevel.String() panicked on out-of-range value: %v", r)
		}
	}()

	got := LogLevel(99).String()
	if got == "" {
		t.Errorf("LogLevel(99).String() returned empty string, want non-empty fallback")
	}
}

func TestDefaultLogger_ImplementsLogger(t *testing.T) {
	var _ Logger = defaultLogger{}
	var _ Logger = (*defaultLogger)(nil)
}

func TestDefaultLogger_MethodsDoNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("defaultLogger method panicked: %v", r)
		}
	}()

	l := defaultLogger{}
	args := map[string]any{"key": "value"}

	l.Debug("debug message", args)
	l.Info("info message", args)
	l.Warn("warn message", args)
	l.Error("error message", args)

	// Also verify nil args map doesn't panic.
	l.Debug("no args", nil)
	l.Info("no args", nil)
	l.Warn("no args", nil)
	l.Error("no args", nil)
}

