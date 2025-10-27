package breaker

import (
	"errors"
	"testing"
	"time"
)

// TestClosedStateLifecycle tests ClosedState OnEntry and OnExit
func TestClosedStateLifecycle(t *testing.T) {
	t.Run("OnEntry should reset failure count", func(t *testing.T) {
		cb := &CircuitBreakerImpl{
			state:            Closed,
			currentState:     &ClosedState{},
			failureThreshold: 3,
			resetTimeout:     1 * time.Second,
			storage:          NewInMemoryStateRepository(),
			failureCount:     5, // Set a high failure count
		}

		state := &ClosedState{}
		state.OnEntry(cb)

		if cb.failureCount != 0 {
			t.Errorf("Expected failure count to be reset to 0, got %d", cb.failureCount)
		}
	})

	t.Run("OnExit should be callable", func(t *testing.T) {
		cb := &CircuitBreakerImpl{
			state:            Closed,
			currentState:     &ClosedState{},
			failureThreshold: 3,
			resetTimeout:     1 * time.Second,
			storage:          NewInMemoryStateRepository(),
		}

		state := &ClosedState{}
		// Should not panic
		state.OnExit(cb)
	})

	t.Run("Name should return Closed", func(t *testing.T) {
		state := &ClosedState{}
		if state.Name() != Closed {
			t.Errorf("Expected state name to be Closed, got %v", state.Name())
		}
	})
}

// TestOpenStateLifecycle tests OpenState OnEntry and OnExit
func TestOpenStateLifecycle(t *testing.T) {
	t.Run("OnEntry should set lastFailure time", func(t *testing.T) {
		cb := &CircuitBreakerImpl{
			state:            Open,
			currentState:     &OpenState{},
			failureThreshold: 3,
			resetTimeout:     1 * time.Second,
			storage:          NewInMemoryStateRepository(),
			lastFailure:      time.Time{}, // Zero time
		}

		beforeTime := time.Now()
		state := &OpenState{}
		state.OnEntry(cb)
		afterTime := time.Now()

		if cb.lastFailure.Before(beforeTime) || cb.lastFailure.After(afterTime) {
			t.Errorf("Expected lastFailure to be set to current time")
		}
	})

	t.Run("OnExit should be callable", func(t *testing.T) {
		cb := &CircuitBreakerImpl{
			state:            Open,
			currentState:     &OpenState{},
			failureThreshold: 3,
			resetTimeout:     1 * time.Second,
			storage:          NewInMemoryStateRepository(),
		}

		state := &OpenState{}
		// Should not panic
		state.OnExit(cb)
	})

	t.Run("Name should return Open", func(t *testing.T) {
		state := &OpenState{}
		if state.Name() != Open {
			t.Errorf("Expected state name to be Open, got %v", state.Name())
		}
	})

	t.Run("Execute should transition to HalfOpen after timeout", func(t *testing.T) {
		cb := &CircuitBreakerImpl{
			state:            Open,
			currentState:     &OpenState{},
			failureThreshold: 3,
			resetTimeout:     100 * time.Millisecond,
			storage:          NewInMemoryStateRepository(),
			lastFailure:      time.Now().Add(-200 * time.Millisecond), // Past timeout
		}

		state := &OpenState{}
		result, err := state.Execute(cb, func() (interface{}, error) {
			return "success", nil
		})

		if err != nil {
			t.Errorf("Expected no error after transition to HalfOpen, got %v", err)
		}

		if result != "success" {
			t.Errorf("Expected result to be 'success', got %v", result)
		}

		// Note: The state will transition to HalfOpen, then execute the action successfully,
		// which causes it to transition to Closed. This is the expected behavior.
		if cb.state != Closed {
			t.Errorf("Expected state to be Closed after successful execution, got %v", cb.state)
		}
	})
}

// TestHalfOpenStateLifecycle tests HalfOpenState OnEntry and OnExit
func TestHalfOpenStateLifecycle(t *testing.T) {
	t.Run("OnEntry should be callable", func(t *testing.T) {
		cb := &CircuitBreakerImpl{
			state:            HalfOpen,
			currentState:     &HalfOpenState{},
			failureThreshold: 3,
			resetTimeout:     1 * time.Second,
			storage:          NewInMemoryStateRepository(),
		}

		state := &HalfOpenState{}
		// Should not panic
		state.OnEntry(cb)
	})

	t.Run("OnExit should be callable", func(t *testing.T) {
		cb := &CircuitBreakerImpl{
			state:            HalfOpen,
			currentState:     &HalfOpenState{},
			failureThreshold: 3,
			resetTimeout:     1 * time.Second,
			storage:          NewInMemoryStateRepository(),
		}

		state := &HalfOpenState{}
		// Should not panic
		state.OnExit(cb)
	})

	t.Run("Name should return HalfOpen", func(t *testing.T) {
		state := &HalfOpenState{}
		if state.Name() != HalfOpen {
			t.Errorf("Expected state name to be HalfOpen, got %v", state.Name())
		}
	})

	t.Run("Execute should transition to Closed on success", func(t *testing.T) {
		cb := &CircuitBreakerImpl{
			state:            HalfOpen,
			currentState:     &HalfOpenState{},
			failureThreshold: 3,
			resetTimeout:     1 * time.Second,
			storage:          NewInMemoryStateRepository(),
		}

		state := &HalfOpenState{}
		result, err := state.Execute(cb, func() (interface{}, error) {
			return "success", nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if result != "success" {
			t.Errorf("Expected result to be 'success', got %v", result)
		}

		// Verify transition to Closed happened
		if cb.state != Closed {
			t.Errorf("Expected state to be Closed, got %v", cb.state)
		}
	})

	t.Run("Execute should transition to Open on failure", func(t *testing.T) {
		cb := &CircuitBreakerImpl{
			state:            HalfOpen,
			currentState:     &HalfOpenState{},
			failureThreshold: 3,
			resetTimeout:     1 * time.Second,
			storage:          NewInMemoryStateRepository(),
		}

		state := &HalfOpenState{}
		result, err := state.Execute(cb, func() (interface{}, error) {
			return nil, errors.New("error")
		})

		if err == nil {
			t.Error("Expected error, got nil")
		}

		if result != nil {
			t.Errorf("Expected result to be nil, got %v", result)
		}

		// Verify transition to Open happened
		if cb.state != Open {
			t.Errorf("Expected state to be Open, got %v", cb.state)
		}
	})
}

// TestStateTransitions tests the transitionTo method
func TestStateTransitions(t *testing.T) {
	t.Run("Should call OnExit on old state and OnEntry on new state", func(t *testing.T) {
		cb := &CircuitBreakerImpl{
			state:            Closed,
			currentState:     &ClosedState{},
			failureThreshold: 3,
			resetTimeout:     1 * time.Second,
			storage:          NewInMemoryStateRepository(),
			failureCount:     5,
		}

		// Transition to Open state
		cb.transitionTo(&OpenState{})

		// Verify state changed
		if cb.state != Open {
			t.Errorf("Expected state to be Open, got %v", cb.state)
		}

		// Verify lastFailure was set by OnEntry
		if cb.lastFailure.IsZero() {
			t.Error("Expected lastFailure to be set")
		}

		// Verify state was saved to storage
		if cb.storage.Load() != Open {
			t.Errorf("Expected storage to contain Open state, got %v", cb.storage.Load())
		}
	})
}
