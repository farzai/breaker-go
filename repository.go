package breaker

import "sync"

type StateRepository interface {
	Save(state CircuitBreakerState)
	Load() CircuitBreakerState
}

type InMemoryStateStorage struct {
	state CircuitBreakerState
	mutex sync.Mutex
}

type InMemoryStateRepository struct {
	StateRepository

	storage *InMemoryStateStorage
}

func NewInMemoryStateRepository() *InMemoryStateRepository {
	return &InMemoryStateRepository{
		storage: &InMemoryStateStorage{
			state: Closed,
		},
	}
}

func (r *InMemoryStateRepository) Save(state CircuitBreakerState) {
	r.storage.mutex.Lock()
	defer r.storage.mutex.Unlock()

	r.storage.state = state
}

func (r *InMemoryStateRepository) Load() CircuitBreakerState {
	r.storage.mutex.Lock()
	defer r.storage.mutex.Unlock()

	return r.storage.state
}
