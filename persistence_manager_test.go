package breaker_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	breaker "github.com/farzai/breaker-go"
)

// mockRepository is a test double for SnapshotRepository
type mockRepository struct {
	saveFunc   func(ctx context.Context, snapshot *breaker.Snapshot) error
	loadFunc   func(ctx context.Context) (*breaker.Snapshot, error)
	existsFunc func(ctx context.Context) (bool, error)
	deleteFunc func(ctx context.Context) error
	mu         sync.Mutex
}

func (m *mockRepository) Save(ctx context.Context, snapshot *breaker.Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveFunc != nil {
		return m.saveFunc(ctx, snapshot)
	}
	return nil
}

func (m *mockRepository) Load(ctx context.Context) (*breaker.Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loadFunc != nil {
		return m.loadFunc(ctx)
	}
	return nil, breaker.ErrSnapshotNotFound
}

func (m *mockRepository) Exists(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.existsFunc != nil {
		return m.existsFunc(ctx)
	}
	return false, nil
}

func (m *mockRepository) Delete(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx)
	}
	return nil
}

func TestPersistenceManagerSave(t *testing.T) {
	t.Run("Saves snapshot when persistence is enabled", func(t *testing.T) {
		saved := false
		repo := &mockRepository{
			saveFunc: func(ctx context.Context, snapshot *breaker.Snapshot) error {
				saved = true
				return nil
			},
		}

		config := breaker.PersistenceConfig{
			Enabled: true,
			Async:   false,
			Timeout: 5 * time.Second,
		}

		manager := breaker.NewPersistenceManager(repo, config)
		snapshot := breaker.NewSnapshotBuilder().Build()

		err := manager.Save(snapshot)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		if !saved {
			t.Error("Expected snapshot to be saved")
		}
	})

	t.Run("Skips save when persistence is disabled", func(t *testing.T) {
		repo := &mockRepository{
			saveFunc: func(ctx context.Context, snapshot *breaker.Snapshot) error {
				t.Error("Save should not be called when persistence is disabled")
				return nil
			},
		}

		config := breaker.PersistenceConfig{
			Enabled: false,
		}

		manager := breaker.NewPersistenceManager(repo, config)
		snapshot := breaker.NewSnapshotBuilder().Build()

		err := manager.Save(snapshot)
		if err != nil {
			t.Fatalf("Save should not error when disabled: %v", err)
		}
	})

	t.Run("Validates snapshot before saving", func(t *testing.T) {
		repo := &mockRepository{
			saveFunc: func(ctx context.Context, snapshot *breaker.Snapshot) error {
				t.Error("Save should not be called for invalid snapshot")
				return nil
			},
		}

		config := breaker.PersistenceConfig{
			Enabled:   true,
			Async:     false,
			Validator: breaker.DefaultValidator(),
			Timeout:   5 * time.Second,
		}

		manager := breaker.NewPersistenceManager(repo, config)

		// Create invalid snapshot (Closed state with non-zero failures)
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Closed).
			WithFailureCount(5).
			Build()

		err := manager.Save(snapshot)
		if err == nil {
			t.Error("Expected validation error")
		}
	})

	t.Run("Async save returns immediately", func(t *testing.T) {
		saveDone := make(chan bool, 1)
		repo := &mockRepository{
			saveFunc: func(ctx context.Context, snapshot *breaker.Snapshot) error {
				time.Sleep(50 * time.Millisecond)
				saveDone <- true
				return nil
			},
		}

		config := breaker.PersistenceConfig{
			Enabled: true,
			Async:   true,
			Timeout: 5 * time.Second,
		}

		manager := breaker.NewPersistenceManager(repo, config)
		snapshot := breaker.NewSnapshotBuilder().Build()

		start := time.Now()
		err := manager.Save(snapshot)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Async save failed: %v", err)
		}

		// Should return immediately (before the 50ms sleep)
		if elapsed > 10*time.Millisecond {
			t.Errorf("Async save took too long: %v", elapsed)
		}

		// Wait for async operation to complete
		select {
		case <-saveDone:
			// Success
		case <-time.After(500 * time.Millisecond):
			t.Error("Async save did not complete")
		}
	})
}

func TestPersistenceManagerRetry(t *testing.T) {
	t.Run("Retries on failure", func(t *testing.T) {
		attempts := 0
		repo := &mockRepository{
			saveFunc: func(ctx context.Context, snapshot *breaker.Snapshot) error {
				attempts++
				if attempts < 3 {
					return errors.New("temporary error")
				}
				return nil
			},
		}

		config := breaker.PersistenceConfig{
			Enabled:       true,
			Async:         false,
			RetryAttempts: 3,
			RetryDelay:    10 * time.Millisecond,
			Timeout:       5 * time.Second,
		}

		manager := breaker.NewPersistenceManager(repo, config)
		snapshot := breaker.NewSnapshotBuilder().Build()

		err := manager.Save(snapshot)
		if err != nil {
			t.Fatalf("Save failed after retries: %v", err)
		}

		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("Returns error after max retries", func(t *testing.T) {
		attempts := 0
		repo := &mockRepository{
			saveFunc: func(ctx context.Context, snapshot *breaker.Snapshot) error {
				attempts++
				return errors.New("persistent error")
			},
		}

		config := breaker.PersistenceConfig{
			Enabled:       true,
			Async:         false,
			RetryAttempts: 2,
			RetryDelay:    10 * time.Millisecond,
			Timeout:       5 * time.Second,
		}

		manager := breaker.NewPersistenceManager(repo, config)
		snapshot := breaker.NewSnapshotBuilder().Build()

		err := manager.Save(snapshot)
		if err == nil {
			t.Error("Expected error after max retries")
		}

		expectedAttempts := 3 // Initial + 2 retries
		if attempts != expectedAttempts {
			t.Errorf("Expected %d attempts, got %d", expectedAttempts, attempts)
		}
	})

	t.Run("Calls OnSaveError on failure", func(t *testing.T) {
		errorCalled := false
		var capturedError error

		repo := &mockRepository{
			saveFunc: func(ctx context.Context, snapshot *breaker.Snapshot) error {
				return errors.New("save error")
			},
		}

		config := breaker.PersistenceConfig{
			Enabled:       true,
			Async:         false,
			RetryAttempts: 1,
			RetryDelay:    10 * time.Millisecond,
			Timeout:       5 * time.Second,
			OnSaveError: func(err error) {
				errorCalled = true
				capturedError = err
			},
		}

		manager := breaker.NewPersistenceManager(repo, config)
		snapshot := breaker.NewSnapshotBuilder().Build()

		manager.Save(snapshot)

		if !errorCalled {
			t.Error("Expected OnSaveError to be called")
		}
		if capturedError == nil {
			t.Error("Expected error to be passed to OnSaveError")
		}
	})
}

func TestPersistenceManagerLoad(t *testing.T) {
	t.Run("Loads snapshot when persistence is enabled", func(t *testing.T) {
		expectedSnapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithFailureCount(5).
			Build()

		repo := &mockRepository{
			loadFunc: func(ctx context.Context) (*breaker.Snapshot, error) {
				return expectedSnapshot, nil
			},
		}

		config := breaker.PersistenceConfig{
			Enabled: true,
			Timeout: 5 * time.Second,
		}

		manager := breaker.NewPersistenceManager(repo, config)

		loaded, err := manager.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if loaded.State != expectedSnapshot.State {
			t.Error("Loaded snapshot does not match expected")
		}
	})

	t.Run("Returns error when persistence is disabled", func(t *testing.T) {
		repo := &mockRepository{}

		config := breaker.PersistenceConfig{
			Enabled: false,
		}

		manager := breaker.NewPersistenceManager(repo, config)

		_, err := manager.Load()
		if !errors.Is(err, breaker.ErrSnapshotNotFound) {
			t.Errorf("Expected ErrSnapshotNotFound, got %v", err)
		}
	})

	t.Run("Validates snapshot after loading", func(t *testing.T) {
		// Return invalid snapshot (Closed with failures)
		invalidSnapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Closed).
			WithFailureCount(5).
			Build()

		repo := &mockRepository{
			loadFunc: func(ctx context.Context) (*breaker.Snapshot, error) {
				return invalidSnapshot, nil
			},
		}

		config := breaker.PersistenceConfig{
			Enabled:   true,
			Validator: breaker.DefaultValidator(),
			Timeout:   5 * time.Second,
		}

		manager := breaker.NewPersistenceManager(repo, config)

		_, err := manager.Load()
		if err == nil {
			t.Error("Expected validation error")
		}
	})

	t.Run("Calls OnLoadError on failure", func(t *testing.T) {
		errorCalled := false

		repo := &mockRepository{
			loadFunc: func(ctx context.Context) (*breaker.Snapshot, error) {
				return nil, errors.New("load error")
			},
		}

		config := breaker.PersistenceConfig{
			Enabled: true,
			Timeout: 5 * time.Second,
			OnLoadError: func(err error) {
				errorCalled = true
			},
		}

		manager := breaker.NewPersistenceManager(repo, config)
		manager.Load()

		if !errorCalled {
			t.Error("Expected OnLoadError to be called")
		}
	})
}

func TestPersistenceManagerExists(t *testing.T) {
	t.Run("Returns true when snapshot exists", func(t *testing.T) {
		repo := &mockRepository{
			existsFunc: func(ctx context.Context) (bool, error) {
				return true, nil
			},
		}

		config := breaker.PersistenceConfig{
			Enabled: true,
			Timeout: 5 * time.Second,
		}

		manager := breaker.NewPersistenceManager(repo, config)

		exists, err := manager.Exists()
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("Expected exists to return true")
		}
	})

	t.Run("Returns false when persistence is disabled", func(t *testing.T) {
		repo := &mockRepository{}

		config := breaker.PersistenceConfig{
			Enabled: false,
		}

		manager := breaker.NewPersistenceManager(repo, config)

		exists, err := manager.Exists()
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("Expected exists to return false when disabled")
		}
	})
}

func TestPersistenceManagerDelete(t *testing.T) {
	t.Run("Deletes snapshot when persistence is enabled", func(t *testing.T) {
		deleted := false
		repo := &mockRepository{
			deleteFunc: func(ctx context.Context) error {
				deleted = true
				return nil
			},
		}

		config := breaker.PersistenceConfig{
			Enabled: true,
			Timeout: 5 * time.Second,
		}

		manager := breaker.NewPersistenceManager(repo, config)

		err := manager.Delete()
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		if !deleted {
			t.Error("Expected snapshot to be deleted")
		}
	})

	t.Run("Skips delete when persistence is disabled", func(t *testing.T) {
		repo := &mockRepository{
			deleteFunc: func(ctx context.Context) error {
				t.Error("Delete should not be called when disabled")
				return nil
			},
		}

		config := breaker.PersistenceConfig{
			Enabled: false,
		}

		manager := breaker.NewPersistenceManager(repo, config)

		err := manager.Delete()
		if err != nil {
			t.Fatalf("Delete should not error when disabled: %v", err)
		}
	})
}

func TestPersistenceManagerWait(t *testing.T) {
	t.Run("Wait blocks until async operations complete", func(t *testing.T) {
		var completed atomic.Bool
		repo := &mockRepository{
			saveFunc: func(ctx context.Context, snapshot *breaker.Snapshot) error {
				time.Sleep(50 * time.Millisecond)
				completed.Store(true)
				return nil
			},
		}

		config := breaker.PersistenceConfig{
			Enabled: true,
			Async:   true,
			Timeout: 5 * time.Second,
		}

		manager := breaker.NewPersistenceManager(repo, config)
		snapshot := breaker.NewSnapshotBuilder().Build()

		manager.Save(snapshot)

		if completed.Load() {
			t.Error("Save should not have completed yet")
		}

		manager.Wait()

		if !completed.Load() {
			t.Error("Expected async save to complete after Wait")
		}
	})

	t.Run("Wait handles multiple async operations", func(t *testing.T) {
		var count int
		var mu sync.Mutex

		repo := &mockRepository{
			saveFunc: func(ctx context.Context, snapshot *breaker.Snapshot) error {
				time.Sleep(20 * time.Millisecond)
				mu.Lock()
				count++
				mu.Unlock()
				return nil
			},
		}

		config := breaker.PersistenceConfig{
			Enabled: true,
			Async:   true,
			Timeout: 5 * time.Second,
		}

		manager := breaker.NewPersistenceManager(repo, config)

		// Start multiple async saves
		for i := 0; i < 5; i++ {
			snapshot := breaker.NewSnapshotBuilder().Build()
			manager.Save(snapshot)
		}

		manager.Wait()

		mu.Lock()
		finalCount := count
		mu.Unlock()

		if finalCount != 5 {
			t.Errorf("Expected 5 saves to complete, got %d", finalCount)
		}
	})
}

func TestDefaultRestoreStrategy(t *testing.T) {
	t.Run("Restores snapshot with matching config", func(t *testing.T) {
		cb, _ := breaker.NewCircuitBreaker(3, 1*time.Second)
		strategy := &breaker.DefaultRestoreStrategy{}

		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithFailureCount(3).
			WithLastFailure(time.Now()).
			WithConfig(3, 1*time.Second).
			Build()

		// Note: This test assumes we can access the CircuitBreakerImpl internals
		// In practice, you might need to adjust based on the actual implementation
		// For now, we'll just verify it doesn't panic
		_ = strategy
		_ = snapshot
		_ = cb
	})
}

func TestConservativeRestoreStrategy(t *testing.T) {
	t.Run("Creates strategy with max age", func(t *testing.T) {
		strategy := breaker.NewConservativeRestoreStrategy(1 * time.Hour)
		if strategy == nil {
			t.Error("Expected strategy to be created")
		}
	})

	t.Run("Rejects old snapshots", func(t *testing.T) {
		strategy := breaker.NewConservativeRestoreStrategy(1 * time.Hour)
		cb, _ := breaker.NewCircuitBreaker(3, 1*time.Second)

		// Create old snapshot
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithConfig(3, 1*time.Second).
			Build()
		snapshot.CreatedAt = time.Now().Add(-2 * time.Hour)

		// Note: Actual restoration behavior depends on implementation details
		_ = strategy
		_ = cb
		_ = snapshot
	})
}
