package breaker

import (
	"time"
)

// State represents the behavioral interface for circuit breaker states
type State interface {
	// Execute attempts to execute the action based on the current state
	Execute(cb *CircuitBreakerImpl, action func() (interface{}, error)) (interface{}, error)

	// OnEntry is called when transitioning into this state
	OnEntry(cb *CircuitBreakerImpl)

	// OnExit is called when transitioning out of this state
	OnExit(cb *CircuitBreakerImpl)

	// Name returns the name of the state
	Name() CircuitBreakerState
}

// ClosedState represents the closed state where requests are allowed
type ClosedState struct{}

func (s *ClosedState) Execute(cb *CircuitBreakerImpl, action func() (interface{}, error)) (interface{}, error) {
	result, err := action()

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.transitionTo(&OpenState{})
		}
	} else {
		cb.failureCount = 0
	}

	return result, err
}

func (s *ClosedState) OnEntry(cb *CircuitBreakerImpl) {
	cb.failureCount = 0
}

func (s *ClosedState) OnExit(cb *CircuitBreakerImpl) {
	// No cleanup needed
}

func (s *ClosedState) Name() CircuitBreakerState {
	return Closed
}

// OpenState represents the open state where requests are blocked
type OpenState struct{}

func (s *OpenState) Execute(cb *CircuitBreakerImpl, action func() (interface{}, error)) (interface{}, error) {
	cb.mutex.Lock()

	// Check if reset timeout has passed
	if time.Since(cb.lastFailure) >= cb.resetTimeout {
		cb.transitionTo(&HalfOpenState{})
		cb.mutex.Unlock()
		// Retry in half-open state
		return cb.currentState.Execute(cb, action)
	}

	cb.mutex.Unlock()
	return nil, ErrCircuitBreakerOpen
}

func (s *OpenState) OnEntry(cb *CircuitBreakerImpl) {
	cb.lastFailure = time.Now()
}

func (s *OpenState) OnExit(cb *CircuitBreakerImpl) {
	// No cleanup needed
}

func (s *OpenState) Name() CircuitBreakerState {
	return Open
}

// HalfOpenState represents the half-open state where limited requests are allowed
type HalfOpenState struct{}

func (s *HalfOpenState) Execute(cb *CircuitBreakerImpl, action func() (interface{}, error)) (interface{}, error) {
	result, err := action()

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		// Failure in half-open state, go back to open
		cb.transitionTo(&OpenState{})
	} else {
		// Success in half-open state, close the circuit
		cb.transitionTo(&ClosedState{})
	}

	return result, err
}

func (s *HalfOpenState) OnEntry(cb *CircuitBreakerImpl) {
	// No initialization needed
}

func (s *HalfOpenState) OnExit(cb *CircuitBreakerImpl) {
	// No cleanup needed
}

func (s *HalfOpenState) Name() CircuitBreakerState {
	return HalfOpen
}
