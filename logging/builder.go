package logging

import (
	"context"
	"io"
	"os"
)

// Builder provides a fluent interface for constructing loggers.
// It implements the Builder pattern for flexible logger configuration.
//
// Example:
//
//	logger := logging.NewBuilder().
//	    WithLevel(logging.LevelInfo).
//	    WithOutput(os.Stdout).
//	    WithFields(logging.String("service", "api")).
//	    Build()
type Builder struct {
	level     Level
	output    io.Writer
	errOutput io.Writer
	fields    []Field
	ctx       context.Context
}

// NewBuilder creates a new logger builder with sensible defaults.
func NewBuilder() *Builder {
	return &Builder{
		level:     LevelInfo,
		output:    os.Stdout,
		errOutput: os.Stderr,
		fields:    []Field{},
		ctx:       context.Background(),
	}
}

// WithLevel sets the minimum log level.
//
// Example:
//
//	builder.WithLevel(logging.LevelDebug)
func (b *Builder) WithLevel(level Level) *Builder {
	b.level = level
	return b
}

// WithOutput sets the output writer for non-error logs.
//
// Example:
//
//	builder.WithOutput(logFile)
func (b *Builder) WithOutput(w io.Writer) *Builder {
	b.output = w
	return b
}

// WithErrorOutput sets the output writer for error logs.
//
// Example:
//
//	builder.WithErrorOutput(errorFile)
func (b *Builder) WithErrorOutput(w io.Writer) *Builder {
	b.errOutput = w
	return b
}

// WithFields adds default fields that will be included in every log entry.
// These fields provide context for all logs from this logger.
//
// Example:
//
//	builder.WithFields(
//	    logging.String("service", "api"),
//	    logging.String("version", "1.0.0"),
//	)
func (b *Builder) WithFields(fields ...Field) *Builder {
	b.fields = append(b.fields, fields...)
	return b
}

// WithContext sets the context for the logger.
//
// Example:
//
//	builder.WithContext(ctx)
func (b *Builder) WithContext(ctx context.Context) *Builder {
	b.ctx = ctx
	return b
}

// Build constructs the logger with the configured options.
//
// Example:
//
//	logger := builder.Build()
func (b *Builder) Build() Logger {
	logger := &DefaultLogger{
		minLevel:  b.level,
		output:    b.output,
		errOutput: b.errOutput,
		fields:    b.fields,
		ctx:       b.ctx,
	}

	return logger
}

// BuildDefault is a convenience method that returns *DefaultLogger.
// This allows access to DefaultLogger-specific methods like SetLevel.
func (b *Builder) BuildDefault() *DefaultLogger {
	return &DefaultLogger{
		minLevel:  b.level,
		output:    b.output,
		errOutput: b.errOutput,
		fields:    b.fields,
		ctx:       b.ctx,
	}
}

// BuildTest creates a test logger for use in tests.
// This ignores most configuration options.
func (b *Builder) BuildTest() *TestLogger {
	return NewTestLogger()
}

// BuildNoOp creates a no-op logger that discards all logs.
// This ignores all configuration options.
func (b *Builder) BuildNoOp() *NoOpLogger {
	return NewNoOpLogger()
}

// Preset configurations for common use cases

// Development returns a builder configured for development.
// Uses debug level and human-readable output.
func Development() *Builder {
	return NewBuilder().
		WithLevel(LevelDebug).
		WithFields(String("env", "development"))
}

// Production returns a builder configured for production.
// Uses info level and structured output.
func Production() *Builder {
	return NewBuilder().
		WithLevel(LevelInfo).
		WithFields(String("env", "production"))
}

// Testing returns a builder configured for testing.
// Returns a test logger that captures all logs.
func Testing() *Builder {
	return NewBuilder().
		WithLevel(LevelDebug)
}

// Silent returns a builder configured for silent mode.
// Returns a no-op logger that discards all logs.
func Silent() *Builder {
	return NewBuilder()
}
