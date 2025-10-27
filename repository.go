package breaker

import (
	"context"
	"sync"
)

// SnapshotRepository defines the interface for persisting circuit breaker snapshots.
// Implementations must be thread-safe and handle concurrent access.
//
// The repository provides context-aware operations for timeout and cancellation control.
type SnapshotRepository interface {
	// Save stores a complete circuit breaker snapshot.
	// Returns an error if the operation fails.
	Save(ctx context.Context, snapshot *Snapshot) error

	// Load retrieves the stored snapshot.
	// Returns ErrSnapshotNotFound if no snapshot exists.
	Load(ctx context.Context) (*Snapshot, error)

	// Exists checks if a snapshot is stored.
	Exists(ctx context.Context) (bool, error)

	// Delete removes the stored snapshot.
	Delete(ctx context.Context) error
}

// InMemorySnapshotRepository provides an in-memory, thread-safe implementation
// of SnapshotRepository. It is suitable for single-instance applications and testing.
type InMemorySnapshotRepository struct {
	snapshot *Snapshot
	mutex    sync.RWMutex
}

// NewInMemorySnapshotRepository creates a new in-memory snapshot repository
func NewInMemorySnapshotRepository() *InMemorySnapshotRepository {
	return &InMemorySnapshotRepository{}
}

// Save stores the circuit breaker snapshot. This method is thread-safe.
func (r *InMemorySnapshotRepository) Save(ctx context.Context, snapshot *Snapshot) error {
	if snapshot == nil {
		return NewValidationError("snapshot", "cannot be nil", nil)
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return NewPersistenceError("save", ctx.Err())
	default:
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Store a clone to prevent external modifications
	r.snapshot = snapshot.Clone()
	return nil
}

// Load retrieves the stored snapshot. This method is thread-safe.
func (r *InMemorySnapshotRepository) Load(ctx context.Context) (*Snapshot, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, NewPersistenceError("load", ctx.Err())
	default:
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.snapshot == nil {
		return nil, ErrSnapshotNotFound
	}

	// Return a clone to prevent external modifications
	return r.snapshot.Clone(), nil
}

// Exists checks if a snapshot is stored. This method is thread-safe.
func (r *InMemorySnapshotRepository) Exists(ctx context.Context) (bool, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return false, NewPersistenceError("exists", ctx.Err())
	default:
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.snapshot != nil, nil
}

// Delete removes the stored snapshot. This method is thread-safe.
func (r *InMemorySnapshotRepository) Delete(ctx context.Context) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return NewPersistenceError("delete", ctx.Err())
	default:
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.snapshot = nil
	return nil
}
