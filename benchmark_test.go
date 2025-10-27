package breaker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	breaker "github.com/farzai/breaker-go"
)

// BenchmarkCircuitBreakerExecute_Closed benchmarks execution in closed state
func BenchmarkCircuitBreakerExecute_Closed(b *testing.B) {
	cb := breaker.NewCircuitBreaker(1000, 5*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(func() (interface{}, error) {
			return "success", nil
		})
	}
}

// BenchmarkCircuitBreakerExecute_Open benchmarks execution in open state
func BenchmarkCircuitBreakerExecute_Open(b *testing.B) {
	cb := breaker.NewCircuitBreaker(1, 5*time.Second)

	// Open the circuit
	cb.Execute(func() (interface{}, error) {
		return nil, errors.New("failure")
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(func() (interface{}, error) {
			return "success", nil
		})
	}
}

// BenchmarkCircuitBreakerExecuteWithContext benchmarks context-based execution
func BenchmarkCircuitBreakerExecuteWithContext(b *testing.B) {
	cb := breaker.NewCircuitBreaker(1000, 5*time.Second)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.ExecuteWithContext(ctx, func(ctx context.Context) (interface{}, error) {
			return "success", nil
		})
	}
}

// BenchmarkCircuitBreakerState benchmarks state checking
func BenchmarkCircuitBreakerState(b *testing.B) {
	cb := breaker.NewCircuitBreaker(1000, 5*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.State()
	}
}

// BenchmarkCircuitBreakerExecute_Parallel benchmarks concurrent execution
func BenchmarkCircuitBreakerExecute_Parallel(b *testing.B) {
	cb := breaker.NewCircuitBreaker(1000, 5*time.Second)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Execute(func() (interface{}, error) {
				return "success", nil
			})
		}
	})
}

// BenchmarkCircuitBreakerWithObserver benchmarks with observer overhead
func BenchmarkCircuitBreakerWithObserver(b *testing.B) {
	observer := &breaker.LoggingObserver{
		LogFunc: func(msg string) {
			// No-op
		},
	}

	cb := breaker.NewWithOptions(
		breaker.WithFailureThreshold(1000),
		breaker.WithResetTimeout(5*time.Second),
		breaker.WithObserver(observer),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(func() (interface{}, error) {
			return "success", nil
		})
	}
}

// BenchmarkNewCircuitBreaker benchmarks circuit breaker creation
func BenchmarkNewCircuitBreaker(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = breaker.NewCircuitBreaker(5, 5*time.Second)
	}
}

// BenchmarkNewWithOptions benchmarks circuit breaker creation with options
func BenchmarkNewWithOptions(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = breaker.NewWithOptions(
			breaker.WithFailureThreshold(5),
			breaker.WithResetTimeout(5*time.Second),
		)
	}
}
