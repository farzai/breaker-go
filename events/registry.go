package events

import (
	"fmt"
	"sync"
)

// Registry manages the lifecycle of event listeners in a thread-safe manner.
// It provides methods for subscribing, unsubscribing, and notifying listeners.
//
// Registry uses the Observer pattern with proper lifecycle management,
// allowing listeners to be added and removed at runtime without race conditions.
type Registry struct {
	// listeners maps subscription IDs to their EventListener implementations
	listeners map[string]EventListener

	// mu protects concurrent access to the listeners map
	mu sync.RWMutex

	// nextID is used to generate unique subscription IDs
	nextID int
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		listeners: make(map[string]EventListener),
		nextID:    1,
	}
}

// Subscribe registers a new event listener and returns a Subscription.
// The subscription can be used to unsubscribe later.
//
// If id is empty, a unique ID will be generated automatically.
// If a listener with the same ID already exists, it will be replaced.
//
// Thread-safe: can be called concurrently with other Registry methods.
//
// Example:
//
//	registry := NewRegistry()
//	sub := registry.Subscribe("", myListener)
//	defer sub.Unsubscribe()
func (r *Registry) Subscribe(id string, listener EventListener) *Subscription {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate ID if not provided
	if id == "" {
		id = fmt.Sprintf("listener-%d", r.nextID)
		r.nextID++
	}

	r.listeners[id] = listener

	// Create subscription with unsubscribe callback
	sub := &Subscription{
		ID: id,
		unsubscribe: func() {
			r.Unsubscribe(id)
		},
	}

	return sub
}

// Unsubscribe removes a listener by its subscription ID.
// Returns an error if the subscription ID doesn't exist.
//
// Thread-safe: can be called concurrently with other Registry methods.
func (r *Registry) Unsubscribe(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.listeners[id]; !exists {
		return fmt.Errorf("subscription not found: %s", id)
	}

	delete(r.listeners, id)
	return nil
}

// UnsubscribeAll removes all listeners from the registry.
// Useful for cleanup and testing.
//
// Thread-safe: can be called concurrently with other Registry methods.
func (r *Registry) UnsubscribeAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.listeners = make(map[string]EventListener)
}

// Count returns the number of currently registered listeners.
//
// Thread-safe: can be called concurrently with other Registry methods.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.listeners)
}

// GetListeners returns a snapshot of all current listeners.
// The returned slice is a copy and safe to iterate over even if
// listeners are added/removed concurrently.
//
// Thread-safe: can be called concurrently with other Registry methods.
func (r *Registry) GetListeners() []ListenerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a snapshot to avoid holding the lock during iteration
	listeners := make([]ListenerInfo, 0, len(r.listeners))
	for id, listener := range r.listeners {
		listeners = append(listeners, ListenerInfo{
			ID:       id,
			Listener: listener,
		})
	}

	return listeners
}

// ListenerInfo contains information about a registered listener.
type ListenerInfo struct {
	ID       string
	Listener EventListener
}

// Notify calls all registered listeners with the given event.
// This is a synchronous operation that calls each listener in sequence.
//
// For asynchronous notification, use this method with a DeliveryStrategy
// via the EventBus.
//
// Returns an error if any listener returns an error. The error will be
// a MultiError if multiple listeners fail.
//
// Thread-safe: uses a snapshot of listeners to avoid holding locks during notification.
func (r *Registry) Notify(event StateChangeEvent) error {
	// Get snapshot of listeners without holding the lock
	listeners := r.GetListeners()

	var errors []error
	for _, info := range listeners {
		if err := info.Listener.OnEvent(event); err != nil {
			errors = append(errors, fmt.Errorf("listener %s: %w", info.ID, err))
		}
	}

	if len(errors) > 0 {
		return &MultiError{Errors: errors}
	}

	return nil
}

// MultiError represents multiple errors from multiple listeners.
type MultiError struct {
	Errors []error
}

// Error implements the error interface
func (m *MultiError) Error() string {
	if len(m.Errors) == 0 {
		return "no errors"
	}
	if len(m.Errors) == 1 {
		return m.Errors[0].Error()
	}
	return fmt.Sprintf("%d errors occurred: %v", len(m.Errors), m.Errors[0])
}

// Unwrap returns the underlying errors for error unwrapping
func (m *MultiError) Unwrap() []error {
	return m.Errors
}
