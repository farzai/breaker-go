// Package logging provides a comprehensive, structured logging system for the circuit breaker.
//
// This package implements several design patterns:
//   - Adapter Pattern: Support for multiple logging frameworks (zap, logrus, slog)
//   - Builder Pattern: Fluent logger configuration
//   - Strategy Pattern: Different output and formatting strategies
//   - Chain of Responsibility: Log filtering and processing
//
// Key Features:
//   - Structured logging with typed fields
//   - Log level filtering (Debug, Info, Warn, Error)
//   - Contextual logging with field inheritance
//   - Zero-allocation in hot paths (when possible)
//   - Thread-safe operations
//   - Easy integration with popular logging frameworks
//
// Example Usage:
//
//	logger := logging.NewDefaultLogger(logging.LevelInfo)
//	logger.Info("Circuit breaker state changed",
//	    logging.String("from", "Closed"),
//	    logging.String("to", "Open"),
//	    logging.Int("failures", 5),
//	)
//
//	// Contextual logging
//	reqLogger := logger.With(
//	    logging.String("request_id", "abc123"),
//	    logging.String("user_id", "user456"),
//	)
//	reqLogger.Error("Operation failed", logging.Error(err))
package logging

import (
	"context"
	"time"
)

// Level represents the severity level of a log message.
type Level int

const (
	// LevelDebug is for detailed diagnostic information, typically of interest
	// only when diagnosing problems. Should not be used in production.
	LevelDebug Level = iota

	// LevelInfo is for general informational messages that highlight the
	// progress of the application at a coarse-grained level.
	LevelInfo

	// LevelWarn is for potentially harmful situations that should be
	// investigated but don't prevent the application from functioning.
	LevelWarn

	// LevelError is for error events that might still allow the application
	// to continue running but indicate a failure in some operation.
	LevelError

	// LevelOff disables all logging.
	LevelOff
)

// String returns the string representation of the log level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelOff:
		return "OFF"
	default:
		return "UNKNOWN"
	}
}

// Field represents a structured logging field with a key and typed value.
// Fields provide type safety and better performance than string concatenation.
type Field struct {
	Key   string
	Type  FieldType
	Value interface{}
}

// FieldType represents the type of a field value for efficient serialization.
type FieldType int

const (
	// StringType represents a string value.
	StringType FieldType = iota
	// IntType represents an integer value.
	IntType
	// Int64Type represents a 64-bit integer value.
	Int64Type
	// Float64Type represents a floating-point value.
	Float64Type
	// BoolType represents a boolean value.
	BoolType
	// TimeType represents a time.Time value.
	TimeType
	// DurationType represents a time.Duration value.
	DurationType
	// ErrorType represents an error value.
	ErrorType
	// AnyType represents any other type.
	AnyType
)

// Logger is the primary interface for structured logging.
// All implementations must be thread-safe.
//
// Implementations should support:
//   - Structured logging with typed fields
//   - Log level filtering
//   - Contextual logging via With()
//   - Efficient field serialization
//
// Example:
//
//	logger.Info("User logged in",
//	    String("user_id", "123"),
//	    String("ip", "192.168.1.1"),
//	    Duration("login_time", time.Since(start)),
//	)
type Logger interface {
	// Debug logs a debug-level message with structured fields.
	// Debug messages are typically filtered out in production.
	Debug(msg string, fields ...Field)

	// Info logs an info-level message with structured fields.
	// Info messages highlight the progress of the application.
	Info(msg string, fields ...Field)

	// Warn logs a warning-level message with structured fields.
	// Warnings indicate potentially harmful situations.
	Warn(msg string, fields ...Field)

	// Error logs an error-level message with structured fields.
	// Errors indicate failure of some operation.
	Error(msg string, fields ...Field)

	// With creates a child logger with additional context fields.
	// The child logger inherits all fields from the parent and adds new ones.
	// This is useful for adding request-scoped or operation-scoped context.
	//
	// Example:
	//	reqLogger := logger.With(String("request_id", reqID))
	//	reqLogger.Info("Processing request")
	//	reqLogger.Error("Request failed", Error(err))
	With(fields ...Field) Logger

	// WithContext creates a child logger with context.
	// This allows propagating tracing information and cancellation.
	WithContext(ctx context.Context) Logger

	// Enabled returns true if the given level would be logged.
	// This allows avoiding expensive field computation when logging is disabled.
	//
	// Example:
	//	if logger.Enabled(LevelDebug) {
	//	    // Only compute expensive fields if debug is enabled
	//	    logger.Debug("Details", String("data", computeExpensiveData()))
	//	}
	Enabled(level Level) bool
}

// ContextLogger extends Logger with context awareness.
// This is useful for propagating request context, trace IDs, etc.
type ContextLogger interface {
	Logger

	// Context returns the context associated with this logger.
	Context() context.Context
}

// LeveledLogger is a logger that supports changing the minimum log level.
// This is useful for dynamically adjusting verbosity at runtime.
type LeveledLogger interface {
	Logger

	// SetLevel changes the minimum log level.
	// Messages below this level will be filtered out.
	SetLevel(level Level)

	// GetLevel returns the current minimum log level.
	GetLevel() Level
}

// FlushableLogger is a logger that supports flushing buffered logs.
// This is important for ensuring logs are written before shutdown.
type FlushableLogger interface {
	Logger

	// Flush ensures all buffered logs are written to their destination.
	// This should be called before application shutdown.
	Flush() error
}

// LogEntry represents a complete log entry with all its metadata.
// This is used internally by formatters and outputs.
type LogEntry struct {
	// Time when the log entry was created
	Time time.Time

	// Level of the log entry
	Level Level

	// Message is the log message
	Message string

	// Fields contains structured data
	Fields []Field

	// Context associated with the log entry (optional)
	Context context.Context

	// Caller information (file, line, function)
	Caller *CallerInfo
}

// CallerInfo contains information about the code location that generated a log.
type CallerInfo struct {
	// File is the source file path
	File string

	// Line is the line number in the source file
	Line int

	// Function is the function name
	Function string
}

// Output defines where and how logs are written.
// Implementations must be thread-safe.
type Output interface {
	// Write writes a log entry to the output.
	Write(entry LogEntry) error

	// Flush ensures all buffered logs are written.
	Flush() error

	// Close closes the output and releases resources.
	Close() error
}

// Formatter formats log entries for output.
// Different formatters can produce JSON, text, or other formats.
type Formatter interface {
	// Format converts a log entry to bytes for writing.
	Format(entry LogEntry) ([]byte, error)
}

// Hook is called for each log entry that passes level filtering.
// Hooks can be used for metrics, alerting, or custom processing.
type Hook interface {
	// Fire is called for each log entry.
	Fire(entry LogEntry) error

	// Levels returns the log levels this hook should fire for.
	Levels() []Level
}

// Factory creates logger instances with specific configurations.
// This implements the Factory pattern for logger creation.
type Factory interface {
	// Create creates a new logger with the given name.
	// The name is typically used for categorizing logs.
	Create(name string) Logger

	// CreateWithFields creates a new logger with predefined fields.
	CreateWithFields(name string, fields ...Field) Logger
}
