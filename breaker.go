package breaker

import (
	"errors"
	"sync"
	"time"
)

type CircuitBreakerState int

const (
	Closed CircuitBreakerState = iota
	Open
	HalfOpen
)

var (
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
)

type CircuitBreaker interface {
	// Execute runs the given action if the CircuitBreaker is closed or half-open,
	// else returns an error immediately.
	Execute(action func() (interface{}, error)) (interface{}, error)

	// State returns the current state of the CircuitBreaker.
	State() CircuitBreakerState

	// Reset changes the state of the CircuitBreaker to closed without any
	// concurrency safety. This should only be used in test code.
	Reset()
}

type CircuitBreakerImpl struct {
	state            CircuitBreakerState
	failureCount     int
	failureThreshold int
	resetTimeout     time.Duration
	mutex            sync.Mutex
	lastFailure      time.Time
	storage          StateRepository
}

// NewCircuitBreaker
// Example:
//
//	breaker := NewCircuitBreaker(3, 5*time.Second)	// 3 failures in 5 seconds
func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) CircuitBreaker {
	return NewCircuitBreakerWithStorage(
		failureThreshold,
		resetTimeout,
		NewInMemoryStateRepository(),
	)
}

// NewCircuitBreakerWithStorage
// Example:
//
//	breaker := NewCircuitBreakerWithStorage(
//		3,
//		5*time.Second,
//		NewInMemoryStateRepository(), // Or your own implementation of breaker.StateRepository
//	)	// 3 failures in 5 seconds
func NewCircuitBreakerWithStorage(failureThreshold int, resetTimeout time.Duration, storage StateRepository) CircuitBreaker {
	return &CircuitBreakerImpl{
		state:            Closed,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		storage:          storage,
	}
}

func (c *CircuitBreakerImpl) Execute(action func() (interface{}, error)) (interface{}, error) {
	currentState := c.State()

	if currentState == Open {
		return nil, ErrCircuitBreakerOpen
	}

	if currentState == Closed || currentState == HalfOpen {
		// If the CircuitBreaker is closed or half-open, execute the action.
		result, err := action()
		if err != nil {
			// If the action returns an error, increment the failure count.
			c.failureCount++

			// If the failure count has reached the threshold, set the state to open.
			if c.failureCount >= c.failureThreshold {
				c.setState(Open)
				c.lastFailure = time.Now()
			}
		} else {
			// If the action returns no error, reset the failure count and set the state to closed.
			c.failureCount = 0
			c.setState(Closed)
		}

		return result, err
	}

	return nil, nil
}

func (c *CircuitBreakerImpl) State() CircuitBreakerState {
	// If the state is not closed, load the current state from the storage.
	if c.state != Closed {
		c.state = c.storage.Load()
	}

	if c.state == Open {
		// If the CircuitBreaker is open, check if the reset timeout has passed.
		if time.Since(c.lastFailure) >= c.resetTimeout {
			// If the reset timeout has passed, set the state to half-open.
			c.setState(HalfOpen)
		}
	}

	// Load the current state from the storage.
	return c.state
}

func (c *CircuitBreakerImpl) Reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Reset the failure count
	c.failureCount = 0

	// Reset the last failure time
	c.lastFailure = time.Time{}

	// Reset the state to closed
	c.setState(Closed)
}

// setState is a helper function to set the state of the CircuitBreaker
func (c *CircuitBreakerImpl) setState(state CircuitBreakerState) {
	c.state = state
	c.storage.Save(c.state)
}
