package golem

import "fmt"

// LogLevel represents the severity of a lifecycle log event.
type LogLevel int

// The four supported log levels, in increasing severity order.
const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// String returns the lowercase name of the level. It never panics: unknown
// values fall back to a safe "unknown(N)" representation.
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "debug"
	case LogLevelInfo:
		return "info"
	case LogLevelWarn:
		return "warn"
	case LogLevelError:
		return "error"
	default:
		return fmt.Sprintf("unknown(%d)", int(l))
	}
}

// Logger is the pluggable lifecycle-event logging interface. Implementations
// receive a message and structured args.
type Logger interface {
	Debug(msg string, args map[string]any)
	Info(msg string, args map[string]any)
	Warn(msg string, args map[string]any)
	Error(msg string, args map[string]any)
}

// defaultLogger is the built-in Logger used when no custom Logger is
// configured. It prints to stdout via fmt.Println.
type defaultLogger struct{}

var _ Logger = (*defaultLogger)(nil)

func (defaultLogger) Debug(msg string, args map[string]any) {
	fmt.Println("[DEBUG]", msg, args)
}

func (defaultLogger) Info(msg string, args map[string]any) {
	fmt.Println("[INFO]", msg, args)
}

func (defaultLogger) Warn(msg string, args map[string]any) {
	fmt.Println("[WARN]", msg, args)
}

func (defaultLogger) Error(msg string, args map[string]any) {
	fmt.Println("[ERROR]", msg, args)
}

// DefaultLogger returns the built-in console Logger used when no custom
// Logger is configured. Exposed so adapter packages (postgres, and later
// mysql/mssql/oracle) can fall back to it explicitly.
func DefaultLogger() Logger {
	return defaultLogger{}
}
