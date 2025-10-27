// Package events provides a flexible, extensible event system for circuit breaker state changes.
//
// This package implements several design patterns:
//   - Event Bus: Central event dispatcher for decoupling
//   - Observer Registry: Thread-safe lifecycle management
//   - Strategy Pattern: Pluggable delivery mechanisms
//   - Middleware Pattern: Composable cross-cutting concerns
//   - Decorator Pattern: Enhanced observer capabilities
package events

import (
	"context"
	"time"
)

// CircuitBreakerState represents the circuit breaker state.
// This is a copy of the state from the parent package to avoid circular dependencies.
type CircuitBreakerState int

const (
	// Closed state: requests are allowed and failures are counted
	Closed CircuitBreakerState = iota
	// Open state: requests are blocked to allow recovery
	Open
	// HalfOpen state: testing if the service has recovered
	HalfOpen
)

// String implements fmt.Stringer for CircuitBreakerState.
// This provides a human-readable representation of the state.
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

// StateChangeEvent represents an immutable event that occurs when the circuit breaker
// state changes. It carries rich context about the state transition.
type StateChangeEvent struct {
	// From is the previous state
	From CircuitBreakerState

	// To is the new state
	To CircuitBreakerState

	// Timestamp when the state change occurred
	Timestamp time.Time

	// Context associated with the execution that triggered this state change.
	// This allows observers to access tracing, cancellation, and other contextual information.
	Context context.Context

	// Metadata contains additional information about the state change.
	// Common keys: "failure_count", "threshold", "last_failure_time"
	Metadata map[string]interface{}
}

// EventListener is the unified interface for receiving circuit breaker events.
// It replaces both the StateObserver interface and callback functions from the legacy API.
//
// Implementations should:
//   - Be thread-safe if used concurrently
//   - Return quickly (use goroutines for heavy processing)
//   - Return an error if the event cannot be processed
//   - Not panic (panics are recovered by middleware)
type EventListener interface {
	// OnEvent is called when a state change event occurs.
	// The error return allows listeners to signal processing failures,
	// which can be logged or handled by middleware.
	OnEvent(event StateChangeEvent) error
}

// EventListenerFunc is a function adapter that implements EventListener.
// This allows using plain functions as event listeners.
//
// Example:
//
//	listener := EventListenerFunc(func(event StateChangeEvent) error {
//	    log.Printf("State changed: %v -> %v", event.From, event.To)
//	    return nil
//	})
type EventListenerFunc func(StateChangeEvent) error

// OnEvent implements EventListener for EventListenerFunc
func (f EventListenerFunc) OnEvent(event StateChangeEvent) error {
	return f(event)
}

// Subscription represents a subscription to events.
// It can be used to manage the lifecycle of event listeners.
type Subscription struct {
	// ID is the unique identifier for this subscription
	ID string

	// unsubscribe is the function to call to remove this subscription
	unsubscribe func()
}

// Unsubscribe removes this subscription from the event bus.
// After calling Unsubscribe, this listener will no longer receive events.
// Calling Unsubscribe multiple times is safe (idempotent).
func (s *Subscription) Unsubscribe() {
	if s.unsubscribe != nil {
		s.unsubscribe()
		s.unsubscribe = nil
	}
}

// PanicHandler is called when a listener panics during event processing.
// It receives the recovered panic value and the event that caused the panic.
type PanicHandler func(recovered interface{}, event StateChangeEvent)

// ErrorHandler is called when a listener returns an error.
type ErrorHandler func(err error, event StateChangeEvent, listenerID string)

// MetricsRecorder is an interface for recording event processing metrics.
type MetricsRecorder interface {
	// RecordEventProcessed is called after an event is processed
	RecordEventProcessed(duration time.Duration, success bool)

	// RecordListenerError is called when a listener returns an error
	RecordListenerError(listenerID string, err error)

	// RecordListenerPanic is called when a listener panics
	RecordListenerPanic(listenerID string, recovered interface{})
}
