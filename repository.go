package breaker

import "sync"

// StateRepository defines the interface for persisting circuit breaker state.
// Implementations must be thread-safe.
type StateRepository interface {
	// Save stores the current circuit breaker state.
	Save(state CircuitBreakerState)

	// Load retrieves the current circuit breaker state.
	Load() CircuitBreakerState

	// Reset resets the state back to Closed.
	Reset()
}

// InMemoryStateRepository provides an in-memory, thread-safe implementation
// of StateRepository. It is suitable for single-instance applications.
type InMemoryStateRepository struct {
	state CircuitBreakerState
	mutex sync.Mutex
}

// NewInMemoryStateRepository creates a new in-memory state repository
// initialized with the Closed state.
func NewInMemoryStateRepository() *InMemoryStateRepository {
	return &InMemoryStateRepository{
		state: Closed,
	}
}

// Save stores the circuit breaker state. This method is thread-safe.
func (r *InMemoryStateRepository) Save(state CircuitBreakerState) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.state = state
}

// Load retrieves the current circuit breaker state. This method is thread-safe.
func (r *InMemoryStateRepository) Load() CircuitBreakerState {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.state
}

// Reset resets the circuit breaker state to Closed. This method is thread-safe.
func (r *InMemoryStateRepository) Reset() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.state = Closed
}
