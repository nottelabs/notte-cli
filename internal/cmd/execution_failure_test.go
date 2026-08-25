package cmd

import (
	"strings"
	"testing"

	"github.com/nottelabs/notte-cli/internal/api"
)

func strPtr(s string) *string { return &s }

// The API sends the failure twice: `exception` is a bare string rendered in the
// server's ErrorConfig mode (often the generic user-facing sentence), while
// `exception_detail` carries the concrete type and the per-audience messages.
// The CLI must report the structured one when it is there.
func TestExecutionFailureError_PrefersStructuredDetail(t *testing.T) {
	detail := &api.SerializedError{
		ErrorType:   "ActionExecutionError",
		DevMessage:  "Failed to execute action: evaluate_js on https://x. Reason: ReferenceError: foo is not defined",
		UserMessage: "Sorry, this action cannot be executed at the moment.",
	}

	err := executionFailureError(detail, strPtr("Sorry, this action cannot be executed at the moment."), "boom")

	got := err.Error()
	if !strings.Contains(got, "ActionExecutionError") {
		t.Fatalf("expected the concrete error type, got %q", got)
	}
	if !strings.Contains(got, "ReferenceError: foo is not defined") {
		t.Fatalf("expected the actual reason, got %q", got)
	}
	if strings.Contains(got, "Sorry, this action cannot be executed") {
		t.Fatalf("expected the generic user message to be dropped, got %q", got)
	}
}

func TestExecutionFailureError_FallsBackToUserMessage(t *testing.T) {
	detail := &api.SerializedError{ErrorType: "BrowserError", UserMessage: "Something unexpected happened."}

	err := executionFailureError(detail, nil, "")

	if got := err.Error(); got != "BrowserError: Something unexpected happened." {
		t.Fatalf("unexpected error: %q", got)
	}
}

// API builds that predate exception_detail send only the legacy string.
func TestExecutionFailureError_LegacyExceptionString(t *testing.T) {
	err := executionFailureError(nil, strPtr("boom"), "click failed")

	if got := err.Error(); got != "boom: click failed" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestExecutionFailureError_MessageOnly(t *testing.T) {
	err := executionFailureError(nil, nil, "click failed")

	if got := err.Error(); got != "action failed: click failed" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestExecutionFailureError_NothingToGoOn(t *testing.T) {
	err := executionFailureError(nil, nil, "")

	if got := err.Error(); got != "action failed" {
		t.Fatalf("unexpected error: %q", got)
	}
}
