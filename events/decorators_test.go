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

// TestListenerWithTimeout tests the timeout decorator
func TestListenerWithTimeout(t *testing.T) {
	t.Run("Returns result before timeout", func(t *testing.T) {
		fastListener := EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		})

		decorated := ListenerWithTimeout(fastListener, 100*time.Millisecond)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Returns timeout error when listener is slow", func(t *testing.T) {
		slowListener := EventListenerFunc(func(event StateChangeEvent) error {
			time.Sleep(200 * time.Millisecond)
			return nil
		})

		decorated := ListenerWithTimeout(slowListener, 50*time.Millisecond)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
	})

	t.Run("Propagates listener error", func(t *testing.T) {
		expectedErr := errors.New("listener error")
		errorListener := EventListenerFunc(func(event StateChangeEvent) error {
			return expectedErr
		})

		decorated := ListenerWithTimeout(errorListener, 100*time.Millisecond)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
		if !errors.Is(err, expectedErr) {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})
}

// TestListenerWithContextTimeout tests context-based timeout decorator
func TestListenerWithContextTimeout(t *testing.T) {
	t.Run("Adds timeout to event context", func(t *testing.T) {
		var receivedCtx context.Context
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			receivedCtx = event.Context
			return nil
		})

		decorated := ListenerWithContextTimeout(listener, 100*time.Millisecond)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
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

	t.Run("Uses background context when event context is nil", func(t *testing.T) {
		var receivedCtx context.Context
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			receivedCtx = event.Context
			return nil
		})

		decorated := ListenerWithContextTimeout(listener, 100*time.Millisecond)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   nil, // nil context
		}

		err := decorated.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if receivedCtx == nil {
			t.Fatal("Expected context to be set")
		}
	})
}

// TestListenerWithRateLimit tests rate limiting decorator
func TestListenerWithRateLimit(t *testing.T) {
	t.Run("Allows first call immediately", func(t *testing.T) {
		var callCount atomic.Int32
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			callCount.Add(1)
			return nil
		})

		decorated := ListenerWithRateLimit(listener, 10) // 10 calls/second

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if callCount.Load() != 1 {
			t.Errorf("Expected 1 call, got %d", callCount.Load())
		}
	})

	t.Run("Drops events when rate limit exceeded", func(t *testing.T) {
		var callCount atomic.Int32
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			callCount.Add(1)
			return nil
		})

		decorated := ListenerWithRateLimit(listener, 10) // 10 calls/second

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		// First call should succeed
		decorated.OnEvent(event)

		// Immediate second call should be dropped
		decorated.OnEvent(event)

		if callCount.Load() != 1 {
			t.Errorf("Expected 1 call (second should be rate limited), got %d", callCount.Load())
		}
	})

	t.Run("Allows call after interval", func(t *testing.T) {
		var callCount atomic.Int32
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			callCount.Add(1)
			return nil
		})

		decorated := ListenerWithRateLimit(listener, 10) // 10 calls/second = 100ms interval

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		// First call
		decorated.OnEvent(event)

		// Wait for rate limit interval
		time.Sleep(150 * time.Millisecond)

		// Second call after interval should succeed
		decorated.OnEvent(event)

		if callCount.Load() != 2 {
			t.Errorf("Expected 2 calls, got %d", callCount.Load())
		}
	})

	t.Run("Handles zero or negative rate", func(t *testing.T) {
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		})

		// Should default to rate of 1
		decorated := ListenerWithRateLimit(listener, 0)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

// TestListenerWithCircuitBreaker tests circuit breaker decorator
func TestListenerWithCircuitBreaker(t *testing.T) {
	t.Run("Allows calls when circuit is closed", func(t *testing.T) {
		var callCount atomic.Int32
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			callCount.Add(1)
			return nil
		})

		decorated := ListenerWithCircuitBreaker(listener, 3, 100*time.Millisecond)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if callCount.Load() != 1 {
			t.Errorf("Expected 1 call, got %d", callCount.Load())
		}
	})

	t.Run("Opens circuit after max failures", func(t *testing.T) {
		var callCount atomic.Int32
		failingListener := EventListenerFunc(func(event StateChangeEvent) error {
			callCount.Add(1)
			return errors.New("failure")
		})

		decorated := ListenerWithCircuitBreaker(failingListener, 3, 100*time.Millisecond)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		// Trigger 3 failures
		for i := 0; i < 3; i++ {
			decorated.OnEvent(event)
		}

		// Circuit should be open, next call should fail fast
		err := decorated.OnEvent(event)
		if err == nil {
			t.Error("Expected circuit breaker open error, got nil")
		}

		// Should have only called 3 times (not 4)
		if callCount.Load() != 3 {
			t.Errorf("Expected 3 calls before circuit opened, got %d", callCount.Load())
		}
	})

	t.Run("Resets circuit after timeout", func(t *testing.T) {
		var callCount atomic.Int32
		failingListener := EventListenerFunc(func(event StateChangeEvent) error {
			callCount.Add(1)
			// Fail first 3, then succeed
			if callCount.Load() <= 3 {
				return errors.New("failure")
			}
			return nil
		})

		decorated := ListenerWithCircuitBreaker(failingListener, 3, 50*time.Millisecond)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		// Trigger 3 failures to open circuit
		for i := 0; i < 3; i++ {
			decorated.OnEvent(event)
		}

		// Wait for reset timeout
		time.Sleep(60 * time.Millisecond)

		// Should be able to call again
		err := decorated.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error after timeout, got %v", err)
		}

		if callCount.Load() != 4 {
			t.Errorf("Expected 4 calls total, got %d", callCount.Load())
		}
	})

	t.Run("Resets failure count on success", func(t *testing.T) {
		var callCount atomic.Int32
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			count := callCount.Add(1)
			// Fail on call 1, succeed on call 2, fail on call 3
			if count == 1 || count == 3 {
				return errors.New("failure")
			}
			return nil
		})

		decorated := ListenerWithCircuitBreaker(listener, 3, 100*time.Millisecond)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		// Call 1: fail
		decorated.OnEvent(event)

		// Call 2: succeed (resets failure count)
		err := decorated.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Call 3: fail (should not open circuit since count was reset)
		decorated.OnEvent(event)

		// Call 4: should still work
		err = decorated.OnEvent(event)
		if err != nil {
			t.Errorf("Expected circuit to still be closed, got error %v", err)
		}
	})
}

// TestListenerWithAsync tests async decorator
func TestListenerWithAsync(t *testing.T) {
	t.Run("Returns immediately", func(t *testing.T) {
		done := make(chan struct{})
		slowListener := EventListenerFunc(func(event StateChangeEvent) error {
			time.Sleep(100 * time.Millisecond)
			close(done)
			return nil
		})

		decorated := ListenerWithAsync(slowListener)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		start := time.Now()
		err := decorated.OnEvent(event)
		duration := time.Since(start)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if duration > 50*time.Millisecond {
			t.Errorf("Expected immediate return, took %v", duration)
		}

		// Verify listener actually runs
		select {
		case <-done:
		case <-time.After(200 * time.Millisecond):
			t.Error("Listener was not executed")
		}
	})

	t.Run("Ignores listener errors", func(t *testing.T) {
		errorListener := EventListenerFunc(func(event StateChangeEvent) error {
			return errors.New("some error")
		})

		decorated := ListenerWithAsync(errorListener)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		// Should return nil even though listener will error
		err := decorated.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error from async listener, got %v", err)
		}
	})
}

// TestListenerWithLogging tests logging decorator
func TestListenerWithLogging(t *testing.T) {
	t.Run("Logs listener execution", func(t *testing.T) {
		logger := logging.NewTestLogger()

		listener := EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		})

		decorated := ListenerWithLogging(listener, logger)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if logger.Count() < 1 { // At least completion should be logged
			t.Errorf("Expected at least 1 log entry, got %d", logger.Count())
		}
	})

	t.Run("Logs listener failure", func(t *testing.T) {
		logger := logging.NewTestLogger()

		expectedErr := errors.New("listener error")
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			return expectedErr
		})

		decorated := ListenerWithLogging(listener, logger)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
		if !errors.Is(err, expectedErr) {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}

		// Should have logged at least the error
		if logger.Count() < 1 {
			t.Errorf("Expected at least 1 log entry, got %d", logger.Count())
		}

		// Should have an error entry
		if logger.CountByLevel(logging.LevelError) < 1 {
			t.Error("Expected at least one error log entry")
		}
	})

	t.Run("Handles nil logger gracefully", func(t *testing.T) {
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		})

		decorated := ListenerWithLogging(listener, nil)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error with nil logger, got %v", err)
		}
	})
}

// TestListenerWithMetrics tests metrics decorator
func TestListenerWithMetrics(t *testing.T) {
	t.Run("Records successful event", func(t *testing.T) {
		recorder := &testMetricsRecorder{}

		listener := EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		})

		decorated := ListenerWithMetrics(listener, recorder, "test-listener")

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
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

		expectedErr := errors.New("listener error")
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			return expectedErr
		})

		decorated := ListenerWithMetrics(listener, recorder, "test-listener")

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
		if !errors.Is(err, expectedErr) {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}

		if recorder.processedCount != 1 {
			t.Errorf("Expected 1 processed event, got %d", recorder.processedCount)
		}

		if recorder.lastSuccess {
			t.Error("Expected failure to be recorded")
		}

		if recorder.errorCount != 1 {
			t.Errorf("Expected 1 error recorded, got %d", recorder.errorCount)
		}
	})

	t.Run("Handles nil recorder gracefully", func(t *testing.T) {
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		})

		decorated := ListenerWithMetrics(listener, nil, "test-listener")

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := decorated.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error with nil recorder, got %v", err)
		}
	})
}

// TestCompose tests decorator composition
func TestCompose(t *testing.T) {
	t.Run("Applies decorators in order", func(t *testing.T) {
		var order []int
		var mu sync.Mutex

		baseListener := EventListenerFunc(func(event StateChangeEvent) error {
			mu.Lock()
			order = append(order, 0)
			mu.Unlock()
			return nil
		})

		decorator1 := func(l EventListener) EventListener {
			return EventListenerFunc(func(event StateChangeEvent) error {
				mu.Lock()
				order = append(order, 1)
				mu.Unlock()
				return l.OnEvent(event)
			})
		}

		decorator2 := func(l EventListener) EventListener {
			return EventListenerFunc(func(event StateChangeEvent) error {
				mu.Lock()
				order = append(order, 2)
				mu.Unlock()
				return l.OnEvent(event)
			})
		}

		composed := Compose(baseListener, decorator1, decorator2)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := composed.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Decorators wrap from inner to outer, so execute: decorator2 -> decorator1 -> base
		expected := []int{2, 1, 0}
		mu.Lock()
		defer mu.Unlock()
		if len(order) != len(expected) {
			t.Fatalf("Expected %d items in order, got %d", len(expected), len(order))
		}
		for i, v := range expected {
			if order[i] != v {
				t.Errorf("Expected order[%d] = %d, got %d", i, v, order[i])
			}
		}
	})

	t.Run("Works with no decorators", func(t *testing.T) {
		listener := EventListenerFunc(func(event StateChangeEvent) error {
			return nil
		})

		composed := Compose(listener)

		event := StateChangeEvent{
			From:      Closed,
			To:        Open,
			Timestamp: time.Now(),
			Context:   context.Background(),
		}

		err := composed.OnEvent(event)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

// Test helpers
type testMetricsRecorder struct {
	processedCount int
	lastSuccess    bool
	errorCount     int
	panicCount     int
	mu             sync.Mutex
}

func (r *testMetricsRecorder) RecordEventProcessed(duration time.Duration, success bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processedCount++
	r.lastSuccess = success
}

func (r *testMetricsRecorder) RecordListenerError(listenerID string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errorCount++
}

func (r *testMetricsRecorder) RecordListenerPanic(listenerID string, recovered interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.panicCount++
}
