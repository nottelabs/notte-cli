package errors

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// apiErrorResponse represents the JSON error format from the API
// Supports multiple formats:
// - Nested: {"error": {"code": "...", "message": "..."}}
// - Flat error string: {"error": "..."}
// - Flat message: {"message": "..."}
// - FastAPI detail string: {"detail": "..."}
// - FastAPI detail array: {"detail": [{"loc": [...], "msg": "...", "type": "..."}]}
type apiErrorResponse struct {
	// Error can be a string or an object, so use RawMessage
	Error json.RawMessage `json:"error,omitempty"`
	// Flat error format
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
	// FastAPI validation error format (can be string or array)
	Detail json.RawMessage `json:"detail,omitempty"`
}

// nestedError represents the nested error object format
type nestedError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
}

// fastAPIValidationError represents a single validation error from FastAPI
type fastAPIValidationError struct {
	Loc  []any  `json:"loc"`
	Msg  string `json:"msg"`
	Type string `json:"type"`
}

// ParseAPIError parses an HTTP response into an appropriate error type.
// The body parameter should contain the already-read response body bytes
// (from the generated client's resp.Body field).
func ParseAPIError(resp *http.Response, body []byte) error {
	if resp == nil {
		return &APIError{Message: "nil response"}
	}

	// Handle rate limiting separately
	if resp.StatusCode == http.StatusTooManyRequests {
		return parseRateLimitError(resp)
	}

	// Try to parse JSON error body for all error responses
	var apiResp apiErrorResponse
	var message string
	if err := json.Unmarshal(body, &apiResp); err == nil {
		message = extractErrorMessage(&apiResp)
	}

	// If JSON parsing failed, use raw body as message
	if message == "" && len(body) > 0 {
		message = string(body)
	}

	// Handle auth-specific status codes with detailed messages
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &AuthError{
			Reason:     "invalid",
			Message:    SanitizeMessage(message),
			StatusCode: resp.StatusCode,
		}
	case http.StatusForbidden:
		return &AuthError{
			Reason:     "forbidden",
			Message:    SanitizeMessage(message),
			StatusCode: resp.StatusCode,
		}
	}

	// Parse the error field for other errors (can be string or object)
	var code, source string
	if len(apiResp.Error) > 0 {
		// Try as nested object first
		var nested nestedError
		if err := json.Unmarshal(apiResp.Error, &nested); err == nil {
			code = nested.Code
			source = nested.Source
		}
	}

	if code == "" {
		code = http.StatusText(resp.StatusCode)
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Code:       code,
		Message:    SanitizeMessage(message),
		Source:     source,
	}
}

// extractErrorMessage extracts the error message from various API response formats
func extractErrorMessage(apiResp *apiErrorResponse) string {
	var message string

	// Try parsing the error field (can be string or object)
	if len(apiResp.Error) > 0 {
		// Try as nested object first
		var nested nestedError
		if err := json.Unmarshal(apiResp.Error, &nested); err == nil {
			message = nested.Message
		} else {
			// Try as string
			var errStr string
			if err := json.Unmarshal(apiResp.Error, &errStr); err == nil {
				message = errStr
			}
		}
	}

	// Fall back to flat message field
	if message == "" {
		message = apiResp.Message
	}

	// Fall back to detail field
	if message == "" {
		message = parseDetailMessage(apiResp.Detail)
	}

	return message
}

// parseDetailMessage extracts a message from FastAPI's detail field
// which can be either a string or an array of validation errors
func parseDetailMessage(detail json.RawMessage) string {
	if len(detail) == 0 {
		return ""
	}

	// Try parsing as a simple string first
	var detailStr string
	if err := json.Unmarshal(detail, &detailStr); err == nil {
		return detailStr
	}

	// Try parsing as an array of validation errors
	var validationErrors []fastAPIValidationError
	if err := json.Unmarshal(detail, &validationErrors); err == nil && len(validationErrors) > 0 {
		// Combine all error messages
		var messages []string
		for _, ve := range validationErrors {
			if ve.Msg != "" {
				messages = append(messages, ve.Msg)
			}
		}
		return strings.Join(messages, "; ")
	}

	// Fallback: return the raw detail as string
	return string(detail)
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
