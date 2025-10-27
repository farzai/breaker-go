package breaker_test

import (
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
