package breaker_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	breaker "github.com/farzai/breaker-go"
)

// Example demonstrates basic usage of the circuit breaker
func Example() {
	// Create a circuit breaker with 3 failure threshold and 5 second timeout
	cb := breaker.NewCircuitBreaker(3, 5*time.Second)

	// Execute a protected operation
	result, err := cb.Execute(func() (interface{}, error) {
		// Simulate external service call
		return "success", nil
	})

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Result:", result)
	// Output: Result: success
}

// ExampleNewWithOptions demonstrates using functional options for configuration
func ExampleNewWithOptions() {
	cb := breaker.NewWithOptions(
		breaker.WithFailureThreshold(5),
		breaker.WithResetTimeout(10*time.Second),
		breaker.WithStateChangeCallback(func(from, to breaker.CircuitBreakerState) {
			fmt.Printf("State changed from %d to %d\n", from, to)
		}),
	)

	// Use the circuit breaker
	result, err := cb.Execute(func() (interface{}, error) {
		return "configured", nil
	})

	if err == nil {
		fmt.Println(result)
	}
	// Output: configured
}

// ExampleCircuitBreaker_ExecuteWithContext demonstrates context-based execution
func ExampleCircuitBreaker_ExecuteWithContext() {
	cb := breaker.NewCircuitBreaker(3, 5*time.Second)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Execute with context
	result, err := cb.ExecuteWithContext(ctx, func(ctx context.Context) (interface{}, error) {
		// Check context during long operation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return "completed", nil
		}
	})

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println(result)
	// Output: completed
}

// ExampleCircuitBreaker_State demonstrates checking circuit breaker state
func ExampleCircuitBreaker_State() {
	cb := breaker.NewCircuitBreaker(2, 1*time.Second)

	fmt.Println("Initial state:", cb.State() == breaker.Closed)

	// Trigger failures
	cb.Execute(func() (interface{}, error) {
		return nil, errors.New("failure 1")
	})
	cb.Execute(func() (interface{}, error) {
		return nil, errors.New("failure 2")
	})

	fmt.Println("After failures:", cb.State() == breaker.Open)
	// Output:
	// Initial state: true
	// After failures: true
}

// ExampleWithObserver demonstrates using an observer for state changes
func ExampleWithObserver() {
	// Create a logging observer
	observer := &breaker.LoggingObserver{
		LogFunc: func(msg string) {
			fmt.Println("Observer:", msg)
		},
	}

	cb := breaker.NewWithOptions(
		breaker.WithFailureThreshold(2),
		breaker.WithResetTimeout(1*time.Second),
		breaker.WithObserver(observer),
	)

	// Trigger failures to change state
	cb.Execute(func() (interface{}, error) {
		return nil, errors.New("failure")
	})
	cb.Execute(func() (interface{}, error) {
		return nil, errors.New("failure")
	})

	// Give goroutine time to notify observer
	time.Sleep(10 * time.Millisecond)

	fmt.Println("Circuit breaker state changed")
	// Output:
	// Observer: Circuit breaker state changed from Closed to Open
	// Circuit breaker state changed
}
