package breaker

import (
	"context"
	"time"
)

// config holds the circuit breaker configuration
type config struct {
	failureThreshold int
	resetTimeout     time.Duration
	storage          StateRepository
	observers        []StateObserver
	onStateChange    func(from, to CircuitBreakerState)
}

// Option is a function that configures a CircuitBreaker
type Option func(*config)

// WithFailureThreshold sets the number of failures before opening the circuit
func WithFailureThreshold(threshold int) Option {
	return func(c *config) {
		c.failureThreshold = threshold
	}
}

// WithResetTimeout sets the timeout before transitioning from Open to HalfOpen
func WithResetTimeout(timeout time.Duration) Option {
	return func(c *config) {
		c.resetTimeout = timeout
	}
}

// WithStorage sets the state storage repository
func WithStorage(storage StateRepository) Option {
	return func(c *config) {
		c.storage = storage
	}
}

// WithObserver adds a state change observer
func WithObserver(observer StateObserver) Option {
	return func(c *config) {
		c.observers = append(c.observers, observer)
	}
}

// WithStateChangeCallback sets a callback function for state changes
func WithStateChangeCallback(callback func(from, to CircuitBreakerState)) Option {
	return func(c *config) {
		c.onStateChange = callback
	}
}

// NewWithOptions creates a new CircuitBreaker with functional options.
// Default values: failureThreshold=5, resetTimeout=5s, storage=InMemoryStateRepository.
// It panics if the configured failureThreshold <= 0, resetTimeout <= 0, or storage is nil.
//
// Example:
//
//	cb := breaker.NewWithOptions(
//		breaker.WithFailureThreshold(5),
//		breaker.WithResetTimeout(10*time.Second),
//		breaker.WithStateChangeCallback(func(from, to breaker.CircuitBreakerState) {
//			log.Printf("State changed from %v to %v", from, to)
//		}),
//	)
func NewWithOptions(opts ...Option) CircuitBreaker {
	// Default configuration
	cfg := &config{
		failureThreshold: 5,
		resetTimeout:     5 * time.Second,
		storage:          NewInMemoryStateRepository(),
		observers:        []StateObserver{},
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	// Validate configuration
	if cfg.failureThreshold <= 0 {
		panic("failureThreshold must be greater than 0")
	}
	if cfg.resetTimeout <= 0 {
		panic("resetTimeout must be greater than 0")
	}
	if cfg.storage == nil {
		panic("storage must not be nil")
	}

	cb := &CircuitBreakerImpl{
		state:            Closed,
		currentState:     &ClosedState{},
		failureThreshold: cfg.failureThreshold,
		resetTimeout:     cfg.resetTimeout,
		storage:          cfg.storage,
		observers:        cfg.observers,
		onStateChange:    cfg.onStateChange,
	}

	cb.currentState.OnEntry(cb)
	return cb
}

// StateObserver is notified when the circuit breaker state changes
type StateObserver interface {
	OnStateChange(ctx context.Context, from, to CircuitBreakerState)
}

// LoggingObserver logs state changes
type LoggingObserver struct {
	LogFunc func(msg string)
}

func (o *LoggingObserver) OnStateChange(ctx context.Context, from, to CircuitBreakerState) {
	if o.LogFunc != nil {
		o.LogFunc(formatStateChange(from, to))
	}
}

func formatStateChange(from, to CircuitBreakerState) string {
	fromStr := stateToString(from)
	toStr := stateToString(to)
	return "Circuit breaker state changed from " + fromStr + " to " + toStr
}

func stateToString(state CircuitBreakerState) string {
	switch state {
	case Closed:
		return "Closed"
	case Open:
		return "Open"
	case HalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}
