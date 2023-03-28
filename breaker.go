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

	if c.state == Open {
		return nil, ErrCircuitBreakerOpen
	}

	result, err := action()
	if err != nil {
		c.failureCount++
		if c.state == Closed && c.failureCount >= c.failureThreshold {
			c.state = Open
			go c.reset()
		}
	} else {
		c.failureCount = 0
		if c.state == HalfOpen {
			c.state = Closed
		}
	}

	return result, err
}

// State returns the current state of the circuit breaker.
func (c *CircuitBreakerImpl) State() CircuitBreakerState {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.state
}

// Reset resets the circuit breaker to its initial state.
func (c *CircuitBreakerImpl) Reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.failureCount = 0
	c.state = Closed
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
