package breaker_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	breaker "github.com/farzai/breaker-go"
)

func TestInMemorySnapshotRepository(t *testing.T) {
	t.Run("Save and Load snapshot", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx := context.Background()

		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithFailureCount(5).
			WithConfig(3, 1*time.Second).
			Build()

		err := repo.Save(ctx, snapshot)
		if err != nil {
			t.Fatalf("Failed to save snapshot: %v", err)
		}

		loaded, err := repo.Load(ctx)
		if err != nil {
			t.Fatalf("Failed to load snapshot: %v", err)
		}

		if loaded.State != breaker.Open {
			t.Errorf("Expected state Open, got %v", loaded.State)
		}
		if loaded.FailureCount != 5 {
			t.Errorf("Expected failure count 5, got %d", loaded.FailureCount)
		}
	})

	t.Run("Load returns error when no snapshot exists", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx := context.Background()

		_, err := repo.Load(ctx)
		if !errors.Is(err, breaker.ErrSnapshotNotFound) {
			t.Errorf("Expected ErrSnapshotNotFound, got %v", err)
		}
	})

	t.Run("Save with nil snapshot returns error", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx := context.Background()

		err := repo.Save(ctx, nil)
		if err == nil {
			t.Error("Expected error when saving nil snapshot")
		}

		var valErr *breaker.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})

	t.Run("Exists returns true when snapshot exists", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx := context.Background()

		snapshot := breaker.NewSnapshotBuilder().Build()
		repo.Save(ctx, snapshot)

		exists, err := repo.Exists(ctx)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("Expected exists to return true")
		}
	})

	t.Run("Exists returns false when no snapshot", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx := context.Background()

		exists, err := repo.Exists(ctx)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("Expected exists to return false")
		}
	})

	t.Run("Delete removes snapshot", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx := context.Background()

		snapshot := breaker.NewSnapshotBuilder().Build()
		repo.Save(ctx, snapshot)

		err := repo.Delete(ctx)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		exists, _ := repo.Exists(ctx)
		if exists {
			t.Error("Expected snapshot to be deleted")
		}
	})

	t.Run("Delete on empty repository does not error", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx := context.Background()

		err := repo.Delete(ctx)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Save with canceled context returns error", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		snapshot := breaker.NewSnapshotBuilder().Build()
		err := repo.Save(ctx, snapshot)

		if err == nil {
			t.Error("Expected error with canceled context")
		}

		var persistErr *breaker.PersistenceError
		if !errors.As(err, &persistErr) {
			t.Errorf("Expected PersistenceError, got %T", err)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled in error chain, got %v", err)
		}
	})

	t.Run("Load with canceled context returns error", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		snapshot := breaker.NewSnapshotBuilder().Build()
		repo.Save(context.Background(), snapshot)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := repo.Load(ctx)
		if err == nil {
			t.Error("Expected error with canceled context")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled in error chain, got %v", err)
		}
	})

	t.Run("Exists with canceled context returns error", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := repo.Exists(ctx)
		if err == nil {
			t.Error("Expected error with canceled context")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled in error chain, got %v", err)
		}
	})

	t.Run("Delete with canceled context returns error", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := repo.Delete(ctx)
		if err == nil {
			t.Error("Expected error with canceled context")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled in error chain, got %v", err)
		}
	})

	t.Run("Snapshots are cloned on save to prevent external modification", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx := context.Background()

		snapshot := breaker.NewSnapshotBuilder().
			WithFailureCount(5).
			WithMetadata("key", "original").
			Build()

		repo.Save(ctx, snapshot)

		// Modify original snapshot's metadata (attempt external modification)
		if snapshot.Metadata != nil {
			snapshot.Metadata["key"] = "modified"
		}

		// Load and verify it wasn't affected
		loaded, _ := repo.Load(ctx)
		if loaded.Metadata["key"] != "original" {
			t.Errorf("Expected metadata to be 'original', got %v (snapshot was not cloned properly)", loaded.Metadata["key"])
		}
	})

	t.Run("Snapshots are cloned on load to prevent external modification", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx := context.Background()

		snapshot := breaker.NewSnapshotBuilder().
			WithMetadata("key", "original").
			Build()

		repo.Save(ctx, snapshot)

		// Load and modify
		loaded1, _ := repo.Load(ctx)
		if loaded1.Metadata != nil {
			loaded1.Metadata["key"] = "modified"
		}

		// Load again and verify it wasn't affected
		loaded2, _ := repo.Load(ctx)
		if loaded2.Metadata["key"] != "original" {
			t.Errorf("Expected metadata to be 'original', got %v (snapshot was not cloned properly)", loaded2.Metadata["key"])
		}
	})

	t.Run("Concurrent Save operations are thread-safe", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx := context.Background()

		const numGoroutines = 50
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(count int) {
				defer wg.Done()
				snapshot := breaker.NewSnapshotBuilder().
					WithFailureCount(count).
					Build()
				repo.Save(ctx, snapshot)
			}(i)
		}

		wg.Wait()

		// Verify we can load without panic
		loaded, err := repo.Load(ctx)
		if err != nil {
			t.Errorf("Failed to load after concurrent saves: %v", err)
		}
		if loaded == nil {
			t.Error("Expected snapshot to be loaded")
		}
	})

	t.Run("Concurrent Load operations are thread-safe", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx := context.Background()

		snapshot := breaker.NewSnapshotBuilder().
			WithFailureCount(10).
			Build()
		repo.Save(ctx, snapshot)

		const numGoroutines = 100
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				_, err := repo.Load(ctx)
				if err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("Concurrent load failed: %v", err)
		}
	})

	t.Run("Concurrent mixed operations are thread-safe", func(t *testing.T) {
		repo := breaker.NewInMemorySnapshotRepository()
		ctx := context.Background()

		const numGoroutines = 100
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()

				switch id % 4 {
				case 0: // Save
					snapshot := breaker.NewSnapshotBuilder().
						WithFailureCount(id).
						Build()
					repo.Save(ctx, snapshot)
				case 1: // Load
					repo.Load(ctx)
				case 2: // Exists
					repo.Exists(ctx)
				case 3: // Delete
					repo.Delete(ctx)
				}
			}(i)
		}

		wg.Wait()
		// Test passes if no race conditions or panics occur
	})
}
