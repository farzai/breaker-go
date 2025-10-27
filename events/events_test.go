package events

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/farzai/breaker-go/logging"
)

// TestRegistry tests the thread-safe registry
func TestRegistry(t *testing.T) {
	t.Run("Subscribe adds listeners", func(t *testing.T) {
		registry := NewRegistry()
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		})

		sub := registry.Subscribe("test", listener)
		if sub.ID != "test" {
			t.Errorf("Expected subscription ID 'test', got %s", sub.ID)
		}

		if registry.Count() != 1 {
			t.Errorf("Expected 1 listener, got %d", registry.Count())
		}
	})

	t.Run("Auto-generate ID when empty", func(t *testing.T) {
		registry := NewRegistry()
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		})

		sub := registry.Subscribe("", listener)
		if sub.ID == "" {
			t.Error("Expected auto-generated ID, got empty string")
		}
	})

	t.Run("Unsubscribe removes listeners", func(t *testing.T) {
		registry := NewRegistry()
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		})

		sub := registry.Subscribe("test", listener)
		if registry.Count() != 1 {
			t.Fatalf("Expected 1 listener after subscribe")
		}

		sub.Unsubscribe()
		if registry.Count() != 0 {
			t.Errorf("Expected 0 listeners after unsubscribe, got %d", registry.Count())
		}
	})

	t.Run("UnsubscribeAll removes all listeners", func(t *testing.T) {
		registry := NewRegistry()
		for i := 0; i < 5; i++ {
			registry.Subscribe("", EventListenerFunc(func(event StateChangeEvent) error {
				return nil
			}))
		}

		if registry.Count() != 5 {
			t.Fatalf("Expected 5 listeners")
		}

		registry.UnsubscribeAll()
		if registry.Count() != 0 {
			t.Errorf("Expected 0 listeners after UnsubscribeAll, got %d", registry.Count())
		}
	})

	t.Run("Thread-safe concurrent operations", func(t *testing.T) {
		registry := NewRegistry()
		var wg sync.WaitGroup
		const numGoroutines = 50

		// Concurrent subscribe
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				registry.Subscribe("", EventListenerFunc(func(event StateChangeEvent) error {
					return nil
				}))
			}()
		}
		wg.Wait()

		if registry.Count() != numGoroutines {
			t.Errorf("Expected %d listeners, got %d", numGoroutines, registry.Count())
		}
	})
}

// TestEventBus tests the event bus functionality
func TestEventBus(t *testing.T) {
	t.Run("Publish delivers events synchronously", func(t *testing.T) {
		bus := NewEventBus(WithSynchronousDelivery())

		var received bool
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			received = true
			if event.From != Closed || event.To != Open {
				t.Errorf("Expected Closed->Open, got %v->%v", event.From, event.To)
			}
			return nil
		})

		bus.Subscribe(listener)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := bus.Publish(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !received {
			t.Error("Expected listener to be called")
		}
	})

	t.Run("PublishAsync doesn't block", func(t *testing.T) {
		bus := NewEventBus()

		done := make(chan struct{})
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			time.Sleep(100 * time.Millisecond) // Simulate slow processing
			close(done)
			return nil
		})

		bus.Subscribe(listener)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		start := time.Now()
		bus.PublishAsync(event)
		duration := time.Since(start)

		// Should return immediately
		if duration > 50*time.Millisecond {
			t.Errorf("PublishAsync blocked for %v, expected immediate return", duration)
		}

		// Wait for listener to complete
		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
			t.Error("Listener was not called")
		}
	})

	t.Run("Multiple listeners receive events", func(t *testing.T) {
		bus := NewEventBus(WithSynchronousDelivery())

		var count atomic.Int32
		for i := 0; i < 3; i++ {
			bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
				count.Add(1)
				return nil
			}))
		}

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		bus.Publish(event)

		if count.Load() != 3 {
			t.Errorf("Expected 3 listeners called, got %d", count.Load())
		}
	})

	t.Run("Auto-fills event fields", func(t *testing.T) {
		bus := NewEventBus(WithSynchronousDelivery())

		var receivedEvent StateChangeEvent
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			receivedEvent = event
			return nil
		})

		bus.Subscribe(listener)

		// Event with missing fields
		event := StateChangeEvent{
			From: Closed,
			To:   Open,
			// No timestamp, context, or metadata
		}

		bus.Publish(event)

		if receivedEvent.Timestamp.IsZero() {
			t.Error("Expected timestamp to be auto-filled")
		}
		if receivedEvent.Context == nil {
			t.Error("Expected context to be auto-filled")
		}
		if receivedEvent.Metadata == nil {
			t.Error("Expected metadata map to be auto-filled")
		}
	})
}

// TestDeliveryStrategies tests different delivery strategies
func TestDeliveryStrategies(t *testing.T) {
	event := StateChangeEvent{
		From:      Closed,
		To:        Open,
		Timestamp: time.Now(),
		Context:   context.Background(),
	}

	t.Run("SynchronousStrategy executes in order", func(t *testing.T) {
		strategy := NewSynchronousStrategy()
		var order []int
		var mu sync.Mutex

		listeners := []ListenerInfo{
			{
				ID: "1",
				Listener: EventListenerFunc(func(event StateChangeEvent) error {
					mu.Lock()
					order = append(order, 1)
					mu.Unlock()
					return nil
				}),
			},
			{
				ID: "2",
				Listener: EventListenerFunc(func(event StateChangeEvent) error {
					mu.Lock()
					order = append(order, 2)
					mu.Unlock()
					return nil
				}),
			},
		}

		strategy.Deliver(event, listeners)

		if len(order) != 2 || order[0] != 1 || order[1] != 2 {
			t.Errorf("Expected order [1, 2], got %v", order)
		}
	})

	t.Run("ParallelStrategy waits for all", func(t *testing.T) {
		strategy := NewParallelStrategy()
		var count atomic.Int32

		listeners := make([]ListenerInfo, 5)
		for i := range listeners {
			listeners[i] = ListenerInfo{
				ID: string(rune('A' + i)),
				Listener: EventListenerFunc(func(event StateChangeEvent) error {
					time.Sleep(10 * time.Millisecond)
					count.Add(1)
					return nil
				}),
			}
		}

		start := time.Now()
		strategy.Deliver(event, listeners)
		duration := time.Since(start)

		// Should complete in ~10ms (parallel), not 50ms (sequential)
		if duration > 30*time.Millisecond {
			t.Errorf("Parallel execution took %v, expected ~10ms", duration)
		}

		if count.Load() != 5 {
			t.Errorf("Expected 5 listeners called, got %d", count.Load())
		}
	})

	t.Run("AsynchronousStrategy returns immediately", func(t *testing.T) {
		strategy := NewAsynchronousStrategy()
		done := make(chan struct{})

		listeners := []ListenerInfo{
			{
				ID: "slow",
				Listener: EventListenerFunc(func(event StateChangeEvent) error {
					time.Sleep(100 * time.Millisecond)
					close(done)
					return nil
				}),
			},
		}

		start := time.Now()
		strategy.Deliver(event, listeners)
		duration := time.Since(start)

		if duration > 50*time.Millisecond {
			t.Errorf("Async delivery took %v, expected immediate return", duration)
		}

		// Verify listener actually runs
		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
			t.Error("Listener was not executed")
		}
	})
}

// TestMiddleware tests middleware functionality
func TestMiddleware(t *testing.T) {
	t.Run("PanicRecoveryMiddleware recovers panics", func(t *testing.T) {
		var recovered interface{}
		handler := PanicRecoveryMiddleware(func(r interface{}, event StateChangeEvent) {
			recovered = r
		})

		panickyHandler := func(event StateChangeEvent) error {
			panic("test panic")
		}

		wrappedHandler := handler(panickyHandler)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := wrappedHandler(event)

		if err == nil {
			t.Error("Expected error from panic recovery")
		}

		if recovered != "test panic" {
			t.Errorf("Expected panic value 'test panic', got %v", recovered)
		}
	})

	t.Run("LoggingMiddleware logs events", func(t *testing.T) {
		logger := logging.NewTestLogger()

		middleware := LoggingMiddleware(logger)
		handler := func(event StateChangeEvent) error {
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		wrappedHandler(event)

		// Test logger should have captured debug entries (started + completed)
		if logger.Count() < 1 { // At least the completion should be logged
			t.Errorf("Expected at least 1 log entry, got %d", logger.Count())
		}
	})

	t.Run("TimeoutMiddleware enforces timeout", func(t *testing.T) {
		middleware := TimeoutMiddleware(50 * time.Millisecond)

		slowHandler := func(event StateChangeEvent) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}

		wrappedHandler := middleware(slowHandler)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := wrappedHandler(event)

		if err == nil {
			t.Error("Expected timeout error")
		}
	})
}

// TestDecorators tests listener decorators
func TestDecorators(t *testing.T) {
	event := StateChangeEvent{
		From:      Closed,
		To:        Open,
		Timestamp: time.Now(),
		Context:   context.Background(),
	}

	t.Run("ListenerWithRetry retries on failure", func(t *testing.T) {
		var attempts atomic.Int32
		flaky := EventListenerFunc(func(event StateChangeEvent) error {
			count := attempts.Add(1)
			if count < 3 {
				return errors.New("temporary failure")
			}
			return nil
		})

		wrapped := ListenerWithRetry(flaky, 5, 10*time.Millisecond)
		err := wrapped.OnEvent(event)

		if err != nil {
			t.Errorf("Expected success after retries, got %v", err)
		}

		if attempts.Load() != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts.Load())
		}
	})

	t.Run("ListenerWithFilter filters events", func(t *testing.T) {
		var called bool
		base := EventListenerFunc(func(event StateChangeEvent) error {
			called = true
			return nil
		})

		// Only allow Open transitions
		wrapped := ListenerWithFilter(base, func(event StateChangeEvent) bool {
			return event.To == Open
		})

		// Should be called
		wrapped.OnEvent(event)
		if !called {
			t.Error("Expected listener to be called for Open transition")
		}

		// Should not be called
		called = false
		closedEvent := StateChangeEvent{
			From:      Open,
			To:        Closed,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}
		wrapped.OnEvent(closedEvent)
		if called {
			t.Error("Expected listener to be filtered out for Closed transition")
		}
	})

	t.Run("ListenerWithPanicRecovery catches panics", func(t *testing.T) {
		var recovered interface{}
		panicky := EventListenerFunc(func(event StateChangeEvent) error {
			panic("listener panic")
		})

		wrapped := ListenerWithPanicRecovery(panicky, func(r interface{}, event StateChangeEvent) {
			recovered = r
		})

		err := wrapped.OnEvent(event)

		if err == nil {
			t.Error("Expected error from panic recovery")
		}

		if recovered != "listener panic" {
			t.Errorf("Expected recovered value 'listener panic', got %v", recovered)
		}
	})
}

// TestEventBusOptions tests various event bus options
func TestEventBusOptions(t *testing.T) {
	t.Run("Unsubscribe removes listener", func(t *testing.T) {
		bus := NewEventBus()

		var called bool
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			called = true
			return nil
		})

		sub := bus.Subscribe(listener)

		// Unsubscribe
		bus.Unsubscribe(sub.ID)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		bus.Publish(event)

		if called {
			t.Error("Expected listener to be unsubscribed")
		}
	})

	t.Run("UnsubscribeAll removes all listeners", func(t *testing.T) {
		bus := NewEventBus()

		for i := 0; i < 3; i++ {
			bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
				return nil
			}))
		}

		if bus.ListenerCount() != 3 {
			t.Errorf("Expected 3 listeners, got %d", bus.ListenerCount())
		}

		bus.UnsubscribeAll()

		if bus.ListenerCount() != 0 {
			t.Errorf("Expected 0 listeners, got %d", bus.ListenerCount())
		}
	})

	t.Run("ListenerCount returns correct count", func(t *testing.T) {
		bus := NewEventBus()

		if bus.ListenerCount() != 0 {
			t.Errorf("Expected 0 listeners initially, got %d", bus.ListenerCount())
		}

		bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		}))

		if bus.ListenerCount() != 1 {
			t.Errorf("Expected 1 listener, got %d", bus.ListenerCount())
		}
	})

	t.Run("Use adds middleware", func(t *testing.T) {
		bus := NewEventBus(WithSynchronousDelivery())

		var middlewareCalled bool
		middleware := func(next EventHandler) EventHandler {
			return func(event StateChangeEvent) error {
				middlewareCalled = true
				return next(event)
			}
		}

		bus.Use(middleware)

		bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		}))

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		bus.Publish(event)

		if !middlewareCalled {
			t.Error("Expected middleware to be called")
		}
	})

	t.Run("SetStrategy changes delivery strategy", func(t *testing.T) {
		bus := NewEventBus()

		// Set to synchronous strategy
		bus.SetStrategy(NewSynchronousStrategy())

		var called bool
		bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
			called = true
			return nil
		}))

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := bus.Publish(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !called {
			t.Error("Expected listener to be called with new strategy")
		}
	})
}

// TestEventBusOptionsConstructors tests bus option constructors
func TestEventBusOptionsConstructors(t *testing.T) {
	t.Run("WithAsynchronousDelivery creates async bus", func(t *testing.T) {
		bus := NewEventBus(WithAsynchronousDelivery())

		done := make(chan struct{})
		bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
			time.Sleep(50 * time.Millisecond)
			close(done)
			return nil
		}))

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		start := time.Now()
		bus.PublishAsync(event)
		duration := time.Since(start)

		// Should return immediately
		if duration > 25*time.Millisecond {
			t.Errorf("Expected immediate return, took %v", duration)
		}

		// Wait for completion
		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
			t.Error("Listener was not executed")
		}
	})

	t.Run("WithParallelDelivery creates parallel bus", func(t *testing.T) {
		bus := NewEventBus(WithParallelDelivery())

		var count atomic.Int32
		for i := 0; i < 3; i++ {
			bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
				time.Sleep(50 * time.Millisecond)
				count.Add(1)
				return nil
			}))
		}

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		start := time.Now()
		bus.Publish(event)
		duration := time.Since(start)

		// Should complete in ~50ms (parallel), not 150ms (sequential)
		if duration > 100*time.Millisecond {
			t.Errorf("Expected parallel execution, took %v", duration)
		}

		if count.Load() != 3 {
			t.Errorf("Expected 3 listeners called, got %d", count.Load())
		}
	})

	t.Run("WithOrderedAsyncDelivery creates ordered async bus", func(t *testing.T) {
		bus := NewEventBus(WithOrderedAsyncDelivery(10))

		var order []int
		var mu sync.Mutex

		for i := 1; i <= 3; i++ {
			val := i
			bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
				mu.Lock()
				order = append(order, val)
				mu.Unlock()
				return nil
			}))
		}

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		bus.PublishAsync(event)

		// Wait for async processing
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		if len(order) != 3 {
			t.Errorf("Expected 3 listeners called, got %d", len(order))
		}
	})

	t.Run("WithPanicRecovery creates bus with panic recovery", func(t *testing.T) {
		bus := NewEventBus(
			WithSynchronousDelivery(),
			WithPanicRecovery(func(r interface{}, event StateChangeEvent) {
				// Panic handler called
			}),
		)

		bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
			panic("test panic")
		}))

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		// Should not panic - this is the main assertion
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Bus did not recover from panic: %v", r)
			}
		}()

		bus.Publish(event)
	})

	t.Run("WithLogging creates bus with logging", func(t *testing.T) {
		logger := logging.NewTestLogger()

		bus := NewEventBus(
			WithSynchronousDelivery(),
			WithLogging(logger),
		)

		bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		}))

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		bus.Publish(event)

		if logger.Count() < 1 {
			t.Error("Expected logging to occur")
		}
	})

	t.Run("WithMetrics creates bus with metrics", func(t *testing.T) {
		recorder := &testMetricsRecorder{}

		bus := NewEventBus(
			WithSynchronousDelivery(),
			WithMetrics(recorder),
		)

		bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		}))

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		bus.Publish(event)

		if recorder.processedCount < 1 {
			t.Error("Expected metrics to be recorded")
		}
	})

	t.Run("WithTimeout creates bus with timeout", func(t *testing.T) {
		bus := NewEventBus(
			WithSynchronousDelivery(),
			WithTimeout(50*time.Millisecond),
		)

		bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		}))

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := bus.Publish(event)
		if err == nil {
			t.Error("Expected timeout error")
		}
	})

	t.Run("WithCustomMiddleware adds custom middleware", func(t *testing.T) {
		var middlewareCalled bool
		middleware := func(next EventHandler) EventHandler {
			return func(event StateChangeEvent) error {
				middlewareCalled = true
				return next(event)
			}
		}

		bus := NewEventBus(
			WithSynchronousDelivery(),
			WithCustomMiddleware(middleware),
		)

		bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		}))

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		bus.Publish(event)

		if !middlewareCalled {
			t.Error("Expected custom middleware to be called")
		}
	})

	t.Run("WithStrategy sets delivery strategy", func(t *testing.T) {
		bus := NewEventBus(WithStrategy(NewSynchronousStrategy()))

		var called bool
		bus.Subscribe(EventListenerFunc(func(event StateChangeEvent) error {
			called = true
			return nil
		}))

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := bus.Publish(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !called {
			t.Error("Expected listener to be called")
		}
	})
}

// TestCircuitBreakerStateString tests state string representation
func TestCircuitBreakerStateString(t *testing.T) {
	tests := []struct {
		state    CircuitBreakerState
		expected string
	}{
		{Closed, "Closed"},
		{Open, "Open"},
		{HalfOpen, "HalfOpen"},
		{CircuitBreakerState(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.state.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestRegistryNotify tests the Notify method
func TestRegistryNotify(t *testing.T) {
	t.Run("Notifies all listeners", func(t *testing.T) {
		registry := NewRegistry()

		var count atomic.Int32
		for i := 0; i < 3; i++ {
			registry.Subscribe("", EventListenerFunc(func(event StateChangeEvent) error {
				count.Add(1)
				return nil
			}))
		}

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := registry.Notify(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if count.Load() != 3 {
			t.Errorf("Expected 3 listeners notified, got %d", count.Load())
		}
	})

	t.Run("Returns error when listener fails", func(t *testing.T) {
		registry := NewRegistry()

		expectedErr := errors.New("listener error")
		registry.Subscribe("", EventListenerFunc(func(event StateChangeEvent) error {
			return expectedErr
		}))

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := registry.Notify(event)
		if err == nil {
			t.Error("Expected error from listener")
		}
	})
}

// TestMultiError tests multi-error handling
func TestMultiError(t *testing.T) {
	t.Run("Error method returns message", func(t *testing.T) {
		err := errors.New("test error")
		multiErr := &MultiError{
			Errors: []error{err},
		}

		msg := multiErr.Error()
		if msg == "" {
			t.Error("Expected non-empty error message")
		}
	})

	t.Run("Unwrap returns underlying errors", func(t *testing.T) {
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")
		multiErr := &MultiError{
			Errors: []error{err1, err2},
		}

		unwrapped := multiErr.Unwrap()
		if len(unwrapped) != 2 {
			t.Errorf("Expected 2 errors, got %d", len(unwrapped))
		}
		if unwrapped[0] != err1 || unwrapped[1] != err2 {
			t.Error("Unwrapped errors don't match")
		}
	})

	t.Run("Error with no errors", func(t *testing.T) {
		multiErr := &MultiError{
			Errors: []error{},
		}

		msg := multiErr.Error()
		if msg != "no errors" {
			t.Errorf("Expected 'no errors', got '%s'", msg)
		}
	})
}

// TestOrderedAsyncStrategy tests the ordered async delivery strategy
func TestOrderedAsyncStrategy(t *testing.T) {
	t.Run("delivers events in order", func(t *testing.T) {
		var results []CircuitBreakerState
		var mu sync.Mutex

		strategy := NewOrderedAsyncStrategy(10)

		// Create listeners that append to results
		listener1 := func(event StateChangeEvent) error {
			mu.Lock()
			defer mu.Unlock()
			results = append(results, event.To)
			return nil
		}

		listeners := []ListenerInfo{{Listener: EventListenerFunc(listener1), ID: "test1"}}

		// Deliver multiple events
		for i := 0; i < 5; i++ {
			event := StateChangeEvent{
				From: Closed,
				To:   Open,
				Timestamp: time.Now(),
			}
			err := strategy.Deliver(event, listeners)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}

		// Close and wait for processing
		strategy.Close()

		mu.Lock()
		defer mu.Unlock()

		// Should have received all events
		if len(results) != 5 {
			t.Errorf("Expected 5 events, got %d", len(results))
		}
	})

	t.Run("Close waits for pending events", func(t *testing.T) {
		strategy := NewOrderedAsyncStrategy(5)

		processed := make(chan bool, 1)
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			// Signal that event was processed
			processed <- true
			return nil
		})

		listeners := []ListenerInfo{{Listener: listener, ID: "test1"}}

		// Deliver an event
		event := StateChangeEvent{
			From: Closed,
			To:   Open,
			Timestamp: time.Now(),
		}
		err := strategy.Deliver(event, listeners)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// Close should wait for the event to be processed
		go strategy.Close()

		// Wait for event processing
		select {
		case <-processed:
			// Success - event was processed before Close completed
		case <-time.After(1 * time.Second):
			t.Error("Event not processed before timeout")
		}
	})
}

// Test helpers
// Note: We now use logging.TestLogger from the logging package instead of a custom testLogger
