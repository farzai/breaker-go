package breaker

import (
	"context"
	"time"
)

// State represents the behavioral interface for circuit breaker states
type State interface {
	// Execute attempts to execute the action based on the current state
	// The context is used for event propagation and tracing
	Execute(ctx context.Context, cb *CircuitBreakerImpl, action func() (interface{}, error)) (interface{}, error)

	// OnEntry is called when transitioning into this state
	OnEntry(cb *CircuitBreakerImpl)

	// OnExit is called when transitioning out of this state
	OnExit(cb *CircuitBreakerImpl)

	// Name returns the name of the state
	Name() CircuitBreakerState
}

// ClosedState represents the closed state where requests are allowed
type ClosedState struct{}

func (s *ClosedState) Execute(ctx context.Context, cb *CircuitBreakerImpl, action func() (interface{}, error)) (interface{}, error) {
	result, err := action()

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		cb.failureCount++
		if cb.failureCount >= cb.failureThreshold {
			cb.transitionTo(ctx, &OpenState{})
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

func (s *OpenState) Execute(ctx context.Context, cb *CircuitBreakerImpl, action func() (interface{}, error)) (interface{}, error) {
	cb.mutex.Lock()

	// Check if reset timeout has passed
	if time.Since(cb.lastFailure) >= cb.resetTimeout {
		// Transition to half-open state
		cb.transitionTo(ctx, &HalfOpenState{})
		// Get the new state and execute with it
		newState := cb.currentState
		cb.mutex.Unlock()
		// Execute with the half-open state
		return newState.Execute(ctx, cb, action)
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

func (s *HalfOpenState) Execute(ctx context.Context, cb *CircuitBreakerImpl, action func() (interface{}, error)) (interface{}, error) {
	result, err := action()

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		// Failure in half-open state, go back to open
		cb.transitionTo(ctx, &OpenState{})
	} else {
		// Success in half-open state, close the circuit
		cb.transitionTo(ctx, &ClosedState{})
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
