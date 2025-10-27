package breaker_test

import (
	"errors"
	"testing"
	"time"

	breaker "github.com/farzai/breaker-go"
)

func TestVersionValidator(t *testing.T) {
	t.Run("Accepts version within range", func(t *testing.T) {
		validator := breaker.NewVersionValidator(1, 3)
		snapshot := breaker.NewSnapshotBuilder().
			WithVersion(2).
			Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for valid version, got %v", err)
		}
	})

	t.Run("Accepts minimum version", func(t *testing.T) {
		validator := breaker.NewVersionValidator(1, 3)
		snapshot := breaker.NewSnapshotBuilder().
			WithVersion(1).
			Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for minimum version, got %v", err)
		}
	})

	t.Run("Accepts maximum version", func(t *testing.T) {
		validator := breaker.NewVersionValidator(1, 3)
		snapshot := breaker.NewSnapshotBuilder().
			WithVersion(3).
			Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for maximum version, got %v", err)
		}
	})

	t.Run("Rejects version below minimum", func(t *testing.T) {
		validator := breaker.NewVersionValidator(2, 5)
		snapshot := breaker.NewSnapshotBuilder().
			WithVersion(1).
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for version below minimum")
		}
		if !errors.Is(err, breaker.ErrUnsupportedVersion) {
			t.Errorf("Expected ErrUnsupportedVersion, got %v", err)
		}
	})

	t.Run("Rejects version above maximum", func(t *testing.T) {
		validator := breaker.NewVersionValidator(1, 3)
		snapshot := breaker.NewSnapshotBuilder().
			WithVersion(4).
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for version above maximum")
		}
		if !errors.Is(err, breaker.ErrUnsupportedVersion) {
			t.Errorf("Expected ErrUnsupportedVersion, got %v", err)
		}
	})
}

func TestStateValidator(t *testing.T) {
	validator := breaker.NewStateValidator()

	t.Run("Accepts Closed state", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Closed).
			Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for Closed state, got %v", err)
		}
	})

	t.Run("Accepts Open state", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for Open state, got %v", err)
		}
	})

	t.Run("Accepts HalfOpen state", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.HalfOpen).
			Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for HalfOpen state, got %v", err)
		}
	})

	t.Run("Rejects invalid state value", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().Build()
		snapshot.State = breaker.CircuitBreakerState(999) // Invalid state

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for invalid state")
		}

		var valErr *breaker.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})
}

func TestConfigValidator(t *testing.T) {
	validator := breaker.NewConfigValidator()

	t.Run("Accepts valid configuration", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithConfig(5, 2*time.Second).
			Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for valid config, got %v", err)
		}
	})

	t.Run("Rejects zero failure threshold", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithConfig(0, 1*time.Second).
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for zero failure threshold")
		}

		var valErr *breaker.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})

	t.Run("Rejects negative failure threshold", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithConfig(-1, 1*time.Second).
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for negative failure threshold")
		}
	})

	t.Run("Rejects zero reset timeout", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithConfig(3, 0).
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for zero reset timeout")
		}

		var valErr *breaker.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})

	t.Run("Rejects negative reset timeout", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithConfig(3, -1*time.Second).
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for negative reset timeout")
		}
	})
}

func TestDataConsistencyValidator(t *testing.T) {
	validator := breaker.NewDataConsistencyValidator()

	t.Run("Accepts valid Closed state snapshot", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Closed).
			WithFailureCount(0).
			WithConfig(3, 1*time.Second).
			Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for valid Closed state, got %v", err)
		}
	})

	t.Run("Accepts valid Open state snapshot", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithFailureCount(3).
			WithLastFailure(time.Now()).
			WithConfig(3, 1*time.Second).
			Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for valid Open state, got %v", err)
		}
	})

	t.Run("Rejects negative failure count", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithFailureCount(-1).
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for negative failure count")
		}

		var valErr *breaker.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
		if valErr.Field != "failure_count" {
			t.Errorf("Expected field 'failure_count', got %s", valErr.Field)
		}
	})

	t.Run("Rejects Open state with zero last failure", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithFailureCount(3).
			WithLastFailure(time.Time{}). // Zero time
			WithConfig(3, 1*time.Second).
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for Open state with zero last failure")
		}

		var valErr *breaker.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})

	t.Run("Rejects Closed state with non-zero failure count", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Closed).
			WithFailureCount(3).
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for Closed state with non-zero failure count")
		}

		var valErr *breaker.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})

	t.Run("Rejects zero CreatedAt timestamp", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().Build()
		snapshot.CreatedAt = time.Time{} // Zero time

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for zero CreatedAt")
		}

		var valErr *breaker.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
		if valErr.Field != "created_at" {
			t.Errorf("Expected field 'created_at', got %s", valErr.Field)
		}
	})

	t.Run("Rejects future CreatedAt timestamp", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().Build()
		snapshot.CreatedAt = time.Now().Add(2 * time.Minute) // Far in the future

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for future CreatedAt")
		}

		var valErr *breaker.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})
}

func TestAgeValidator(t *testing.T) {
	t.Run("Accepts recent snapshot", func(t *testing.T) {
		validator := breaker.NewAgeValidator(1 * time.Hour)
		snapshot := breaker.NewSnapshotBuilder().Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for recent snapshot, got %v", err)
		}
	})

	t.Run("Rejects old snapshot", func(t *testing.T) {
		validator := breaker.NewAgeValidator(1 * time.Hour)
		snapshot := breaker.NewSnapshotBuilder().Build()
		snapshot.CreatedAt = time.Now().Add(-2 * time.Hour)

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for old snapshot")
		}

		var valErr *breaker.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})

	t.Run("Accepts snapshot at exact age limit", func(t *testing.T) {
		maxAge := 30 * time.Minute
		validator := breaker.NewAgeValidator(maxAge)
		snapshot := breaker.NewSnapshotBuilder().Build()
		snapshot.CreatedAt = time.Now().Add(-maxAge + 1*time.Second) // Just under the limit

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for snapshot at age limit, got %v", err)
		}
	})
}

func TestCompositeValidator(t *testing.T) {
	t.Run("Runs all validators in sequence", func(t *testing.T) {
		validator := breaker.NewCompositeValidator(
			breaker.NewVersionValidator(1, 3),
			breaker.NewStateValidator(),
			breaker.NewConfigValidator(),
		)

		snapshot := breaker.NewSnapshotBuilder().
			WithVersion(2).
			WithState(breaker.Open).
			WithConfig(5, 1*time.Second).
			Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for valid snapshot, got %v", err)
		}
	})

	t.Run("Stops at first validation failure", func(t *testing.T) {
		validator := breaker.NewCompositeValidator(
			breaker.NewVersionValidator(1, 3),
			breaker.NewStateValidator(),
			breaker.NewConfigValidator(),
		)

		snapshot := breaker.NewSnapshotBuilder().
			WithVersion(5). // Invalid version
			WithConfig(0, 1*time.Second). // Invalid config
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected validation error")
		}

		// Should get version error first
		if !errors.Is(err, breaker.ErrUnsupportedVersion) {
			t.Errorf("Expected version error first, got %v", err)
		}
	})

	t.Run("Works with no validators", func(t *testing.T) {
		validator := breaker.NewCompositeValidator()
		snapshot := breaker.NewSnapshotBuilder().Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error with no validators, got %v", err)
		}
	})
}

func TestDefaultValidator(t *testing.T) {
	t.Run("Validates snapshot with default rules", func(t *testing.T) {
		validator := breaker.DefaultValidator()
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithFailureCount(3).
			WithLastFailure(time.Now()).
			WithConfig(5, 2*time.Second).
			Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for valid snapshot, got %v", err)
		}
	})

	t.Run("Rejects snapshot with invalid version", func(t *testing.T) {
		validator := breaker.DefaultValidator()
		snapshot := breaker.NewSnapshotBuilder().
			WithVersion(999).
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for invalid version")
		}
	})

	t.Run("Rejects snapshot with invalid config", func(t *testing.T) {
		validator := breaker.DefaultValidator()
		snapshot := breaker.NewSnapshotBuilder().
			WithConfig(0, 1*time.Second).
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for invalid config")
		}
	})

	t.Run("Rejects snapshot with data inconsistency", func(t *testing.T) {
		validator := breaker.DefaultValidator()
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Closed).
			WithFailureCount(5). // Inconsistent: Closed should have 0 failures
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for data inconsistency")
		}
	})
}

func TestStrictValidator(t *testing.T) {
	t.Run("Validates snapshot with strict rules", func(t *testing.T) {
		validator := breaker.StrictValidator(1 * time.Hour)
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithFailureCount(3).
			WithLastFailure(time.Now()).
			WithConfig(5, 2*time.Second).
			Build()

		err := validator.Validate(snapshot)
		if err != nil {
			t.Errorf("Expected no error for valid snapshot, got %v", err)
		}
	})

	t.Run("Rejects old snapshot", func(t *testing.T) {
		validator := breaker.StrictValidator(30 * time.Minute)
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithFailureCount(3).
			WithLastFailure(time.Now()).
			WithConfig(5, 2*time.Second).
			Build()
		snapshot.CreatedAt = time.Now().Add(-1 * time.Hour)

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for old snapshot with strict validator")
		}
	})

	t.Run("Includes all default validations", func(t *testing.T) {
		validator := breaker.StrictValidator(1 * time.Hour)
		snapshot := breaker.NewSnapshotBuilder().
			WithVersion(999). // Invalid version
			Build()

		err := validator.Validate(snapshot)
		if err == nil {
			t.Error("Expected error for invalid version")
		}
	})
}
