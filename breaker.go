// Package breaker provides a thread-safe circuit breaker implementation.
//
// The Circuit Breaker pattern prevents cascading failures by temporarily blocking
// requests to a failing service, allowing it time to recover.
//
// # States
//
// The circuit breaker has three states:
//
//   - Closed: Requests are allowed through. Failures increment a counter.
//   - Open: Requests are blocked immediately. After a timeout, transitions to Half-Open.
//   - Half-Open: A single request is allowed through to test if the service recovered.
//
// # Usage
//
// Basic usage:
//
//	cb := breaker.NewCircuitBreaker(3, 5*time.Second)
//	result, err := cb.Execute(func() (interface{}, error) {
//	    return someExternalCall()
//	})
//
// With functional options:
//
//	cb := breaker.NewWithOptions(
//	    breaker.WithFailureThreshold(5),
//	    breaker.WithResetTimeout(10*time.Second),
//	    breaker.WithStateChangeCallback(func(from, to breaker.CircuitBreakerState) {
//	        log.Printf("State changed: %v -> %v", from, to)
//	    }),
//	)
//
// With context support:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
//	defer cancel()
//	result, err := cb.ExecuteWithContext(ctx, func(ctx context.Context) (interface{}, error) {
//	    return someExternalCallWithContext(ctx)
//	})
package breaker

import (
	"context"
	"sync"
	"time"

	"github.com/farzai/breaker-go/events"
	"github.com/farzai/breaker-go/logging"
)

// CircuitBreakerState represents the current state of the circuit breaker.
type CircuitBreakerState int

const (
	// Closed state: requests are allowed and failures are counted
	Closed CircuitBreakerState = iota
	// Open state: requests are blocked to allow recovery
	Open
	// HalfOpen state: testing if the service has recovered
	HalfOpen
)

// String returns the string representation of the circuit breaker state.
func (s CircuitBreakerState) String() string {
	switch s {
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

// Removed: ErrCircuitBreakerOpen now defined in errors.go

// CircuitBreaker interface defines the contract for circuit breaker implementations.
// All methods are thread-safe and can be called concurrently.
type CircuitBreaker interface {
	// Execute runs the given action if the CircuitBreaker is closed or half-open.
	// Returns ErrCircuitBreakerOpen if the circuit breaker is open.
	//
	// The action function should perform the protected operation and return
	// its result and any error. If the action returns an error, it's counted
	// as a failure.
	Execute(action func() (interface{}, error)) (interface{}, error)

	// ExecuteWithContext runs the given action with context support.
	// Returns ErrCircuitBreakerOpen if the circuit breaker is open,
	// or context.Canceled/context.DeadlineExceeded if the context is cancelled.
	//
	// The context can be used for timeouts and cancellation within the action.
	ExecuteWithContext(ctx context.Context, action func(ctx context.Context) (interface{}, error)) (interface{}, error)

	// State returns the current state of the CircuitBreaker (Closed, Open, or HalfOpen).
	// This method is thread-safe and can be called concurrently.
	State() CircuitBreakerState

	// Reset forcibly transitions the circuit breaker to the Closed state and resets
	// all counters. This method is primarily intended for testing.
	//
	// In production, the circuit breaker should be allowed to manage its own state
	// transitions based on failure thresholds and timeouts.
	Reset()
}

type CircuitBreakerImpl struct {
	currentState       State
	failureCount       int
	failureThreshold   int
	resetTimeout       time.Duration
	mutex              sync.Mutex
	lastFailure        time.Time
	persistenceManager *PersistenceManager
	eventBus           *events.EventBus
	logger             logging.Logger
}

// NewCircuitBreaker creates a new circuit breaker with the given parameters.
// This is a convenience function that wraps New() with basic configuration.
//
// Example:
//
//	breaker, err := NewCircuitBreaker(3, 5*time.Second)  // 3 failures in 5 seconds
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) (CircuitBreaker, error) {
	return New(
		WithFailureThreshold(failureThreshold),
		WithResetTimeout(resetTimeout),
	)
}

func (c *CircuitBreakerImpl) Execute(action func() (interface{}, error)) (interface{}, error) {
	// Get current state with mutex protection
	c.mutex.Lock()
	state := c.currentState
	c.mutex.Unlock()

	if c.logger != nil && c.logger.Enabled(logging.LevelDebug) {
		c.logger.Debug("Executing action",
			logging.String("state", state.Name().String()),
		)
	}

	// Delegate to the current state with background context
	result, err := state.Execute(context.Background(), c, action)

	if c.logger != nil && err != nil {
		if err == ErrCircuitBreakerOpen {
			c.logger.Warn("Action blocked by circuit breaker",
				logging.String("state", "Open"),
				logging.Error(err),
			)
		} else {
			c.logger.Debug("Action execution failed",
				logging.String("state", state.Name().String()),
				logging.Error(err),
			)
		}
	}

	return result, err
}

// ExecuteWithContext runs the action with context support
func (c *CircuitBreakerImpl) ExecuteWithContext(ctx context.Context, action func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Get current state
	c.mutex.Lock()
	state := c.currentState
	c.mutex.Unlock()

	// Wrap the action to pass context
	wrappedAction := func() (interface{}, error) {
		return action(ctx)
	}

	// Delegate to the current state, passing context for event propagation
	return state.Execute(ctx, c, wrappedAction)
}

func (c *CircuitBreakerImpl) State() CircuitBreakerState {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if we need to transition from Open to HalfOpen
	if c.currentState.Name() == Open && time.Since(c.lastFailure) >= c.resetTimeout {
		c.transitionTo(context.Background(), &HalfOpenState{})
	}

	return c.currentState.Name()
}

// transitionTo changes the state (must be called with mutex held).
// ctx is the context from the current execution for event propagation and tracing.
func (c *CircuitBreakerImpl) transitionTo(ctx context.Context, newState State) {
	if c.currentState != nil {
		c.currentState.OnExit(c)
	}

	// Capture old state before transition
	oldState := c.currentState.Name()

	// Update state
	c.currentState = newState
	newStateName := newState.Name()

	if c.logger != nil {
		c.logger.Info("Circuit breaker state transition",
			logging.String("from", oldState.String()),
			logging.String("to", newStateName.String()),
			logging.Int("failure_count", c.failureCount),
			logging.Int("failure_threshold", c.failureThreshold),
		)
	}

	// Call OnEntry which may modify internal state (failureCount, lastFailure)
	c.currentState.OnEntry(c)

	// Create snapshot AFTER OnEntry to capture complete state
	snapshot := FromCircuitBreaker(c)

	// Persist using persistence manager (handles validation, retry, async)
	// Note: errors are handled internally by persistence manager
	// (logged or passed to error handlers)
	_ = c.persistenceManager.Save(snapshot)

	// Publish state change event using the EventBus
	event := events.StateChangeEvent{
		From:      events.CircuitBreakerState(oldState),
		To:        events.CircuitBreakerState(newStateName),
		Timestamp: time.Now(),
		Context:   ctx,
		Metadata: map[string]interface{}{
			"failure_count":     c.failureCount,
			"failure_threshold": c.failureThreshold,
			"last_failure":      c.lastFailure,
		},
	}

	// Publish asynchronously to avoid blocking
	c.eventBus.PublishAsync(event)
}

func (c *CircuitBreakerImpl) Reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.logger != nil {
		c.logger.Info("Manually resetting circuit breaker",
			logging.String("current_state", c.currentState.Name().String()),
			logging.Int("failure_count", c.failureCount),
		)
	}

	// Reset the failure count
	c.failureCount = 0

	// Reset the last failure time
	c.lastFailure = time.Time{}

	// Transition to closed state
	c.transitionTo(context.Background(), &ClosedState{})
}
