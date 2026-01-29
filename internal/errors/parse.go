package errors

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// apiErrorResponse represents the JSON error format from the API
// Supports both nested format {"error": {"message": "..."}} and flat format {"message": "..."}
type apiErrorResponse struct {
	// Nested error format
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Source  string `json:"source,omitempty"`
	} `json:"error"`
	// Flat error format (used by validation errors)
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
}

// ParseAPIError parses an HTTP response into an appropriate error type.
// The body parameter should contain the already-read response body bytes
// (from the generated client's resp.Body field).
func ParseAPIError(resp *http.Response, body []byte) error {
	if resp == nil {
		return &APIError{Message: "nil response"}
	}

	// Handle specific status codes
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &AuthError{Reason: "invalid"}
	case http.StatusForbidden:
		return &AuthError{Reason: "forbidden"}
	case http.StatusTooManyRequests:
		return parseRateLimitError(resp)
	}

	// Try to parse JSON error
	var apiResp apiErrorResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		// Fallback for non-JSON responses
		return &APIError{
			StatusCode: resp.StatusCode,
			Code:       http.StatusText(resp.StatusCode),
			Message:    SanitizeMessage(string(body)),
		}
	}

	// Check nested error format first, then flat format
	message := apiResp.Error.Message
	if message == "" {
		message = apiResp.Message
	}
	code := apiResp.Error.Code
	if code == "" {
		code = http.StatusText(resp.StatusCode)
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Code:       code,
		Message:    SanitizeMessage(message),
		Source:     apiResp.Error.Source,
	}
}

func parseRateLimitError(resp *http.Response) *RateLimitError {
	retryAfter := 60 * time.Second // Default

	if header := resp.Header.Get("Retry-After"); header != "" {
		if seconds, err := strconv.Atoi(header); err == nil {
			retryAfter = time.Duration(seconds) * time.Second
		}
	}

	return &RateLimitError{RetryAfter: retryAfter}
}

// SanitizeMessage cleans error messages for safe display
func SanitizeMessage(msg string) string {
	const maxLen = 500

	// Truncate if too long
	if len(msg) > maxLen {
		msg = msg[:maxLen] + "..."
	}

	// Remove control characters except newline
	var sb strings.Builder
	sb.Grow(len(msg))
	for _, r := range msg {
		if r >= 32 || r == '\n' {
			sb.WriteRune(r)
		}
	}

	return sb.String()
}
