//go:build ignore
// +build ignore

package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	breaker "github.com/farzai/breaker-go"
	"github.com/farzai/breaker-go/logging"
)

// This example demonstrates basic logging setup with the circuit breaker.
// It shows how to:
// - Create a logger with different log levels
// - Attach the logger to the circuit breaker
// - See logs for state transitions and operations

func main() {
	fmt.Println("=== Basic Logging Example ===\n")

	// Example 1: Default logger with INFO level
	fmt.Println("1. Using default logger with INFO level:")
	basicExample()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 2: Debug level for detailed logging
	fmt.Println("2. Using debug level for detailed logging:")
	debugExample()

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 3: Using convenience WithLogLevel option
	fmt.Println("3. Using WithLogLevel convenience option:")
	convenienceExample()
}

func basicExample() {
	// Create a logger with INFO level
	logger := logging.NewDefaultLogger(logging.LevelInfo)

	// Create circuit breaker with the logger
	cb, err := breaker.New(
		breaker.WithFailureThreshold(2),
		breaker.WithResetTimeout(5*time.Second),
		breaker.WithLogger(logger),
	)
	if err != nil {
		panic(err)
	}

	// Execute some operations - INFO logs will show state transitions
	fmt.Println("\nExecuting successful operation:")
	_, _ = cb.Execute(func() (interface{}, error) {
		fmt.Println("  → Operation succeeded")
		return "success", nil
	})

	// Trigger failures to see state transition logs
	fmt.Println("\nTriggering failures:")
	for i := 1; i <= 2; i++ {
		_, err := cb.Execute(func() (interface{}, error) {
			return nil, errors.New("simulated failure")
		})
		fmt.Printf("  → Attempt %d failed: %v\n", i, err)
		time.Sleep(100 * time.Millisecond)
	}

	// Try to execute when circuit is open
	fmt.Println("\nTrying to execute with open circuit:")
	_, err = cb.Execute(func() (interface{}, error) {
		return "should not execute", nil
	})
	if err != nil {
		fmt.Printf("  → Blocked: %v\n", err)
	}
}

func debugExample() {
	// Create a logger with DEBUG level for detailed logs
	logger := logging.NewDefaultLogger(logging.LevelDebug)

	cb, err := breaker.New(
		breaker.WithFailureThreshold(2),
		breaker.WithResetTimeout(3*time.Second),
		breaker.WithLogger(logger),
	)
	if err != nil {
		panic(err)
	}

	// With DEBUG level, you'll see:
	// - Action execution attempts
	// - Detailed state transition information
	// - Persistence operations (save/load)

	fmt.Println("\nExecuting operation (watch for DEBUG logs):")
	_, _ = cb.Execute(func() (interface{}, error) {
		fmt.Println("  → Executing...")
		return "result", nil
	})
}

func convenienceExample() {
	// The WithLogLevel option is a convenience that creates
	// a default logger for you

	cb, err := breaker.New(
		breaker.WithFailureThreshold(3),
		breaker.WithResetTimeout(5*time.Second),
		breaker.WithLogLevel(logging.LevelInfo),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("\nExecuting with convenience logger:")
	_, _ = cb.Execute(func() (interface{}, error) {
		fmt.Println("  → Operation executed")
		return "success", nil
	})

	// Reset operation will also be logged
	fmt.Println("\nManually resetting circuit breaker:")
	cb.Reset()
}
