package breaker

import (
	"fmt"
	"time"
)

// SnapshotValidator defines the interface for validating snapshots
type SnapshotValidator interface {
	// Validate checks if a snapshot is valid and safe to restore
	Validate(snapshot *Snapshot) error
}

// CompositeValidator combines multiple validators
type CompositeValidator struct {
	validators []SnapshotValidator
}

// NewCompositeValidator creates a validator that runs all provided validators
func NewCompositeValidator(validators ...SnapshotValidator) *CompositeValidator {
	return &CompositeValidator{
		validators: validators,
	}
}

// Validate runs all validators in sequence
func (v *CompositeValidator) Validate(snapshot *Snapshot) error {
	for _, validator := range v.validators {
		if err := validator.Validate(snapshot); err != nil {
			return err
		}
	}
	return nil
}

// VersionValidator validates snapshot version compatibility
type VersionValidator struct {
	minVersion int
	maxVersion int
}

// NewVersionValidator creates a version validator
func NewVersionValidator(minVersion, maxVersion int) *VersionValidator {
	return &VersionValidator{
		minVersion: minVersion,
		maxVersion: maxVersion,
	}
}

// Validate checks if the snapshot version is supported
func (v *VersionValidator) Validate(snapshot *Snapshot) error {
	if snapshot.Version < v.minVersion || snapshot.Version > v.maxVersion {
		return fmt.Errorf("%w: version %d not in range [%d, %d]",
			ErrUnsupportedVersion, snapshot.Version, v.minVersion, v.maxVersion)
	}
	return nil
}

// StateValidator validates the state field
type StateValidator struct{}

// NewStateValidator creates a state validator
func NewStateValidator() *StateValidator {
	return &StateValidator{}
}

// Validate checks if the state is valid
func (v *StateValidator) Validate(snapshot *Snapshot) error {
	switch snapshot.State {
	case Closed, Open, HalfOpen:
		return nil
	default:
		return NewValidationError("state", "invalid state value", snapshot.State)
	}
}

// ConfigValidator validates configuration values
type ConfigValidator struct{}

// NewConfigValidator creates a config validator
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{}
}

// Validate checks if configuration values are valid
func (v *ConfigValidator) Validate(snapshot *Snapshot) error {
	if snapshot.Config.FailureThreshold <= 0 {
		return NewValidationError("config.failure_threshold",
			"must be greater than 0", snapshot.Config.FailureThreshold)
	}

	if snapshot.Config.ResetTimeout <= 0 {
		return NewValidationError("config.reset_timeout",
			"must be greater than 0", snapshot.Config.ResetTimeout)
	}

	return nil
}

// DataConsistencyValidator validates internal data consistency
type DataConsistencyValidator struct{}

// NewDataConsistencyValidator creates a data consistency validator
func NewDataConsistencyValidator() *DataConsistencyValidator {
	return &DataConsistencyValidator{}
}

// Validate checks if snapshot data is internally consistent
func (v *DataConsistencyValidator) Validate(snapshot *Snapshot) error {
	// Failure count should be non-negative
	if snapshot.FailureCount < 0 {
		return NewValidationError("failure_count",
			"cannot be negative", snapshot.FailureCount)
	}

	// If state is Open, lastFailure should be set
	if snapshot.State == Open && snapshot.LastFailure.IsZero() {
		return NewValidationError("last_failure",
			"must be set when state is Open", snapshot.LastFailure)
	}

	// If state is Closed, failure count should be 0
	if snapshot.State == Closed && snapshot.FailureCount != 0 {
		return NewValidationError("failure_count",
			"should be 0 when state is Closed", snapshot.FailureCount)
	}

	// CreatedAt should not be zero
	if snapshot.CreatedAt.IsZero() {
		return NewValidationError("created_at",
			"timestamp is not set", snapshot.CreatedAt)
	}

	// CreatedAt should not be in the future
	if snapshot.CreatedAt.After(time.Now().Add(time.Minute)) {
		return NewValidationError("created_at",
			"timestamp is in the future", snapshot.CreatedAt)
	}

	return nil
}

// AgeValidator validates that snapshot is not too old
type AgeValidator struct {
	maxAge time.Duration
}

// NewAgeValidator creates an age validator
func NewAgeValidator(maxAge time.Duration) *AgeValidator {
	return &AgeValidator{
		maxAge: maxAge,
	}
}

// Validate checks if snapshot is not too old
func (v *AgeValidator) Validate(snapshot *Snapshot) error {
	age := time.Since(snapshot.CreatedAt)
	if age > v.maxAge {
		return NewValidationError("created_at",
			fmt.Sprintf("snapshot too old (age: %v, max: %v)", age, v.maxAge),
			snapshot.CreatedAt)
	}
	return nil
}

// DefaultValidator creates a validator with standard validation rules
func DefaultValidator() SnapshotValidator {
	return NewCompositeValidator(
		NewVersionValidator(1, CurrentSnapshotVersion),
		NewStateValidator(),
		NewConfigValidator(),
		NewDataConsistencyValidator(),
	)
}

// StrictValidator creates a validator with strict rules including age check
func StrictValidator(maxAge time.Duration) SnapshotValidator {
	return NewCompositeValidator(
		NewVersionValidator(1, CurrentSnapshotVersion),
		NewStateValidator(),
		NewConfigValidator(),
		NewDataConsistencyValidator(),
		NewAgeValidator(maxAge),
	)
}
