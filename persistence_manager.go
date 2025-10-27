package breaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/farzai/breaker-go/logging"
)

// PersistenceConfig configures persistence behavior
type PersistenceConfig struct {
	// Enabled controls whether persistence is active
	Enabled bool

	// Async enables asynchronous persistence (fire and forget)
	Async bool

	// RetryAttempts is the number of times to retry failed saves (0 = no retry)
	RetryAttempts int

	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration

	// Validator validates snapshots before saving/after loading
	Validator SnapshotValidator

	// OnSaveError is called when save fails (optional)
	OnSaveError func(error)

	// OnLoadError is called when load fails (optional)
	OnLoadError func(error)

	// OnRestoreError is called when state restoration fails (optional)
	// This includes cases like config mismatch or validation failures
	OnRestoreError func(error)

	// RestoreOnInit enables state restoration during initialization
	RestoreOnInit bool

	// Timeout for persistence operations
	Timeout time.Duration

	// Logger for structured logging of persistence operations (optional)
	// If nil, logging will be disabled
	Logger logging.Logger
}

// DefaultPersistenceConfig returns sensible defaults
func DefaultPersistenceConfig() PersistenceConfig {
	return PersistenceConfig{
		Enabled:       true,
		Async:         true,
		RetryAttempts: 3,
		RetryDelay:    100 * time.Millisecond,
		Validator:     DefaultValidator(),
		RestoreOnInit: true,
		Timeout:       5 * time.Second,
	}
}

// PersistenceManager coordinates all persistence operations with validation,
// retry logic, and async support. It decouples the circuit breaker from
// direct storage access.
type PersistenceManager struct {
	repository SnapshotRepository
	config     PersistenceConfig
	mutex      sync.Mutex
	wg         sync.WaitGroup // Track async operations
}

// NewPersistenceManager creates a new persistence manager
func NewPersistenceManager(repo SnapshotRepository, config PersistenceConfig) *PersistenceManager {
	return &PersistenceManager{
		repository: repo,
		config:     config,
	}
}

// Save persists a snapshot with validation and retry logic
func (pm *PersistenceManager) Save(snapshot *Snapshot) error {
	if !pm.config.Enabled {
		return nil
	}

	// Validate before saving
	if pm.config.Validator != nil {
		if err := pm.config.Validator.Validate(snapshot); err != nil {
			if pm.config.Logger != nil {
				pm.config.Logger.Error("Snapshot validation failed",
					logging.Error(err),
					logging.String("state", snapshot.State.String()),
				)
			}
			return err
		}
	}

	if pm.config.Logger != nil && pm.config.Logger.Enabled(logging.LevelDebug) {
		pm.config.Logger.Debug("Saving circuit breaker snapshot",
			logging.String("state", snapshot.State.String()),
			logging.Int("failure_count", snapshot.FailureCount),
			logging.Bool("async", pm.config.Async),
		)
	}

	if pm.config.Async {
		pm.saveAsync(snapshot)
		return nil
	}

	return pm.saveSync(snapshot)
}

// saveSync performs synchronous save with retry
func (pm *PersistenceManager) saveSync(snapshot *Snapshot) error {
	ctx, cancel := context.WithTimeout(context.Background(), pm.config.Timeout)
	defer cancel()

	var lastErr error
	attempts := pm.config.RetryAttempts + 1 // +1 for initial attempt

	for i := 0; i < attempts; i++ {
		if i > 0 {
			// Exponential backoff
			delay := pm.config.RetryDelay * time.Duration(1<<uint(i-1))

			if pm.config.Logger != nil && pm.config.Logger.Enabled(logging.LevelDebug) {
				pm.config.Logger.Debug("Retrying persistence save",
					logging.Int("attempt", i+1),
					logging.Int("max_attempts", attempts),
					logging.Duration("delay", delay),
					logging.Error(lastErr),
				)
			}

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return NewPersistenceError("save", ctx.Err())
			}
		}

		start := time.Now()
		err := pm.repository.Save(ctx, snapshot)
		duration := time.Since(start)

		if err == nil {
			if pm.config.Logger != nil && pm.config.Logger.Enabled(logging.LevelDebug) {
				pm.config.Logger.Debug("Snapshot saved successfully",
					logging.String("state", snapshot.State.String()),
					logging.Duration("duration", duration),
					logging.Int("attempt", i+1),
				)
			}
			return nil
		}

		lastErr = err
	}

	if pm.config.OnSaveError != nil {
		pm.config.OnSaveError(lastErr)
	}

	if pm.config.Logger != nil {
		pm.config.Logger.Error("Failed to save snapshot after retries",
			logging.Error(lastErr),
			logging.Int("attempts", attempts),
			logging.String("state", snapshot.State.String()),
		)
	}

	return NewPersistenceError("save", lastErr)
}

// saveAsync performs asynchronous save in background.
// Errors are handled via OnSaveError callback if set, otherwise logged if a logger is configured.
// To avoid losing errors in production, always set OnSaveError callback or configure a logger.
func (pm *PersistenceManager) saveAsync(snapshot *Snapshot) {
	pm.wg.Add(1)
	go func() {
		defer pm.wg.Done()

		if err := pm.saveSync(snapshot); err != nil {
			// Log error but don't propagate (async mode)
			if pm.config.OnSaveError != nil {
				pm.config.OnSaveError(err)
			} else if pm.config.Logger != nil {
				// Log using structured logger
				pm.config.Logger.Error("Async persistence save failed",
					logging.Error(err),
					logging.String("state", snapshot.State.String()),
					logging.Int("failure_count", snapshot.FailureCount),
				)
			}
			// If neither error handler nor logger is configured, error is silently dropped
		}
	}()
}

// Load retrieves and validates a snapshot
func (pm *PersistenceManager) Load() (*Snapshot, error) {
	if !pm.config.Enabled {
		return nil, ErrSnapshotNotFound
	}

	if pm.config.Logger != nil && pm.config.Logger.Enabled(logging.LevelDebug) {
		pm.config.Logger.Debug("Loading circuit breaker snapshot")
	}

	ctx, cancel := context.WithTimeout(context.Background(), pm.config.Timeout)
	defer cancel()

	start := time.Now()
	snapshot, err := pm.repository.Load(ctx)
	duration := time.Since(start)

	if err != nil {
		if pm.config.OnLoadError != nil {
			pm.config.OnLoadError(err)
		}

		if pm.config.Logger != nil {
			pm.config.Logger.Error("Failed to load snapshot",
				logging.Error(err),
				logging.Duration("duration", duration),
			)
		}

		return nil, NewPersistenceError("load", err)
	}

	// Validate after loading
	if pm.config.Validator != nil {
		if err := pm.config.Validator.Validate(snapshot); err != nil {
			if pm.config.Logger != nil {
				pm.config.Logger.Error("Loaded snapshot validation failed",
					logging.Error(err),
					logging.String("state", snapshot.State.String()),
				)
			}
			return nil, err
		}
	}

	if pm.config.Logger != nil {
		pm.config.Logger.Info("Snapshot loaded successfully",
			logging.String("state", snapshot.State.String()),
			logging.Int("failure_count", snapshot.FailureCount),
			logging.Duration("duration", duration),
			logging.Time("snapshot_created_at", snapshot.CreatedAt),
		)
	}

	return snapshot, nil
}

// Exists checks if a snapshot is available
func (pm *PersistenceManager) Exists() (bool, error) {
	if !pm.config.Enabled {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), pm.config.Timeout)
	defer cancel()

	return pm.repository.Exists(ctx)
}

// Delete removes the stored snapshot
func (pm *PersistenceManager) Delete() error {
	if !pm.config.Enabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), pm.config.Timeout)
	defer cancel()

	return pm.repository.Delete(ctx)
}

// Wait blocks until all async operations complete
func (pm *PersistenceManager) Wait() {
	pm.wg.Wait()
}

// RestoreStrategy defines how to apply a restored snapshot
type RestoreStrategy interface {
	// Restore applies a snapshot to the circuit breaker
	// Returns an error if restoration failed, nil if successful
	Restore(cb *CircuitBreakerImpl, snapshot *Snapshot) error
}

// DefaultRestoreStrategy restores all state from snapshot
type DefaultRestoreStrategy struct{}

// Restore applies the snapshot completely
func (s *DefaultRestoreStrategy) Restore(cb *CircuitBreakerImpl, snapshot *Snapshot) error {
	// Only restore if configuration matches
	if snapshot.Config.FailureThreshold != cb.failureThreshold ||
		snapshot.Config.ResetTimeout != cb.resetTimeout {
		return fmt.Errorf("configuration mismatch: snapshot has threshold=%d timeout=%v, breaker has threshold=%d timeout=%v",
			snapshot.Config.FailureThreshold, snapshot.Config.ResetTimeout,
			cb.failureThreshold, cb.resetTimeout)
	}

	// Apply snapshot state
	cb.failureCount = snapshot.FailureCount
	cb.lastFailure = snapshot.LastFailure

	// Set the appropriate state
	var newState State
	switch snapshot.State {
	case Closed:
		newState = &ClosedState{}
	case Open:
		newState = &OpenState{}
	case HalfOpen:
		newState = &HalfOpenState{}
	default:
		return fmt.Errorf("invalid snapshot state: %v", snapshot.State)
	}

	cb.currentState = newState
	return nil
}

// ConservativeRestoreStrategy only restores if snapshot is recent and safe
type ConservativeRestoreStrategy struct {
	maxAge time.Duration
}

// NewConservativeRestoreStrategy creates a conservative restoration strategy
func NewConservativeRestoreStrategy(maxAge time.Duration) *ConservativeRestoreStrategy {
	return &ConservativeRestoreStrategy{
		maxAge: maxAge,
	}
}

// Restore applies snapshot only if it's recent and safe
func (s *ConservativeRestoreStrategy) Restore(cb *CircuitBreakerImpl, snapshot *Snapshot) error {
	// Check age
	if time.Since(snapshot.CreatedAt) > s.maxAge {
		return fmt.Errorf("snapshot too old: age=%v max=%v", time.Since(snapshot.CreatedAt), s.maxAge)
	}

	// If snapshot shows Open state but timeout expired, start as HalfOpen instead
	if snapshot.ShouldTransitionToHalfOpen() {
		cb.currentState = &HalfOpenState{}
		cb.failureCount = 0
		cb.lastFailure = snapshot.LastFailure
		return nil
	}

	// Otherwise use default strategy
	strategy := &DefaultRestoreStrategy{}
	return strategy.Restore(cb, snapshot)
}
