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
	// Execute executes the given function and returns the result or an error.
	Execute(action func() (interface{}, error)) (interface{}, error)

	// State returns the current state of the circuit breaker.
	State() CircuitBreakerState

	// Reset resets the circuit breaker to its initial state.
	Reset()
}

type CircuitBreakerImpl struct {
	state            CircuitBreakerState
	failureCount     int
	failureThreshold int
	resetTimeout     time.Duration
	mutex            sync.Mutex

	lastFailure time.Time
}

func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) CircuitBreaker {
	return &CircuitBreakerImpl{
		state:            Closed,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
	}
}

func (c *CircuitBreakerImpl) Execute(action func() (interface{}, error)) (interface{}, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// If the circuit breaker is open,
	// Check if the reset timeout has elapsed since the last failure.
	// If so, transition the state from Open to HalfOpen.
	if c.state == Open {
		if time.Since(c.lastFailure) >= c.resetTimeout {
			c.state = HalfOpen
		} else {
			return nil, ErrCircuitBreakerOpen
		}
	}

	// If the circuit breaker is half open,
	// Execute the action.
	// If the action succeeds, transition the state from HalfOpen to Closed.
	// If the action fails, transition the state from HalfOpen to Open.
	if c.state == HalfOpen {
		result, err := action()
		if err != nil {
			c.state = Open
			c.lastFailure = time.Now()
			return nil, err
		} else {
			c.state = Closed
			return result, nil
		}
	}

	// If the circuit breaker is closed,
	// Execute the action.
	// If the action fails, increment the failure count.
	// If the failure count exceeds the failure threshold, transition the state from Closed to Open.
	if c.state == Closed {
		result, err := action()
		if err != nil {
			c.failureCount++
			if c.failureCount >= c.failureThreshold {
				c.state = Open
				c.lastFailure = time.Now()
			}
			return nil, err
		} else {
			return result, nil
		}
	}

	return nil, nil
}

// State returns the current state of the circuit breaker.
func (c *CircuitBreakerImpl) State() CircuitBreakerState {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.state == Open {
		if time.Since(c.lastFailure) >= c.resetTimeout {
			// If the reset timeout has elapsed since the last failure,
			// transition the state from Open to HalfOpen.
			c.state = HalfOpen

			// Start a background goroutine which sleeps for the reset timeout duration,
			// and then transitions the state from HalfOpen to Closed.
			go c.reset()

			return c.state
		}
	}

	return c.state
}

// Reset resets the circuit breaker to its initial state.
func (c *CircuitBreakerImpl) Reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.failureCount = 0
	c.state = Closed
	c.lastFailure = time.Time{}
}

// reset is a background goroutine which sleeps for the resetTimeout
// duration, and then transitions the state from Open to HalfOpen.
func (c *CircuitBreakerImpl) reset() {
	time.Sleep(c.resetTimeout)
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.state == Open {
		c.state = HalfOpen
	}
}
