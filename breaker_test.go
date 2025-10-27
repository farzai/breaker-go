package breaker_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	breaker "github.com/farzai/breaker-go"
)

func TestCircuitBreakerInitialState(t *testing.T) {
	cb := breaker.NewCircuitBreaker(3, 1*time.Second)

	if cb.State() != breaker.Closed {
		t.Errorf("Expected initial state to be Closed, got %v", cb.State())
	}
}

func TestCircuitBreakerState(t *testing.T) {
	t.Run("Should be open when failure threshold reached", func(t *testing.T) {
		// 3 failures in 1 second
		cb := breaker.NewCircuitBreaker(3, 1*time.Second)

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
		cb := breaker.NewCircuitBreaker(3, 1*time.Second)

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
		cb := breaker.NewCircuitBreaker(3, 1*time.Second)

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
		cb := breaker.NewCircuitBreaker(3, 1*time.Second)

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
		cb := breaker.NewCircuitBreaker(3, 1*time.Second)

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
		cb := breaker.NewCircuitBreaker(3, 1*time.Second)

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

// TestWithStorageOption tests the WithStorage option
func TestWithStorageOption(t *testing.T) {
	t.Run("Should use custom storage", func(t *testing.T) {
		storage := breaker.NewInMemoryStateRepository()

		cb := breaker.NewWithOptions(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithStorage(storage),
		)

		// Trigger open state
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		// Verify state is saved to storage
		if storage.Load() != breaker.Open {
			t.Errorf("Expected storage to contain Open state, got %v", storage.Load())
		}
	})
}

// TestExecuteWithContext tests ExecuteWithContext method
func TestExecuteWithContext(t *testing.T) {
	t.Run("Should return error when context is cancelled before execution", func(t *testing.T) {
		cb := breaker.NewCircuitBreaker(3, 1*time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := cb.ExecuteWithContext(ctx, func(ctx context.Context) (interface{}, error) {
			return "success", nil
		})

		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got %v", err)
		}
	})

	t.Run("Should handle context timeout during execution", func(t *testing.T) {
		cb := breaker.NewCircuitBreaker(3, 1*time.Second)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		_, err := cb.ExecuteWithContext(ctx, func(ctx context.Context) (interface{}, error) {
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
		cb := breaker.NewCircuitBreaker(3, 1*time.Second)
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

// TestStateObserversAndCallbacks tests state change observers and callbacks
func TestStateObserversAndCallbacks(t *testing.T) {
	t.Run("Should notify state change callback", func(t *testing.T) {
		var mu sync.Mutex
		var callbackCalled bool
		var fromState, toState breaker.CircuitBreakerState

		cb := breaker.NewWithOptions(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithStateChangeCallback(func(from, to breaker.CircuitBreakerState) {
				mu.Lock()
				defer mu.Unlock()
				callbackCalled = true
				fromState = from
				toState = to
			}),
		)

		// Trigger state change
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		// Wait for callback to be called (it runs in a goroutine)
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if !callbackCalled {
			t.Error("Expected callback to be called")
		}

		if fromState != breaker.Closed || toState != breaker.Open {
			t.Errorf("Expected state change from Closed to Open, got %v to %v", fromState, toState)
		}
	})

	t.Run("Should notify multiple observers", func(t *testing.T) {
		var mu sync.Mutex
		observer1Called := false
		observer2Called := false

		observer1 := &testObserver{
			onStateChange: func(ctx context.Context, from, to breaker.CircuitBreakerState) {
				mu.Lock()
				defer mu.Unlock()
				observer1Called = true
			},
		}

		observer2 := &testObserver{
			onStateChange: func(ctx context.Context, from, to breaker.CircuitBreakerState) {
				mu.Lock()
				defer mu.Unlock()
				observer2Called = true
			},
		}

		cb := breaker.NewWithOptions(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithObserver(observer1),
			breaker.WithObserver(observer2),
		)

		// Trigger state change
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		// Wait for observers to be notified (they run in a goroutine)
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if !observer1Called {
			t.Error("Expected observer1 to be called")
		}

		if !observer2Called {
			t.Error("Expected observer2 to be called")
		}
	})

	t.Run("Should handle logging observer", func(t *testing.T) {
		var mu sync.Mutex
		var logMessage string
		observer := &breaker.LoggingObserver{
			LogFunc: func(msg string) {
				mu.Lock()
				defer mu.Unlock()
				logMessage = msg
			},
		}

		cb := breaker.NewWithOptions(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithObserver(observer),
		)

		// Trigger state change
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("error")
			})
		}

		// Wait for observer to be notified
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

// testObserver is a helper for testing observers
type testObserver struct {
	onStateChange func(ctx context.Context, from, to breaker.CircuitBreakerState)
}

func (o *testObserver) OnStateChange(ctx context.Context, from, to breaker.CircuitBreakerState) {
	if o.onStateChange != nil {
		o.onStateChange(ctx, from, to)
	}
}

// TestCircuitBreakerConcurrency tests concurrent access to the circuit breaker
func TestCircuitBreakerConcurrency(t *testing.T) {
	t.Run("Concurrent Execute calls should be thread-safe", func(t *testing.T) {
		cb := breaker.NewCircuitBreaker(100, 1*time.Second)

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
		cb := breaker.NewCircuitBreaker(5, 500*time.Millisecond)

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
		cb := breaker.NewCircuitBreaker(3, 100*time.Millisecond)

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
