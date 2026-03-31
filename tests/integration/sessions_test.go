//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionsLifecycle(t *testing.T) {
	// Start a new session
	result := runCLI(t, "sessions", "start", "--headless")
	requireSuccess(t, result)

	// Parse the response to get session ID
	var startResp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &startResp); err != nil {
		t.Fatalf("Failed to parse session start response: %v", err)
	}
	sessionID := startResp.SessionID
	if sessionID == "" {
		t.Fatal("No session ID returned from start command")
	}
	t.Logf("Started session: %s", sessionID)

	// Ensure cleanup
	defer cleanupSession(t, sessionID)

	// Get session status
	result = runCLI(t, "sessions", "status", "--session-id", sessionID)
	requireSuccess(t, result)
	if !containsString(result.Stdout, sessionID) {
		t.Error("Session status did not contain session ID")
	}

	// List sessions - should include our session
	result = runCLI(t, "sessions", "list")
	requireSuccess(t, result)
	if !containsString(result.Stdout, sessionID) {
		t.Error("Session list did not contain our session")
	}
	t.Log("Session lifecycle test completed successfully")
}

func TestSessionsStartWithOptions(t *testing.T) {
	// Start session with custom options
	result := runCLI(t, "sessions", "start",
		"--headless",
		"--browser-type", "chromium",
		"--idle-timeout-minutes", "5",
		"--max-duration-minutes", "10",
	)
	requireSuccess(t, result)

	var startResp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &startResp); err != nil {
		t.Fatalf("Failed to parse session start response: %v", err)
	}
	sessionID := startResp.SessionID
	if sessionID == "" {
		t.Fatal("No session ID returned from start command")
	}
	t.Logf("Started session with options: %s", sessionID)

	defer cleanupSession(t, sessionID)

	// Verify session is running
	result = runCLI(t, "sessions", "status", "--session-id", sessionID)
	requireSuccess(t, result)
	t.Log("Session with options test completed successfully")
}

func TestSessionsCookies(t *testing.T) {
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

	// Get cookies (should be empty or minimal initially)
	result = runCLI(t, "sessions", "cookies", "--session-id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully retrieved session cookies")
}

func TestSessionsObserve(t *testing.T) {
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

	// Wait a moment for the session to be fully ready
	time.Sleep(2 * time.Second)

	// Navigate to a URL
	result = runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--session-id", sessionID)
	requireSuccess(t, result)

	// Observe the page
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "observe", "--session-id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully observed page")
}

func TestSessionsScrape(t *testing.T) {
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

	// Wait a moment for the session to be fully ready
	time.Sleep(2 * time.Second)

	// First navigate to a page
	result = runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--session-id", sessionID)
	requireSuccess(t, result)

	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "observe", "--session-id", sessionID)
	requireSuccess(t, result)

	// Scrape the page content
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "scrape", "--session-id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully scraped page content")
}

func TestSessionsNetwork(t *testing.T) {
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

	// Wait a moment for the session to be fully ready
	time.Sleep(2 * time.Second)

	// Navigate to generate some network activity
	result = runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--session-id", sessionID)
	requireSuccess(t, result)

	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "observe", "--session-id", sessionID)
	requireSuccess(t, result)

	// Get network logs
	result = runCLI(t, "sessions", "network", "--session-id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully retrieved network logs")
}

func TestSessionsList(t *testing.T) {
	// List sessions - this should always work, even if empty
	result := runCLI(t, "sessions", "list")
	requireSuccess(t, result)
	t.Log("Successfully listed sessions")
}

func TestSessionsReplay(t *testing.T) {
	// Start a headless session
	result := runCLI(t, "sessions", "start", "--headless")
	requireSuccess(t, result)

	var startResp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &startResp); err != nil {
		t.Fatalf("Failed to parse session start response: %v", err)
	}
	sessionID := startResp.SessionID
	if sessionID == "" {
		t.Fatal("No session ID returned from start command")
	}
	t.Logf("Started session: %s", sessionID)

	// Safe cleanup — cleanupSession ignores errors if already stopped
	defer cleanupSession(t, sessionID)

	// Wait for session to be fully ready
	time.Sleep(2 * time.Second)

	// Navigate to a page to generate replay content
	result = runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--session-id", sessionID)
	requireSuccess(t, result)

	// Stop the session — replay is only available after stop
	result = runCLI(t, "sessions", "stop", "--session-id", sessionID)
	requireSuccess(t, result)
	t.Log("Session stopped, waiting for replay generation...")

	// Wait for replay to be generated
	time.Sleep(10 * time.Second)

	// Download the replay video
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "replay.mp4")
	result = runCLIWithTimeout(t, 120*time.Second, "sessions", "replay", "--session-id", sessionID, "--path", outputPath)
	requireSuccess(t, result)

	// Validate JSON response
	var replayResp struct {
		Success   bool   `json:"success"`
		SessionID string `json:"session_id"`
		Path      string `json:"path"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &replayResp); err != nil {
		t.Fatalf("Failed to parse replay response: %v", err)
	}

	if !replayResp.Success {
		t.Fatal("Replay response indicates failure")
	}
	if replayResp.SessionID != sessionID {
		t.Fatalf("Expected session_id '%s', got '%s'", sessionID, replayResp.SessionID)
	}
	if replayResp.Path == "" {
		t.Fatal("Replay path is empty")
	}

	// Verify the file was actually downloaded
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Replay file not found at %s: %v", outputPath, err)
	}
	if info.Size() == 0 {
		t.Fatal("Replay file is empty")
	}
	t.Logf("Replay video downloaded: %s (%d bytes)", outputPath, info.Size())
}

func TestSessionsStatusNonexistent(t *testing.T) {
	// Try to get status of a non-existent session
	result := runCLI(t, "sessions", "status", "--session-id", "nonexistent-session-id-12345")
	requireFailure(t, result)
	t.Log("Correctly failed to get status of non-existent session")
}
