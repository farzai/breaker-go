//go:build ignore
// +build ignore

package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	breaker "github.com/farzai/breaker-go"
	"github.com/farzai/breaker-go/logging"
)

// This example demonstrates advanced logging patterns including:
// - Integration with Go's standard slog package
// - Custom logger adapters
// - Different loggers for different components
// - Contextual logging with fields

func main() {
	fmt.Println("=== Advanced Logging Example ===\n")

	// Example 1: Using slog adapter
	slogExample()

	// Example 2: Builder pattern for configuration
	builderExample()

	// Example 3: Separate persistence logger
	separateLoggersExample()
}

func slogExample() {
	fmt.Println("1. Using slog adapter (Go 1.21+ standard library)")
	fmt.Println(strings.Repeat("=", 50) + "\n")

	// Create a JSON slog handler for structured output
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slogger := slog.New(handler)

	// Wrap slog with our adapter
	logger := logging.NewSlogAdapter(slogger)

	// Create circuit breaker with slog logger
	cb, err := breaker.New(
		breaker.WithFailureThreshold(2),
		breaker.WithResetTimeout(3*time.Second),
		breaker.WithLogger(logger),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("Executing operations with JSON-formatted logs:\n")

	// Execute operations - logs will be in JSON format
	cb.Execute(func() (interface{}, error) {
		return "success", nil
	})

	cb.Execute(func() (interface{}, error) {
		return nil, errors.New("failure 1")
	})

	cb.Execute(func() (interface{}, error) {
		return nil, errors.New("failure 2")
	})

	fmt.Println()
}

func builderExample() {
	fmt.Println("\n2. Using builder pattern for configuration")
	fmt.Println(strings.Repeat("=", 50) + "\n")

	// Use the builder pattern for fluent configuration
	logger := logging.NewBuilder().
		WithLevel(logging.LevelInfo).
		WithFields(
			logging.String("service", "my-service"),
			logging.String("version", "1.0.0"),
			logging.String("environment", "production"),
		).
		Build()

	cb, err := breaker.New(
		breaker.WithFailureThreshold(3),
		breaker.WithResetTimeout(5*time.Second),
		breaker.WithLogger(logger),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("Logs will include service context fields:\n")

	cb.Execute(func() (interface{}, error) {
		return "result", nil
	})

	fmt.Println()
}

func separateLoggersExample() {
	fmt.Println("\n3. Using separate logger for persistence operations")
	fmt.Println(strings.Repeat("=", 50) + "\n")

	// Main logger at INFO level
	mainLogger := logging.NewBuilder().
		WithLevel(logging.LevelInfo).
		WithFields(logging.String("component", "circuit-breaker")).
		Build()

	// Persistence logger at DEBUG level for detailed persistence logs
	persistLogger := logging.NewBuilder().
		WithLevel(logging.LevelDebug).
		WithFields(logging.String("component", "persistence")).
		Build()

	// Create repository for persistence
	repo, err := breaker.NewTempFileSnapshotRepository()
	if err != nil {
		panic(err)
	}

	cb, err := breaker.New(
		breaker.WithFailureThreshold(2),
		breaker.WithResetTimeout(5*time.Second),
		breaker.WithLogger(mainLogger),              // Main logger
		breaker.WithPersistenceLogger(persistLogger), // Separate persistence logger
		breaker.WithSnapshotRepository(repo),
		breaker.WithStateRestoration(true),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("Main operations logged at INFO level,")
	fmt.Println("Persistence operations logged at DEBUG level:\n")

	// Trigger state change which will trigger persistence
	cb.Execute(func() (interface{}, error) {
		return nil, errors.New("failure")
	})

	cb.Execute(func() (interface{}, error) {
		return nil, errors.New("failure")
	})

	fmt.Println()
}
