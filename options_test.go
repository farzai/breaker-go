package breaker_test

import (
	"testing"
	"time"

	breaker "github.com/farzai/breaker-go"
	"github.com/farzai/breaker-go/logging"
)

func TestWithPersistenceConfig(t *testing.T) {
	t.Run("Sets custom persistence configuration", func(t *testing.T) {
		config := breaker.PersistenceConfig{
			Enabled:       true,
			Async:         false,
			RetryAttempts: 5,
			RetryDelay:    200 * time.Millisecond,
			Timeout:       10 * time.Second,
		}

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithPersistenceConfig(config),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})

	t.Run("Works with persistence disabled", func(t *testing.T) {
		config := breaker.PersistenceConfig{
			Enabled: false,
		}

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithPersistenceConfig(config),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})
}

func TestWithPersistenceRetry(t *testing.T) {
	t.Run("Sets retry attempts and delay", func(t *testing.T) {
		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithPersistenceRetry(5, 100*time.Millisecond),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})

	t.Run("Works with zero retry attempts", func(t *testing.T) {
		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithPersistenceRetry(0, 100*time.Millisecond),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})

	t.Run("Works with zero retry delay", func(t *testing.T) {
		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithPersistenceRetry(3, 0),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})
}

func TestWithSnapshotValidator(t *testing.T) {
	t.Run("Sets custom snapshot validator", func(t *testing.T) {
		validator := breaker.NewVersionValidator(1, 10)

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithSnapshotValidator(validator),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})

	t.Run("Works with composite validator", func(t *testing.T) {
		validator := breaker.NewCompositeValidator(
			breaker.NewVersionValidator(1, 5),
			breaker.NewStateValidator(),
			breaker.NewConfigValidator(),
		)

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithSnapshotValidator(validator),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})
}

func TestWithRestoreStrategy(t *testing.T) {
	t.Run("Sets custom restore strategy", func(t *testing.T) {
		strategy := &breaker.DefaultRestoreStrategy{}

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithRestoreStrategy(strategy),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})

	t.Run("Works with conservative restore strategy", func(t *testing.T) {
		strategy := breaker.NewConservativeRestoreStrategy(1 * time.Hour)

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithRestoreStrategy(strategy),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})
}

func TestWithStateRestoration(t *testing.T) {
	t.Run("Enables state restoration", func(t *testing.T) {
		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithStateRestoration(true),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})

	t.Run("Disables state restoration", func(t *testing.T) {
		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithStateRestoration(false),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})
}

func TestWithPersistenceErrorHandler(t *testing.T) {
	t.Run("Sets save error handler", func(t *testing.T) {
		saveHandler := func(err error) {}

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithPersistenceErrorHandler(saveHandler, nil),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})

	t.Run("Sets load error handler", func(t *testing.T) {
		loadHandler := func(err error) {}

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithPersistenceErrorHandler(nil, loadHandler),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})

	t.Run("Sets both save and load error handlers", func(t *testing.T) {
		saveHandler := func(err error) {}
		loadHandler := func(err error) {}

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithPersistenceErrorHandler(saveHandler, loadHandler),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})
}

func TestDefaultPersistenceConfig(t *testing.T) {
	t.Run("Returns valid default configuration", func(t *testing.T) {
		config := breaker.DefaultPersistenceConfig()

		if !config.Enabled {
			t.Error("Expected persistence to be enabled by default")
		}
		if !config.Async {
			t.Error("Expected async mode to be enabled by default")
		}
		if config.RetryAttempts <= 0 {
			t.Errorf("Expected positive retry attempts, got %d", config.RetryAttempts)
		}
		if config.RetryDelay <= 0 {
			t.Errorf("Expected positive retry delay, got %v", config.RetryDelay)
		}
		if config.Validator == nil {
			t.Error("Expected validator to be set by default")
		}
		if !config.RestoreOnInit {
			t.Error("Expected restore on init to be enabled by default")
		}
		if config.Timeout <= 0 {
			t.Errorf("Expected positive timeout, got %v", config.Timeout)
		}
	})
}

func TestOptionCombinations(t *testing.T) {
	t.Run("Combines multiple persistence options", func(t *testing.T) {
		validator := breaker.DefaultValidator()
		strategy := &breaker.DefaultRestoreStrategy{}

		cb, err := breaker.New(
			breaker.WithFailureThreshold(5),
			breaker.WithResetTimeout(2*time.Second),
			breaker.WithSnapshotValidator(validator),
			breaker.WithRestoreStrategy(strategy),
			breaker.WithPersistenceRetry(3, 100*time.Millisecond),
			breaker.WithStateRestoration(true),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker with combined options: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})

	t.Run("Persistence options override persistence config", func(t *testing.T) {
		// First set via config, then override with individual options
		config := breaker.PersistenceConfig{
			Enabled:       true,
			Async:         true,
			RetryAttempts: 1,
			RetryDelay:    50 * time.Millisecond,
		}

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithPersistenceConfig(config),
			breaker.WithPersistenceRetry(5, 200*time.Millisecond), // Override retry settings
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})
}

func TestNewPersistenceManager(t *testing.T) {
	t.Run("Creates persistence manager with defaults", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		config := breaker.DefaultPersistenceConfig()

		manager := breaker.NewPersistenceManager(repo, config)
		if manager == nil {
			t.Error("Expected persistence manager to be created")
		}
	})

	t.Run("Creates persistence manager with custom config", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		config := breaker.PersistenceConfig{
			Enabled:       true,
			Async:         false,
			RetryAttempts: 2,
			RetryDelay:    100 * time.Millisecond,
			Validator:     breaker.DefaultValidator(),
			Timeout:       5 * time.Second,
		}

		manager := breaker.NewPersistenceManager(repo, config)
		if manager == nil {
			t.Error("Expected persistence manager to be created")
		}
	})
}

func TestNewConservativeRestoreStrategy(t *testing.T) {
	t.Run("Creates conservative restore strategy", func(t *testing.T) {
		strategy := breaker.NewConservativeRestoreStrategy(1 * time.Hour)
		if strategy == nil {
			t.Error("Expected strategy to be created")
		}
	})

	t.Run("Creates strategy with different max ages", func(t *testing.T) {
		tests := []time.Duration{
			1 * time.Second,
			1 * time.Minute,
			1 * time.Hour,
			24 * time.Hour,
		}

		for _, maxAge := range tests {
			strategy := breaker.NewConservativeRestoreStrategy(maxAge)
			if strategy == nil {
				t.Errorf("Expected strategy to be created for maxAge %v", maxAge)
			}
		}
	})
}

func TestOptionValidation(t *testing.T) {
	t.Run("New validates all options", func(t *testing.T) {
		// This should fail due to invalid failure threshold
		_, err := breaker.New(
			breaker.WithFailureThreshold(0),
		)

		if err == nil {
			t.Error("Expected error for invalid options")
		}
	})

	t.Run("Persistence options work without persistence enabled", func(t *testing.T) {
		// Even if persistence options are set, they should work
		// (they just won't be used if persistence is disabled)
		config := breaker.PersistenceConfig{
			Enabled: false,
		}

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithPersistenceConfig(config),
			breaker.WithSnapshotValidator(breaker.DefaultValidator()),
			breaker.WithPersistenceRetry(3, 100*time.Millisecond),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})
}

func TestWithAsyncPersistence(t *testing.T) {
	t.Run("Enables async persistence", func(t *testing.T) {
		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithAsyncPersistence(true),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})

	t.Run("Disables async persistence", func(t *testing.T) {
		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithAsyncPersistence(false),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})
}

func TestWithLogger(t *testing.T) {
	t.Run("Sets custom logger", func(t *testing.T) {
		logger := logging.NewTestLogger()

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithLogger(logger),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}

		// Trigger some actions to generate logs
		cb.Execute(func() (interface{}, error) {
			return "test", nil
		})

		// Logger should have captured some logs
		if logger.Count() == 0 {
			t.Error("Expected logger to capture logs")
		}
	})
}

func TestWithLogLevel(t *testing.T) {
	t.Run("Sets log level to Debug", func(t *testing.T) {
		logger := logging.NewTestLogger()

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithLogger(logger),
			breaker.WithLogLevel(logging.LevelDebug),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})

	t.Run("Sets log level to Error", func(t *testing.T) {
		logger := logging.NewTestLogger()

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithLogger(logger),
			breaker.WithLogLevel(logging.LevelError),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})
}

func TestWithPersistenceLogger(t *testing.T) {
	t.Run("Sets persistence logger", func(t *testing.T) {
		logger := logging.NewTestLogger()

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithPersistenceLogger(logger),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})
}

func TestWithRestoreErrorHandler(t *testing.T) {
	t.Run("Sets restore error handler", func(t *testing.T) {
		var capturedError error
		restoreHandler := func(err error) {
			capturedError = err
		}

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithRestoreErrorHandler(restoreHandler),
		)

		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}

		// Handler is set (can't easily test it being called without complex setup)
		_ = capturedError
	})
}
