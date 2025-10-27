package breaker_test

import (
	"sync"
	"testing"

	breaker "github.com/farzai/breaker-go"
)

func TestInMemoryStateStorage(t *testing.T) {

	t.Run("Should be standby when initialized", func(t *testing.T) {
		storage := breaker.NewInMemoryStateRepository()

		if storage.Load() != breaker.Closed {
			t.Errorf("Expected initial state to be Closed, got %v", storage.Load())
		}
	})

	t.Run("Should be open when set to open", func(t *testing.T) {
		storage := breaker.NewInMemoryStateRepository()

		storage.Save(breaker.Open)

		if storage.Load() != breaker.Open {
			t.Errorf("Expected state to be Open, got %v", storage.Load())
		}
	})

	t.Run("Should be closed when set to closed", func(t *testing.T) {
		storage := breaker.NewInMemoryStateRepository()

		storage.Save(breaker.Closed)

		if storage.Load() != breaker.Closed {
			t.Errorf("Expected state to be Closed, got %v", storage.Load())
		}
	})

	t.Run("Should be half open when set to half open", func(t *testing.T) {
		storage := breaker.NewInMemoryStateRepository()

		storage.Save(breaker.HalfOpen)

		if storage.Load() != breaker.HalfOpen {
			t.Errorf("Expected state to be HalfOpen, got %v", storage.Load())
		}
	})

	t.Run("Should be standby when reset", func(t *testing.T) {
		storage := breaker.NewInMemoryStateRepository()

		storage.Save(breaker.Open)
		storage.Reset()

		if storage.Load() != breaker.Closed {
			t.Errorf("Expected state to be Closed, got %v", storage.Load())
		}
	})

	t.Run("Should be thread-safe for concurrent Save operations", func(t *testing.T) {
		storage := breaker.NewInMemoryStateRepository()
		const numGoroutines = 100
		const numOperations = 50

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Launch multiple goroutines performing Save operations
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					// Cycle through states
					state := breaker.CircuitBreakerState(j % 3)
					storage.Save(state)
				}
			}(i)
		}

		wg.Wait()

		// Verify we can still read the state without error
		_ = storage.Load()
	})

	t.Run("Should be thread-safe for concurrent Load operations", func(t *testing.T) {
		storage := breaker.NewInMemoryStateRepository()
		storage.Save(breaker.Open)
		const numGoroutines = 100
		const numOperations = 50

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Launch multiple goroutines performing Load operations
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					_ = storage.Load()
				}
			}()
		}

		wg.Wait()
	})

	t.Run("Should be thread-safe for mixed Save and Load operations", func(t *testing.T) {
		storage := breaker.NewInMemoryStateRepository()
		const numGoroutines = 100

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Half save, half load
		for i := 0; i < numGoroutines; i++ {
			if i%2 == 0 {
				go func(id int) {
					defer wg.Done()
					for j := 0; j < 50; j++ {
						state := breaker.CircuitBreakerState(j % 3)
						storage.Save(state)
					}
				}(i)
			} else {
				go func() {
					defer wg.Done()
					for j := 0; j < 50; j++ {
						_ = storage.Load()
					}
				}()
			}
		}

		wg.Wait()
	})

	t.Run("Should be thread-safe for concurrent Reset operations", func(t *testing.T) {
		storage := breaker.NewInMemoryStateRepository()
		storage.Save(breaker.Open)
		const numGoroutines = 50

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		// Launch multiple goroutines performing Reset operations
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				storage.Reset()
			}()
		}

		wg.Wait()

		// Verify state is Closed
		if storage.Load() != breaker.Closed {
			t.Errorf("Expected state to be Closed after concurrent resets, got %v", storage.Load())
		}
	})
}
