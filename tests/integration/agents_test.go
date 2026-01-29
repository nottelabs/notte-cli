//go:build integration

package integration

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAgentsList(t *testing.T) {
	// List agents - should work even if empty
	result := runCLI(t, "agents", "list")
	requireSuccess(t, result)
	t.Log("Successfully listed agents")
}

func TestAgentsStartWithSession(t *testing.T) {
	// First start a session
	result := runCLI(t, "sessions", "start", "--headless")
	requireSuccess(t, result)

	var sessionResp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &sessionResp); err != nil {
		t.Fatalf("Failed to parse session start response: %v", err)
	}
	sessionID := sessionResp.SessionID
	if sessionID == "" {
		t.Fatal("No session ID returned")
	}
	defer cleanupSession(t, sessionID)

	// Start an agent on the existing session
	result = runCLIWithTimeout(t, 120*time.Second, "agents", "start",
		"--task", "Report the current page URL",
		"--session", sessionID,
		"--max-steps", "3",
	)
	requireSuccess(t, result)

	var agentResp struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &agentResp); err != nil {
		t.Fatalf("Failed to parse agent start response: %v", err)
	}
	agentID := agentResp.AgentID
	if agentID == "" {
		t.Fatal("No agent ID returned")
	}
	t.Logf("Started agent on session: %s", agentID)

	defer cleanupAgent(t, agentID)

	// Get agent status
	result = runCLI(t, "agents", "status", "--id", agentID)
	requireSuccess(t, result)

	// Stop agent
	result = runCLI(t, "agents", "stop", "--id", agentID)
	t.Log("Agent on session test completed")
}

func TestAgentsStatusNonexistent(t *testing.T) {
	// Try to get status of a non-existent agent
	result := runCLI(t, "agents", "status", "--id", "nonexistent-agent-id-12345")
	requireFailure(t, result)
	t.Log("Correctly failed to get status of non-existent agent")
}

func TestAgentsStopNonexistent(t *testing.T) {
	// Try to stop a non-existent agent
	result := runCLI(t, "agents", "stop", "--id", "nonexistent-agent-id-12345")
	requireFailure(t, result)
	t.Log("Correctly failed to stop non-existent agent")
}
