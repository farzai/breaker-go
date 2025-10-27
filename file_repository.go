package breaker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileSnapshotRepository provides a file-based, thread-safe implementation
// of SnapshotRepository. It persists snapshots to disk using JSON serialization.
//
// The repository uses atomic file writes (write to temp file, then rename)
// to prevent corruption in case of crashes or interruptions.
type FileSnapshotRepository struct {
	filePath string
	mutex    sync.RWMutex
}

// FileSnapshotRepositoryOptions configures the file repository
type FileSnapshotRepositoryOptions struct {
	// FilePath is the full path to the snapshot file.
	// If empty, a default path in the system temp directory will be used.
	FilePath string

	// DirPath is the directory where the snapshot file will be stored.
	// Only used if FilePath is empty.
	// If both are empty, defaults to os.TempDir().
	DirPath string

	// FileName is the name of the snapshot file.
	// Only used if FilePath is empty.
	// Defaults to "circuit_breaker_snapshot.json".
	FileName string
}

// NewFileSnapshotRepository creates a new file-based snapshot repository
// with the given file path.
func NewFileSnapshotRepository(filePath string) (*FileSnapshotRepository, error) {
	if filePath == "" {
		return nil, NewValidationError("filePath", "cannot be empty", nil)
	}

	// Ensure the directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, NewPersistenceError("create directory", err)
	}

	return &FileSnapshotRepository{
		filePath: filePath,
	}, nil
}

// NewFileSnapshotRepositoryWithOptions creates a new file-based snapshot repository
// with the given options.
func NewFileSnapshotRepositoryWithOptions(opts FileSnapshotRepositoryOptions) (*FileSnapshotRepository, error) {
	var filePath string

	if opts.FilePath != "" {
		filePath = opts.FilePath
	} else {
		// Build path from components
		dirPath := opts.DirPath
		if dirPath == "" {
			dirPath = os.TempDir()
		}

		fileName := opts.FileName
		if fileName == "" {
			fileName = "circuit_breaker_snapshot.json"
		}

		filePath = filepath.Join(dirPath, fileName)
	}

	return NewFileSnapshotRepository(filePath)
}

// NewTempFileSnapshotRepository creates a new file-based snapshot repository
// using a temporary file in the system temp directory.
func NewTempFileSnapshotRepository() (*FileSnapshotRepository, error) {
	return NewFileSnapshotRepositoryWithOptions(FileSnapshotRepositoryOptions{
		DirPath:  os.TempDir(),
		FileName: "circuit_breaker_snapshot.json",
	})
}

// Save stores the circuit breaker snapshot to a file.
// This method uses atomic writes to prevent corruption.
func (r *FileSnapshotRepository) Save(ctx context.Context, snapshot *Snapshot) error {
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

	// Serialize snapshot to JSON
	data, err := snapshot.ToJSON()
	if err != nil {
		return NewPersistenceError("serialize", err)
	}

	// Write to a temporary file first (atomic write)
	tempFile := r.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return NewPersistenceError("write temp file", err)
	}

	// Atomically rename temp file to target file
	if err := os.Rename(tempFile, r.filePath); err != nil {
		// Clean up temp file on failure
		os.Remove(tempFile)
		return NewPersistenceError("rename file", err)
	}

	return nil
}

// Load retrieves the stored snapshot from the file.
func (r *FileSnapshotRepository) Load(ctx context.Context) (*Snapshot, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, NewPersistenceError("load", ctx.Err())
	default:
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Check if file exists
	if _, err := os.Stat(r.filePath); os.IsNotExist(err) {
		return nil, ErrSnapshotNotFound
	}

	// Read file
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return nil, NewPersistenceError("read file", err)
	}

	// Deserialize snapshot
	snapshot, err := FromJSON(data)
	if err != nil {
		return nil, err // FromJSON already wraps with PersistenceError
	}

	return snapshot, nil
}

// Exists checks if a snapshot file exists.
func (r *FileSnapshotRepository) Exists(ctx context.Context) (bool, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return false, NewPersistenceError("exists", ctx.Err())
	default:
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	_, err := os.Stat(r.filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, NewPersistenceError("stat file", err)
}

// Delete removes the snapshot file.
func (r *FileSnapshotRepository) Delete(ctx context.Context) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return NewPersistenceError("delete", ctx.Err())
	default:
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if file exists
	if _, err := os.Stat(r.filePath); os.IsNotExist(err) {
		// File doesn't exist, consider it a success
		return nil
	}

	// Remove the file
	if err := os.Remove(r.filePath); err != nil {
		return NewPersistenceError("remove file", err)
	}

	return nil
}

// FilePath returns the path to the snapshot file.
// This is useful for debugging and testing.
func (r *FileSnapshotRepository) FilePath() string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.filePath
}

// String returns a string representation of the repository.
func (r *FileSnapshotRepository) String() string {
	return fmt.Sprintf("FileSnapshotRepository{filePath: %s}", r.filePath)
}
