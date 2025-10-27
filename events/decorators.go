package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/farzai/breaker-go/logging"
)

// Decorator wraps an EventListener to add additional behavior.
// Decorators use the Decorator pattern to enhance listeners without modifying them.
//
// Common decorators:
//   - TimeoutDecorator: Add timeout to listener execution
//   - RetryDecorator: Retry failed listeners
//   - FilterDecorator: Conditionally execute listeners
//   - RateLimitDecorator: Throttle listener execution
//   - CircuitBreakerDecorator: Protect against cascading failures

// ListenerWithTimeout wraps a listener with a timeout.
// If the listener doesn't complete within the timeout, an error is returned.
//
// Note: This doesn't cancel the listener goroutine. For true cancellation,
// the listener should respect context.Context.
//
// Example:
//
//	listener := ListenerWithTimeout(slowListener, 100*time.Millisecond)
func ListenerWithTimeout(listener EventListener, timeout time.Duration) EventListener {
	return EventListenerFunc(func(event StateChangeEvent) error {
		done := make(chan error, 1)

		go func() {
			done <- listener.OnEvent(event)
		}()

		select {
		case err := <-done:
			return err
		case <-time.After(timeout):
			return fmt.Errorf("listener timeout after %v", timeout)
		}
	})
}

// ListenerWithContextTimeout wraps a listener with a context-based timeout.
// The listener should respect the context for proper cancellation.
//
// Example:
//
//	listener := ListenerWithContextTimeout(contextAwareListener, 100*time.Millisecond)
func ListenerWithContextTimeout(listener EventListener, timeout time.Duration) EventListener {
	return EventListenerFunc(func(event StateChangeEvent) error {
		ctx := event.Context
		if ctx == nil {
			ctx = context.Background()
		}

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Update event with timeout context
		event.Context = ctx

		return listener.OnEvent(event)
	})
}

// ListenerWithRetry wraps a listener with retry logic.
// If the listener returns an error, it will be retried up to maxRetries times
// with a delay between attempts.
//
// Example:
//
//	listener := ListenerWithRetry(flakyListener, 3, 100*time.Millisecond)
func ListenerWithRetry(listener EventListener, maxRetries int, delay time.Duration) EventListener {
	return EventListenerFunc(func(event StateChangeEvent) error {
		var err error

		for attempt := 0; attempt <= maxRetries; attempt++ {
			err = listener.OnEvent(event)
			if err == nil {
				return nil
			}

			// Don't delay after the last attempt
			if attempt < maxRetries {
				time.Sleep(delay)
			}
		}

		return fmt.Errorf("failed after %d retries: %w", maxRetries, err)
	})
}

// ListenerWithFilter wraps a listener with a filter predicate.
// The listener is only called if the predicate returns true.
//
// Example:
//
//	// Only notify on state transitions to Open
//	listener := ListenerWithFilter(myListener, func(event StateChangeEvent) bool {
//	    return event.To == Open
//	})
func ListenerWithFilter(listener EventListener, predicate func(StateChangeEvent) bool) EventListener {
	return EventListenerFunc(func(event StateChangeEvent) error {
		if predicate == nil || predicate(event) {
			return listener.OnEvent(event)
		}
		// Filtered out, no error
		return nil
	})
}

// ListenerWithRateLimit wraps a listener with rate limiting.
// The listener will only be called at most 'rate' times per second.
// Additional events are silently dropped.
//
// Example:
//
//	listener := ListenerWithRateLimit(expensiveListener, 10) // Max 10 calls/second
func ListenerWithRateLimit(listener EventListener, rate int) EventListener {
	if rate <= 0 {
		rate = 1
	}

	interval := time.Second / time.Duration(rate)
	limiter := &rateLimiter{
		interval:   interval,
		lastCalled: time.Now().Add(-interval), // Allow immediate first call
	}

	return EventListenerFunc(func(event StateChangeEvent) error {
		if !limiter.allow() {
			// Rate limit exceeded, drop event
			return nil
		}
		return listener.OnEvent(event)
	})
}

type rateLimiter struct {
	mu         sync.Mutex
	interval   time.Duration
	lastCalled time.Time
}

func (r *rateLimiter) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.Sub(r.lastCalled) >= r.interval {
		r.lastCalled = now
		return true
	}
	return false
}

// ListenerWithPanicRecovery wraps a listener with panic recovery.
// If the listener panics, the panic is recovered and converted to an error.
// The optional PanicHandler is called with the recovered value.
//
// Example:
//
//	listener := ListenerWithPanicRecovery(panickyListener, func(recovered interface{}, event StateChangeEvent) {
//	    log.Printf("Listener panicked: %v", recovered)
//	})
func ListenerWithPanicRecovery(listener EventListener, handler PanicHandler) EventListener {
	return EventListenerFunc(func(event StateChangeEvent) (err error) {
		defer func() {
			if r := recover(); r != nil {
				if handler != nil {
					handler(r, event)
				}
				err = fmt.Errorf("listener panicked: %v", r)
			}
		}()

		return listener.OnEvent(event)
	})
}

// ListenerWithCircuitBreaker wraps a listener with a circuit breaker to prevent cascading failures.
// If the listener fails consecutively more than maxFailures times, the circuit opens
// and subsequent calls fail fast until the resetTimeout expires.
//
// This is useful when a listener depends on an external service that might be down.
//
// Example:
//
//	listener := ListenerWithCircuitBreaker(externalServiceListener, 5, 30*time.Second)
func ListenerWithCircuitBreaker(listener EventListener, maxFailures int, resetTimeout time.Duration) EventListener {
	cb := &listenerCircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
	}

	return EventListenerFunc(func(event StateChangeEvent) error {
		return cb.call(func() error {
			return listener.OnEvent(event)
		})
	})
}

type listenerCircuitBreaker struct {
	mu            sync.Mutex
	failures      int
	maxFailures   int
	resetTimeout  time.Duration
	lastFailure   time.Time
	isOpen        bool
}

func (cb *listenerCircuitBreaker) call(fn func() error) error {
	cb.mu.Lock()

	// Check if we should transition from open to half-open
	if cb.isOpen && time.Since(cb.lastFailure) >= cb.resetTimeout {
		cb.isOpen = false
		cb.failures = 0
	}

	// Fail fast if circuit is open
	if cb.isOpen {
		cb.mu.Unlock()
		return fmt.Errorf("circuit breaker is open")
	}

	cb.mu.Unlock()

	// Execute the function
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.failures >= cb.maxFailures {
			cb.isOpen = true
		}
	} else {
		// Success resets the counter
		cb.failures = 0
	}

	return err
}

// ListenerWithAsync wraps a listener to execute asynchronously in a goroutine.
// The listener always returns nil immediately, and errors are silently ignored.
//
// Use this when you want a specific listener to not block, even if others are synchronous.
//
// Example:
//
//	listener := ListenerWithAsync(slowListener)
func ListenerWithAsync(listener EventListener) EventListener {
	return EventListenerFunc(func(event StateChangeEvent) error {
		go func() {
			_ = listener.OnEvent(event)
		}()
		return nil
	})
}

// ListenerWithLogging wraps a listener with structured logging of event processing.
//
// Example:
//
//	logger := logging.NewDefaultLogger(logging.LevelDebug)
//	listener := ListenerWithLogging(myListener, logger)
func ListenerWithLogging(listener EventListener, logger logging.Logger) EventListener {
	return EventListenerFunc(func(event StateChangeEvent) error {
		if logger != nil && logger.Enabled(logging.LevelDebug) {
			logger.Debug("Listener started",
				logging.String("from", event.From.String()),
				logging.String("to", event.To.String()),
			)
		}

		start := time.Now()
		err := listener.OnEvent(event)
		duration := time.Since(start)

		if logger != nil {
			if err != nil {
				logger.Error("Listener failed",
					logging.String("from", event.From.String()),
					logging.String("to", event.To.String()),
					logging.Duration("duration", duration),
					logging.Error(err),
				)
			} else if logger.Enabled(logging.LevelDebug) {
				logger.Debug("Listener completed",
					logging.String("from", event.From.String()),
					logging.String("to", event.To.String()),
					logging.Duration("duration", duration),
				)
			}
		}

		return err
	})
}

// ListenerWithMetrics wraps a listener with metrics recording.
//
// Example:
//
//	listener := ListenerWithMetrics(myListener, recorder, "my-listener")
func ListenerWithMetrics(listener EventListener, recorder MetricsRecorder, listenerID string) EventListener {
	return EventListenerFunc(func(event StateChangeEvent) error {
		start := time.Now()

		err := listener.OnEvent(event)

		if recorder != nil {
			duration := time.Since(start)
			recorder.RecordEventProcessed(duration, err == nil)

			if err != nil {
				recorder.RecordListenerError(listenerID, err)
			}
		}

		return err
	})
}

// Compose combines multiple decorators into a single decorator.
// Decorators are applied in the order provided (inner to outer).
//
// Example:
//
//	listener := Compose(
//	    baseListener,
//	    WithRetry(3, 100*time.Millisecond),
//	    WithTimeout(1*time.Second),
//	    WithPanicRecovery(panicHandler),
//	)
func Compose(listener EventListener, decorators ...func(EventListener) EventListener) EventListener {
	result := listener
	for _, decorator := range decorators {
		result = decorator(result)
	}
	return result
}
