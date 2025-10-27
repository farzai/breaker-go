package events

import (
	"fmt"
	"sync"
)

// DeliveryStrategy defines how events are delivered to listeners.
// Different strategies can be used based on performance and ordering requirements.
//
// Implementations include:
//   - SynchronousStrategy: Sequential, blocking delivery
//   - AsynchronousStrategy: Fire-and-forget with goroutines
//   - ParallelStrategy: Concurrent delivery with error collection
type DeliveryStrategy interface {
	// Deliver sends an event to all listeners using the strategy's delivery mechanism.
	// Returns an error if delivery fails (strategy-dependent).
	Deliver(event StateChangeEvent, listeners []ListenerInfo) error
}

// SynchronousStrategy delivers events to listeners sequentially in the calling goroutine.
// This strategy:
//   - Blocks until all listeners complete
//   - Preserves order of listener execution
//   - Returns errors from listeners
//   - Is deterministic and easy to test
//
// Use this when you need guaranteed ordering and error handling.
type SynchronousStrategy struct{}

// NewSynchronousStrategy creates a new synchronous delivery strategy.
func NewSynchronousStrategy() *SynchronousStrategy {
	return &SynchronousStrategy{}
}

// Deliver implements DeliveryStrategy
func (s *SynchronousStrategy) Deliver(event StateChangeEvent, listeners []ListenerInfo) error {
	var errors []error

	for _, info := range listeners {
		if err := info.Listener.OnEvent(event); err != nil {
			errors = append(errors, fmt.Errorf("listener %s: %w", info.ID, err))
		}
	}

	if len(errors) > 0 {
		return &MultiError{Errors: errors}
	}

	return nil
}

// AsynchronousStrategy delivers events to listeners in separate goroutines (fire-and-forget).
// This strategy:
//   - Returns immediately without waiting for listeners
//   - Does not guarantee order of execution
//   - Does not return errors from listeners (errors are lost unless middleware captures them)
//   - Maximizes throughput
//
// Use this when you don't need to know if listeners succeed and want maximum performance.
type AsynchronousStrategy struct{}

// NewAsynchronousStrategy creates a new asynchronous delivery strategy.
func NewAsynchronousStrategy() *AsynchronousStrategy {
	return &AsynchronousStrategy{}
}

// Deliver implements DeliveryStrategy
func (s *AsynchronousStrategy) Deliver(event StateChangeEvent, listeners []ListenerInfo) error {
	// Fire and forget - spawn goroutines for each listener
	for _, info := range listeners {
		go func(listener EventListener) {
			// Errors are silently ignored in fire-and-forget mode
			// Use middleware to capture errors if needed
			_ = listener.OnEvent(event)
		}(info.Listener)
	}

	return nil
}

// ParallelStrategy delivers events to all listeners concurrently and waits for completion.
// This strategy:
//   - Executes all listeners in parallel
//   - Waits for all to complete before returning
//   - Collects and returns all errors
//   - Does not guarantee order
//
// Use this when you want parallel execution but need to know the results.
type ParallelStrategy struct{}

// NewParallelStrategy creates a new parallel delivery strategy.
func NewParallelStrategy() *ParallelStrategy {
	return &ParallelStrategy{}
}

// Deliver implements DeliveryStrategy
func (s *ParallelStrategy) Deliver(event StateChangeEvent, listeners []ListenerInfo) error {
	if len(listeners) == 0 {
		return nil
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		errors []error
	)

	wg.Add(len(listeners))

	for _, info := range listeners {
		go func(id string, listener EventListener) {
			defer wg.Done()

			if err := listener.OnEvent(event); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("listener %s: %w", id, err))
				mu.Unlock()
			}
		}(info.ID, info.Listener)
	}

	wg.Wait()

	if len(errors) > 0 {
		return &MultiError{Errors: errors}
	}

	return nil
}

// OrderedAsyncStrategy delivers events asynchronously but maintains order.
// Events are queued and processed sequentially in a background goroutine.
// This strategy:
//   - Returns immediately (non-blocking)
//   - Preserves event order
//   - Processes events in background
//   - Suitable for high-throughput scenarios where order matters
//
// Use this when you need ordered processing without blocking the publisher.
type OrderedAsyncStrategy struct {
	queue chan eventDelivery
	wg    sync.WaitGroup
	once  sync.Once
}

type eventDelivery struct {
	event     StateChangeEvent
	listeners []ListenerInfo
}

// NewOrderedAsyncStrategy creates a new ordered async delivery strategy.
// bufferSize determines how many events can be queued before Deliver blocks.
func NewOrderedAsyncStrategy(bufferSize int) *OrderedAsyncStrategy {
	if bufferSize < 1 {
		bufferSize = 100 // default buffer
	}

	s := &OrderedAsyncStrategy{
		queue: make(chan eventDelivery, bufferSize),
	}

	s.start()
	return s
}

func (s *OrderedAsyncStrategy) start() {
	s.once.Do(func() {
		s.wg.Add(1)
		go s.processQueue()
	})
}

func (s *OrderedAsyncStrategy) processQueue() {
	defer s.wg.Done()

	for delivery := range s.queue {
		// Process synchronously to maintain order
		for _, info := range delivery.listeners {
			_ = info.Listener.OnEvent(delivery.event)
		}
	}
}

// Deliver implements DeliveryStrategy
func (s *OrderedAsyncStrategy) Deliver(event StateChangeEvent, listeners []ListenerInfo) error {
	// Make a copy of listeners to avoid race conditions
	listenersCopy := make([]ListenerInfo, len(listeners))
	copy(listenersCopy, listeners)

	select {
	case s.queue <- eventDelivery{event: event, listeners: listenersCopy}:
		return nil
	default:
		// Queue is full
		return fmt.Errorf("event queue is full")
	}
}

// Close stops the background processor and waits for pending events.
// After Close, Deliver will panic.
func (s *OrderedAsyncStrategy) Close() {
	close(s.queue)
	s.wg.Wait()
}
