package cmd

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/testutil"
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

	origTask := agentsStartTask
	origSession := agentsStartSession
	origVault := agentsStartVault
	origPersona := agentsStartPersona
	origMaxSteps := agentsStartMaxSteps
	origReasoning := agentsStartReasoningModel
	t.Cleanup(func() {
		agentsStartTask = origTask
		agentsStartSession = origSession
		agentsStartVault = origVault
		agentsStartPersona = origPersona
		agentsStartMaxSteps = origMaxSteps
		agentsStartReasoningModel = origReasoning
	})

	agentsStartTask = "do the thing"
	agentsStartSession = "sess_123"
	agentsStartVault = "vault_123"
	agentsStartPersona = "persona_123"
	agentsStartMaxSteps = 5
	agentsStartReasoningModel = "custom-model"

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

	origTask := agentsStartTask
	origSession := agentsStartSession
	origVault := agentsStartVault
	origPersona := agentsStartPersona
	origMaxSteps := agentsStartMaxSteps
	origReasoning := agentsStartReasoningModel
	t.Cleanup(func() {
		agentsStartTask = origTask
		agentsStartSession = origSession
		agentsStartVault = origVault
		agentsStartPersona = origPersona
		agentsStartMaxSteps = origMaxSteps
		agentsStartReasoningModel = origReasoning
	})

	agentsStartTask = "do the thing"
	agentsStartSession = ""
	agentsStartVault = ""
	agentsStartPersona = ""
	agentsStartMaxSteps = 30
	agentsStartReasoningModel = ""

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
