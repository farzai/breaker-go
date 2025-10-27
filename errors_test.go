package breaker_test

import (
	"errors"
	"strings"
	"testing"

	breaker "github.com/farzai/breaker-go"
)

func TestPersistenceError(t *testing.T) {
	t.Run("Error method returns formatted message", func(t *testing.T) {
		baseErr := errors.New("connection failed")
		err := breaker.NewPersistenceError("save", baseErr)

		errMsg := err.Error()
		if !strings.Contains(errMsg, "persistence save failed") {
			t.Errorf("Expected error message to contain 'persistence save failed', got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "connection failed") {
			t.Errorf("Expected error message to contain underlying error, got: %s", errMsg)
		}
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		baseErr := errors.New("disk full")
		err := breaker.NewPersistenceError("load", baseErr)

		unwrapped := errors.Unwrap(err)
		if unwrapped != baseErr {
			t.Errorf("Expected unwrapped error to be base error, got: %v", unwrapped)
		}
	})

	t.Run("Works with errors.Is", func(t *testing.T) {
		baseErr := errors.New("timeout")
		err := breaker.NewPersistenceError("save", baseErr)

		if !errors.Is(err, baseErr) {
			t.Error("Expected errors.Is to find base error")
		}
	})

	t.Run("Works with errors.As for PersistenceError type", func(t *testing.T) {
		baseErr := errors.New("network error")
		err := breaker.NewPersistenceError("delete", baseErr)

		var persistErr *breaker.PersistenceError
		if !errors.As(err, &persistErr) {
			t.Fatal("Expected errors.As to extract PersistenceError")
		}

		if persistErr.Op != "delete" {
			t.Errorf("Expected op to be 'delete', got: %s", persistErr.Op)
		}
		if persistErr.Err != baseErr {
			t.Error("Expected embedded error to match base error")
		}
	})
}

func TestValidationError(t *testing.T) {
	t.Run("Error method returns formatted message", func(t *testing.T) {
		err := breaker.NewValidationError("state", "invalid value", "unknown")

		errMsg := err.Error()
		if !strings.Contains(errMsg, "validation failed for state") {
			t.Errorf("Expected error message to contain field name, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "invalid value") {
			t.Errorf("Expected error message to contain validation message, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "unknown") {
			t.Errorf("Expected error message to contain value, got: %s", errMsg)
		}
	})

	t.Run("Works with errors.As for ValidationError type", func(t *testing.T) {
		err := breaker.NewValidationError("timeout", "must be positive", -5)

		var valErr *breaker.ValidationError
		if !errors.As(err, &valErr) {
			t.Fatal("Expected errors.As to extract ValidationError")
		}

		if valErr.Field != "timeout" {
			t.Errorf("Expected field to be 'timeout', got: %s", valErr.Field)
		}
		if valErr.Message != "must be positive" {
			t.Errorf("Expected message to be 'must be positive', got: %s", valErr.Message)
		}
		if valErr.Value != -5 {
			t.Errorf("Expected value to be -5, got: %v", valErr.Value)
		}
	})

	t.Run("Handles various value types", func(t *testing.T) {
		tests := []struct {
			name  string
			value interface{}
		}{
			{"nil value", nil},
			{"int value", 42},
			{"string value", "test"},
			{"bool value", true},
			{"struct value", struct{ x int }{x: 10}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := breaker.NewValidationError("field", "error", tt.value)
				errMsg := err.Error()

				if errMsg == "" {
					t.Error("Expected non-empty error message")
				}
			})
		}
	})
}

func TestErrorConstants(t *testing.T) {
	t.Run("ErrCircuitBreakerOpen has correct message", func(t *testing.T) {
		if breaker.ErrCircuitBreakerOpen.Error() != "circuit breaker is open" {
			t.Errorf("Unexpected error message: %s", breaker.ErrCircuitBreakerOpen.Error())
		}
	})

	t.Run("ErrPersistenceFailed has correct message", func(t *testing.T) {
		if breaker.ErrPersistenceFailed.Error() != "persistence operation failed" {
			t.Errorf("Unexpected error message: %s", breaker.ErrPersistenceFailed.Error())
		}
	})

	t.Run("ErrCorruptedSnapshot has correct message", func(t *testing.T) {
		if breaker.ErrCorruptedSnapshot.Error() != "snapshot is corrupted or invalid" {
			t.Errorf("Unexpected error message: %s", breaker.ErrCorruptedSnapshot.Error())
		}
	})

	t.Run("ErrUnsupportedVersion has correct message", func(t *testing.T) {
		if breaker.ErrUnsupportedVersion.Error() != "snapshot version is not supported" {
			t.Errorf("Unexpected error message: %s", breaker.ErrUnsupportedVersion.Error())
		}
	})

	t.Run("ErrSnapshotNotFound has correct message", func(t *testing.T) {
		if breaker.ErrSnapshotNotFound.Error() != "snapshot not found" {
			t.Errorf("Unexpected error message: %s", breaker.ErrSnapshotNotFound.Error())
		}
	})

	t.Run("ErrInvalidSnapshot has correct message", func(t *testing.T) {
		if breaker.ErrInvalidSnapshot.Error() != "snapshot validation failed" {
			t.Errorf("Unexpected error message: %s", breaker.ErrInvalidSnapshot.Error())
		}
	})
}
