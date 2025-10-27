package events

import (
	"context"
	"time"

	"github.com/farzai/breaker-go/logging"
)

// EventBus is the central dispatcher for circuit breaker events.
// It combines:
//   - Registry: Thread-safe listener management
//   - Strategy: Pluggable delivery mechanism (sync/async/parallel)
//   - Middleware: Composable cross-cutting concerns
//
// EventBus decouples the circuit breaker from event notification logic,
// making the system more maintainable and testable.
//
// Example:
//
//	bus := NewEventBus(
//	    WithSynchronousDelivery(),
//	    WithPanicRecovery(panicHandler),
//	    WithLogging(logger),
//	)
//
//	subscription := bus.Subscribe(myListener)
//	defer subscription.Unsubscribe()
//
//	bus.Publish(StateChangeEvent{From: Closed, To: Open})
type EventBus struct {
	registry   *Registry
	strategy   DeliveryStrategy
	middleware []Middleware
}

// NewEventBus creates a new EventBus with the given options.
// If no options are provided, it uses sensible defaults:
//   - AsynchronousStrategy for non-blocking delivery
//   - PanicRecoveryMiddleware to prevent crashes
//
// Example:
//
//	bus := NewEventBus()
func NewEventBus(options ...BusOption) *EventBus {
	bus := &EventBus{
		registry:   NewRegistry(),
		strategy:   NewAsynchronousStrategy(), // Default: async
		middleware: []Middleware{},
	}

	// Apply default middleware: panic recovery
	bus.middleware = append(bus.middleware, PanicRecoveryMiddleware(nil))

	// Apply user options
	for _, option := range options {
		option(bus)
	}

	return bus
}

// Subscribe registers a new event listener.
// Returns a Subscription that can be used to unsubscribe later.
//
// Thread-safe: can be called concurrently.
//
// Example:
//
//	sub := bus.Subscribe(myListener)
//	defer sub.Unsubscribe()
func (b *EventBus) Subscribe(listener EventListener) *Subscription {
	return b.SubscribeWithID("", listener)
}

// SubscribeWithID registers a new event listener with a specific ID.
// If id is empty, a unique ID will be generated.
// If a listener with the same ID already exists, it will be replaced.
//
// Thread-safe: can be called concurrently.
//
// Example:
//
//	sub := bus.SubscribeWithID("logger", loggingListener)
func (b *EventBus) SubscribeWithID(id string, listener EventListener) *Subscription {
	return b.registry.Subscribe(id, listener)
}

// Unsubscribe removes a listener by its subscription ID.
//
// Thread-safe: can be called concurrently.
func (b *EventBus) Unsubscribe(id string) error {
	return b.registry.Unsubscribe(id)
}

// UnsubscribeAll removes all listeners.
// Useful for cleanup and testing.
//
// Thread-safe: can be called concurrently.
func (b *EventBus) UnsubscribeAll() {
	b.registry.UnsubscribeAll()
}

// ListenerCount returns the number of registered listeners.
//
// Thread-safe: can be called concurrently.
func (b *EventBus) ListenerCount() int {
	return b.registry.Count()
}

// Publish publishes an event to all registered listeners using the configured strategy.
// The delivery behavior (sync/async/parallel) depends on the configured DeliveryStrategy.
//
// Returns an error if delivery fails (strategy-dependent).
//
// Thread-safe: can be called concurrently.
//
// Example:
//
//	err := bus.Publish(StateChangeEvent{
//	    From:      Closed,
//	    To:        Open,
//	    Timestamp: time.Now(),
//	    Context:   ctx,
//	})
func (b *EventBus) Publish(event StateChangeEvent) error {
	// Ensure event has required fields
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Context == nil {
		event.Context = context.Background()
	}
	if event.Metadata == nil {
		event.Metadata = make(map[string]interface{})
	}

	// Get snapshot of listeners
	listeners := b.registry.GetListeners()

	// If no listeners, nothing to do
	if len(listeners) == 0 {
		return nil
	}

	// Apply middleware to each listener
	wrappedListeners := make([]ListenerInfo, len(listeners))
	for i, info := range listeners {
		wrappedListeners[i] = ListenerInfo{
			ID:       info.ID,
			Listener: b.wrapListenerWithMiddleware(info.Listener, info.ID),
		}
	}

	// Deliver using the configured strategy
	return b.strategy.Deliver(event, wrappedListeners)
}

// wrapListenerWithMiddleware applies all configured middleware to a listener.
func (b *EventBus) wrapListenerWithMiddleware(listener EventListener, listenerID string) EventListener {
	// Convert EventListener to EventHandler
	handler := func(event StateChangeEvent) error {
		return listener.OnEvent(event)
	}

	// Apply middleware in order
	for _, mw := range b.middleware {
		handler = mw(handler)
	}

	// Convert back to EventListener
	return EventListenerFunc(handler)
}

// PublishAsync publishes an event asynchronously, regardless of the configured strategy.
// It always returns immediately without waiting for listeners.
//
// Errors from listeners are silently ignored. Use middleware to capture errors if needed.
//
// Thread-safe: can be called concurrently.
//
// Example:
//
//	bus.PublishAsync(event) // Fire and forget
func (b *EventBus) PublishAsync(event StateChangeEvent) {
	go func() {
		_ = b.Publish(event)
	}()
}

// Use adds middleware to the event bus.
// Middleware are applied in the order they are added.
//
// Not thread-safe: should only be called during initialization.
//
// Example:
//
//	bus.Use(LoggingMiddleware(logger))
//	bus.Use(MetricsMiddleware(recorder))
func (b *EventBus) Use(middleware Middleware) {
	b.middleware = append(b.middleware, middleware)
}

// SetStrategy changes the delivery strategy.
//
// Not thread-safe: should only be called during initialization.
//
// Example:
//
//	bus.SetStrategy(NewSynchronousStrategy())
func (b *EventBus) SetStrategy(strategy DeliveryStrategy) {
	if strategy != nil {
		b.strategy = strategy
	}
}

// BusOption is a functional option for configuring an EventBus.
type BusOption func(*EventBus)

// WithSynchronousDelivery configures the bus to use synchronous delivery.
// Events are delivered sequentially in the calling goroutine.
//
// Example:
//
//	bus := NewEventBus(WithSynchronousDelivery())
func WithSynchronousDelivery() BusOption {
	return func(b *EventBus) {
		b.strategy = NewSynchronousStrategy()
	}
}

// WithAsynchronousDelivery configures the bus to use asynchronous delivery (default).
// Events are delivered in separate goroutines (fire-and-forget).
//
// Example:
//
//	bus := NewEventBus(WithAsynchronousDelivery())
func WithAsynchronousDelivery() BusOption {
	return func(b *EventBus) {
		b.strategy = NewAsynchronousStrategy()
	}
}

// WithParallelDelivery configures the bus to use parallel delivery.
// All listeners execute concurrently, and the method waits for all to complete.
//
// Example:
//
//	bus := NewEventBus(WithParallelDelivery())
func WithParallelDelivery() BusOption {
	return func(b *EventBus) {
		b.strategy = NewParallelStrategy()
	}
}

// WithOrderedAsyncDelivery configures the bus to use ordered async delivery.
// Events are queued and processed sequentially in a background goroutine.
//
// Example:
//
//	bus := NewEventBus(WithOrderedAsyncDelivery(100))
func WithOrderedAsyncDelivery(bufferSize int) BusOption {
	return func(b *EventBus) {
		b.strategy = NewOrderedAsyncStrategy(bufferSize)
	}
}

// WithPanicRecovery adds panic recovery middleware.
//
// Example:
//
//	bus := NewEventBus(WithPanicRecovery(func(recovered interface{}, event StateChangeEvent) {
//	    log.Printf("Panic: %v", recovered)
//	}))
func WithPanicRecovery(handler PanicHandler) BusOption {
	return func(b *EventBus) {
		b.middleware = append(b.middleware, PanicRecoveryMiddleware(handler))
	}
}

// WithLogging adds structured logging middleware.
//
// Example:
//
//	logger := logging.NewDefaultLogger(logging.LevelInfo)
//	bus := NewEventBus(WithLogging(logger))
func WithLogging(logger logging.Logger) BusOption {
	return func(b *EventBus) {
		b.middleware = append(b.middleware, LoggingMiddleware(logger))
	}
}

// WithMetrics adds metrics middleware.
//
// Example:
//
//	bus := NewEventBus(WithMetrics(recorder))
func WithMetrics(recorder MetricsRecorder) BusOption {
	return func(b *EventBus) {
		b.middleware = append(b.middleware, MetricsMiddleware(recorder))
	}
}

// WithTimeout adds timeout middleware.
//
// Example:
//
//	bus := NewEventBus(WithTimeout(100*time.Millisecond))
func WithTimeout(timeout time.Duration) BusOption {
	return func(b *EventBus) {
		b.middleware = append(b.middleware, TimeoutMiddleware(timeout))
	}
}

// WithCustomMiddleware adds custom middleware.
//
// Example:
//
//	bus := NewEventBus(WithCustomMiddleware(myMiddleware))
func WithCustomMiddleware(middleware Middleware) BusOption {
	return func(b *EventBus) {
		b.middleware = append(b.middleware, middleware)
	}
}

// WithStrategy sets a custom delivery strategy.
//
// Example:
//
//	bus := NewEventBus(WithStrategy(myCustomStrategy))
func WithStrategy(strategy DeliveryStrategy) BusOption {
	return func(b *EventBus) {
		if strategy != nil {
			b.strategy = strategy
		}
	}
}
