//go:build integration

package integration

import (
	"encoding/json"
	"testing"
)

// TestErrorParsing_InvalidBrowserType tests that validation errors show proper messages
func TestErrorParsing_InvalidBrowserType(t *testing.T) {
	// Try to start a session with an invalid browser type
	result := runCLI(t, "sessions", "start", "--browser", "invalid-browser")
	requireFailure(t, result)

	// Verify the error message contains useful information
	// The API should return a validation error with details about valid browser types
	if !containsString(result.Stderr, "chromium") && !containsString(result.Stderr, "chrome") {
		t.Errorf("Error message should mention valid browser types, got: %s", result.Stderr)
	}

	// Verify we don't get the old "failed to read response body" error
	if containsString(result.Stderr, "failed to read response body") {
		t.Error("Error message should not contain 'failed to read response body'")
	}

	t.Logf("Got expected error message: %s", result.Stderr)
}

// TestErrorParsing_NonexistentSession tests that 404 errors show proper messages
func TestErrorParsing_NonexistentSession(t *testing.T) {
	// Try to get status of a non-existent session
	result := runCLI(t, "sessions", "status", "--id", "nonexistent-session-id-xyz789")
	requireFailure(t, result)

	// Verify we get a proper error (not "failed to read response body")
	if containsString(result.Stderr, "failed to read response body") {
		t.Error("Error message should not contain 'failed to read response body'")
	}

	// Should contain some indication of the error
	if result.Stderr == "" {
		t.Error("Expected error message in stderr")
	}

	t.Logf("Got expected error message: %s", result.Stderr)
}

// TestErrorParsing_NonexistentVault tests that vault not found errors show proper messages
func TestErrorParsing_NonexistentVault(t *testing.T) {
	// Try to get credentials from a non-existent vault
	result := runCLI(t, "vaults", "credentials", "list", "--id", "nonexistent-vault-id-abc123")
	requireFailure(t, result)

	// Verify we get a proper error (not "failed to read response body")
	if containsString(result.Stderr, "failed to read response body") {
		t.Error("Error message should not contain 'failed to read response body'")
	}

	t.Logf("Got expected error message: %s", result.Stderr)
}

// TestErrorParsing_NonexistentPersona tests that persona not found errors show proper messages
func TestErrorParsing_NonexistentPersona(t *testing.T) {
	// Try to get a non-existent persona
	result := runCLI(t, "personas", "show", "--id", "nonexistent-persona-id-def456")
	requireFailure(t, result)

	// Verify we get a proper error (not "failed to read response body")
	if containsString(result.Stderr, "failed to read response body") {
		t.Error("Error message should not contain 'failed to read response body'")
	}

	t.Logf("Got expected error message: %s", result.Stderr)
}

// TestErrorParsing_NonexistentProfile tests that profile not found errors show proper messages
func TestErrorParsing_NonexistentProfile(t *testing.T) {
	// Try to get a non-existent profile
	result := runCLI(t, "profiles", "show", "--id", "nonexistent-profile-id-ghi789")
	requireFailure(t, result)

	// Verify we get a proper error (not "failed to read response body")
	if containsString(result.Stderr, "failed to read response body") {
		t.Error("Error message should not contain 'failed to read response body'")
	}

	t.Logf("Got expected error message: %s", result.Stderr)
}

// TestErrorParsing_NonexistentAgent tests that agent not found errors show proper messages
func TestErrorParsing_NonexistentAgent(t *testing.T) {
	// Try to get status of a non-existent agent
	result := runCLI(t, "agents", "status", "--id", "nonexistent-agent-id-jkl012")
	requireFailure(t, result)

	// Verify we get a proper error (not "failed to read response body")
	if containsString(result.Stderr, "failed to read response body") {
		t.Error("Error message should not contain 'failed to read response body'")
	}

	t.Logf("Got expected error message: %s", result.Stderr)
}

// TestErrorParsing_InvalidSessionExecuteAction tests that invalid action JSON shows proper error
func TestErrorParsing_InvalidSessionExecuteAction(t *testing.T) {
	// Start a session first
	result := runCLI(t, "sessions", "start", "--headless")
	if result.ExitCode != 0 {
		t.Skipf("Could not start session for test: %s", result.Stderr)
	}

	// Parse session ID
	var startResp struct {
		SessionID string `json:"session_id"`
	}
	if err := parseJSON(result.Stdout, &startResp); err != nil {
		t.Fatalf("Failed to parse session start response: %v", err)
	}
	sessionID := startResp.SessionID
	defer cleanupSession(t, sessionID)

	// Try to execute an invalid action
	result = runCLI(t, "sessions", "execute", "--id", sessionID, "--action", `{"type": "invalid_action_type"}`)
	requireFailure(t, result)

	// Verify we get a proper error (not "failed to read response body")
	if containsString(result.Stderr, "failed to read response body") {
		t.Error("Error message should not contain 'failed to read response body'")
	}

	t.Logf("Got expected error message: %s", result.Stderr)
}

// TestErrorParsing_ValidationErrorContainsDetails tests that validation errors contain helpful details
func TestErrorParsing_ValidationErrorContainsDetails(t *testing.T) {
	// Try to start a session with an invalid browser type "brave"
	result := runCLI(t, "sessions", "start", "--browser", "brave")
	requireFailure(t, result)

	// The error should contain information about valid options
	stderr := result.Stderr

	// Check that error mentions at least one valid browser type
	validBrowsers := []string{"chromium", "chrome", "firefox"}
	foundValidBrowser := false
	for _, browser := range validBrowsers {
		if containsString(stderr, browser) {
			foundValidBrowser = true
			break
		}
	}

	if !foundValidBrowser {
		t.Errorf("Validation error should mention valid browser types, got: %s", stderr)
	}

	// Check that error mentions the invalid input
	if !containsString(stderr, "brave") {
		t.Errorf("Validation error should mention the invalid input 'brave', got: %s", stderr)
	}

	t.Logf("Got properly formatted validation error: %s", stderr)
}

// parseJSON is a helper to parse JSON from string
func parseJSON(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}
