package errors

import (
	"fmt"
	"time"
)

// APIError represents an error from the Notte API
type APIError struct {
	Code       string // Error code from API (e.g., "INVALID_REQUEST")
	Message    string // Human-readable message
	StatusCode int    // HTTP status code
	Source     string // Which field caused the error (optional)
	Cause      error  // Underlying error (optional)
}

func (e *APIError) Error() string {
	if e.Source != "" {
		return fmt.Sprintf("API error (%d): %s - %s [%s]", e.StatusCode, e.Code, e.Message, e.Source)
	}
	return fmt.Sprintf("API error (%d): %s - %s", e.StatusCode, e.Code, e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Cause
}

// ValidationError represents client-side input validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message)
}

// RateLimitError indicates rate limiting with retry guidance
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited: retry after %s", e.RetryAfter)
}

// AuthError represents authentication/authorization failures
type AuthError struct {
	Reason string // "expired", "invalid", "missing"
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication error: %s", e.Reason)
}

// CircuitBreakerError indicates the circuit breaker is open
type CircuitBreakerError struct {
	OpenUntil time.Time
}

func (e *CircuitBreakerError) Error() string {
	remaining := time.Until(e.OpenUntil)
	if remaining < 0 {
		remaining = 0
	}
	return fmt.Sprintf("service unavailable: circuit breaker open, retry in %s", remaining.Round(time.Second))
}

// IsRetryable returns true if the error is potentially recoverable via retry
func IsRetryable(err error) bool {
	switch e := err.(type) {
	case *RateLimitError:
		return true
	case *APIError:
		// Only 5xx errors are retryable
		return e.StatusCode >= 500 && e.StatusCode < 600
	default:
		return false
	}
}
