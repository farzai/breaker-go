package events

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestChainMiddleware tests middleware chaining
func TestChainMiddleware(t *testing.T) {
	t.Run("Executes middleware in order", func(t *testing.T) {
		var order []int
		var mu sync.Mutex

		middleware1 := func(next EventHandler) EventHandler {
			return func(event StateChangeEvent) error {
				mu.Lock()
				order = append(order, 1)
				mu.Unlock()
				return next(event)
			}
		}

		middleware2 := func(next EventHandler) EventHandler {
			return func(event StateChangeEvent) error {
				mu.Lock()
				order = append(order, 2)
				mu.Unlock()
				return next(event)
			}
		}

		middleware3 := func(next EventHandler) EventHandler {
			return func(event StateChangeEvent) error {
				mu.Lock()
				order = append(order, 3)
				mu.Unlock()
				return next(event)
			}
		}

		handler := func(event StateChangeEvent) error {
			mu.Lock()
			order = append(order, 0)
			mu.Unlock()
			return nil
		}

		chained := Chain(middleware1, middleware2, middleware3)(handler)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := chained(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Should execute: 1 -> 2 -> 3 -> handler
		expected := []int{1, 2, 3, 0}
		mu.Lock()
		defer mu.Unlock()
		if len(order) != len(expected) {
			t.Fatalf("Expected %d items, got %d", len(expected), len(order))
		}
		for i, v := range expected {
			if order[i] != v {
				t.Errorf("Expected order[%d] = %d, got %d", i, v, order[i])
			}
		}
	})

	t.Run("Works with no middleware", func(t *testing.T) {
		var called bool
		handler := func(event StateChangeEvent) error {
			called = true
			return nil
		}

		chained := Chain()(handler)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := chained(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !called {
			t.Error("Expected handler to be called")
		}
	})
}

// TestMetricsMiddleware tests metrics recording middleware
func TestMetricsMiddlewareFunc(t *testing.T) {
	t.Run("Records successful event", func(t *testing.T) {
		recorder := &testMetricsRecorder{}

		middleware := MetricsMiddleware(recorder)
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

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if recorder.processedCount != 1 {
			t.Errorf("Expected 1 processed event, got %d", recorder.processedCount)
		}

		if !recorder.lastSuccess {
			t.Error("Expected success to be recorded")
		}
	})

	t.Run("Records failed event", func(t *testing.T) {
		recorder := &testMetricsRecorder{}

		middleware := MetricsMiddleware(recorder)
		expectedErr := errors.New("handler error")
		handler := func(event StateChangeEvent) error {
			return expectedErr
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := wrappedHandler(event)
		if !errors.Is(err, expectedErr) {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}

		if recorder.processedCount != 1 {
			t.Errorf("Expected 1 processed event, got %d", recorder.processedCount)
		}

		if recorder.lastSuccess {
			t.Error("Expected failure to be recorded")
		}
	})

	t.Run("Handles nil recorder gracefully", func(t *testing.T) {
		middleware := MetricsMiddleware(nil)
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

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error with nil recorder, got %v", err)
		}
	})
}

// TestContextTimeoutMiddleware tests context timeout middleware
func TestContextTimeoutMiddlewareFunc(t *testing.T) {
	t.Run("Adds timeout to event context", func(t *testing.T) {
		var receivedCtx context.Context
		middleware := ContextTimeoutMiddleware(100 * time.Millisecond)
		handler := func(event StateChangeEvent) error {
			receivedCtx = event.Context
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if receivedCtx == nil {
			t.Fatal("Expected context to be set")
		}

		_, hasDeadline := receivedCtx.Deadline()
		if !hasDeadline {
			t.Error("Expected context to have deadline")
		}
	})

	t.Run("Uses background context when nil", func(t *testing.T) {
		var receivedCtx context.Context
		middleware := ContextTimeoutMiddleware(100 * time.Millisecond)
		handler := func(event StateChangeEvent) error {
			receivedCtx = event.Context
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   nil, // nil context
		}

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if receivedCtx == nil {
			t.Fatal("Expected context to be set")
		}
	})

	t.Run("Preserves existing deadline", func(t *testing.T) {
		existingDeadline := time.Now().Add(500 * time.Millisecond)
		ctx, cancel := context.WithDeadline(context.Background(), existingDeadline)
		defer cancel()

		var receivedCtx context.Context
		middleware := ContextTimeoutMiddleware(100 * time.Millisecond)
		handler := func(event StateChangeEvent) error {
			receivedCtx = event.Context
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   ctx,
		}

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Should preserve existing context
		if receivedCtx != ctx {
			t.Error("Expected original context with deadline to be preserved")
		}
	})
}

// TestErrorHandlerMiddleware tests error handler middleware
func TestErrorHandlerMiddlewareFunc(t *testing.T) {
	t.Run("Calls error handler on error", func(t *testing.T) {
		var handledErr error
		var handledID string

		errorHandler := func(err error, event StateChangeEvent, listenerID string) {
			handledErr = err
			handledID = listenerID
		}

		middleware := ErrorHandlerMiddleware(errorHandler)
		expectedErr := errors.New("handler error")
		handler := func(event StateChangeEvent) error {
			return expectedErr
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From:     Closed,
			To:       Open,
			Metadata: map[string]interface{}{"listener_id": "test-listener"},
		}

		err := wrappedHandler(event)
		if !errors.Is(err, expectedErr) {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}

		if !errors.Is(handledErr, expectedErr) {
			t.Errorf("Expected error handler to receive %v, got %v", expectedErr, handledErr)
		}

		if handledID != "test-listener" {
			t.Errorf("Expected listener ID 'test-listener', got '%s'", handledID)
		}
	})

	t.Run("Does not call error handler on success", func(t *testing.T) {
		var called bool
		errorHandler := func(err error, event StateChangeEvent, listenerID string) {
			called = true
		}

		middleware := ErrorHandlerMiddleware(errorHandler)
		handler := func(event StateChangeEvent) error {
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From: Closed,
			To:   Open,
		}

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if called {
			t.Error("Expected error handler not to be called on success")
		}
	})

	t.Run("Handles nil error handler gracefully", func(t *testing.T) {
		middleware := ErrorHandlerMiddleware(nil)
		handler := func(event StateChangeEvent) error {
			return errors.New("some error")
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From: Closed,
			To:   Open,
		}

		// Should not panic
		err := wrappedHandler(event)
		if err == nil {
			t.Error("Expected error to be propagated")
		}
	})

	t.Run("Uses 'unknown' for missing listener ID", func(t *testing.T) {
		var handledID string

		errorHandler := func(err error, event StateChangeEvent, listenerID string) {
			handledID = listenerID
		}

		middleware := ErrorHandlerMiddleware(errorHandler)
		handler := func(event StateChangeEvent) error {
			return errors.New("error")
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From:     Closed,
			To:       Open,
			Metadata: map[string]interface{}{}, // no listener_id
		}

		wrappedHandler(event)

		if handledID != "unknown" {
			t.Errorf("Expected listener ID 'unknown', got '%s'", handledID)
		}
	})
}

// TestFilterMiddleware tests event filtering middleware
func TestFilterMiddlewareFunc(t *testing.T) {
	t.Run("Allows events that match predicate", func(t *testing.T) {
		var called bool
		middleware := FilterMiddleware(func(event StateChangeEvent) bool {
			return event.To == Open
		})

		handler := func(event StateChangeEvent) error {
			called = true
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From: Closed,
			To:   Open,
		}

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !called {
			t.Error("Expected handler to be called for matching event")
		}
	})

	t.Run("Filters events that don't match predicate", func(t *testing.T) {
		var called bool
		middleware := FilterMiddleware(func(event StateChangeEvent) bool {
			return event.To == Open
		})

		handler := func(event StateChangeEvent) error {
			called = true
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From: Open,
			To:   Closed,
		}

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if called {
			t.Error("Expected handler not to be called for filtered event")
		}
	})

	t.Run("Allows all events when predicate is nil", func(t *testing.T) {
		var called bool
		middleware := FilterMiddleware(nil)

		handler := func(event StateChangeEvent) error {
			called = true
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From: Closed,
			To:   Open,
		}

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !called {
			t.Error("Expected handler to be called with nil predicate")
		}
	})
}

// TestRetryMiddleware tests retry middleware
func TestRetryMiddlewareFunc(t *testing.T) {
	t.Run("Succeeds on first try", func(t *testing.T) {
		var attempts atomic.Int32
		middleware := RetryMiddleware(3, 10*time.Millisecond)

		handler := func(event StateChangeEvent) error {
			attempts.Add(1)
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From: Closed,
			To:   Open,
		}

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if attempts.Load() != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts.Load())
		}
	})

	t.Run("Retries on failure", func(t *testing.T) {
		var attempts atomic.Int32
		middleware := RetryMiddleware(3, 10*time.Millisecond)

		handler := func(event StateChangeEvent) error {
			count := attempts.Add(1)
			if count < 3 {
				return errors.New("temporary failure")
			}
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From: Closed,
			To:   Open,
		}

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error after retries, got %v", err)
		}

		if attempts.Load() != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts.Load())
		}
	})

	t.Run("Returns error after max retries", func(t *testing.T) {
		var attempts atomic.Int32
		middleware := RetryMiddleware(3, 10*time.Millisecond)

		handler := func(event StateChangeEvent) error {
			attempts.Add(1)
			return errors.New("persistent failure")
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From: Closed,
			To:   Open,
		}

		err := wrappedHandler(event)
		if err == nil {
			t.Error("Expected error after max retries")
		}

		// Should try once + 3 retries = 4 attempts
		if attempts.Load() != 4 {
			t.Errorf("Expected 4 attempts (1 initial + 3 retries), got %d", attempts.Load())
		}
	})

	t.Run("Respects retry delay", func(t *testing.T) {
		var attempts atomic.Int32
		delay := 50 * time.Millisecond
		middleware := RetryMiddleware(2, delay)

		handler := func(event StateChangeEvent) error {
			attempts.Add(1)
			return errors.New("failure")
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From: Closed,
			To:   Open,
		}

		start := time.Now()
		wrappedHandler(event)
		duration := time.Since(start)

		// Should take at least 2 * delay (2 retries with delays)
		expectedMin := 2 * delay
		if duration < expectedMin {
			t.Errorf("Expected at least %v with retry delays, took %v", expectedMin, duration)
		}
	})
}

// TestContextEnrichmentMiddleware tests context enrichment middleware
func TestContextEnrichmentMiddlewareFunc(t *testing.T) {
	t.Run("Enriches event context", func(t *testing.T) {
		var receivedEvent StateChangeEvent
		enricher := func(event StateChangeEvent) StateChangeEvent {
			if event.Metadata == nil {
				event.Metadata = make(map[string]interface{})
			}
			event.Metadata["enriched"] = true
			event.Metadata["trace_id"] = "12345"
			return event
		}

		middleware := ContextEnrichmentMiddleware(enricher)
		handler := func(event StateChangeEvent) error {
			receivedEvent = event
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From: Closed,
			To:   Open,
		}

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if receivedEvent.Metadata == nil {
			t.Fatal("Expected metadata to be set")
		}

		if enriched, ok := receivedEvent.Metadata["enriched"].(bool); !ok || !enriched {
			t.Error("Expected enriched flag to be set")
		}

		if traceID, ok := receivedEvent.Metadata["trace_id"].(string); !ok || traceID != "12345" {
			t.Errorf("Expected trace_id '12345', got '%v'", traceID)
		}
	})

	t.Run("Handles nil enricher gracefully", func(t *testing.T) {
		middleware := ContextEnrichmentMiddleware(nil)
		handler := func(event StateChangeEvent) error {
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From: Closed,
			To:   Open,
		}

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error with nil enricher, got %v", err)
		}
	})

	t.Run("Enricher can modify multiple fields", func(t *testing.T) {
		var receivedEvent StateChangeEvent
		enricher := func(event StateChangeEvent) StateChangeEvent {
			event.Metadata = map[string]interface{}{
				"request_id": "req-123",
				"user_id":    "user-456",
			}
			// Can even modify the context
			event.Context = context.WithValue(event.Context, "key", "value")
			return event
		}

		middleware := ContextEnrichmentMiddleware(enricher)
		handler := func(event StateChangeEvent) error {
			receivedEvent = event
			return nil
		}

		wrappedHandler := middleware(handler)

		event := StateChangeEvent{
			From:    Closed,
			To:      Open,
			Context: context.Background(),
		}

		err := wrappedHandler(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if receivedEvent.Metadata["request_id"] != "req-123" {
			t.Error("Expected request_id to be enriched")
		}

		if receivedEvent.Context.Value("key") != "value" {
			t.Error("Expected context to be enriched")
		}
	})
}
