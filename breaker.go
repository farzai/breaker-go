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
	Execute(action func() (interface{}, error)) (interface{}, error)
	State() CircuitBreakerState
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

func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) CircuitBreaker {
	return NewCircuitBreakerWithStorage(
		failureThreshold,
		resetTimeout,
		NewInMemoryStateRepository(),
	)
}

func NewCircuitBreakerWithStorage(failureThreshold int, resetTimeout time.Duration, storage StateRepository) CircuitBreaker {
	return &CircuitBreakerImpl{
		state:            Closed,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		storage:          storage,
	}
}

func (c *CircuitBreakerImpl) Execute(action func() (interface{}, error)) (interface{}, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.state = c.storage.Load()

	if c.state == Open {
		if time.Since(c.lastFailure) >= c.resetTimeout {
			c.state = HalfOpen
		} else {
			return nil, ErrCircuitBreakerOpen
		}
	}

	if c.state == HalfOpen {
		result, err := action()
		if err != nil {
			c.state = Open
			c.lastFailure = time.Now()
			c.storage.Save(c.state)
			return nil, err
		} else {
			c.state = Closed
			c.storage.Save(c.state)
			return result, nil
		}
	}

	if c.state == Closed {
		result, err := action()
		if err != nil {
			c.failureCount++
			if c.failureCount >= c.failureThreshold {
				c.state = Open
				c.lastFailure = time.Now()
				c.storage.Save(c.state)
			}
			return nil, err
		} else {
			c.storage.Save(c.state)
			return result, nil
		}
	}

	return nil, nil
}

func (c *CircuitBreakerImpl) State() CircuitBreakerState {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.state = c.storage.Load()

	if c.state == Open {
		if time.Since(c.lastFailure) >= c.resetTimeout {
			c.state = HalfOpen
			c.storage.Save(c.state)
		}
	}

	return c.state
}

func (c *CircuitBreakerImpl) Reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.failureCount = 0
	c.state = Closed
	c.lastFailure = time.Time{}
	c.storage.Reset()
}
