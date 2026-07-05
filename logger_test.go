package golem

import "testing"

func TestLogger_DefaultLogger(t *testing.T) {
	l := DefaultLogger()
	if l == nil {
		t.Fatal("expected default logger, got nil")
	}
	// just call it to cover
	l.Debug("test", nil)
	l.Info("test", nil)
	l.Warn("test", nil)
	l.Error("test", nil)
}

func TestLogLevel_String(t *testing.T) {
	cases := []struct {
		level LogLevel
		want  string
	}{
		{LogLevelDebug, "debug"},
		{LogLevelInfo, "info"},
		{LogLevelWarn, "warn"},
		{LogLevelError, "error"},
		{LogLevel(99), "unknown(99)"},
	}
	for _, tc := range cases {
		if got := tc.level.String(); got != tc.want {
			t.Errorf("LogLevel(%d).String() = %q, want %q", tc.level, got, tc.want)
		}
	}
}
