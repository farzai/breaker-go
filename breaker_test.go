package breaker_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	breaker "github.com/farzai/breaker-go"
	"github.com/farzai/breaker-go/events"
)

func TestCircuitBreakerInitialState(t *testing.T) {
	cb, err := breaker.NewCircuitBreaker(3, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to create circuit breaker: %v", err)
	}

	if cb.State() != breaker.Closed {
		t.Errorf("Expected initial state to be Closed, got %v", cb.State())
	}
}

func TestCircuitBreakerState(t *testing.T) {
	t.Run("Should be open when failure threshold reached", func(t *testing.T) {
		// 3 failures in 1 second
		cb, err := breaker.NewCircuitBreaker(3, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// 3 failures
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		if cb.State() != breaker.Open {
			t.Errorf("Expected state to be Open, got %v", cb.State())
		}
	})

	t.Run("Should be got error of open state", func(t *testing.T) {
		// 3 failures in 1 second
		cb, err := breaker.NewCircuitBreaker(3, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// 3 failures
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		if _, err := cb.Execute(func() (interface{}, error) {
			return nil, errors.New("error")
		}); err != breaker.ErrCircuitBreakerOpen {
			t.Errorf("Expected error to be ErrCircuitBreakerOpen, got %v", err)
		}
	})

	t.Run("Should be half-open when reset timeout reached", func(t *testing.T) {
		// 3 failures in 1 second
		cb, err := breaker.NewCircuitBreaker(3, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// 3 failures
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		if cb.State() != breaker.Open {
			t.Errorf("Expected state to be Open, got %v", cb.State())
		}

		// wait 1 second
		time.Sleep(1 * time.Second)

		if cb.State() != breaker.HalfOpen {
			t.Errorf("Expected state to be HalfOpen, got %v", cb.State())
		}
	})

	t.Run("Should be closed when success threshold reached", func(t *testing.T) {
		// 3 failures in 1 second
		cb, err := breaker.NewCircuitBreaker(3, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// 3 failures
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		if cb.State() != breaker.Open {
			t.Errorf("Expected state to be Open, got %v", cb.State())
		}

		// wait 1 second
		time.Sleep(1 * time.Second)

		if cb.State() != breaker.HalfOpen {
			t.Errorf("Expected state to be Closed, got %v", cb.State())
		}

		// 1 success
		cb.Execute(func() (interface{}, error) {
			return nil, nil
		})

		if cb.State() != breaker.Closed {
			t.Errorf("Expected state to be Closed, got %v", cb.State())
		}
	})

	t.Run("Should be open when failure threshold reached after reset", func(t *testing.T) {
		// 3 failures in 1 second
		cb, err := breaker.NewCircuitBreaker(3, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// 3 failures
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		if cb.State() != breaker.Open {
			t.Errorf("Expected state to be Open, got %v", cb.State())
		}

		// wait 1 second
		time.Sleep(1 * time.Second)

		if cb.State() != breaker.HalfOpen {
			t.Errorf("Expected state to be Closed, got %v", cb.State())
		}

		// 1 failure
		cb.Execute(func() (interface{}, error) {
			return nil, errors.New("error")
		})

		if cb.State() != breaker.Open {
			t.Errorf("Expected state to be Open, got %v", cb.State())
		}
	})

	t.Run("Should be closed state if Reset is called", func(t *testing.T) {
		// 3 failures in 1 second
		cb, err := breaker.NewCircuitBreaker(3, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// 3 failures
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		if cb.State() != breaker.Open {
			t.Errorf("Expected state to be Open, got %v", cb.State())
		}

		// wait 1 second
		time.Sleep(1 * time.Second)

		if cb.State() != breaker.HalfOpen {
			t.Errorf("Expected state to be Closed, got %v", cb.State())
		}

		// 1 failure
		cb.Execute(func() (interface{}, error) {
			return nil, errors.New("error")
		})

		if cb.State() != breaker.Open {
			t.Errorf("Expected state to be Open, got %v", cb.State())
		}

		cb.Reset()

		if cb.State() != breaker.Closed {
			t.Errorf("Expected state to be Closed, got %v", cb.State())
		}
	})
}

// TestWithSnapshotRepository tests the WithSnapshotRepository option
func TestWithSnapshotRepository(t *testing.T) {
	t.Run("Should use custom snapshot repository", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithSnapshotRepository(repo),
			breaker.WithAsyncPersistence(false), // Use synchronous persistence for testing
		)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// Trigger open state
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		// Verify state is saved to repository
		ctx := context.Background()
		snapshot, err := repo.Load(ctx)
		if err != nil {
			t.Fatalf("Failed to load snapshot: %v", err)
		}
		if snapshot.State != breaker.Open {
			t.Errorf("Expected snapshot state to be Open, got %v", snapshot.State)
		}
	})
}

// TestExecuteWithContext tests ExecuteWithContext method
func TestExecuteWithContext(t *testing.T) {
	t.Run("Should return error when context is cancelled before execution", func(t *testing.T) {
		cb, err := breaker.NewCircuitBreaker(3, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err = cb.ExecuteWithContext(ctx, func(ctx context.Context) (interface{}, error) {
			return "success", nil
		})

		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got %v", err)
		}
	})

	t.Run("Should handle context timeout during execution", func(t *testing.T) {
		cb, err := breaker.NewCircuitBreaker(3, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err = cb.ExecuteWithContext(ctx, func(ctx context.Context) (interface{}, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return "success", nil
			}
		})

		if err != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded error, got %v", err)
		}
	})

	t.Run("Should execute successfully with valid context", func(t *testing.T) {
		cb, err := breaker.NewCircuitBreaker(3, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		ctx := context.Background()

		result, err := cb.ExecuteWithContext(ctx, func(ctx context.Context) (interface{}, error) {
			return "success", nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if result != "success" {
			t.Errorf("Expected result to be 'success', got %v", result)
		}
	})
}

// TestEventListeners tests event listener functionality
func TestEventListeners(t *testing.T) {
	t.Run("Should notify event listener on state change", func(t *testing.T) {
		var mu sync.Mutex
		var listenerCalled bool
		var fromState, toState events.CircuitBreakerState

		listener := events.EventListenerFunc(func(event events.StateChangeEvent) error {
			mu.Lock()
			defer mu.Unlock()
			listenerCalled = true
			fromState = event.From
			toState = event.To
			return nil
		})

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithEventListener(listener),
		)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// Trigger state change
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		// Wait for listener to be called (it runs in a goroutine)
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if !listenerCalled {
			t.Error("Expected listener to be called")
		}

		if fromState != events.Closed || toState != events.Open {
			t.Errorf("Expected state change from Closed to Open, got %v to %v", fromState, toState)
		}
	})

	t.Run("Should notify multiple listeners", func(t *testing.T) {
		var mu sync.Mutex
		listener1Called := false
		listener2Called := false

		listener1 := events.EventListenerFunc(func(event events.StateChangeEvent) error {
			mu.Lock()
			defer mu.Unlock()
			listener1Called = true
			return nil
		})

		listener2 := events.EventListenerFunc(func(event events.StateChangeEvent) error {
			mu.Lock()
			defer mu.Unlock()
			listener2Called = true
			return nil
		})

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithEventListener(listener1),
			breaker.WithEventListener(listener2),
		)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// Trigger state change
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		// Wait for listeners to be notified (they run in a goroutine)
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if !listener1Called {
			t.Error("Expected listener1 to be called")
		}

		if !listener2Called {
			t.Error("Expected listener2 to be called")
		}
	})

	t.Run("Should handle logging listener", func(t *testing.T) {
		var mu sync.Mutex
		var logMessage string

		listener := events.EventListenerFunc(func(event events.StateChangeEvent) error {
			mu.Lock()
			defer mu.Unlock()
			logMessage = "Circuit breaker state changed from " + event.From.String() + " to " + event.To.String()
			return nil
		})

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithEventListener(listener),
		)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// Trigger state change
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		// Wait for listener to be notified
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if logMessage == "" {
			t.Error("Expected log message to be set")
		}

		expectedMsg := "Circuit breaker state changed from Closed to Open"
		if logMessage != expectedMsg {
			t.Errorf("Expected log message '%s', got '%s'", expectedMsg, logMessage)
		}
	})
}

// TestWithEventBus tests the WithEventBus option
func TestWithEventBus(t *testing.T) {
	t.Run("Uses custom event bus", func(t *testing.T) {
		customBus := events.NewEventBus(events.WithSynchronousDelivery())

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithEventBus(customBus),
		)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		var listenerCalled atomic.Bool
		customBus.Subscribe(events.EventListenerFunc(func(event events.StateChangeEvent) error {
			listenerCalled.Store(true)
			return nil
		}))

		// Trigger state change
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		// Wait for synchronous delivery
		time.Sleep(50 * time.Millisecond)

		if !listenerCalled.Load() {
			t.Error("Expected custom event bus listener to be called")
		}
	})

	t.Run("Returns error for nil event bus", func(t *testing.T) {
		_, err := breaker.New(
			breaker.WithEventBus(nil),
		)
		if err == nil {
			t.Error("Expected error for nil event bus")
		}
	})
}

// TestWithEventMiddleware tests the WithEventMiddleware option
func TestWithEventMiddleware(t *testing.T) {
	t.Run("Returns error for nil middleware", func(t *testing.T) {
		_, err := breaker.New(
			breaker.WithEventMiddleware(nil),
		)
		if err == nil {
			t.Error("Expected error for nil middleware")
		}
	})

	t.Run("Creates bus with middleware via option", func(t *testing.T) {
		middleware := func(next events.EventHandler) events.EventHandler {
			return func(event events.StateChangeEvent) error {
				return next(event)
			}
		}

		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithEventMiddleware(middleware),
		)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// Just verify it was created successfully
		if cb == nil {
			t.Error("Expected circuit breaker to be created")
		}
	})
}

// TestInvalidOptions tests error handling for invalid options
func TestInvalidOptions(t *testing.T) {
	t.Run("Returns error for invalid failure threshold", func(t *testing.T) {
		_, err := breaker.New(
			breaker.WithFailureThreshold(0),
		)
		if err == nil {
			t.Error("Expected error for zero failure threshold")
		}

		_, err = breaker.New(
			breaker.WithFailureThreshold(-1),
		)
		if err == nil {
			t.Error("Expected error for negative failure threshold")
		}
	})

	t.Run("Returns error for invalid reset timeout", func(t *testing.T) {
		_, err := breaker.New(
			breaker.WithResetTimeout(0),
		)
		if err == nil {
			t.Error("Expected error for zero reset timeout")
		}

		_, err = breaker.New(
			breaker.WithResetTimeout(-1*time.Second),
		)
		if err == nil {
			t.Error("Expected error for negative reset timeout")
		}
	})

	t.Run("Returns error for nil snapshot repository", func(t *testing.T) {
		_, err := breaker.New(
			breaker.WithSnapshotRepository(nil),
		)
		if err == nil {
			t.Error("Expected error for nil snapshot repository")
		}
	})

	t.Run("Returns error for nil event listener", func(t *testing.T) {
		_, err := breaker.New(
			breaker.WithEventListener(nil),
		)
		if err == nil {
			t.Error("Expected error for nil event listener")
		}
	})
}

// TestCircuitBreakerConcurrency tests concurrent access to the circuit breaker
func TestCircuitBreakerConcurrency(t *testing.T) {
	t.Run("Concurrent Execute calls should be thread-safe", func(t *testing.T) {
		cb, err := breaker.NewCircuitBreaker(100, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		const numGoroutines = 50
		const numExecutions = 50

		// Use WaitGroup for synchronization
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Launch multiple goroutines
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numExecutions; j++ {
					_, err := cb.Execute(func() (interface{}, error) {
						// Simulate some work
						time.Sleep(time.Microsecond)

						// Randomly succeed or fail
						if (id+j)%5 == 0 {
							return nil, errors.New("error")
						}
						return "success", nil
					})

					// We only care about thread safety, not specific errors
					// ErrCircuitBreakerOpen is expected when threshold is reached
					_ = err
				}
			}(i)
		}

		// Wait for all goroutines to finish
		wg.Wait()

		// Test passes if no race conditions detected
	})

	t.Run("Concurrent State and Execute calls should be thread-safe", func(t *testing.T) {
		cb, err := breaker.NewCircuitBreaker(5, 500*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		const numGoroutines = 50
		done := make(chan bool, numGoroutines)

		// Launch goroutines that execute actions
		for i := 0; i < numGoroutines/2; i++ {
			go func(id int) {
				for j := 0; j < 20; j++ {
					cb.Execute(func() (interface{}, error) {
						if id%2 == 0 {
							return nil, errors.New("error")
						}
						return "success", nil
					})
					time.Sleep(time.Millisecond)
				}
				done <- true
			}(i)
		}

		// Launch goroutines that check state
		for i := 0; i < numGoroutines/2; i++ {
			go func() {
				for j := 0; j < 20; j++ {
					_ = cb.State()
					time.Sleep(time.Millisecond)
				}
				done <- true
			}()
		}

		// Wait for all goroutines to finish
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
	})

	t.Run("Concurrent Reset calls should be thread-safe", func(t *testing.T) {
		cb, err := breaker.NewCircuitBreaker(3, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// Trigger open state
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		const numGoroutines = 20
		done := make(chan bool, numGoroutines)

		// Launch multiple goroutines calling Reset
		for i := 0; i < numGoroutines; i++ {
			go func() {
				cb.Reset()
				done <- true
			}()
		}

		// Wait for all goroutines to finish
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Verify state is closed after resets
		if cb.State() != breaker.Closed {
			t.Errorf("Expected state to be Closed after concurrent resets, got %v", cb.State())
		}
	})
}
