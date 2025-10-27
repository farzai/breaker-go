package breaker

import (
	"fmt"
	"time"

	"github.com/farzai/breaker-go/events"
	"github.com/farzai/breaker-go/logging"
)

// config holds the circuit breaker configuration
type config struct {
	failureThreshold   int
	resetTimeout       time.Duration
	snapshotRepository SnapshotRepository
	persistenceConfig  PersistenceConfig
	restoreStrategy    RestoreStrategy
	eventBus           *events.EventBus
	logger             logging.Logger
}

// Option is a function that configures a CircuitBreaker
type Option func(*config) error

// WithFailureThreshold sets the number of failures before opening the circuit.
// The threshold must be greater than 0.
func WithFailureThreshold(threshold int) Option {
	return func(c *config) error {
		if threshold <= 0 {
			return fmt.Errorf("failureThreshold must be greater than 0, got %d", threshold)
		}
		c.failureThreshold = threshold
		return nil
	}
}

// WithResetTimeout sets the timeout before transitioning from Open to HalfOpen.
// The timeout must be greater than 0.
func WithResetTimeout(timeout time.Duration) Option {
	return func(c *config) error {
		if timeout <= 0 {
			return fmt.Errorf("resetTimeout must be greater than 0, got %v", timeout)
		}
		c.resetTimeout = timeout
		return nil
	}
}

// WithSnapshotRepository sets the snapshot repository for persistence.
// This is the modern way to configure persistence with full error handling.
func WithSnapshotRepository(repo SnapshotRepository) Option {
	return func(c *config) error {
		if repo == nil {
			return fmt.Errorf("snapshot repository must not be nil")
		}
		c.snapshotRepository = repo
		return nil
	}
}

// WithPersistenceConfig sets the complete persistence configuration.
// This gives you full control over persistence behavior.
func WithPersistenceConfig(persistConfig PersistenceConfig) Option {
	return func(c *config) error {
		c.persistenceConfig = persistConfig
		return nil
	}
}

// WithAsyncPersistence enables or disables asynchronous persistence.
// When enabled, state saves don't block circuit breaker operations.
func WithAsyncPersistence(async bool) Option {
	return func(c *config) error {
		c.persistenceConfig.Async = async
		return nil
	}
}

// WithPersistenceRetry configures retry behavior for failed persistence operations.
func WithPersistenceRetry(attempts int, delay time.Duration) Option {
	return func(c *config) error {
		if attempts < 0 {
			return fmt.Errorf("retry attempts must be non-negative, got %d", attempts)
		}
		if delay < 0 {
			return fmt.Errorf("retry delay must be non-negative, got %v", delay)
		}
		c.persistenceConfig.RetryAttempts = attempts
		c.persistenceConfig.RetryDelay = delay
		return nil
	}
}

// WithSnapshotValidator sets a custom validator for snapshots.
// The validator is used both when saving and loading snapshots.
func WithSnapshotValidator(validator SnapshotValidator) Option {
	return func(c *config) error {
		if validator == nil {
			return fmt.Errorf("validator must not be nil")
		}
		c.persistenceConfig.Validator = validator
		return nil
	}
}

// WithRestoreStrategy sets how snapshots are restored during initialization.
func WithRestoreStrategy(strategy RestoreStrategy) Option {
	return func(c *config) error {
		if strategy == nil {
			return fmt.Errorf("restore strategy must not be nil")
		}
		c.restoreStrategy = strategy
		return nil
	}
}

// WithStateRestoration enables or disables state restoration on initialization.
// When enabled, the circuit breaker will attempt to restore its state from
// the repository when created.
func WithStateRestoration(enabled bool) Option {
	return func(c *config) error {
		c.persistenceConfig.RestoreOnInit = enabled
		return nil
	}
}

// WithPersistenceErrorHandler sets callbacks for persistence errors.
func WithPersistenceErrorHandler(onSave, onLoad func(error)) Option {
	return func(c *config) error {
		if onSave != nil {
			c.persistenceConfig.OnSaveError = onSave
		}
		if onLoad != nil {
			c.persistenceConfig.OnLoadError = onLoad
		}
		return nil
	}
}

// WithRestoreErrorHandler sets a callback for state restoration errors.
// This is called when state restoration fails due to config mismatch,
// validation failures, or other issues.
//
// Example:
//
//	cb, err := breaker.New(
//	    breaker.WithRestoreErrorHandler(func(err error) {
//	        log.Printf("Failed to restore state: %v", err)
//	    }),
//	)
func WithRestoreErrorHandler(handler func(error)) Option {
	return func(c *config) error {
		if handler != nil {
			c.persistenceConfig.OnRestoreError = handler
		}
		return nil
	}
}

// WithEventBus sets a custom EventBus for event notifications.
// This gives you full control over event delivery strategies and middleware.
//
// Example:
//
//	bus := events.NewEventBus(
//	    events.WithSynchronousDelivery(),
//	    events.WithLogging(logger),
//	)
//	cb, err := breaker.New(
//	    breaker.WithEventBus(bus),
//	)
func WithEventBus(bus *events.EventBus) Option {
	return func(c *config) error {
		if bus == nil {
			return fmt.Errorf("eventBus must not be nil")
		}
		c.eventBus = bus
		return nil
	}
}

// WithEventListener adds an event listener to receive state change notifications.
// This is the primary way to observe state changes.
//
// Multiple listeners can be added by calling this option multiple times.
//
// Example:
//
//	listener := events.EventListenerFunc(func(event events.StateChangeEvent) error {
//	    log.Printf("State changed: %v -> %v", event.From, event.To)
//	    return nil
//	})
//	cb, err := breaker.New(
//	    breaker.WithEventListener(listener),
//	)
func WithEventListener(listener events.EventListener) Option {
	return func(c *config) error {
		if listener == nil {
			return fmt.Errorf("event listener must not be nil")
		}
		if c.eventBus == nil {
			c.eventBus = events.NewEventBus()
		}
		c.eventBus.Subscribe(listener)
		return nil
	}
}

// WithEventMiddleware adds middleware to the event bus.
// Middleware can be used for logging, metrics, panic recovery, etc.
//
// Example:
//
//	cb, err := breaker.New(
//	    breaker.WithEventMiddleware(
//	        events.LoggingMiddleware(logger),
//	        events.MetricsMiddleware(recorder),
//	    ),
//	)
func WithEventMiddleware(middleware ...events.Middleware) Option {
	return func(c *config) error {
		if c.eventBus == nil {
			c.eventBus = events.NewEventBus()
		}
		for _, mw := range middleware {
			if mw == nil {
				return fmt.Errorf("middleware must not be nil")
			}
			c.eventBus.Use(mw)
		}
		return nil
	}
}

// WithLogger sets a structured logger for the circuit breaker.
// The logger is used for logging circuit breaker operations, state transitions,
// and persistence operations. If not set, logging is disabled.
//
// Example:
//
//	logger := logging.NewDefaultLogger(logging.LevelInfo)
//	cb, err := breaker.New(
//	    breaker.WithLogger(logger),
//	)
func WithLogger(logger logging.Logger) Option {
	return func(c *config) error {
		if logger == nil {
			return fmt.Errorf("logger must not be nil")
		}
		c.logger = logger
		// Also set the logger for persistence
		c.persistenceConfig.Logger = logger
		return nil
	}
}

// WithLogLevel creates a default logger with the specified level.
// This is a convenience function that creates a default logger.
// For more control, use WithLogger with a custom logger.
//
// Example:
//
//	cb, err := breaker.New(
//	    breaker.WithLogLevel(logging.LevelDebug),
//	)
func WithLogLevel(level logging.Level) Option {
	return func(c *config) error {
		logger := logging.NewDefaultLogger(level)
		c.logger = logger
		c.persistenceConfig.Logger = logger
		return nil
	}
}

// WithPersistenceLogger sets a logger specifically for persistence operations.
// This is useful if you want different logging for persistence vs other operations.
// If WithLogger is also used, this will override the persistence logger.
//
// Example:
//
//	persistLogger := logging.NewDefaultLogger(logging.LevelDebug)
//	cb, err := breaker.New(
//	    breaker.WithPersistenceLogger(persistLogger),
//	)
func WithPersistenceLogger(logger logging.Logger) Option {
	return func(c *config) error {
		if logger == nil {
			return fmt.Errorf("persistence logger must not be nil")
		}
		c.persistenceConfig.Logger = logger
		return nil
	}
}

// New creates a new CircuitBreaker with functional options.
//
// Default values:
//   - failureThreshold: 5
//   - resetTimeout: 5 seconds
//   - storage: InMemoryStateRepository
//   - eventBus: New EventBus with async delivery
//
// Returns an error if configuration is invalid.
//
// Example:
//
//	cb, err := breaker.New(
//	    breaker.WithFailureThreshold(3),
//	    breaker.WithResetTimeout(10*time.Second),
//	    breaker.WithEventListener(events.EventListenerFunc(func(event events.StateChangeEvent) error {
//	        log.Printf("State changed: %v -> %v", event.From, event.To)
//	        return nil
//	    })),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
func New(opts ...Option) (CircuitBreaker, error) {
	// Default configuration
	cfg := &config{
		failureThreshold:   5,
		resetTimeout:       5 * time.Second,
		snapshotRepository: nil,
		persistenceConfig:  DefaultPersistenceConfig(),
		restoreStrategy:    &DefaultRestoreStrategy{},
		eventBus:           nil,
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Create default EventBus if not provided
	if cfg.eventBus == nil {
		cfg.eventBus = events.NewEventBus()
	}

	// Create default snapshot repository if not provided
	if cfg.snapshotRepository == nil {
		cfg.snapshotRepository = NewInMemorySnapshotRepository()
	}

	// Create persistence manager
	persistenceManager := NewPersistenceManager(cfg.snapshotRepository, cfg.persistenceConfig)

	cb := &CircuitBreakerImpl{
		currentState:       &ClosedState{},
		failureThreshold:   cfg.failureThreshold,
		resetTimeout:       cfg.resetTimeout,
		persistenceManager: persistenceManager,
		eventBus:           cfg.eventBus,
		logger:             cfg.logger,
	}

	// Attempt to restore state from persistence
	if cfg.persistenceConfig.RestoreOnInit {
		if snapshot, err := persistenceManager.Load(); err == nil {
			// Attempt to restore state from snapshot
			if restoreErr := cfg.restoreStrategy.Restore(cb, snapshot); restoreErr == nil {
				// State restored successfully, skip OnEntry
				return cb, nil
			} else {
				// Restoration failed, call error handler if set
				if cfg.persistenceConfig.OnRestoreError != nil {
					cfg.persistenceConfig.OnRestoreError(restoreErr)
				}
				// Continue with default initialization
			}
		}
		// If load failed (no snapshot exists), continue with default initialization
	}

	// Default initialization: enter closed state
	cb.currentState.OnEntry(cb)
	return cb, nil
}
