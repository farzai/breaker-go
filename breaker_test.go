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

func TestCircuitBreakerSuccessfulExecution(t *testing.T) {
	cb := breaker.NewCircuitBreaker(3, 1*time.Second)
	successAction := func() (interface{}, error) {
		return "Success", nil
	}

	result, err := cb.Execute(successAction)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != "Success" {
		t.Errorf("Expected result to be 'Success', got %v", result)
	}
}

func TestCircuitBreakerOpenOnFailures(t *testing.T) {
	cb := breaker.NewCircuitBreaker(3, 1*time.Second)
	failureAction := func() (interface{}, error) {
		return nil, errors.New("Error")
	}

	for i := 0; i < 3; i++ {
		_, _ = cb.Execute(failureAction)
	}

	if cb.State() != breaker.Open {
		t.Errorf("Expected state to be Open, got %v", cb.State())
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := breaker.NewCircuitBreaker(3, 1*time.Second)
	failureAction := func() (interface{}, error) {
		return nil, errors.New("Error")
	}

	for i := 0; i < 3; i++ {
		_, _ = cb.Execute(failureAction)
	}

	cb.Reset()

	if cb.State() != breaker.Closed {
		t.Errorf("Expected state to be Closed after reset, got %v", cb.State())
	}
}

func TestCircuitBreakerOpenToHalfOpen(t *testing.T) {
	cb := breaker.NewCircuitBreaker(3, 1*time.Second)
	failureAction := func() (interface{}, error) {
		return nil, errors.New("Error")
	}

	for i := 0; i < 3; i++ {
		_, _ = cb.Execute(failureAction)
	}

	if cb.State() != breaker.Open {
		t.Errorf("Expected state to be Open, got %v", cb.State())
	}

	time.Sleep(2 * time.Second) // wait for the reset timeout to expire

	if cb.State() != breaker.HalfOpen {
		t.Errorf("Expected state to be HalfOpen, got %v", cb.State())
	}
}

func TestCircuitBreakerHalfOpenToClosed(t *testing.T) {
	cb := breaker.NewCircuitBreaker(3, 1*time.Second)
	failureAction := func() (interface{}, error) {
		return nil, errors.New("Error")
	}
	successAction := func() (interface{}, error) {
		return "Success", nil
	}

	for i := 0; i < 3; i++ {
		_, _ = cb.Execute(failureAction)
	}

	time.Sleep(2 * time.Second) // wait for the reset timeout to expire

	_, _ = cb.Execute(successAction)

	if cb.State() != breaker.Closed {
		t.Errorf("Expected state to be Closed, got %v", cb.State())
	}
}

func TestCircuitBreakerHalfOpenToOpen(t *testing.T) {
	cb := breaker.NewCircuitBreaker(3, 1*time.Second)
	failureAction := func() (interface{}, error) {
		return nil, errors.New("Error")
	}

	for i := 0; i < 3; i++ {
		_, _ = cb.Execute(failureAction)
	}

	if cb.State() != breaker.Open {
		t.Errorf("Expected state to be Open, got %v", cb.State())
	}

	time.Sleep(2 * time.Second) // wait for the reset timeout to expire

	_, _ = cb.Execute(failureAction)

	if cb.State() != breaker.Open {
		t.Errorf("Expected state to be Open, got %v", cb.State())
	}
}

func TestCircuitBreakerHalfOpenToOpenOnFailure(t *testing.T) {
	cb := breaker.NewCircuitBreaker(3, 1*time.Second)
	failureAction := func() (interface{}, error) {
		return nil, errors.New("Error")
	}
	successAction := func() (interface{}, error) {
		return "Success", nil
	}

	for i := 0; i < 3; i++ {
		_, _ = cb.Execute(failureAction)
	}

	time.Sleep(2 * time.Second) // wait for the reset timeout to expire

	_, _ = cb.Execute(successAction)

	if cb.State() != breaker.Closed {
		t.Errorf("Expected state to be Closed, got %v", cb.State())
	}

	_, _ = cb.Execute(failureAction)

	if cb.State() != breaker.Open {
		t.Errorf("Expected state to be Open, got %v", cb.State())
	}
}
