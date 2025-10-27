package breaker

import (
	"encoding/json"
	"time"
)

const (
	// CurrentSnapshotVersion represents the current snapshot format version
	CurrentSnapshotVersion = 1
)

// Snapshot represents an immutable point-in-time capture of circuit breaker state.
// It follows the Memento pattern, allowing state to be saved and restored.
type Snapshot struct {
	// Version of the snapshot format for forward compatibility
	Version int `json:"version"`

	// State is the current circuit breaker state
	State CircuitBreakerState `json:"state"`

	// FailureCount is the number of consecutive failures
	FailureCount int `json:"failure_count"`

	// LastFailure is the timestamp of the last failure
	LastFailure time.Time `json:"last_failure"`

	// Configuration captures the circuit breaker settings
	Config SnapshotConfig `json:"config"`

	// Metadata for additional context (extensible)
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// CreatedAt is when this snapshot was created
	CreatedAt time.Time `json:"created_at"`
}

// SnapshotConfig captures the circuit breaker configuration
type SnapshotConfig struct {
	FailureThreshold int           `json:"failure_threshold"`
	ResetTimeout     time.Duration `json:"reset_timeout"`
}

// SnapshotBuilder provides a fluent interface for creating snapshots
type SnapshotBuilder struct {
	snapshot *Snapshot
}

// NewSnapshotBuilder creates a new snapshot builder with defaults
func NewSnapshotBuilder() *SnapshotBuilder {
	return &SnapshotBuilder{
		snapshot: &Snapshot{
			Version:   CurrentSnapshotVersion,
			State:     Closed,
			Metadata:  make(map[string]interface{}),
			CreatedAt: time.Now(),
		},
	}
}

// WithState sets the circuit breaker state
func (b *SnapshotBuilder) WithState(state CircuitBreakerState) *SnapshotBuilder {
	b.snapshot.State = state
	return b
}

// WithFailureCount sets the failure count
func (b *SnapshotBuilder) WithFailureCount(count int) *SnapshotBuilder {
	b.snapshot.FailureCount = count
	return b
}

// WithLastFailure sets the last failure timestamp
func (b *SnapshotBuilder) WithLastFailure(t time.Time) *SnapshotBuilder {
	b.snapshot.LastFailure = t
	return b
}

// WithConfig sets the configuration
func (b *SnapshotBuilder) WithConfig(failureThreshold int, resetTimeout time.Duration) *SnapshotBuilder {
	b.snapshot.Config = SnapshotConfig{
		FailureThreshold: failureThreshold,
		ResetTimeout:     resetTimeout,
	}
	return b
}

// WithMetadata adds metadata to the snapshot
func (b *SnapshotBuilder) WithMetadata(key string, value interface{}) *SnapshotBuilder {
	if b.snapshot.Metadata == nil {
		b.snapshot.Metadata = make(map[string]interface{})
	}
	b.snapshot.Metadata[key] = value
	return b
}

// WithVersion sets a specific version (for testing/migration)
func (b *SnapshotBuilder) WithVersion(version int) *SnapshotBuilder {
	b.snapshot.Version = version
	return b
}

// Build creates the final immutable snapshot
func (b *SnapshotBuilder) Build() *Snapshot {
	// Return a copy to ensure immutability
	snapshot := *b.snapshot

	// Deep copy metadata
	if b.snapshot.Metadata != nil {
		snapshot.Metadata = make(map[string]interface{}, len(b.snapshot.Metadata))
		for k, v := range b.snapshot.Metadata {
			snapshot.Metadata[k] = v
		}
	}

	return &snapshot
}

// FromCircuitBreaker creates a snapshot from the current circuit breaker state.
// This should be called with the circuit breaker mutex held to ensure consistency.
func FromCircuitBreaker(cb *CircuitBreakerImpl) *Snapshot {
	return NewSnapshotBuilder().
		WithState(cb.currentState.Name()).
		WithFailureCount(cb.failureCount).
		WithLastFailure(cb.lastFailure).
		WithConfig(cb.failureThreshold, cb.resetTimeout).
		Build()
}

// ToJSON serializes the snapshot to JSON
func (s *Snapshot) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// FromJSON deserializes a snapshot from JSON
func FromJSON(data []byte) (*Snapshot, error) {
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, NewPersistenceError("unmarshal", err)
	}
	return &snapshot, nil
}

// Clone creates a deep copy of the snapshot
func (s *Snapshot) Clone() *Snapshot {
	clone := *s

	// Deep copy metadata
	if s.Metadata != nil {
		clone.Metadata = make(map[string]interface{}, len(s.Metadata))
		for k, v := range s.Metadata {
			clone.Metadata[k] = v
		}
	}

	return &clone
}

// IsExpired checks if the snapshot is too old based on reset timeout
func (s *Snapshot) IsExpired() bool {
	if s.State != Open {
		return false
	}

	// Check if enough time has passed since last failure
	return time.Since(s.LastFailure) >= s.Config.ResetTimeout
}

// ShouldTransitionToHalfOpen determines if the breaker should transition to half-open
func (s *Snapshot) ShouldTransitionToHalfOpen() bool {
	return s.State == Open && s.IsExpired()
}
