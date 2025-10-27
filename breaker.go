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
	"errors"
	"sync"
	"time"
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

var (
	// ErrCircuitBreakerOpen is returned when the circuit breaker is open
	// and requests are being blocked.
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
)

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
	state            CircuitBreakerState
	currentState     State
	failureCount     int
	failureThreshold int
	resetTimeout     time.Duration
	mutex            sync.Mutex
	lastFailure      time.Time
	storage          StateRepository
	observers        []StateObserver
	onStateChange    func(from, to CircuitBreakerState)
}

// NewCircuitBreaker creates a new circuit breaker with the given parameters.
// It panics if failureThreshold <= 0 or resetTimeout <= 0.
//
// Example:
//
//	breaker := NewCircuitBreaker(3, 5*time.Second)	// 3 failures in 5 seconds
func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) CircuitBreaker {
	if failureThreshold <= 0 {
		panic("failureThreshold must be greater than 0")
	}
	if resetTimeout <= 0 {
		panic("resetTimeout must be greater than 0")
	}
	return NewCircuitBreakerWithStorage(
		failureThreshold,
		resetTimeout,
		NewInMemoryStateRepository(),
	)
}

// NewCircuitBreakerWithStorage creates a new circuit breaker with custom storage.
// It panics if failureThreshold <= 0, resetTimeout <= 0, or storage is nil.
//
// Example:
//
//	breaker := NewCircuitBreakerWithStorage(
//		3,
//		5*time.Second,
//		NewInMemoryStateRepository(), // Or your own implementation of breaker.StateRepository
//	)	// 3 failures in 5 seconds
func NewCircuitBreakerWithStorage(failureThreshold int, resetTimeout time.Duration, storage StateRepository) CircuitBreaker {
	if failureThreshold <= 0 {
		panic("failureThreshold must be greater than 0")
	}
	if resetTimeout <= 0 {
		panic("resetTimeout must be greater than 0")
	}
	if storage == nil {
		panic("storage must not be nil")
	}
	cb := &CircuitBreakerImpl{
		state:            Closed,
		currentState:     &ClosedState{},
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		storage:          storage,
	}
	cb.currentState.OnEntry(cb)
	return cb
}

func (c *CircuitBreakerImpl) Execute(action func() (interface{}, error)) (interface{}, error) {
	// Get current state with mutex protection
	c.mutex.Lock()
	state := c.currentState
	c.mutex.Unlock()

	// Delegate to the current state
	return state.Execute(c, action)
}

// ExecuteWithContext runs the action with context support
func (c *CircuitBreakerImpl) ExecuteWithContext(ctx context.Context, action func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Get current state with mutex protection
	c.mutex.Lock()
	state := c.currentState
	c.mutex.Unlock()

	// Wrap the action to pass context
	wrappedAction := func() (interface{}, error) {
		return action(ctx)
	}

	// Delegate to the current state
	return state.Execute(c, wrappedAction)
}

func (c *CircuitBreakerImpl) State() CircuitBreakerState {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if we need to transition from Open to HalfOpen
	if c.currentState.Name() == Open && time.Since(c.lastFailure) >= c.resetTimeout {
		c.transitionTo(&HalfOpenState{})
	}

	return c.currentState.Name()
}

// transitionTo changes the state (must be called with mutex held)
func (c *CircuitBreakerImpl) transitionTo(newState State) {
	if c.currentState != nil {
		c.currentState.OnExit(c)
	}

	oldState := c.state
	c.currentState = newState
	c.state = newState.Name()
	c.storage.Save(c.state)

	c.currentState.OnEntry(c)

	// Notify observers (in background to avoid blocking)
	go c.notifyStateChange(oldState, c.state)
}

// notifyStateChange notifies all observers about a state change
func (c *CircuitBreakerImpl) notifyStateChange(from, to CircuitBreakerState) {
	// Call the callback if set (with panic recovery)
	if c.onStateChange != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Callback panicked, ignore and continue
				}
			}()
			c.onStateChange(from, to)
		}()
	}

	// Notify all observers (with panic recovery for each)
	ctx := context.Background()
	for _, observer := range c.observers {
		func(obs StateObserver) {
			defer func() {
				if r := recover(); r != nil {
					// Observer panicked, ignore and continue
				}
			}()
			obs.OnStateChange(ctx, from, to)
		}(observer)
	}
}

func (c *CircuitBreakerImpl) Reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Reset the failure count
	c.failureCount = 0

	// Reset the last failure time
	c.lastFailure = time.Time{}

	// Transition to closed state
	c.transitionTo(&ClosedState{})
}
