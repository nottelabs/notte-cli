//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestMain verifies required environment variables are set before running tests
func TestMain(m *testing.M) {
	if os.Getenv("NOTTE_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "NOTTE_API_KEY is required for integration tests")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// CLIResult holds the output from a CLI command execution
type CLIResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// runCLI executes the notte CLI with the given arguments
func runCLI(t *testing.T, args ...string) CLIResult {
	t.Helper()
	return runCLIWithTimeout(t, 60*time.Second, args...)
}

// runCLIWithTimeout executes the notte CLI with a custom timeout
func runCLIWithTimeout(t *testing.T, timeout time.Duration, args ...string) CLIResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Add -o json flag for machine-readable output and --yes to skip confirmations
	fullArgs := append([]string{"-o", "json", "--yes"}, args...)

	cmd := exec.CommandContext(ctx, "go", append([]string{"run", "./cmd/notte"}, fullArgs...)...)
	cmd.Dir = getProjectRoot()

	// Set environment variables
	cmd.Env = append(os.Environ(),
		"NOTTE_API_KEY="+os.Getenv("NOTTE_API_KEY"),
	)
	if apiURL := os.Getenv("NOTTE_API_URL"); apiURL != "" {
		cmd.Env = append(cmd.Env, "NOTTE_API_URL="+apiURL)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			t.Logf("Command timed out after %v", timeout)
			exitCode = -1
		} else {
			exitCode = -1
		}
	}

	return CLIResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// getProjectRoot returns the path to the project root directory
func getProjectRoot() string {
	// When running tests, we're in tests/integration, so go up two levels
	return "../.."
}

// requireSuccess fails the test if the CLI command did not exit with code 0
func requireSuccess(t *testing.T, result CLIResult) {
	t.Helper()
	if result.ExitCode != 0 {
		t.Fatalf("CLI command failed with exit code %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
}

// requireFailure fails the test if the CLI command exited with code 0
func requireFailure(t *testing.T, result CLIResult) {
	t.Helper()
	if result.ExitCode == 0 {
		t.Fatalf("CLI command succeeded but expected failure\nstdout: %s\nstderr: %s",
			result.Stdout, result.Stderr)
	}
}

// containsString checks if the output contains a specific string
func containsString(output, substr string) bool {
	return strings.Contains(output, substr)
}

// cleanupSession stops a session, ignoring errors (for deferred cleanup)
func cleanupSession(t *testing.T, sessionID string) {
	t.Helper()
	if sessionID == "" {
		return
	}
	result := runCLI(t, "sessions", "stop", "--id", sessionID)
	if result.ExitCode != 0 {
		t.Logf("Warning: failed to cleanup session %s: %s", sessionID, result.Stderr)
	}
}

// cleanupAgent stops an agent, ignoring errors (for deferred cleanup)
func cleanupAgent(t *testing.T, agentID string) {
	t.Helper()
	if agentID == "" {
		return
	}
	result := runCLI(t, "agents", "stop", "--id", agentID)
	if result.ExitCode != 0 {
		t.Logf("Warning: failed to cleanup agent %s: %s", agentID, result.Stderr)
	}
}

// cleanupVault deletes a vault, ignoring errors (for deferred cleanup)
func cleanupVault(t *testing.T, vaultID string) {
	t.Helper()
	if vaultID == "" {
		return
	}
	result := runCLI(t, "vaults", "delete", "--id", vaultID)
	if result.ExitCode != 0 {
		t.Logf("Warning: failed to cleanup vault %s: %s", vaultID, result.Stderr)
	}
}

// cleanupPersona deletes a persona, ignoring errors (for deferred cleanup)
func cleanupPersona(t *testing.T, personaID string) {
	t.Helper()
	if personaID == "" {
		return
	}
	result := runCLI(t, "personas", "delete", "--id", personaID)
	if result.ExitCode != 0 {
		t.Logf("Warning: failed to cleanup persona %s: %s", personaID, result.Stderr)
	}
}

// cleanupProfile deletes a profile, ignoring errors (for deferred cleanup)
func cleanupProfile(t *testing.T, profileID string) {
	t.Helper()
	if profileID == "" {
		return
	}
	result := runCLI(t, "profiles", "delete", "--id", profileID)
	if result.ExitCode != 0 {
		t.Logf("Warning: failed to cleanup profile %s: %s", profileID, result.Stderr)
	}
}
