package logging

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// This file provides adapters for popular logging frameworks.
// The Adapter Pattern allows seamless integration with existing logging infrastructure.
//
// Supported frameworks:
//   - slog (Go 1.21+ standard library)
//   - Custom adapters can be created by implementing the Logger interface

// SlogAdapter adapts a slog.Logger to our Logger interface.
// This allows using Go's standard structured logging with our circuit breaker.
//
// Example:
//
//	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
//	slogger := slog.New(handler)
//	logger := logging.NewSlogAdapter(slogger)
//	cb, _ := breaker.New(breaker.WithLogger(logger))
type SlogAdapter struct {
	logger *slog.Logger
	fields []Field
	ctx    context.Context
}

// NewSlogAdapter creates a new adapter for slog.Logger.
func NewSlogAdapter(logger *slog.Logger) *SlogAdapter {
	return &SlogAdapter{
		logger: logger,
		fields: []Field{},
		ctx:    context.Background(),
	}
}

// Debug logs a debug message.
func (a *SlogAdapter) Debug(msg string, fields ...Field) {
	allFields := append(a.fields, fields...)
	a.logger.LogAttrs(a.ctx, slog.LevelDebug, msg, convertFieldsToSlogAttrs(allFields)...)
}

// Info logs an info message.
func (a *SlogAdapter) Info(msg string, fields ...Field) {
	allFields := append(a.fields, fields...)
	a.logger.LogAttrs(a.ctx, slog.LevelInfo, msg, convertFieldsToSlogAttrs(allFields)...)
}

// Warn logs a warning message.
func (a *SlogAdapter) Warn(msg string, fields ...Field) {
	allFields := append(a.fields, fields...)
	a.logger.LogAttrs(a.ctx, slog.LevelWarn, msg, convertFieldsToSlogAttrs(allFields)...)
}

// Error logs an error message.
func (a *SlogAdapter) Error(msg string, fields ...Field) {
	allFields := append(a.fields, fields...)
	a.logger.LogAttrs(a.ctx, slog.LevelError, msg, convertFieldsToSlogAttrs(allFields)...)
}

// With creates a child logger with additional context fields.
func (a *SlogAdapter) With(fields ...Field) Logger {
	newFields := make([]Field, 0, len(a.fields)+len(fields))
	newFields = append(newFields, a.fields...)
	newFields = append(newFields, fields...)

	// Create a new slog logger with the fields
	return &SlogAdapter{
		logger: a.logger.With(convertFieldsToSlogAny(fields)...),
		fields: newFields,
		ctx:    a.ctx,
	}
}

// WithContext creates a child logger with context.
func (a *SlogAdapter) WithContext(ctx context.Context) Logger {
	return &SlogAdapter{
		logger: a.logger,
		fields: a.fields,
		ctx:    ctx,
	}
}

// Enabled returns true if the given level would be logged.
func (a *SlogAdapter) Enabled(level Level) bool {
	slogLevel := convertLevelToSlog(level)
	return a.logger.Enabled(a.ctx, slogLevel)
}

// convertFieldsToSlogAttrs converts our fields to slog attributes.
func convertFieldsToSlogAttrs(fields []Field) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(fields))
	for _, f := range fields {
		attrs = append(attrs, convertFieldToSlogAttr(f))
	}
	return attrs
}

// convertFieldsToSlogAny converts our fields to slog any values.
func convertFieldsToSlogAny(fields []Field) []any {
	anys := make([]any, 0, len(fields)*2)
	for _, f := range fields {
		anys = append(anys, f.Key, f.Value)
	}
	return anys
}

// convertFieldToSlogAttr converts a single field to a slog attribute.
func convertFieldToSlogAttr(f Field) slog.Attr {
	switch f.Type {
	case StringType:
		return slog.String(f.Key, f.Value.(string))
	case IntType:
		return slog.Int(f.Key, f.Value.(int))
	case Int64Type:
		return slog.Int64(f.Key, f.Value.(int64))
	case Float64Type:
		return slog.Float64(f.Key, f.Value.(float64))
	case BoolType:
		return slog.Bool(f.Key, f.Value.(bool))
	case TimeType:
		return slog.Time(f.Key, f.Value.(time.Time))
	case DurationType:
		return slog.Duration(f.Key, f.Value.(time.Duration))
	case ErrorType:
		if err, ok := f.Value.(error); ok {
			return slog.Any(f.Key, err)
		}
		return slog.Any(f.Key, nil)
	default:
		return slog.Any(f.Key, f.Value)
	}
}

// convertLevelToSlog converts our log level to slog level.
func convertLevelToSlog(level Level) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// FuncLogger adapts a simple logging function to our Logger interface.
// This is useful for quick integrations or custom logging implementations.
//
// Example:
//
//	logger := logging.NewFuncLogger(func(level logging.Level, msg string, fields []logging.Field) {
//	    fmt.Printf("[%s] %s %v\n", level, msg, fields)
//	})
type FuncLogger struct {
	logFunc func(level Level, msg string, fields []Field)
	fields  []Field
	ctx     context.Context
}

// NewFuncLogger creates a logger from a simple function.
func NewFuncLogger(fn func(level Level, msg string, fields []Field)) *FuncLogger {
	return &FuncLogger{
		logFunc: fn,
		fields:  []Field{},
		ctx:     context.Background(),
	}
}

// Debug logs a debug message.
func (f *FuncLogger) Debug(msg string, fields ...Field) {
	allFields := append(f.fields, fields...)
	f.logFunc(LevelDebug, msg, allFields)
}

// Info logs an info message.
func (f *FuncLogger) Info(msg string, fields ...Field) {
	allFields := append(f.fields, fields...)
	f.logFunc(LevelInfo, msg, allFields)
}

// Warn logs a warning message.
func (f *FuncLogger) Warn(msg string, fields ...Field) {
	allFields := append(f.fields, fields...)
	f.logFunc(LevelWarn, msg, allFields)
}

// Error logs an error message.
func (f *FuncLogger) Error(msg string, fields ...Field) {
	allFields := append(f.fields, fields...)
	f.logFunc(LevelError, msg, allFields)
}

// With creates a child logger with additional context fields.
func (f *FuncLogger) With(fields ...Field) Logger {
	newFields := make([]Field, 0, len(f.fields)+len(fields))
	newFields = append(newFields, f.fields...)
	newFields = append(newFields, fields...)

	return &FuncLogger{
		logFunc: f.logFunc,
		fields:  newFields,
		ctx:     f.ctx,
	}
}

// WithContext creates a child logger with context.
func (f *FuncLogger) WithContext(ctx context.Context) Logger {
	return &FuncLogger{
		logFunc: f.logFunc,
		fields:  f.fields,
		ctx:     ctx,
	}
}

// Enabled always returns true for func logger.
func (f *FuncLogger) Enabled(level Level) bool {
	return true
}

// PrintfLogger wraps a function with Printf-style signature.
// This is useful for integrating with loggers that use fmt.Printf style.
//
// Example:
//
//	logger := logging.NewPrintfLogger(log.Printf)
type PrintfLogger struct {
	printfFunc func(format string, v ...interface{})
	fields     []Field
}

// NewPrintfLogger creates a logger from a printf-style function.
func NewPrintfLogger(fn func(format string, v ...interface{})) *PrintfLogger {
	return &PrintfLogger{
		printfFunc: fn,
		fields:     []Field{},
	}
}

// Debug logs a debug message.
func (p *PrintfLogger) Debug(msg string, fields ...Field) {
	p.log("DEBUG", msg, fields...)
}

// Info logs an info message.
func (p *PrintfLogger) Info(msg string, fields ...Field) {
	p.log("INFO", msg, fields...)
}

// Warn logs a warning message.
func (p *PrintfLogger) Warn(msg string, fields ...Field) {
	p.log("WARN", msg, fields...)
}

// Error logs an error message.
func (p *PrintfLogger) Error(msg string, fields ...Field) {
	p.log("ERROR", msg, fields...)
}

// log formats and logs a message with fields.
func (p *PrintfLogger) log(level, msg string, fields ...Field) {
	allFields := append(p.fields, fields...)

	// Build field string
	fieldStr := ""
	for _, f := range allFields {
		fieldStr += fmt.Sprintf(" %s=%v", f.Key, f.Value)
	}

	p.printfFunc("[%s] %s%s", level, msg, fieldStr)
}

// With creates a child logger with additional context fields.
func (p *PrintfLogger) With(fields ...Field) Logger {
	newFields := make([]Field, 0, len(p.fields)+len(fields))
	newFields = append(newFields, p.fields...)
	newFields = append(newFields, fields...)

	return &PrintfLogger{
		printfFunc: p.printfFunc,
		fields:     newFields,
	}
}

// WithContext returns the same logger (printf logger doesn't support context).
func (p *PrintfLogger) WithContext(ctx context.Context) Logger {
	return p
}

// Enabled always returns true.
func (p *PrintfLogger) Enabled(level Level) bool {
	return true
}
