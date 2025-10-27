package events

import (
	"context"
	"fmt"
	"time"

	"github.com/farzai/breaker-go/logging"
)

// Middleware wraps an EventHandler to add cross-cutting concerns.
// Middleware can be chained together to compose behaviors like:
//   - Panic recovery
//   - Logging
//   - Metrics
//   - Timeouts
//   - Context enrichment
//
// Middleware is applied in the order they are added to the EventBus.
type Middleware func(next EventHandler) EventHandler

// EventHandler is a function that processes an event.
// It's the core abstraction that middleware wraps.
type EventHandler func(StateChangeEvent) error

// Chain combines multiple middleware into a single middleware.
// Middleware are applied in the order provided (left to right).
//
// Example:
//
//	combined := Chain(
//	    PanicRecoveryMiddleware(handler),
//	    LoggingMiddleware(logger),
//	    MetricsMiddleware(recorder),
//	)
func Chain(middleware ...Middleware) Middleware {
	return func(final EventHandler) EventHandler {
		// Apply middleware in reverse order so they execute in forward order
		for i := len(middleware) - 1; i >= 0; i-- {
			final = middleware[i](final)
		}
		return final
	}
}

// PanicRecoveryMiddleware recovers from panics in event handlers.
// If a handler panics, the panic is recovered and passed to the PanicHandler.
// The middleware returns an error indicating a panic occurred.
//
// This prevents a single misbehaving listener from crashing the entire application.
//
// Example:
//
//	middleware := PanicRecoveryMiddleware(func(recovered interface{}, event StateChangeEvent) {
//	    log.Printf("Listener panicked: %v", recovered)
//	})
func PanicRecoveryMiddleware(handler PanicHandler) Middleware {
	return func(next EventHandler) EventHandler {
		return func(event StateChangeEvent) (err error) {
			defer func() {
				if r := recover(); r != nil {
					if handler != nil {
						handler(r, event)
					}
					err = fmt.Errorf("listener panicked: %v", r)
				}
			}()

			return next(event)
		}
	}
}

// LoggingMiddleware logs event processing using structured logging.
// It logs before and after the event is processed, including any errors.
//
// Example:
//
//	logger := logging.NewDefaultLogger(logging.LevelInfo)
//	middleware := LoggingMiddleware(logger)
func LoggingMiddleware(logger logging.Logger) Middleware {
	return func(next EventHandler) EventHandler {
		return func(event StateChangeEvent) error {
			if logger != nil && logger.Enabled(logging.LevelDebug) {
				logger.Debug("Event processing started",
					logging.String("from", event.From.String()),
					logging.String("to", event.To.String()),
					logging.Time("timestamp", event.Timestamp),
				)
			}

			start := time.Now()
			err := next(event)
			duration := time.Since(start)

			if logger != nil {
				if err != nil {
					logger.Error("Event processing failed",
						logging.String("from", event.From.String()),
						logging.String("to", event.To.String()),
						logging.Duration("duration", duration),
						logging.Error(err),
					)
				} else if logger.Enabled(logging.LevelDebug) {
					logger.Debug("Event processing completed",
						logging.String("from", event.From.String()),
						logging.String("to", event.To.String()),
						logging.Duration("duration", duration),
					)
				}
			}

			return err
		}
	}
}

// MetricsMiddleware records metrics about event processing.
// It tracks:
//   - Processing duration
//   - Success/failure counts
//   - Errors and panics
//
// Example:
//
//	recorder := &SimpleMetricsRecorder{}
//	middleware := MetricsMiddleware(recorder)
func MetricsMiddleware(recorder MetricsRecorder) Middleware {
	return func(next EventHandler) EventHandler {
		return func(event StateChangeEvent) error {
			start := time.Now()

			err := next(event)

			duration := time.Since(start)
			success := err == nil

			if recorder != nil {
				recorder.RecordEventProcessed(duration, success)
			}

			return err
		}
	}
}

// TimeoutMiddleware adds a timeout to event processing.
// If the handler doesn't complete within the timeout, it returns an error.
//
// Note: This doesn't actually cancel the handler (that would require context).
// It just returns early. For true cancellation, use ContextTimeoutMiddleware.
//
// Example:
//
//	middleware := TimeoutMiddleware(100 * time.Millisecond)
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next EventHandler) EventHandler {
		return func(event StateChangeEvent) error {
			done := make(chan error, 1)

			go func() {
				done <- next(event)
			}()

			select {
			case err := <-done:
				return err
			case <-time.After(timeout):
				return fmt.Errorf("event processing timeout after %v", timeout)
			}
		}
	}
}

// ContextTimeoutMiddleware adds a timeout using the event's context.
// If the event context already has a deadline, it uses that.
// Otherwise, it creates a new context with the specified timeout.
//
// This provides true cancellation if the handler respects context cancellation.
//
// Example:
//
//	middleware := ContextTimeoutMiddleware(100 * time.Millisecond)
func ContextTimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next EventHandler) EventHandler {
		return func(event StateChangeEvent) error {
			ctx := event.Context
			if ctx == nil {
				ctx = context.Background()
			}

			// Check if context already has a deadline
			if _, hasDeadline := ctx.Deadline(); !hasDeadline {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()

				// Update event with new context
				event.Context = ctx
			}

			return next(event)
		}
	}
}

// ErrorHandlerMiddleware wraps errors from listeners with additional context.
// It calls the provided ErrorHandler for each error, allowing centralized error handling.
//
// Example:
//
//	middleware := ErrorHandlerMiddleware(func(err error, event StateChangeEvent, listenerID string) {
//	    log.Printf("Listener %s failed: %v", listenerID, err)
//	})
func ErrorHandlerMiddleware(handler ErrorHandler) Middleware {
	return func(next EventHandler) EventHandler {
		return func(event StateChangeEvent) error {
			err := next(event)
			if err != nil && handler != nil {
				// Extract listener ID from metadata if available
				listenerID := "unknown"
				if id, ok := event.Metadata["listener_id"].(string); ok {
					listenerID = id
				}
				handler(err, event, listenerID)
			}
			return err
		}
	}
}

// FilterMiddleware conditionally processes events based on a predicate.
// If the predicate returns false, the event is skipped.
//
// Example:
//
//	// Only process Open state transitions
//	middleware := FilterMiddleware(func(event StateChangeEvent) bool {
//	    return event.To == Open
//	})
func FilterMiddleware(predicate func(StateChangeEvent) bool) Middleware {
	return func(next EventHandler) EventHandler {
		return func(event StateChangeEvent) error {
			if predicate == nil || predicate(event) {
				return next(event)
			}
			// Event filtered out, no error
			return nil
		}
	}
}

// RetryMiddleware retries failed event processing.
// It retries up to maxRetries times with a fixed delay between retries.
//
// Example:
//
//	middleware := RetryMiddleware(3, 100*time.Millisecond)
func RetryMiddleware(maxRetries int, delay time.Duration) Middleware {
	return func(next EventHandler) EventHandler {
		return func(event StateChangeEvent) error {
			var err error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				err = next(event)
				if err == nil {
					return nil
				}

				// Don't delay after the last attempt
				if attempt < maxRetries {
					time.Sleep(delay)
				}
			}

			return fmt.Errorf("failed after %d retries: %w", maxRetries, err)
		}
	}
}

// ContextEnrichmentMiddleware adds additional context to events.
// This is useful for adding tracing IDs, request IDs, or other metadata.
//
// Example:
//
//	middleware := ContextEnrichmentMiddleware(func(event StateChangeEvent) StateChangeEvent {
//	    if event.Metadata == nil {
//	        event.Metadata = make(map[string]interface{})
//	    }
//	    event.Metadata["trace_id"] = generateTraceID()
//	    return event
//	})
func ContextEnrichmentMiddleware(enricher func(StateChangeEvent) StateChangeEvent) Middleware {
	return func(next EventHandler) EventHandler {
		return func(event StateChangeEvent) error {
			if enricher != nil {
				event = enricher(event)
			}
			return next(event)
		}
	}
}
