package errors

import (
	"net/http"
	"strings"
	"testing"
)

func TestParseAPIError_400(t *testing.T) {
	body := []byte(`{
		"error": {
			"code": "INVALID_REQUEST",
			"message": "Invalid session ID format"
		}
	}`)
	resp := &http.Response{
		StatusCode: 400,
	}

	err := ParseAPIError(resp, body)

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}

	if apiErr.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", apiErr.StatusCode)
	}
	if apiErr.Code != "INVALID_REQUEST" {
		t.Errorf("Code = %q, want 'INVALID_REQUEST'", apiErr.Code)
	}
	if apiErr.Message != "Invalid session ID format" {
		t.Errorf("Message = %q, want 'Invalid session ID format'", apiErr.Message)
	}
}

func TestParseAPIError_401(t *testing.T) {
	body := []byte(`{
		"error": {
			"code": "UNAUTHORIZED",
			"message": "Invalid API key"
		}
	}`)
	resp := &http.Response{
		StatusCode: 401,
	}

	err := ParseAPIError(resp, body)

	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatalf("expected *AuthError, got %T", err)
	}

	if authErr.Reason != "invalid" {
		t.Errorf("Reason = %q, want 'invalid'", authErr.Reason)
	}
}

func TestParseAPIError_429(t *testing.T) {
	body := []byte(`{"error": {"code": "RATE_LIMITED"}}`)
	resp := &http.Response{
		StatusCode: 429,
		Header:     http.Header{"Retry-After": []string{"30"}},
	}

	err := ParseAPIError(resp, body)

	rateLimitErr, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("expected *RateLimitError, got %T", err)
	}

	if rateLimitErr.RetryAfter.Seconds() != 30 {
		t.Errorf("RetryAfter = %v, want 30s", rateLimitErr.RetryAfter)
	}
}

func TestParseAPIError_500(t *testing.T) {
	body := []byte(`{"error": {"message": "Internal server error"}}`)
	resp := &http.Response{
		StatusCode: 500,
	}

	err := ParseAPIError(resp, body)

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}

	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", apiErr.StatusCode)
	}
}

func TestParseAPIError_MalformedJSON(t *testing.T) {
	body := []byte(`not json`)
	resp := &http.Response{
		StatusCode: 500,
	}

	err := ParseAPIError(resp, body)

	// Should still return an APIError with status code
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}

	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", apiErr.StatusCode)
	}
}

func TestSanitizeMessage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"short message", "short message"},
		{strings.Repeat("a", 600), strings.Repeat("a", 500) + "..."},
		{"has\x00null\x01bytes", "hasnullbytes"},
		{"keeps\nnewlines", "keeps\nnewlines"},
	}

	for _, tt := range tests {
		got := SanitizeMessage(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeMessage(%q) = %q, want %q", tt.input[:min(20, len(tt.input))], got[:min(20, len(got))], tt.want[:min(20, len(tt.want))])
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
