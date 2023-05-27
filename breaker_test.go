package breaker_test

import (
	"errors"
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
