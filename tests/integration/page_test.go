//go:build integration

package integration

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPageObserve(t *testing.T) {
	// Start a session first
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

	// Observe a page
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "observe", "--session-id", sessionID, "--url", "https://example.com")
	requireSuccess(t, result)

	// Verify we got some observation data
	if result.Stdout == "" {
		t.Error("Observe returned empty output")
	}
	t.Log("Successfully observed page")
}

func TestPageExecuteAction(t *testing.T) {
	// Start a session
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

	// First navigate to a page
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "observe", "--session-id", sessionID, "--url", "https://example.com")
	requireSuccess(t, result)

	// Execute a simple goto action
	actionJSON := `{"type":"goto","url":"https://example.org"}`
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "execute", "--session-id", sessionID, "--action", actionJSON)
	requireSuccess(t, result)
	t.Log("Successfully executed action")
}

func TestPageScrapeBasic(t *testing.T) {
	// Start a session
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

	// Navigate to a page
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "observe", "--session-id", sessionID, "--url", "https://example.com")
	requireSuccess(t, result)

	// Scrape the page
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "scrape", "--session-id", sessionID)
	requireSuccess(t, result)

	// Verify we got some content
	if result.Stdout == "" {
		t.Error("Scrape returned empty output")
	}
	t.Log("Successfully scraped page content")
}

func TestPageScrapeWithInstructions(t *testing.T) {
	// Start a session
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

	// Navigate to a page
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "observe", "--session-id", sessionID, "--url", "https://example.com")
	requireSuccess(t, result)

	// Scrape with specific instructions
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "scrape", "--session-id", sessionID, "--instructions", "Extract the main heading text")
	requireSuccess(t, result)
	t.Log("Successfully scraped with instructions")
}

func TestPageScrapeOnlyMainContent(t *testing.T) {
	// Start a session
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

	// Navigate to a page
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "observe", "--session-id", sessionID, "--url", "https://example.com")
	requireSuccess(t, result)

	// Scrape only main content
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "scrape", "--session-id", sessionID, "--only-main-content")
	requireSuccess(t, result)
	t.Log("Successfully scraped main content only")
}

func TestPageObserveExecuteScrapeFlow(t *testing.T) {
	// Start a session
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

	// Step 1: Observe initial page
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "observe", "--session-id", sessionID, "--url", "https://example.com")
	requireSuccess(t, result)
	t.Log("Step 1: Observed initial page")

	// Step 2: Execute navigation
	actionJSON := `{"type":"goto","url":"https://example.org"}`
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "execute", "--session-id", sessionID, "--action", actionJSON)
	requireSuccess(t, result)
	t.Log("Step 2: Executed navigation")

	// Step 3: Observe new page
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "observe", "--session-id", sessionID)
	requireSuccess(t, result)
	t.Log("Step 3: Observed new page")

	// Step 4: Scrape content
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "scrape", "--session-id", sessionID)
	requireSuccess(t, result)
	t.Log("Observe-Execute-Scrape flow completed successfully")
}
