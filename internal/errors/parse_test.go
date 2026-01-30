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
	if authErr.Message != "Invalid API key" {
		t.Errorf("Message = %q, want 'Invalid API key'", authErr.Message)
	}
	if authErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", authErr.StatusCode)
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

// TestParseAPIError_FastAPIDetailString tests that the detail field (string format)
// is parsed correctly for any status code, not just 422
func TestParseAPIError_FastAPIDetailString(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantCode   string
	}{
		{"400 Bad Request", 400, "Bad Request"},
		{"404 Not Found", 404, "Not Found"},
		{"422 Unprocessable Entity", 422, "Unprocessable Entity"},
		{"500 Internal Server Error", 500, "Internal Server Error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"detail": "something went wrong"}`)
			resp := &http.Response{
				StatusCode: tt.statusCode,
			}

			err := ParseAPIError(resp, body)

			apiErr, ok := err.(*APIError)
			if !ok {
				t.Fatalf("expected *APIError, got %T", err)
			}

			if apiErr.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.statusCode)
			}
			if apiErr.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", apiErr.Code, tt.wantCode)
			}
			if apiErr.Message != "something went wrong" {
				t.Errorf("Message = %q, want 'something went wrong'", apiErr.Message)
			}
		})
	}
}

// TestParseAPIError_FastAPIDetailArray tests that the detail field (array format)
// is parsed correctly for any status code
func TestParseAPIError_FastAPIDetailArray(t *testing.T) {
	body := []byte(`{
		"detail": [
			{"loc": ["body", "url"], "msg": "field required", "type": "value_error.missing"},
			{"loc": ["body", "action"], "msg": "invalid action type", "type": "value_error"}
		]
	}`)
	resp := &http.Response{
		StatusCode: 422,
	}

	err := ParseAPIError(resp, body)

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}

	if apiErr.StatusCode != 422 {
		t.Errorf("StatusCode = %d, want 422", apiErr.StatusCode)
	}
	// Should contain both error messages
	if !strings.Contains(apiErr.Message, "field required") {
		t.Errorf("Message = %q, should contain 'field required'", apiErr.Message)
	}
	if !strings.Contains(apiErr.Message, "invalid action type") {
		t.Errorf("Message = %q, should contain 'invalid action type'", apiErr.Message)
	}
}

func TestParseAPIError_FastAPISingleError(t *testing.T) {
	body := []byte(`{
		"detail": [
			{"loc": ["body"], "msg": "Invalid URL format", "type": "value_error"}
		]
	}`)
	resp := &http.Response{
		StatusCode: 400,
	}

	err := ParseAPIError(resp, body)

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}

	if apiErr.Message != "Invalid URL format" {
		t.Errorf("Message = %q, want 'Invalid URL format'", apiErr.Message)
	}
}

// TestParseAPIError_ErrorAsString tests the case where "error" is a string, not an object
func TestParseAPIError_ErrorAsString(t *testing.T) {
	body := []byte(`{
		"detail": "No snapshot is available",
		"error": "No snapshot is available",
		"message": "No snapshot is available",
		"status": 500
	}`)
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
	if apiErr.Message != "No snapshot is available" {
		t.Errorf("Message = %q, want 'No snapshot is available'", apiErr.Message)
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
