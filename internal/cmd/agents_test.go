package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

const agentIDTest = "agent_123"

func setupAgentTest(t *testing.T) *testutil.MockServer {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	t.Cleanup(func() { server.Close() })
	env.SetEnv("NOTTE_API_URL", server.URL())

	origAgentID := agentID
	agentID = agentIDTest
	t.Cleanup(func() { agentID = origAgentID })

	return server
}

func agentStatusJSON() string {
	return `{"agent_id":"` + agentIDTest + `","session_id":"sess_1","status":"RUNNING","created_at":"2020-01-01T00:00:00Z","replay_start_offset":0,"replay_stop_offset":0}`
}

func TestRunAgentsList_Success(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/agents", 200, `{"items":[{"agent_id":"agent_1","session_id":"sess_1","status":"RUNNING","created_at":"2020-01-01T00:00:00Z"}]}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAgentsList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunAgentsList_Empty(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/agents", 200, `{"items":[]}`)

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAgentsList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "No running agents.") {
		t.Errorf("expected empty message, got %q", stdout)
	}
}

func TestRunAgentsStart_Success(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/agents/start", 200, `{"agent_id":"agent_1","session_id":"sess_1","status":"RUNNING","created_at":"2020-01-01T00:00:00Z"}`)

	origTask := AgentStartTask
	origSession := AgentStartSessionId
	origVault := AgentStartVaultId
	origPersona := AgentStartPersonaId
	origMaxSteps := AgentStartMaxSteps
	origReasoning := AgentStartReasoningModel
	t.Cleanup(func() {
		AgentStartTask = origTask
		AgentStartSessionId = origSession
		AgentStartVaultId = origVault
		AgentStartPersonaId = origPersona
		AgentStartMaxSteps = origMaxSteps
		AgentStartReasoningModel = origReasoning
	})

	AgentStartTask = "do the thing"
	AgentStartSessionId = "sess_123"
	AgentStartVaultId = "vault_123"
	AgentStartPersonaId = "persona_123"
	AgentStartMaxSteps = 5
	AgentStartReasoningModel = "custom-model"

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAgentsStart(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunAgentsStart_Minimal(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/agents/start", 200, `{"agent_id":"agent_2","session_id":"sess_2","status":"RUNNING","created_at":"2020-01-01T00:00:00Z"}`)

	origTask := AgentStartTask
	origSession := AgentStartSessionId
	origVault := AgentStartVaultId
	origPersona := AgentStartPersonaId
	origMaxSteps := AgentStartMaxSteps
	origReasoning := AgentStartReasoningModel
	t.Cleanup(func() {
		AgentStartTask = origTask
		AgentStartSessionId = origSession
		AgentStartVaultId = origVault
		AgentStartPersonaId = origPersona
		AgentStartMaxSteps = origMaxSteps
		AgentStartReasoningModel = origReasoning
	})

	AgentStartTask = "do the thing"
	AgentStartSessionId = ""
	AgentStartVaultId = ""
	AgentStartPersonaId = ""
	AgentStartMaxSteps = 30
	AgentStartReasoningModel = ""

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAgentsStart(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunAgentStatus(t *testing.T) {
	server := setupAgentTest(t)
	server.AddResponse("/agents/"+agentIDTest, 200, agentStatusJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAgentStatus(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunAgentStop(t *testing.T) {
	server := setupAgentTest(t)
	server.AddResponse("/agents/"+agentIDTest+"/stop", 200, agentStatusJSON())

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAgentStop(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "stopped") {
		t.Errorf("expected stop message, got %q", stdout)
	}
}

func TestRunAgentWorkflowCode(t *testing.T) {
	server := setupAgentTest(t)
	server.AddResponse("/agents/"+agentIDTest+"/workflow/code", 200, `{"json_actions":[{"type":"noop"}],"python_script":"print('hi')"}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAgentWorkflowCode(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunAgentReplay(t *testing.T) {
	server := setupAgentTest(t)
	server.AddResponse("/agents/"+agentIDTest+"/replay", 200, "replay-data")

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAgentReplay(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunAgentStopCancelled(t *testing.T) {
	_ = setupAgentTest(t)

	origSkip := skipConfirmation
	t.Cleanup(func() { skipConfirmation = origSkip })
	skipConfirmation = false

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	_, _ = w.WriteString("n\n")
	_ = w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origStdin
		_ = r.Close()
	})

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAgentStop(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Cancelled.") {
		t.Errorf("expected cancel message, got %q", stdout)
	}
}

// Tests for agent ID resolution (file-based tracking)

func setupAgentFileTest(t *testing.T) string {
	t.Helper()

	// Create a temporary config directory
	tmpDir := t.TempDir()
	config.SetTestConfigDir(tmpDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })

	return tmpDir
}

func TestGetCurrentAgentID_FromFlag(t *testing.T) {
	origID := agentID
	agentID = "flag_agent"
	t.Cleanup(func() { agentID = origID })

	got := GetCurrentAgentID()
	if got != "flag_agent" {
		t.Errorf("GetCurrentAgentID() = %q, want %q", got, "flag_agent")
	}
}

func TestGetCurrentAgentID_FromEnvVar(t *testing.T) {
	origID := agentID
	agentID = ""
	t.Cleanup(func() { agentID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_AGENT_ID", "env_agent")

	got := GetCurrentAgentID()
	if got != "env_agent" {
		t.Errorf("GetCurrentAgentID() = %q, want %q", got, "env_agent")
	}
}

func TestGetCurrentAgentID_FromFile(t *testing.T) {
	origID := agentID
	agentID = ""
	t.Cleanup(func() { agentID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_AGENT_ID", "") // Ensure env var is empty

	// Create temp config dir
	tmpDir := setupAgentFileTest(t)

	// Write agent file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	agentFile := filepath.Join(configDir, config.CurrentAgentFile)
	if err := os.WriteFile(agentFile, []byte("file_agent"), 0o600); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	got := GetCurrentAgentID()
	if got != "file_agent" {
		t.Errorf("GetCurrentAgentID() = %q, want %q", got, "file_agent")
	}
}

func TestGetCurrentAgentID_Priority(t *testing.T) {
	origID := agentID
	t.Cleanup(func() { agentID = origID })

	env := testutil.SetupTestEnv(t)
	tmpDir := setupAgentFileTest(t)

	// Create agent file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	agentFile := filepath.Join(configDir, config.CurrentAgentFile)
	if err := os.WriteFile(agentFile, []byte("file_agent"), 0o600); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	// Test: flag > env > file
	agentID = "flag_agent"
	env.SetEnv("NOTTE_AGENT_ID", "env_agent")

	got := GetCurrentAgentID()
	if got != "flag_agent" {
		t.Errorf("flag should have highest priority: got %q, want %q", got, "flag_agent")
	}

	// Test: env > file
	agentID = ""
	got = GetCurrentAgentID()
	if got != "env_agent" {
		t.Errorf("env should have priority over file: got %q, want %q", got, "env_agent")
	}

	// Test: file as fallback
	env.SetEnv("NOTTE_AGENT_ID", "")
	got = GetCurrentAgentID()
	if got != "file_agent" {
		t.Errorf("file should be fallback: got %q, want %q", got, "file_agent")
	}
}

func TestSetCurrentAgent(t *testing.T) {
	tmpDir := setupAgentFileTest(t)

	err := setCurrentAgent("test_agent_id")
	if err != nil {
		t.Fatalf("setCurrentAgent() error = %v", err)
	}

	// Verify file was created
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	agentFile := filepath.Join(configDir, config.CurrentAgentFile)

	data, err := os.ReadFile(agentFile)
	if err != nil {
		t.Fatalf("failed to read agent file: %v", err)
	}

	if string(data) != "test_agent_id" {
		t.Errorf("agent file content = %q, want %q", string(data), "test_agent_id")
	}
}

func TestClearCurrentAgent(t *testing.T) {
	tmpDir := setupAgentFileTest(t)

	// First create an agent file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	agentFile := filepath.Join(configDir, config.CurrentAgentFile)
	if err := os.WriteFile(agentFile, []byte("test_agent"), 0o600); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	// Clear it
	err := clearCurrentAgent()
	if err != nil {
		t.Fatalf("clearCurrentAgent() error = %v", err)
	}

	// Verify file was removed
	if _, err := os.Stat(agentFile); !os.IsNotExist(err) {
		t.Error("agent file should have been removed")
	}
}

func TestClearCurrentAgent_NoFile(t *testing.T) {
	_ = setupAgentFileTest(t)

	// Should not error when file doesn't exist
	err := clearCurrentAgent()
	if err != nil {
		t.Errorf("clearCurrentAgent() should not error when file doesn't exist: %v", err)
	}
}

func TestRequireAgentID_NoAgent(t *testing.T) {
	origID := agentID
	agentID = ""
	t.Cleanup(func() { agentID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_AGENT_ID", "")
	_ = setupAgentFileTest(t)

	err := RequireAgentID()
	if err == nil {
		t.Fatal("RequireAgentID() should error when no agent ID available")
	}

	expectedMsg := "agent ID required"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("error message should contain %q, got %q", expectedMsg, err.Error())
	}
}

func TestRequireAgentID_FromFile(t *testing.T) {
	origID := agentID
	agentID = ""
	t.Cleanup(func() { agentID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_AGENT_ID", "")
	tmpDir := setupAgentFileTest(t)

	// Create agent file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	agentFile := filepath.Join(configDir, config.CurrentAgentFile)
	if err := os.WriteFile(agentFile, []byte("file_agent"), 0o600); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	err := RequireAgentID()
	if err != nil {
		t.Fatalf("RequireAgentID() error = %v", err)
	}

	if agentID != "file_agent" {
		t.Errorf("agentID = %q, want %q", agentID, "file_agent")
	}
}

func TestAgentsStart_SetsCurrentAgent(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupAgentFileTest(t)

	server.AddResponse("/agents/start", 200, `{"agent_id":"agent_new_123","session_id":"sess_1","status":"RUNNING","created_at":"2020-01-01T00:00:00Z"}`)

	origTask := AgentStartTask
	origSession := AgentStartSessionId
	t.Cleanup(func() {
		AgentStartTask = origTask
		AgentStartSessionId = origSession
	})

	AgentStartTask = "do the thing"
	AgentStartSessionId = ""

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		err := runAgentsStart(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify agent was saved to file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	agentFile := filepath.Join(configDir, config.CurrentAgentFile)

	data, err := os.ReadFile(agentFile)
	if err != nil {
		t.Fatalf("failed to read agent file: %v", err)
	}

	if string(data) != "agent_new_123" {
		t.Errorf("agent file content = %q, want %q", string(data), "agent_new_123")
	}
}

func TestAgentsStart_UsesCurrentSession(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupAgentFileTest(t)

	// Create session file first
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	sessionFile := filepath.Join(configDir, config.CurrentSessionFile)
	if err := os.WriteFile(sessionFile, []byte("current_sess_123"), 0o600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	// Clear session ID flag to test auto-pickup
	origSessionID := sessionID
	sessionID = ""
	t.Cleanup(func() { sessionID = origSessionID })

	server.AddResponse("/agents/start", 200, `{"agent_id":"agent_with_session","session_id":"current_sess_123","status":"RUNNING","created_at":"2020-01-01T00:00:00Z"}`)

	origTask := AgentStartTask
	origSession := AgentStartSessionId
	t.Cleanup(func() {
		AgentStartTask = origTask
		AgentStartSessionId = origSession
	})

	AgentStartTask = "do the thing"
	AgentStartSessionId = "" // Empty to trigger auto-pickup

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAgentsStart(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify the response includes the session ID (indicating the session was used)
	if !strings.Contains(stdout, "current_sess_123") {
		t.Errorf("expected response to contain session ID, got: %s", stdout)
	}
}

func TestAgentStop_ClearsCurrentAgent(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupAgentFileTest(t)

	// Create agent file first
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	agentFile := filepath.Join(configDir, config.CurrentAgentFile)
	if err := os.WriteFile(agentFile, []byte(agentIDTest), 0o600); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	server.AddResponse("/agents/"+agentIDTest+"/stop", 200, agentStatusJSON())

	origID := agentID
	agentID = agentIDTest
	t.Cleanup(func() { agentID = origID })

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		err := runAgentStop(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify agent file was cleared
	if _, err := os.Stat(agentFile); !os.IsNotExist(err) {
		t.Error("agent file should have been removed after stop")
	}
}

func TestAgentStop_DifferentAgent_DoesNotClearCurrentAgent(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupAgentFileTest(t)

	// Create agent file with "agent_current"
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	agentFile := filepath.Join(configDir, config.CurrentAgentFile)
	if err := os.WriteFile(agentFile, []byte("agent_current"), 0o600); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	// Stop a different agent "agent_different"
	server.AddResponse("/agents/agent_different/stop", 200, `{"agent_id":"agent_different","session_id":"sess_1","status":"STOPPED","created_at":"2020-01-01T00:00:00Z","replay_start_offset":0,"replay_stop_offset":0}`)

	origID := agentID
	agentID = "agent_different"
	t.Cleanup(func() { agentID = origID })

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		err := runAgentStop(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify agent file still contains "agent_current"
	data, err := os.ReadFile(agentFile)
	if err != nil {
		t.Fatalf("agent file should still exist: %v", err)
	}
	if strings.TrimSpace(string(data)) != "agent_current" {
		t.Errorf("agent file content = %q, want %q", string(data), "agent_current")
	}
}

func TestAgentStatus_UsesCurrentAgent(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupAgentFileTest(t)

	// Create agent file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	agentFile := filepath.Join(configDir, config.CurrentAgentFile)
	if err := os.WriteFile(agentFile, []byte(agentIDTest), 0o600); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}

	server.AddResponse("/agents/"+agentIDTest, 200, agentStatusJSON())

	// Clear agentID to test file-based resolution
	origID := agentID
	agentID = ""
	t.Cleanup(func() { agentID = origID })

	// Clear env var too
	env.SetEnv("NOTTE_AGENT_ID", "")

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAgentStatus(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}
