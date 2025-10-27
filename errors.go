package breaker

import (
	"errors"
	"fmt"
)

var (
	// ErrCircuitBreakerOpen is returned when the circuit breaker is open
	// and requests are being blocked.
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")

	// ErrPersistenceFailed is returned when saving or loading state fails
	ErrPersistenceFailed = errors.New("persistence operation failed")

	// ErrCorruptedSnapshot is returned when a loaded snapshot is invalid or corrupted
	ErrCorruptedSnapshot = errors.New("snapshot is corrupted or invalid")

	// ErrUnsupportedVersion is returned when snapshot version is not supported
	ErrUnsupportedVersion = errors.New("snapshot version is not supported")

	// ErrSnapshotNotFound is returned when no snapshot exists in storage
	ErrSnapshotNotFound = errors.New("snapshot not found")

	// ErrInvalidSnapshot is returned when snapshot validation fails
	ErrInvalidSnapshot = errors.New("snapshot validation failed")
)

// PersistenceError wraps underlying persistence errors with context
type PersistenceError struct {
	Op  string // Operation that failed (e.g., "save", "load")
	Err error  // Underlying error
}

func (e *PersistenceError) Error() string {
	return fmt.Sprintf("persistence %s failed: %v", e.Op, e.Err)
}

func (e *PersistenceError) Unwrap() error {
	return e.Err
}

// ValidationError contains details about snapshot validation failures
type ValidationError struct {
	Field   string // Field that failed validation
	Message string // Validation failure message
	Value   interface{}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %s (value: %v)", e.Field, e.Message, e.Value)
}

// NewPersistenceError creates a new persistence error
func NewPersistenceError(op string, err error) error {
	return &PersistenceError{
		Op:  op,
		Err: err,
	}
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string, value interface{}) error {
	return &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	}
}
