//go:build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestJSONOutputValidity tests that commands produce valid JSON when using -o json.
// Commands that produce invalid JSON will be logged and the test will fail.
func TestJSONOutputValidity(t *testing.T) {
	// Start a session for commands that need one
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

	// Navigate to a page for commands that need page content
	result = runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--session-id", sessionID)
	requireSuccess(t, result)

	// Commands to test, grouped by requirements
	tests := []struct {
		name       string
		args       []string
		skipReason string // if non-empty, skip with this reason
	}{
		// List commands (no setup required)
		{"sessions list", []string{"sessions", "list"}, ""},
		{"agents list", []string{"agents", "list"}, ""},
		{"personas list", []string{"personas", "list"}, ""},
		{"profiles list", []string{"profiles", "list"}, ""},
		{"vaults list", []string{"vaults", "list"}, ""},
		{"functions list", []string{"functions", "list"}, ""},
		{"files list", []string{"files", "list"}, ""},

		// Session commands
		{"sessions status", []string{"sessions", "status", "--session-id", sessionID}, ""},
		{"sessions cookies", []string{"sessions", "cookies", "--session-id", sessionID}, ""},
		{"sessions network", []string{"sessions", "network", "--session-id", sessionID}, ""},
		{"sessions observe", []string{"sessions", "observe", "--session-id", sessionID}, ""},
		{"sessions scrape", []string{"sessions", "scrape", "--session-id", sessionID}, ""},
		{"sessions replay", []string{"sessions", "replay", "--session-id", sessionID}, ""},
		{"sessions offset", []string{"sessions", "offset", "--session-id", sessionID}, ""},
		{"sessions workflow-code", []string{"sessions", "workflow-code", "--session-id", sessionID}, ""},

		// Page commands
		{"page observe", []string{"page", "observe", "--session-id", sessionID}, ""},
		{"page scrape", []string{"page", "scrape", "--session-id", sessionID}, ""},
		{"page scroll-down", []string{"page", "scroll-down", "--session-id", sessionID}, ""},
		{"page scroll-up", []string{"page", "scroll-up", "--session-id", sessionID}, ""},
		{"page back", []string{"page", "back", "--session-id", sessionID}, ""},
		{"page forward", []string{"page", "forward", "--session-id", sessionID}, ""},
		{"page reload", []string{"page", "reload", "--session-id", sessionID}, ""},
		{"page eval-js", []string{"page", "eval-js", "document.title", "--session-id", sessionID}, ""},

		// Auth commands
		{"auth status", []string{"auth", "status"}, ""},

		// Fixed commands - now produce valid JSON
		{"clear", []string{"clear"}, ""},
		{"sessions code", []string{"sessions", "code", "--session-id", sessionID}, ""},
		{"sessions viewer", []string{"sessions", "viewer", "--session-id", sessionID}, ""},
		{"version", []string{"version"}, ""},
		{"page screenshot", []string{"page", "screenshot", "--session-id", sessionID}, ""},

		// files download - will fail but error should still be valid JSON
		{"files download", []string{"files", "download", "nonexistent.txt", "--session-id", sessionID}, ""},

		// Known broken commands - skip these (pass through to external tools)
		{"skill add", []string{"skill", "add"}, "passes through to npx, no JSON support"},
		{"skill remove", []string{"skill", "remove"}, "passes through to npx, no JSON support"},
	}

	var brokenCommands []string

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipReason != "" {
				t.Skipf("Skipping: %s", tc.skipReason)
				return
			}

			result := runCLIWithTimeout(t, 60*time.Second, tc.args...)

			// Check if stdout is valid JSON
			stdout := strings.TrimSpace(result.Stdout)
			stderr := strings.TrimSpace(result.Stderr)

			// If command failed, check stderr for valid JSON error
			if result.ExitCode != 0 {
				if stderr == "" {
					brokenCommands = append(brokenCommands, tc.name)
					t.Errorf("%s: command failed with no stderr output", tc.name)
					return
				}

				// Only parse the first line of stderr (ignore "exit status 1" etc.)
				stderrFirstLine := strings.Split(stderr, "\n")[0]

				// Try to parse stderr as JSON error
				var parsed any
				if err := json.Unmarshal([]byte(stderrFirstLine), &parsed); err != nil {
					brokenCommands = append(brokenCommands, tc.name)
					t.Errorf("%s: command failed with invalid JSON error: %v\nStderr: %s",
						tc.name, err, stderrFirstLine)
					return
				}

				t.Logf("%s: valid JSON error (command failed as expected)", tc.name)
				return
			}

			// Command succeeded - check stdout
			if stdout == "" {
				t.Logf("%s: empty output (acceptable)", tc.name)
				return
			}

			// Try to parse as JSON
			var parsed any
			if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
				brokenCommands = append(brokenCommands, tc.name)
				t.Errorf("%s: invalid JSON output: %v\nStdout: %s\nStderr: %s",
					tc.name, err, stdout, stderr)
				return
			}

			t.Logf("%s: valid JSON", tc.name)
		})
	}

	if len(brokenCommands) > 0 {
		t.Logf("\nCommands with broken JSON output: %v", brokenCommands)
	}
}

