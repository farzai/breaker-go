package breaker_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	breaker "github.com/farzai/breaker-go"
)

// Helper function to create a temporary file path for testing
func tempFilePath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "test_snapshot.json")
}

func TestNewFileSnapshotRepository(t *testing.T) {
	t.Run("creates repository with valid path", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, err := breaker.NewFileSnapshotRepository(filePath)

		if err != nil {
			t.Fatalf("Failed to create repository: %v", err)
		}
		if repo == nil {
			t.Fatal("Expected repository to be created")
		}
		if repo.FilePath() != filePath {
			t.Errorf("Expected file path %s, got %s", filePath, repo.FilePath())
		}
	})

	t.Run("returns error when path is empty", func(t *testing.T) {
		_, err := breaker.NewFileSnapshotRepository("")

		if err == nil {
			t.Error("Expected error when path is empty")
		}

		var valErr *breaker.ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})

	t.Run("creates parent directory if it doesn't exist", func(t *testing.T) {
		baseDir := t.TempDir()
		filePath := filepath.Join(baseDir, "nested", "dir", "snapshot.json")

		repo, err := breaker.NewFileSnapshotRepository(filePath)
		if err != nil {
			t.Fatalf("Failed to create repository: %v", err)
		}

		// Verify directory was created
		dir := filepath.Dir(repo.FilePath())
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Error("Expected parent directory to be created")
		}
	})
}

func TestNewFileSnapshotRepositoryWithOptions(t *testing.T) {
	t.Run("creates repository with full file path", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, err := breaker.NewFileSnapshotRepositoryWithOptions(breaker.FileSnapshotRepositoryOptions{
			FilePath: filePath,
		})

		if err != nil {
			t.Fatalf("Failed to create repository: %v", err)
		}
		if repo.FilePath() != filePath {
			t.Errorf("Expected file path %s, got %s", filePath, repo.FilePath())
		}
	})

	t.Run("creates repository with dir and filename", func(t *testing.T) {
		dir := t.TempDir()
		fileName := "custom_snapshot.json"

		repo, err := breaker.NewFileSnapshotRepositoryWithOptions(breaker.FileSnapshotRepositoryOptions{
			DirPath:  dir,
			FileName: fileName,
		})

		if err != nil {
			t.Fatalf("Failed to create repository: %v", err)
		}

		expectedPath := filepath.Join(dir, fileName)
		if repo.FilePath() != expectedPath {
			t.Errorf("Expected file path %s, got %s", expectedPath, repo.FilePath())
		}
	})

	t.Run("uses defaults when options are empty", func(t *testing.T) {
		repo, err := breaker.NewFileSnapshotRepositoryWithOptions(breaker.FileSnapshotRepositoryOptions{})

		if err != nil {
			t.Fatalf("Failed to create repository: %v", err)
		}

		expectedPath := filepath.Join(os.TempDir(), "circuit_breaker_snapshot.json")
		if repo.FilePath() != expectedPath {
			t.Errorf("Expected file path %s, got %s", expectedPath, repo.FilePath())
		}

		// Clean up
		repo.Delete(context.Background())
	})
}

func TestNewTempFileSnapshotRepository(t *testing.T) {
	repo, err := breaker.NewTempFileSnapshotRepository()

	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	expectedPath := filepath.Join(os.TempDir(), "circuit_breaker_snapshot.json")
	if repo.FilePath() != expectedPath {
		t.Errorf("Expected file path %s, got %s", expectedPath, repo.FilePath())
	}

	// Clean up
	repo.Delete(context.Background())
}

func TestFileSnapshotRepository(t *testing.T) {
	t.Run("Save and Load snapshot", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
		ctx := context.Background()

		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithFailureCount(5).
			WithConfig(3, 1*time.Second).
			WithMetadata("test", "value").
			Build()

		err := repo.Save(ctx, snapshot)
		if err != nil {
			t.Fatalf("Failed to save snapshot: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("Expected file to exist after save")
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
		if loaded.Metadata["test"] != "value" {
			t.Errorf("Expected metadata test=value, got %v", loaded.Metadata["test"])
		}
	})

	t.Run("Load returns error when no file exists", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
		ctx := context.Background()

		_, err := repo.Load(ctx)
		if !errors.Is(err, breaker.ErrSnapshotNotFound) {
			t.Errorf("Expected ErrSnapshotNotFound, got %v", err)
		}
	})

	t.Run("Save with nil snapshot returns error", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
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

	t.Run("Exists returns true when file exists", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
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

	t.Run("Exists returns false when no file", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
		ctx := context.Background()

		exists, err := repo.Exists(ctx)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("Expected exists to return false")
		}
	})

	t.Run("Delete removes file", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
		ctx := context.Background()

		snapshot := breaker.NewSnapshotBuilder().Build()
		repo.Save(ctx, snapshot)

		err := repo.Delete(ctx)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify file is gone
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Error("Expected file to be deleted")
		}

		exists, _ := repo.Exists(ctx)
		if exists {
			t.Error("Expected exists to return false after delete")
		}
	})

	t.Run("Delete on non-existent file does not error", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
		ctx := context.Background()

		err := repo.Delete(ctx)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Save with canceled context returns error", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
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
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)

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
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
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
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
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

	t.Run("Concurrent Save operations are thread-safe", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
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

		// Verify file is valid JSON
		if loaded.Version != breaker.CurrentSnapshotVersion {
			t.Error("Loaded snapshot has invalid version")
		}
	})

	t.Run("Concurrent Load operations are thread-safe", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
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
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
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

	t.Run("Atomic write prevents corruption", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
		ctx := context.Background()

		// Save initial snapshot
		snapshot1 := breaker.NewSnapshotBuilder().
			WithFailureCount(5).
			Build()
		repo.Save(ctx, snapshot1)

		// Verify temp file is cleaned up
		tempFile := filePath + ".tmp"
		if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
			t.Error("Expected temp file to be cleaned up after save")
		}

		// Overwrite with new snapshot
		snapshot2 := breaker.NewSnapshotBuilder().
			WithFailureCount(10).
			Build()
		repo.Save(ctx, snapshot2)

		// Verify we can still load
		loaded, err := repo.Load(ctx)
		if err != nil {
			t.Fatalf("Failed to load after overwrite: %v", err)
		}
		if loaded.FailureCount != 10 {
			t.Errorf("Expected failure count 10, got %d", loaded.FailureCount)
		}

		// Verify temp file is still cleaned up
		if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
			t.Error("Expected temp file to be cleaned up after second save")
		}
	})

	t.Run("Persists across multiple save/load cycles", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)
		ctx := context.Background()

		for i := 0; i < 10; i++ {
			snapshot := breaker.NewSnapshotBuilder().
				WithFailureCount(i).
				Build()

			err := repo.Save(ctx, snapshot)
			if err != nil {
				t.Fatalf("Save failed on iteration %d: %v", i, err)
			}

			loaded, err := repo.Load(ctx)
			if err != nil {
				t.Fatalf("Load failed on iteration %d: %v", i, err)
			}

			if loaded.FailureCount != i {
				t.Errorf("Iteration %d: expected failure count %d, got %d", i, i, loaded.FailureCount)
			}
		}
	})

	t.Run("String returns readable representation", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, _ := breaker.NewFileSnapshotRepository(filePath)

		str := repo.String()
		if str == "" {
			t.Error("Expected non-empty string representation")
		}
		// Should contain the file path
		if len(str) < len(filePath) {
			t.Error("Expected string representation to contain file path")
		}
	})
}

func TestFileSnapshotRepositoryIntegrationWithCircuitBreaker(t *testing.T) {
	t.Run("Circuit breaker can use file repository for persistence", func(t *testing.T) {
		filePath := tempFilePath(t)
		repo, err := breaker.NewFileSnapshotRepository(filePath)
		if err != nil {
			t.Fatalf("Failed to create repository: %v", err)
		}

		// Create circuit breaker with file repository
		cb, err := breaker.New(
			breaker.WithFailureThreshold(3),
			breaker.WithResetTimeout(1*time.Second),
			breaker.WithSnapshotRepository(repo),
			breaker.WithAsyncPersistence(false), // Use synchronous for testing
		)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// Cause some failures to change state
		for i := 0; i < 3; i++ {
			cb.Execute(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}

		// Verify file was created
		exists, _ := repo.Exists(context.Background())
		if !exists {
			t.Error("Expected snapshot file to exist after state changes")
		}

		// Load snapshot directly
		snapshot, err := repo.Load(context.Background())
		if err != nil {
			t.Fatalf("Failed to load snapshot: %v", err)
		}

		if snapshot.State != breaker.Open {
			t.Errorf("Expected state to be Open, got %v", snapshot.State)
		}
	})
}
