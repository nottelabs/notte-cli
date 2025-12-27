// internal/api/circuit.go
package api

import (
	"sync"
	"time"
)

// CircuitState represents the circuit breaker state
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker prevents cascading failures
type CircuitBreaker struct {
	mu          sync.RWMutex
	failures    int
	threshold   int
	resetPeriod time.Duration
	openUntil   time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(threshold int, resetPeriod time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:   threshold,
		resetPeriod: resetPeriod,
	}
}

// Allow returns true if the circuit allows a request
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	state := cb.stateUnlocked()
	return state != CircuitOpen
}

// State returns the current circuit state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.stateUnlocked()
}

func (cb *CircuitBreaker) stateUnlocked() CircuitState {
	if cb.failures < cb.threshold {
		return CircuitClosed
	}

	if time.Now().After(cb.openUntil) {
		return CircuitHalfOpen
	}

	return CircuitOpen
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.openUntil = time.Time{}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++

	if cb.failures >= cb.threshold {
		cb.openUntil = time.Now().Add(cb.resetPeriod)
	}
}

// OpenUntil returns when the circuit will close (zero if closed)
func (cb *CircuitBreaker) OpenUntil() time.Time {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.openUntil
}
