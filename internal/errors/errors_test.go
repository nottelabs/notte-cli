package errors

import (
	"errors"
	"testing"
	"time"
)

func TestAPIError_Error(t *testing.T) {
	err := &APIError{
		Code:       "INVALID_REQUEST",
		Message:    "Invalid session ID",
		StatusCode: 400,
	}

	got := err.Error()
	want := "API error (400): INVALID_REQUEST - Invalid session ID"

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAPIError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &APIError{
		Code:    "INTERNAL",
		Message: "Something went wrong",
		Cause:   cause,
	}

	if !errors.Is(err, cause) {
		t.Error("APIError should unwrap to cause")
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "browser",
		Message: "expected chromium|chrome, got 'firefox'",
	}

	got := err.Error()
	want := "validation error: browser: expected chromium|chrome, got 'firefox'"

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRateLimitError_Error(t *testing.T) {
	tests := []struct {
		name       string
		retryAfter time.Duration
		message    string
		want       string
	}{
		{
			name:       "without message",
			retryAfter: 30 * time.Second,
			message:    "",
			want:       "rate limit exceeded: too many requests (retry after 30 seconds)",
		},
		{
			name:       "with message",
			retryAfter: 60 * time.Second,
			message:    "Max active sessions limit exceeded",
			want:       "rate limit exceeded: Max active sessions limit exceeded (retry after 1 minutes)",
		},
		{
			name:       "under 60 seconds",
			retryAfter: 45 * time.Second,
			message:    "",
			want:       "rate limit exceeded: too many requests (retry after 45 seconds)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &RateLimitError{
				RetryAfter: tt.retryAfter,
				Message:    tt.message,
			}

			got := err.Error()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAuthError_Error(t *testing.T) {
	for _, tc := range []struct {
		err  *AuthError
		want string
	}{
		{&AuthError{Reason: "expired"}, "authentication error: expired"},
		{&AuthError{Reason: "invalid", Message: "bad key"}, "authentication error: invalid - bad key"},
		{&AuthError{Reason: "expired", Message: "token expired", Hint: "mint a new one"}, "authentication error: expired - token expired (mint a new one)"},
		{&AuthError{Reason: "expired", Hint: "mint a new one"}, "authentication error: expired (mint a new one)"},
	} {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestCircuitBreakerError_Error(t *testing.T) {
	openUntil := time.Now().Add(30 * time.Second)
	err := &CircuitBreakerError{OpenUntil: openUntil}

	got := err.Error()
	if got == "" {
		t.Error("error message should not be empty")
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"rate limit", &RateLimitError{RetryAfter: time.Second}, true},
		{"api 500", &APIError{StatusCode: 500}, true},
		{"api 502", &APIError{StatusCode: 502}, true},
		{"api 503", &APIError{StatusCode: 503}, true},
		{"api 504", &APIError{StatusCode: 504}, true},
		{"api 400", &APIError{StatusCode: 400}, false},
		{"api 401", &APIError{StatusCode: 401}, false},
		{"validation", &ValidationError{Field: "x"}, false},
		{"auth", &AuthError{Reason: "expired"}, false},
		{"circuit breaker", &CircuitBreakerError{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryable(tt.err)
			if got != tt.retryable {
				t.Errorf("IsRetryable(%T) = %v, want %v", tt.err, got, tt.retryable)
			}
		})
	}
}
