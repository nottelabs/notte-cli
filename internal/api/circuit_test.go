// internal/api/circuit_test.go
package api

import (
	"testing"
	"time"
)

func TestCircuitBreaker_StartsOK(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second)

	if !cb.Allow() {
		t.Error("circuit breaker should allow requests initially")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	// Record failures
	cb.RecordFailure()
	cb.RecordFailure()

	// Should still be closed
	if !cb.Allow() {
		t.Error("should allow after 2 failures")
	}

	cb.RecordFailure() // 3rd failure

	// Should now be open
	if cb.Allow() {
		t.Error("should not allow after 3 failures")
	}
}

func TestCircuitBreaker_ResetsOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()

	// Failure count should be reset
	cb.RecordFailure()
	cb.RecordFailure()

	// Should still allow (only 2 consecutive failures)
	if !cb.Allow() {
		t.Error("should allow - success reset the counter")
	}
}

func TestCircuitBreaker_ClosesAfterResetPeriod(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()

	// Should be open
	if cb.Allow() {
		t.Error("should be open")
	}

	// Wait for reset period
	time.Sleep(60 * time.Millisecond)

	// Should allow (half-open state)
	if !cb.Allow() {
		t.Error("should allow after reset period")
	}
}

func TestCircuitBreaker_State(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)

	if cb.State() != CircuitClosed {
		t.Errorf("initial state should be Closed, got %v", cb.State())
	}

	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Errorf("state should be Open after failures, got %v", cb.State())
	}

	time.Sleep(60 * time.Millisecond)

	if cb.State() != CircuitHalfOpen {
		t.Errorf("state should be HalfOpen after reset period, got %v", cb.State())
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state CircuitState
		want  string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("CircuitState(%d).String() = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}
