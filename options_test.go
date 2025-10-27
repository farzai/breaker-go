package breaker

import (
	"context"
	"testing"
)

// TestStateToString tests the stateToString function including the default case
func TestStateToString(t *testing.T) {
	tests := []struct {
		name     string
		state    CircuitBreakerState
		expected string
	}{
		{
			name:     "Closed state",
			state:    Closed,
			expected: "Closed",
		},
		{
			name:     "Open state",
			state:    Open,
			expected: "Open",
		},
		{
			name:     "HalfOpen state",
			state:    HalfOpen,
			expected: "HalfOpen",
		},
		{
			name:     "Unknown state",
			state:    CircuitBreakerState(999), // Invalid state
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stateToString(tt.state)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestFormatStateChange tests the formatStateChange function
func TestFormatStateChange(t *testing.T) {
	result := formatStateChange(Closed, Open)
	expected := "Circuit breaker state changed from Closed to Open"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

// TestLoggingObserver tests the LoggingObserver
func TestLoggingObserver(t *testing.T) {
	t.Run("Should call LogFunc when state changes", func(t *testing.T) {
		var logMessage string
		observer := &LoggingObserver{
			LogFunc: func(msg string) {
				logMessage = msg
			},
		}

		observer.OnStateChange(context.Background(), Closed, Open)

		expected := "Circuit breaker state changed from Closed to Open"
		if logMessage != expected {
			t.Errorf("Expected '%s', got '%s'", expected, logMessage)
		}
	})

	t.Run("Should not panic when LogFunc is nil", func(t *testing.T) {
		observer := &LoggingObserver{
			LogFunc: nil,
		}

		// Should not panic
		observer.OnStateChange(context.Background(), Closed, Open)
	})
}

// TestNewWithOptions tests the NewWithOptions function
func TestNewWithOptions(t *testing.T) {
	t.Run("Should create circuit breaker with default options", func(t *testing.T) {
		cb := NewWithOptions()

		if cb.State() != Closed {
			t.Errorf("Expected initial state to be Closed, got %v", cb.State())
		}
	})

	t.Run("Should create circuit breaker with custom options", func(t *testing.T) {
		cb := NewWithOptions(
			WithFailureThreshold(10),
			WithResetTimeout(10),
		)

		if cb.State() != Closed {
			t.Errorf("Expected initial state to be Closed, got %v", cb.State())
		}
	})
}
