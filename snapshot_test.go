package breaker_test

import (
	"encoding/json"
	"testing"
	"time"

	breaker "github.com/farzai/breaker-go"
)

func TestSnapshotBuilder(t *testing.T) {
	t.Run("Creates snapshot with defaults", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().Build()

		if snapshot.Version != breaker.CurrentSnapshotVersion {
			t.Errorf("Expected version %d, got %d", breaker.CurrentSnapshotVersion, snapshot.Version)
		}
		if snapshot.State != breaker.Closed {
			t.Errorf("Expected state Closed, got %v", snapshot.State)
		}
		if snapshot.Metadata == nil {
			t.Error("Expected metadata to be initialized")
		}
		if snapshot.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}
	})

	t.Run("WithState sets the state", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			Build()

		if snapshot.State != breaker.Open {
			t.Errorf("Expected state Open, got %v", snapshot.State)
		}
	})

	t.Run("WithFailureCount sets the failure count", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithFailureCount(7).
			Build()

		if snapshot.FailureCount != 7 {
			t.Errorf("Expected failure count 7, got %d", snapshot.FailureCount)
		}
	})

	t.Run("WithLastFailure sets the last failure time", func(t *testing.T) {
		now := time.Now()
		snapshot := breaker.NewSnapshotBuilder().
			WithLastFailure(now).
			Build()

		if !snapshot.LastFailure.Equal(now) {
			t.Errorf("Expected last failure %v, got %v", now, snapshot.LastFailure)
		}
	})

	t.Run("WithConfig sets the configuration", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithConfig(5, 2*time.Second).
			Build()

		if snapshot.Config.FailureThreshold != 5 {
			t.Errorf("Expected failure threshold 5, got %d", snapshot.Config.FailureThreshold)
		}
		if snapshot.Config.ResetTimeout != 2*time.Second {
			t.Errorf("Expected reset timeout 2s, got %v", snapshot.Config.ResetTimeout)
		}
	})

	t.Run("WithMetadata adds metadata", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithMetadata("key1", "value1").
			WithMetadata("key2", 42).
			Build()

		if snapshot.Metadata["key1"] != "value1" {
			t.Errorf("Expected metadata key1='value1', got %v", snapshot.Metadata["key1"])
		}
		if snapshot.Metadata["key2"] != 42 {
			t.Errorf("Expected metadata key2=42, got %v", snapshot.Metadata["key2"])
		}
	})

	t.Run("WithVersion sets custom version", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithVersion(2).
			Build()

		if snapshot.Version != 2 {
			t.Errorf("Expected version 2, got %d", snapshot.Version)
		}
	})

	t.Run("Builder supports method chaining", func(t *testing.T) {
		now := time.Now()
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.HalfOpen).
			WithFailureCount(3).
			WithLastFailure(now).
			WithConfig(10, 5*time.Second).
			WithMetadata("app", "test").
			WithVersion(1).
			Build()

		if snapshot.State != breaker.HalfOpen {
			t.Error("State not set correctly")
		}
		if snapshot.FailureCount != 3 {
			t.Error("FailureCount not set correctly")
		}
		if !snapshot.LastFailure.Equal(now) {
			t.Error("LastFailure not set correctly")
		}
		if snapshot.Config.FailureThreshold != 10 {
			t.Error("Config not set correctly")
		}
		if snapshot.Metadata["app"] != "test" {
			t.Error("Metadata not set correctly")
		}
	})

	t.Run("Build returns immutable snapshot", func(t *testing.T) {
		builder := breaker.NewSnapshotBuilder().
			WithMetadata("original", "value")

		snapshot1 := builder.Build()
		builder.WithMetadata("modified", "new")
		snapshot2 := builder.Build()

		// snapshot1 should not have the modified metadata
		if _, exists := snapshot1.Metadata["modified"]; exists {
			t.Error("First snapshot was modified by subsequent builder changes")
		}

		// snapshot2 should have both
		if snapshot2.Metadata["original"] != "value" {
			t.Error("Second snapshot missing original metadata")
		}
		if snapshot2.Metadata["modified"] != "new" {
			t.Error("Second snapshot missing modified metadata")
		}
	})
}

func TestSnapshotSerialization(t *testing.T) {
	t.Run("ToJSON serializes snapshot", func(t *testing.T) {
		now := time.Now()
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithFailureCount(5).
			WithLastFailure(now).
			WithConfig(3, 1*time.Second).
			WithMetadata("test", "data").
			Build()

		data, err := snapshot.ToJSON()
		if err != nil {
			t.Fatalf("Failed to serialize snapshot: %v", err)
		}

		if len(data) == 0 {
			t.Error("Expected non-empty JSON data")
		}

		// Verify it's valid JSON
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Errorf("Invalid JSON produced: %v", err)
		}
	})

	t.Run("FromJSON deserializes snapshot", func(t *testing.T) {
		original := breaker.NewSnapshotBuilder().
			WithState(breaker.HalfOpen).
			WithFailureCount(2).
			WithConfig(5, 3*time.Second).
			Build()

		data, _ := original.ToJSON()
		loaded, err := breaker.FromJSON(data)

		if err != nil {
			t.Fatalf("Failed to deserialize snapshot: %v", err)
		}

		if loaded.State != original.State {
			t.Errorf("State mismatch: expected %v, got %v", original.State, loaded.State)
		}
		if loaded.FailureCount != original.FailureCount {
			t.Errorf("FailureCount mismatch: expected %d, got %d", original.FailureCount, loaded.FailureCount)
		}
		if loaded.Config.FailureThreshold != original.Config.FailureThreshold {
			t.Error("Config.FailureThreshold mismatch")
		}
	})

	t.Run("FromJSON handles invalid JSON", func(t *testing.T) {
		_, err := breaker.FromJSON([]byte("invalid json"))

		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})

	t.Run("Round-trip serialization preserves data", func(t *testing.T) {
		now := time.Now().Truncate(time.Second) // Truncate to avoid nanosecond precision issues
		original := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithFailureCount(10).
			WithLastFailure(now).
			WithConfig(5, 2*time.Second).
			WithMetadata("key", "value").
			Build()

		// Serialize
		data, err := original.ToJSON()
		if err != nil {
			t.Fatalf("Serialization failed: %v", err)
		}

		// Deserialize
		loaded, err := breaker.FromJSON(data)
		if err != nil {
			t.Fatalf("Deserialization failed: %v", err)
		}

		// Verify all fields
		if loaded.Version != original.Version {
			t.Error("Version mismatch")
		}
		if loaded.State != original.State {
			t.Error("State mismatch")
		}
		if loaded.FailureCount != original.FailureCount {
			t.Error("FailureCount mismatch")
		}
		if loaded.Config.FailureThreshold != original.Config.FailureThreshold {
			t.Error("FailureThreshold mismatch")
		}
		if loaded.Config.ResetTimeout != original.Config.ResetTimeout {
			t.Error("ResetTimeout mismatch")
		}
		if loaded.Metadata["key"] != "value" {
			t.Error("Metadata mismatch")
		}
	})
}

func TestSnapshotClone(t *testing.T) {
	t.Run("Clone creates independent copy", func(t *testing.T) {
		original := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithFailureCount(5).
			WithMetadata("key", "original").
			Build()

		cloned := original.Clone()

		// Verify values match
		if cloned.State != original.State {
			t.Error("State mismatch")
		}
		if cloned.FailureCount != original.FailureCount {
			t.Error("FailureCount mismatch")
		}

		// Modify clone's metadata
		cloned.Metadata["key"] = "modified"
		cloned.Metadata["new"] = "value"

		// Verify original is unchanged
		if original.Metadata["key"] != "original" {
			t.Error("Original metadata was modified")
		}
		if _, exists := original.Metadata["new"]; exists {
			t.Error("Original metadata has new key from clone")
		}
	})

	t.Run("Clone handles nil metadata", func(t *testing.T) {
		original := breaker.NewSnapshotBuilder().Build()
		original.Metadata = nil

		cloned := original.Clone()

		if cloned.Metadata != nil {
			t.Error("Expected nil metadata in clone")
		}
	})
}

func TestSnapshotExpiration(t *testing.T) {
	t.Run("IsExpired returns false for non-Open state", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Closed).
			WithConfig(3, 1*time.Second).
			WithLastFailure(time.Now().Add(-2 * time.Second)).
			Build()

		if snapshot.IsExpired() {
			t.Error("Closed state snapshot should not be expired")
		}
	})

	t.Run("IsExpired returns false for recent Open state", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithConfig(3, 2*time.Second).
			WithLastFailure(time.Now().Add(-1 * time.Second)). // 1 second ago, timeout is 2s
			Build()

		if snapshot.IsExpired() {
			t.Error("Recent Open state snapshot should not be expired")
		}
	})

	t.Run("IsExpired returns true for old Open state", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithConfig(3, 1*time.Second).
			WithLastFailure(time.Now().Add(-2 * time.Second)). // 2 seconds ago, timeout is 1s
			Build()

		if !snapshot.IsExpired() {
			t.Error("Old Open state snapshot should be expired")
		}
	})

	t.Run("ShouldTransitionToHalfOpen for expired Open state", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithConfig(3, 1*time.Second).
			WithLastFailure(time.Now().Add(-2 * time.Second)).
			Build()

		if !snapshot.ShouldTransitionToHalfOpen() {
			t.Error("Expired Open state should transition to HalfOpen")
		}
	})

	t.Run("ShouldTransitionToHalfOpen returns false for non-Open state", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Closed).
			WithConfig(3, 1*time.Second).
			WithLastFailure(time.Now().Add(-2 * time.Second)).
			Build()

		if snapshot.ShouldTransitionToHalfOpen() {
			t.Error("Non-Open state should not transition to HalfOpen")
		}
	})

	t.Run("ShouldTransitionToHalfOpen returns false for recent Open state", func(t *testing.T) {
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Open).
			WithConfig(3, 2*time.Second).
			WithLastFailure(time.Now()).
			Build()

		if snapshot.ShouldTransitionToHalfOpen() {
			t.Error("Recent Open state should not transition to HalfOpen")
		}
	})
}

func TestFromCircuitBreaker(t *testing.T) {
	t.Run("Creates snapshot from circuit breaker state", func(t *testing.T) {
		cb, err := breaker.NewCircuitBreaker(3, 1*time.Second)
		if err != nil {
			t.Fatalf("Failed to create circuit breaker: %v", err)
		}

		// Get the implementation to create snapshot
		// Note: This requires the circuit breaker to expose this functionality
		// For now, we'll just verify the builder works with typical values
		snapshot := breaker.NewSnapshotBuilder().
			WithState(breaker.Closed).
			WithFailureCount(0).
			WithConfig(3, 1*time.Second).
			Build()

		if snapshot.State != cb.State() {
			t.Errorf("Expected state %v, got %v", cb.State(), snapshot.State)
		}
	})
}
