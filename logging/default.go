package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// DefaultLogger is a production-ready logger with level filtering and structured output.
// It writes to stdout/stderr with a simple, readable format.
//
// Features:
//   - Level filtering
//   - Structured field support
//   - Thread-safe
//   - Contextual logging with inherited fields
//   - Configurable output streams
//
// Example:
//
//	logger := logging.NewDefaultLogger(logging.LevelInfo)
//	logger.Info("Server started", String("port", "8080"))
type DefaultLogger struct {
	minLevel  Level
	output    io.Writer
	errOutput io.Writer
	fields    []Field
	ctx       context.Context
	mu        sync.Mutex
}

// NewDefaultLogger creates a new default logger with the specified minimum level.
// It writes Info/Debug/Warn to stdout and Error to stderr.
func NewDefaultLogger(minLevel Level) *DefaultLogger {
	return &DefaultLogger{
		minLevel:  minLevel,
		output:    os.Stdout,
		errOutput: os.Stderr,
		fields:    []Field{},
		ctx:       context.Background(),
	}
}

// NewDefaultLoggerWithOutput creates a default logger with custom output streams.
func NewDefaultLoggerWithOutput(minLevel Level, output, errOutput io.Writer) *DefaultLogger {
	return &DefaultLogger{
		minLevel:  minLevel,
		output:    output,
		errOutput: errOutput,
		fields:    []Field{},
		ctx:       context.Background(),
	}
}

// Debug logs a debug message.
func (l *DefaultLogger) Debug(msg string, fields ...Field) {
	l.log(LevelDebug, msg, fields...)
}

// Info logs an info message.
func (l *DefaultLogger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields...)
}

// Warn logs a warning message.
func (l *DefaultLogger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields...)
}

// Error logs an error message.
func (l *DefaultLogger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields...)
}

// With creates a child logger with additional context fields.
func (l *DefaultLogger) With(fields ...Field) Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create a new field slice combining parent and new fields
	newFields := make([]Field, 0, len(l.fields)+len(fields))
	newFields = append(newFields, l.fields...)
	newFields = append(newFields, fields...)

	return &DefaultLogger{
		minLevel:  l.minLevel,
		output:    l.output,
		errOutput: l.errOutput,
		fields:    newFields,
		ctx:       l.ctx,
	}
}

// WithContext creates a child logger with context.
func (l *DefaultLogger) WithContext(ctx context.Context) Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	return &DefaultLogger{
		minLevel:  l.minLevel,
		output:    l.output,
		errOutput: l.errOutput,
		fields:    l.fields,
		ctx:       ctx,
	}
}

// Enabled returns true if the given level would be logged.
func (l *DefaultLogger) Enabled(level Level) bool {
	return level >= l.minLevel
}

// SetLevel changes the minimum log level dynamically.
func (l *DefaultLogger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minLevel = level
}

// GetLevel returns the current minimum log level.
func (l *DefaultLogger) GetLevel() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.minLevel
}

// log is the internal logging function.
func (l *DefaultLogger) log(level Level, msg string, fields ...Field) {
	// Quick check without lock for performance
	if level < l.minLevel {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check with lock held (minLevel might have changed)
	if level < l.minLevel {
		return
	}

	// Choose output stream
	out := l.output
	if level == LevelError {
		out = l.errOutput
	}

	// Build the log line
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	line := fmt.Sprintf("[%s] %s %s", timestamp, level.String(), msg)

	// Append parent fields
	for _, f := range l.fields {
		line += " " + l.formatField(f)
	}

	// Append new fields
	for _, f := range fields {
		line += " " + l.formatField(f)
	}

	line += "\n"

	// Write to output
	fmt.Fprint(out, line)
}

// formatField formats a field for output.
func (l *DefaultLogger) formatField(f Field) string {
	switch f.Type {
	case StringType:
		return fmt.Sprintf("%s=%q", f.Key, f.Value)
	case IntType, Int64Type:
		return fmt.Sprintf("%s=%d", f.Key, f.Value)
	case Float64Type:
		return fmt.Sprintf("%s=%.2f", f.Key, f.Value)
	case BoolType:
		return fmt.Sprintf("%s=%t", f.Key, f.Value)
	case TimeType:
		if t, ok := f.Value.(time.Time); ok {
			return fmt.Sprintf("%s=%s", f.Key, t.Format(time.RFC3339))
		}
		return fmt.Sprintf("%s=%v", f.Key, f.Value)
	case DurationType:
		if d, ok := f.Value.(time.Duration); ok {
			return fmt.Sprintf("%s=%s", f.Key, d.String())
		}
		return fmt.Sprintf("%s=%v", f.Key, f.Value)
	case ErrorType:
		if err, ok := f.Value.(error); ok && err != nil {
			return fmt.Sprintf("%s=%q", f.Key, err.Error())
		}
		return fmt.Sprintf("%s=nil", f.Key)
	default:
		return fmt.Sprintf("%s=%v", f.Key, f.Value)
	}
}

// NoOpLogger is a logger that discards all log messages.
// Use this when you want to disable logging completely without nil checks.
//
// Example:
//
//	logger := logging.NewNoOpLogger()
//	logger.Info("This will not be logged")
type NoOpLogger struct{}

// NewNoOpLogger creates a new no-op logger.
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}

// Debug does nothing.
func (l *NoOpLogger) Debug(msg string, fields ...Field) {}

// Info does nothing.
func (l *NoOpLogger) Info(msg string, fields ...Field) {}

// Warn does nothing.
func (l *NoOpLogger) Warn(msg string, fields ...Field) {}

// Error does nothing.
func (l *NoOpLogger) Error(msg string, fields ...Field) {}

// With returns the same no-op logger.
func (l *NoOpLogger) With(fields ...Field) Logger {
	return l
}

// WithContext returns the same no-op logger.
func (l *NoOpLogger) WithContext(ctx context.Context) Logger {
	return l
}

// Enabled always returns false.
func (l *NoOpLogger) Enabled(level Level) bool {
	return false
}

// TestLogger is a logger that captures log entries for testing.
// It stores all log entries in memory for later inspection.
//
// Example:
//
//	logger := logging.NewTestLogger()
//	logger.Info("Test message", String("key", "value"))
//	entries := logger.Entries()
//	assert.Equal(t, 1, len(entries))
//	assert.Equal(t, "Test message", entries[0].Message)
type TestLogger struct {
	entries []LogEntry
	mu      sync.Mutex
}

// NewTestLogger creates a new test logger.
func NewTestLogger() *TestLogger {
	return &TestLogger{
		entries: []LogEntry{},
	}
}

// Debug logs a debug message.
func (l *TestLogger) Debug(msg string, fields ...Field) {
	l.log(LevelDebug, msg, fields...)
}

// Info logs an info message.
func (l *TestLogger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields...)
}

// Warn logs a warning message.
func (l *TestLogger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields...)
}

// Error logs an error message.
func (l *TestLogger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields...)
}

// With creates a child logger with additional context fields.
func (l *TestLogger) With(fields ...Field) Logger {
	// For testing, we return the same logger but with fields applied to future logs
	// This is a simplified implementation
	return l
}

// WithContext creates a child logger with context.
func (l *TestLogger) WithContext(ctx context.Context) Logger {
	return l
}

// Enabled always returns true for test logger.
func (l *TestLogger) Enabled(level Level) bool {
	return true
}

// log captures a log entry.
func (l *TestLogger) log(level Level, msg string, fields ...Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: msg,
		Fields:  fields,
		Context: context.Background(),
	}

	l.entries = append(l.entries, entry)
}

// Entries returns all captured log entries.
func (l *TestLogger) Entries() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Return a copy to prevent concurrent modification
	entries := make([]LogEntry, len(l.entries))
	copy(entries, l.entries)
	return entries
}

// EntriesByLevel returns all log entries for a specific level.
func (l *TestLogger) EntriesByLevel(level Level) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []LogEntry
	for _, entry := range l.entries {
		if entry.Level == level {
			result = append(result, entry)
		}
	}
	return result
}

// Reset clears all captured log entries.
func (l *TestLogger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = []LogEntry{}
}

// Count returns the number of captured log entries.
func (l *TestLogger) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// CountByLevel returns the number of log entries for a specific level.
func (l *TestLogger) CountByLevel(level Level) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	count := 0
	for _, entry := range l.entries {
		if entry.Level == level {
			count++
		}
	}
	return count
}

// HasMessage checks if any log entry contains the given message.
func (l *TestLogger) HasMessage(msg string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.entries {
		if entry.Message == msg {
			return true
		}
	}
	return false
}

// HasField checks if any log entry contains a field with the given key.
func (l *TestLogger) HasField(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.entries {
		for _, field := range entry.Fields {
			if field.Key == key {
				return true
			}
		}
	}
	return false
}
