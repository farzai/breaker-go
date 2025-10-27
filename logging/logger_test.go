package logging

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// TestLevel tests log level string representation
func TestLevel(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelOff, "OFF"},
		{Level(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("Level.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestDefaultLogger tests the default logger implementation
func TestDefaultLogger(t *testing.T) {
	t.Run("logs at appropriate levels", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewDefaultLoggerWithOutput(LevelInfo, &buf, &buf)

		logger.Debug("debug message") // Should be filtered out
		logger.Info("info message")
		logger.Warn("warn message")
		logger.Error("error message")

		output := buf.String()

		if strings.Contains(output, "debug message") {
			t.Error("Debug message should be filtered out")
		}
		if !strings.Contains(output, "info message") {
			t.Error("Info message should be logged")
		}
		if !strings.Contains(output, "warn message") {
			t.Error("Warn message should be logged")
		}
		if !strings.Contains(output, "error message") {
			t.Error("Error message should be logged")
		}
	})

	t.Run("logs with structured fields", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewDefaultLoggerWithOutput(LevelInfo, &buf, &buf)

		logger.Info("test message",
			String("key1", "value1"),
			Int("key2", 42),
			Bool("key3", true),
		)

		output := buf.String()

		if !strings.Contains(output, "key1=\"value1\"") {
			t.Error("String field not logged correctly")
		}
		if !strings.Contains(output, "key2=42") {
			t.Error("Int field not logged correctly")
		}
		if !strings.Contains(output, "key3=true") {
			t.Error("Bool field not logged correctly")
		}
	})

	t.Run("With creates child logger with inherited fields", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewDefaultLoggerWithOutput(LevelInfo, &buf, &buf)

		childLogger := logger.With(String("parent_field", "parent_value"))
		childLogger.Info("test",
			String("child_field", "child_value"),
		)

		output := buf.String()

		if !strings.Contains(output, "parent_field=\"parent_value\"") {
			t.Error("Parent field not inherited")
		}
		if !strings.Contains(output, "child_field=\"child_value\"") {
			t.Error("Child field not logged")
		}
	})

	t.Run("Enabled returns correct value", func(t *testing.T) {
		logger := NewDefaultLogger(LevelWarn)

		if logger.Enabled(LevelDebug) {
			t.Error("Debug should be disabled")
		}
		if logger.Enabled(LevelInfo) {
			t.Error("Info should be disabled")
		}
		if !logger.Enabled(LevelWarn) {
			t.Error("Warn should be enabled")
		}
		if !logger.Enabled(LevelError) {
			t.Error("Error should be enabled")
		}
	})

	t.Run("SetLevel changes minimum level", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewDefaultLoggerWithOutput(LevelWarn, &buf, &buf)

		logger.Info("should be filtered")

		logger.SetLevel(LevelInfo)
		logger.Info("should be logged")

		output := buf.String()

		if strings.Contains(output, "should be filtered") {
			t.Error("Message before SetLevel should be filtered")
		}
		if !strings.Contains(output, "should be logged") {
			t.Error("Message after SetLevel should be logged")
		}
	})

	t.Run("logs errors with proper formatting", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewDefaultLoggerWithOutput(LevelInfo, &buf, &buf)

		testErr := errors.New("test error")
		logger.Error("operation failed", Error(testErr))

		output := buf.String()

		if !strings.Contains(output, "test error") {
			t.Error("Error message not logged")
		}
	})

	t.Run("logs time and duration fields", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewDefaultLoggerWithOutput(LevelInfo, &buf, &buf)

		now := time.Now()
		duration := 100 * time.Millisecond

		logger.Info("test",
			Time("timestamp", now),
			Duration("elapsed", duration),
		)

		output := buf.String()

		if !strings.Contains(output, "timestamp=") {
			t.Error("Time field not logged")
		}
		if !strings.Contains(output, "elapsed=") {
			t.Error("Duration field not logged")
		}
	})
}

// TestNoOpLogger tests the no-op logger
func TestNoOpLogger(t *testing.T) {
	logger := NewNoOpLogger()

	// These should not panic
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	child := logger.With(String("key", "value"))
	child.Info("test")

	if logger.Enabled(LevelError) {
		t.Error("NoOpLogger should never be enabled")
	}
}

// TestTestLogger tests the test logger
func TestTestLogger(t *testing.T) {
	t.Run("captures log entries", func(t *testing.T) {
		logger := NewTestLogger()

		logger.Debug("debug message", String("level", "debug"))
		logger.Info("info message", String("level", "info"))
		logger.Warn("warn message", String("level", "warn"))
		logger.Error("error message", String("level", "error"))

		entries := logger.Entries()
		if len(entries) != 4 {
			t.Errorf("Expected 4 entries, got %d", len(entries))
		}

		if entries[0].Level != LevelDebug || entries[0].Message != "debug message" {
			t.Error("Debug entry not captured correctly")
		}
		if entries[1].Level != LevelInfo || entries[1].Message != "info message" {
			t.Error("Info entry not captured correctly")
		}
		if entries[2].Level != LevelWarn || entries[2].Message != "warn message" {
			t.Error("Warn entry not captured correctly")
		}
		if entries[3].Level != LevelError || entries[3].Message != "error message" {
			t.Error("Error entry not captured correctly")
		}
	})

	t.Run("EntriesByLevel filters correctly", func(t *testing.T) {
		logger := NewTestLogger()

		logger.Info("info 1")
		logger.Error("error 1")
		logger.Info("info 2")

		infoEntries := logger.EntriesByLevel(LevelInfo)
		if len(infoEntries) != 2 {
			t.Errorf("Expected 2 info entries, got %d", len(infoEntries))
		}

		errorEntries := logger.EntriesByLevel(LevelError)
		if len(errorEntries) != 1 {
			t.Errorf("Expected 1 error entry, got %d", len(errorEntries))
		}
	})

	t.Run("Reset clears entries", func(t *testing.T) {
		logger := NewTestLogger()

		logger.Info("test")
		if logger.Count() != 1 {
			t.Error("Expected 1 entry before reset")
		}

		logger.Reset()
		if logger.Count() != 0 {
			t.Error("Expected 0 entries after reset")
		}
	})

	t.Run("HasMessage finds messages", func(t *testing.T) {
		logger := NewTestLogger()

		logger.Info("test message")

		if !logger.HasMessage("test message") {
			t.Error("HasMessage should return true")
		}
		if logger.HasMessage("nonexistent") {
			t.Error("HasMessage should return false for nonexistent message")
		}
	})

	t.Run("HasField finds fields", func(t *testing.T) {
		logger := NewTestLogger()

		logger.Info("test", String("key", "value"))

		if !logger.HasField("key") {
			t.Error("HasField should return true")
		}
		if logger.HasField("nonexistent") {
			t.Error("HasField should return false for nonexistent field")
		}
	})

	t.Run("CountByLevel counts correctly", func(t *testing.T) {
		logger := NewTestLogger()

		logger.Info("info 1")
		logger.Info("info 2")
		logger.Error("error 1")

		if logger.CountByLevel(LevelInfo) != 2 {
			t.Errorf("Expected 2 info logs, got %d", logger.CountByLevel(LevelInfo))
		}
		if logger.CountByLevel(LevelError) != 1 {
			t.Errorf("Expected 1 error log, got %d", logger.CountByLevel(LevelError))
		}
	})
}

// TestFields tests field helper functions
func TestFields(t *testing.T) {
	t.Run("String creates string field", func(t *testing.T) {
		f := String("key", "value")
		if f.Key != "key" || f.Type != StringType || f.Value != "value" {
			t.Error("String field not created correctly")
		}
	})

	t.Run("Int creates int field", func(t *testing.T) {
		f := Int("key", 42)
		if f.Key != "key" || f.Type != IntType || f.Value != 42 {
			t.Error("Int field not created correctly")
		}
	})

	t.Run("Error creates error field", func(t *testing.T) {
		err := errors.New("test error")
		f := Error(err)
		if f.Key != "error" || f.Type != ErrorType {
			t.Error("Error field not created correctly")
		}
	})

	t.Run("Error with nil creates nil field", func(t *testing.T) {
		f := Error(nil)
		if f.Value != nil {
			t.Error("Error with nil should have nil value")
		}
	})

	t.Run("Fields.With combines fields", func(t *testing.T) {
		base := Fields{String("key1", "value1")}
		combined := base.With(String("key2", "value2"))

		if len(combined) != 2 {
			t.Errorf("Expected 2 fields, got %d", len(combined))
		}
	})

	t.Run("Fields.ToMap converts to map", func(t *testing.T) {
		fields := Fields{
			String("key1", "value1"),
			Int("key2", 42),
		}

		m := fields.ToMap()

		if m["key1"] != "value1" {
			t.Error("key1 not in map correctly")
		}
		if m["key2"] != 42 {
			t.Error("key2 not in map correctly")
		}
	})

	t.Run("Namespace creates prefixed fields", func(t *testing.T) {
		fields := Namespace("http", Fields{
			String("method", "GET"),
			String("path", "/api/users"),
		})

		if fields[0].Key != "http.method" {
			t.Errorf("Expected http.method, got %s", fields[0].Key)
		}
		if fields[1].Key != "http.path" {
			t.Errorf("Expected http.path, got %s", fields[1].Key)
		}
	})
}

// TestBuilder tests the logger builder
func TestBuilder(t *testing.T) {
	t.Run("Build creates logger with configuration", func(t *testing.T) {
		var buf bytes.Buffer

		logger := NewBuilder().
			WithLevel(LevelWarn).
			WithOutput(&buf).
			WithFields(String("service", "test")).
			Build()

		// Test level filtering
		logger.Info("should be filtered")
		logger.Warn("should be logged")

		output := buf.String()

		if strings.Contains(output, "should be filtered") {
			t.Error("Info should be filtered at Warn level")
		}
		if !strings.Contains(output, "should be logged") {
			t.Error("Warn should be logged")
		}
		if !strings.Contains(output, "service=\"test\"") {
			t.Error("Default fields not applied")
		}
	})

	t.Run("BuildTest creates test logger", func(t *testing.T) {
		logger := NewBuilder().BuildTest()

		logger.Info("test")

		if logger.Count() != 1 {
			t.Error("Test logger should capture logs")
		}
	})

	t.Run("BuildNoOp creates no-op logger", func(t *testing.T) {
		logger := NewBuilder().BuildNoOp()

		if logger.Enabled(LevelError) {
			t.Error("NoOp logger should always be disabled")
		}
	})

	t.Run("Development preset", func(t *testing.T) {
		logger := Development().BuildDefault()

		if logger.GetLevel() != LevelDebug {
			t.Error("Development preset should use Debug level")
		}
	})

	t.Run("Production preset", func(t *testing.T) {
		logger := Production().BuildDefault()

		if logger.GetLevel() != LevelInfo {
			t.Error("Production preset should use Info level")
		}
	})
}

// TestSlogAdapter tests the slog adapter
func TestSlogAdapter(t *testing.T) {
	t.Run("adapts slog logger", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		slogger := slog.New(handler)

		logger := NewSlogAdapter(slogger)

		logger.Info("test message",
			String("key", "value"),
			Int("count", 42),
		)

		output := buf.String()

		if !strings.Contains(output, "test message") {
			t.Error("Message not logged")
		}
		if !strings.Contains(output, "key=value") {
			t.Error("String field not logged")
		}
		if !strings.Contains(output, "count=42") {
			t.Error("Int field not logged")
		}
	})

	t.Run("With creates child with fields", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
		slogger := slog.New(handler)

		logger := NewSlogAdapter(slogger)
		child := logger.With(String("parent", "value"))

		child.Info("test", String("child", "value"))

		output := buf.String()

		if !strings.Contains(output, "parent=value") {
			t.Error("Parent field not inherited")
		}
		if !strings.Contains(output, "child=value") {
			t.Error("Child field not logged")
		}
	})

	t.Run("WithContext creates logger with context", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
		slogger := slog.New(handler)

		logger := NewSlogAdapter(slogger)
		ctx := context.WithValue(context.Background(), "key", "value")
		ctxLogger := logger.WithContext(ctx)

		ctxLogger.Info("test")

		// Should not panic
	})
}

// TestFuncLogger tests the function logger adapter
func TestFuncLogger(t *testing.T) {
	t.Run("calls function with log data", func(t *testing.T) {
		var capturedLevel Level
		var capturedMsg string
		var capturedFields []Field

		fn := func(level Level, msg string, fields []Field) {
			capturedLevel = level
			capturedMsg = msg
			capturedFields = fields
		}

		logger := NewFuncLogger(fn)
		logger.Info("test message", String("key", "value"))

		if capturedLevel != LevelInfo {
			t.Errorf("Expected Info level, got %v", capturedLevel)
		}
		if capturedMsg != "test message" {
			t.Errorf("Expected 'test message', got %s", capturedMsg)
		}
		if len(capturedFields) != 1 || capturedFields[0].Key != "key" {
			t.Error("Fields not passed correctly")
		}
	})

	t.Run("With adds fields", func(t *testing.T) {
		var capturedFields []Field

		fn := func(level Level, msg string, fields []Field) {
			capturedFields = fields
		}

		logger := NewFuncLogger(fn)
		child := logger.With(String("parent", "value"))
		child.Info("test", String("child", "value"))

		if len(capturedFields) != 2 {
			t.Errorf("Expected 2 fields, got %d", len(capturedFields))
		}
	})
}

// TestPrintfLogger tests the printf-style logger adapter
func TestPrintfLogger(t *testing.T) {
	t.Run("formats logs with printf style", func(t *testing.T) {
		var output string

		fn := func(format string, v ...interface{}) {
			// Format the string with values
			output = fmt.Sprintf(format, v...)
		}

		logger := NewPrintfLogger(fn)
		logger.Info("test message", String("key", "value"))

		if !strings.Contains(output, "INFO") {
			t.Error("Level not included in output")
		}
		if !strings.Contains(output, "test message") {
			t.Error("Message not included in output")
		}
	})
}
