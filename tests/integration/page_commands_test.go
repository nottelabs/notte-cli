//go:build integration

package integration

import (
	"encoding/json"
	"testing"
	"time"
)

// startTestSession is a helper that starts a session and returns its ID
func startTestSession(t *testing.T) string {
	t.Helper()
	result := runCLI(t, "sessions", "start", "--headless")
	requireSuccess(t, result)

	var startResp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &startResp); err != nil {
		t.Fatalf("Failed to parse session start response: %v", err)
	}

	// Wait for session to be ready
	time.Sleep(2 * time.Second)

	return startResp.SessionID
}

// TestPageGoto tests the page goto command
func TestPageGoto(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate using page goto
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	if result.Stdout == "" {
		t.Error("page goto returned empty output")
	}
	t.Log("Successfully executed page goto")
}

// TestPageClick tests the page click command with a CSS selector
func TestPageClick(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// First navigate to a page
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	// Click on a link (example.com has an "a" tag with "More information...")
	result = runCLIWithTimeout(t, 120*time.Second, "page", "click", "a", "--id", sessionID)
	requireSuccess(t, result)

	if result.Stdout == "" {
		t.Error("page click returned empty output")
	}
	t.Log("Successfully executed page click with selector")
}

// TestPageScrollDown tests the page scroll-down command
func TestPageScrollDown(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page first
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	// Scroll down without amount (default)
	result = runCLIWithTimeout(t, 120*time.Second, "page", "scroll-down", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page scroll-down (default)")

	// Scroll down with specific amount
	result = runCLIWithTimeout(t, 120*time.Second, "page", "scroll-down", "300", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page scroll-down with amount")
}

// TestPageScrollUp tests the page scroll-up command
func TestPageScrollUp(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page first
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	// Scroll down first
	result = runCLIWithTimeout(t, 120*time.Second, "page", "scroll-down", "500", "--id", sessionID)
	requireSuccess(t, result)

	// Then scroll up
	result = runCLIWithTimeout(t, 120*time.Second, "page", "scroll-up", "200", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page scroll-up")
}

// TestPageWait tests the page wait command
func TestPageWait(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page first
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	// Wait for 500ms
	result = runCLIWithTimeout(t, 120*time.Second, "page", "wait", "500", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page wait")
}

// TestPagePress tests the page press command
func TestPagePress(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page first
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	// Press Tab key
	result = runCLIWithTimeout(t, 120*time.Second, "page", "press", "Tab", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page press")
}

// TestPageReload tests the page reload command
func TestPageReload(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page first
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	// Reload the page
	result = runCLIWithTimeout(t, 120*time.Second, "page", "reload", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page reload")
}

// TestPageBackForward tests the page back and forward commands
func TestPageBackForward(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to first page
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	// Navigate to second page
	result = runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.org", "--id", sessionID)
	requireSuccess(t, result)

	// Go back
	result = runCLIWithTimeout(t, 120*time.Second, "page", "back", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page back")

	// Go forward
	result = runCLIWithTimeout(t, 120*time.Second, "page", "forward", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page forward")
}

// TestPageNewTab tests the page new-tab command
func TestPageNewTab(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to initial page
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	// Open new tab
	result = runCLIWithTimeout(t, 120*time.Second, "page", "new-tab", "https://example.org", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page new-tab")
}

// TestPageSwitchTab tests the page switch-tab command
func TestPageSwitchTab(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to initial page
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	// Open new tab
	result = runCLIWithTimeout(t, 120*time.Second, "page", "new-tab", "https://example.org", "--id", sessionID)
	requireSuccess(t, result)

	// Switch back to first tab (index 0)
	result = runCLIWithTimeout(t, 120*time.Second, "page", "switch-tab", "0", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page switch-tab")
}

// TestPageCloseTab tests the page close-tab command
func TestPageCloseTab(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to initial page
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	// Open new tab
	result = runCLIWithTimeout(t, 120*time.Second, "page", "new-tab", "https://example.org", "--id", sessionID)
	requireSuccess(t, result)

	// Close current tab (the new one)
	result = runCLIWithTimeout(t, 120*time.Second, "page", "close-tab", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page close-tab")
}

// TestPageScrape tests the page scrape command
func TestPageScrape(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page first
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	// Scrape with instructions
	result = runCLIWithTimeout(t, 120*time.Second, "page", "scrape", "Extract the page title", "--id", sessionID)
	requireSuccess(t, result)

	if result.Stdout == "" {
		t.Error("page scrape returned empty output")
	}
	t.Log("Successfully executed page scrape")
}

// TestPageScrapeMainOnly tests the page scrape only-main-content flag
func TestPageScrapeMainOnly(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page first
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)

	// Scrape with main-only flag
	result = runCLIWithTimeout(t, 120*time.Second, "page", "scrape", "Extract the main content", "only-main-content", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page scrape with only-main-content")
}

// TestPageCommandsWorkflow tests a complete workflow using page commands
func TestPageCommandsWorkflow(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Step 1: Navigate to a page
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Step 1: Navigated to example.com")

	// Step 2: Wait for page to stabilize
	result = runCLIWithTimeout(t, 120*time.Second, "page", "wait", "500", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Step 2: Waited 500ms")

	// Step 3: Scroll down
	result = runCLIWithTimeout(t, 120*time.Second, "page", "scroll-down", "200", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Step 3: Scrolled down")

	// Step 4: Scrape content
	result = runCLIWithTimeout(t, 120*time.Second, "page", "scrape", "Extract all text content", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Step 4: Scraped content")

	// Step 5: Navigate to another page
	result = runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.org", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Step 5: Navigated to example.org")

	// Step 6: Go back
	result = runCLIWithTimeout(t, 120*time.Second, "page", "back", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Step 6: Went back to example.com")

	// Step 7: Reload
	result = runCLIWithTimeout(t, 120*time.Second, "page", "reload", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Step 7: Reloaded page")

	t.Log("Page commands workflow completed successfully")
}

// TestPageFormFill tests the page form-fill command
func TestPageFormFill(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page with a form (using httpbin for testing)
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://httpbin.org/forms/post", "--id", sessionID)
	requireSuccess(t, result)

	// Fill form with JSON data using valid field names from API schema
	// Valid fields: first_name, last_name, email, phone, etc.
	formData := `{"first_name":"Test","last_name":"User","email":"test@example.com","phone":"555-1234"}`
	result = runCLIWithTimeout(t, 120*time.Second, "page", "form-fill", "--data", formData, "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page form-fill")
}

// TestPageUsesCurrentSession tests that page commands use the current session when --id is not specified
func TestPageUsesCurrentSession(t *testing.T) {
	// Start a session (this sets the current session)
	result := runCLI(t, "sessions", "start", "--headless")
	requireSuccess(t, result)

	var startResp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &startResp); err != nil {
		t.Fatalf("Failed to parse session start response: %v", err)
	}
	sessionID := startResp.SessionID
	defer cleanupSession(t, sessionID)

	// Wait for session to be ready
	time.Sleep(2 * time.Second)

	// Run page command WITHOUT --id flag (should use current session)
	result = runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com")
	requireSuccess(t, result)
	t.Log("Successfully used current session without --id flag")
}

// TestPageCommandErrors tests error handling for page commands
func TestPageCommandErrors(t *testing.T) {
	// Test without a session
	result := runCLI(t, "page", "goto", "https://example.com", "--id", "nonexistent-session-id")
	requireFailure(t, result)
	t.Log("Correctly failed with invalid session ID")
}

// TestPageFillCommand tests the page fill command
func TestPageFillCommand(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page with an input field
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://httpbin.org/forms/post", "--id", sessionID)
	requireSuccess(t, result)

	// Fill an input field using CSS selector
	result = runCLIWithTimeout(t, 120*time.Second, "page", "fill", "input[name=custname]", "Test User", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page fill with selector")
}

// TestPageFillWithFlags tests the page fill command with --clear and --enter flags
func TestPageFillWithFlags(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page with an input field
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://httpbin.org/forms/post", "--id", sessionID)
	requireSuccess(t, result)

	// Fill with --clear flag
	result = runCLIWithTimeout(t, 120*time.Second, "page", "fill", "input[name=custname]", "Cleared and filled", "--clear", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page fill with --clear flag")
}

// TestPageSelect tests the page select command
func TestPageSelect(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page with a select dropdown
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://httpbin.org/forms/post", "--id", sessionID)
	requireSuccess(t, result)

	// Select a dropdown option
	result = runCLIWithTimeout(t, 120*time.Second, "page", "select", "select[name=size]", "medium", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page select")
}

// TestPageCheck tests the page check command
func TestPageCheck(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page with checkboxes
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://httpbin.org/forms/post", "--id", sessionID)
	requireSuccess(t, result)

	// Check a checkbox
	result = runCLIWithTimeout(t, 120*time.Second, "page", "check", "input[name=topping][value=bacon]", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page check")
}

// TestPageCheckUncheck tests the page check command with --value=false
func TestPageCheckUncheck(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	// Navigate to a page with checkboxes
	result := runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://httpbin.org/forms/post", "--id", sessionID)
	requireSuccess(t, result)

	// First check
	result = runCLIWithTimeout(t, 120*time.Second, "page", "check", "input[name=topping][value=bacon]", "--value=true", "--id", sessionID)
	requireSuccess(t, result)

	// Then uncheck
	result = runCLIWithTimeout(t, 120*time.Second, "page", "check", "input[name=topping][value=bacon]", "--value=false", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully executed page check/uncheck")
}
