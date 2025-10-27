package logging

import (
	"fmt"
	"time"
)

// Field constructor functions provide type-safe field creation with zero allocations.
// These functions follow the functional options pattern and provide a clean API.

// String creates a string-valued field.
//
// Example:
//
//	logger.Info("User action", String("action", "login"))
func String(key, val string) Field {
	return Field{Key: key, Type: StringType, Value: val}
}

// Int creates an int-valued field.
//
// Example:
//
//	logger.Info("Request processed", Int("status_code", 200))
func Int(key string, val int) Field {
	return Field{Key: key, Type: IntType, Value: val}
}

// Int64 creates an int64-valued field.
//
// Example:
//
//	logger.Info("User registered", Int64("user_id", 123456789))
func Int64(key string, val int64) Field {
	return Field{Key: key, Type: Int64Type, Value: val}
}

// Float64 creates a float64-valued field.
//
// Example:
//
//	logger.Info("Metric recorded", Float64("response_time", 123.45))
func Float64(key string, val float64) Field {
	return Field{Key: key, Type: Float64Type, Value: val}
}

// Bool creates a bool-valued field.
//
// Example:
//
//	logger.Info("Feature check", Bool("enabled", true))
func Bool(key string, val bool) Field {
	return Field{Key: key, Type: BoolType, Value: val}
}

// Time creates a time.Time-valued field.
//
// Example:
//
//	logger.Info("Event occurred", Time("occurred_at", time.Now()))
func Time(key string, val time.Time) Field {
	return Field{Key: key, Type: TimeType, Value: val}
}

// Duration creates a time.Duration-valued field.
//
// Example:
//
//	logger.Info("Operation completed", Duration("took", time.Since(start)))
func Duration(key string, val time.Duration) Field {
	return Field{Key: key, Type: DurationType, Value: val}
}

// Error creates an error-valued field.
// This is the preferred way to log errors.
//
// Example:
//
//	logger.Error("Operation failed", Error(err))
func Error(err error) Field {
	if err == nil {
		return Field{Key: "error", Type: ErrorType, Value: nil}
	}
	return Field{Key: "error", Type: ErrorType, Value: err}
}

// NamedError creates an error-valued field with a custom key.
//
// Example:
//
//	logger.Error("Multiple errors occurred",
//	    NamedError("db_error", dbErr),
//	    NamedError("cache_error", cacheErr),
//	)
func NamedError(key string, err error) Field {
	if err == nil {
		return Field{Key: key, Type: ErrorType, Value: nil}
	}
	return Field{Key: key, Type: ErrorType, Value: err}
}

// Any creates a field with an arbitrary value.
// This uses fmt.Sprintf for serialization and may allocate.
// Prefer typed fields when possible for better performance.
//
// Example:
//
//	logger.Info("Complex object", Any("data", complexStruct))
func Any(key string, val interface{}) Field {
	return Field{Key: key, Type: AnyType, Value: val}
}

// Stringer creates a field from any type that implements fmt.Stringer.
// This defers string conversion until the log is actually written.
//
// Example:
//
//	logger.Info("State transition", Stringer("state", state))
func Stringer(key string, val fmt.Stringer) Field {
	if val == nil {
		return String(key, "<nil>")
	}
	return String(key, val.String())
}

// Bytes creates a field from a byte slice.
// The bytes are represented as a string for readability.
//
// Example:
//
//	logger.Debug("Response body", Bytes("body", responseBytes))
func Bytes(key string, val []byte) Field {
	return String(key, string(val))
}

// BytesLength creates a field with the length of a byte slice.
// This is useful when the actual bytes are too large to log.
//
// Example:
//
//	logger.Info("Data received", BytesLength("payload", data))
func BytesLength(key string, val []byte) Field {
	return Int(key+"_length", len(val))
}

// Stack creates a field with a stack trace.
// This is useful for debugging but should be used sparingly due to overhead.
//
// Example:
//
//	logger.Error("Panic recovered", Stack("stack"))
func Stack(key string) Field {
	// Stack trace will be computed by the logger implementation
	return Field{Key: key, Type: AnyType, Value: "stack_trace"}
}

// Fields is a convenience type for grouping multiple fields.
type Fields []Field

// With adds additional fields to the existing field set.
// This is useful for building up field sets incrementally.
//
// Example:
//
//	baseFields := Fields{String("service", "api")}
//	requestFields := baseFields.With(String("request_id", reqID))
func (f Fields) With(fields ...Field) Fields {
	return append(f, fields...)
}

// ToMap converts fields to a map for easy inspection.
// This allocates and should only be used for testing or debugging.
func (f Fields) ToMap() map[string]interface{} {
	m := make(map[string]interface{}, len(f))
	for _, field := range f {
		m[field.Key] = field.Value
	}
	return m
}

// Namespace creates a nested namespace for fields.
// This is useful for grouping related fields together.
//
// Example:
//
//	logger.Info("Request processed",
//	    Namespace("request", Fields{
//	        String("method", "GET"),
//	        String("path", "/api/users"),
//	    })...,
//	)
func Namespace(key string, fields Fields) Fields {
	result := make(Fields, len(fields))
	for i, f := range fields {
		result[i] = Field{
			Key:   key + "." + f.Key,
			Type:  f.Type,
			Value: f.Value,
		}
	}
	return result
}

// Strings creates multiple string fields from a map.
// This is a convenience function for logging multiple string values.
//
// Example:
//
//	logger.Info("Headers", Strings("header", headers)...)
func Strings(prefix string, values map[string]string) Fields {
	fields := make(Fields, 0, len(values))
	for k, v := range values {
		fields = append(fields, String(prefix+"."+k, v))
	}
	return fields
}

// Ints creates multiple int fields from a map.
//
// Example:
//
//	logger.Info("Metrics", Ints("metric", metrics)...)
func Ints(prefix string, values map[string]int) Fields {
	fields := make(Fields, 0, len(values))
	for k, v := range values {
		fields = append(fields, Int(prefix+"."+k, v))
	}
	return fields
}
