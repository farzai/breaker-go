package breaker_test

import (
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
}
