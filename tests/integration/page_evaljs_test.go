//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// runCLIText runs the CLI in its default (human) output mode. The shared
// helpers prepend `-o json`, which is exactly what these tests must not do:
// the point is what a shell sees on stdout without --output json.
func runCLIText(t *testing.T, args ...string) CLIResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", append([]string{"run", "./cmd/notte", "--yes"}, args...)...)
	cmd.Dir = getProjectRoot()
	cmd.Env = append(os.Environ(), "NOTTE_API_KEY="+os.Getenv("NOTTE_API_KEY"))
	if apiURL := os.Getenv("NOTTE_API_URL"); apiURL != "" {
		cmd.Env = append(cmd.Env, "NOTTE_API_URL="+apiURL)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return CLIResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: exitCode}
}

// evalJs runs `notte page eval-js` in text mode, the way a shell would.
func evalJs(t *testing.T, sessionID string, code string) CLIResult {
	t.Helper()
	return runCLIText(t, "page", "eval-js", code, "--session-id", sessionID)
}

// The evaluated value is the point of the command: it must land on stdout with
// nothing else, so `title=$(notte page eval-js "document.title")` captures the
// title alone and a pipe into jq needs no post-processing. The status line goes
// to stderr.
func TestPageEvalJsPrintsOnlyTheValue(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	requireSuccess(t, runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--session-id", sessionID))

	result := evalJs(t, sessionID, "document.title")
	requireSuccess(t, result)

	// what `title=$(...)` would capture
	title := strings.TrimSpace(result.Stdout)
	if title != "Example Domain" {
		t.Fatalf("expected the title alone on stdout, got %q (stderr: %q)", result.Stdout, result.Stderr)
	}
	if strings.Contains(result.Stdout, "Result:") || strings.Contains(result.Stdout, "Successfully executed") {
		t.Errorf("stdout must carry the value only, got %q", result.Stdout)
	}
}

// `notte page eval-js "JSON.stringify(...)" | jq length` — stdout has to be a
// standalone JSON document for the pipe to work.
func TestPageEvalJsStdoutIsPipeableJSON(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	requireSuccess(t, runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--session-id", sessionID))

	result := evalJs(t, sessionID, "JSON.stringify([...document.links].map(a => a.href))")
	requireSuccess(t, result)

	var links []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &links); err != nil {
		t.Fatalf("stdout is not parseable JSON (%v): %q", err, result.Stdout)
	}
	if len(links) == 0 {
		t.Errorf("expected at least one link on example.com, got %v", links)
	}
}

// A JS null is a successful evaluation and must arrive as the string "null",
// not as an empty stdout that a caller would read as "no result".
func TestPageEvalJsNullPrintsNull(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	requireSuccess(t, runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--session-id", sessionID))

	result := evalJs(t, sessionID, "null")
	requireSuccess(t, result)

	if got := strings.TrimSpace(result.Stdout); got != "null" {
		t.Fatalf("expected \"null\" on stdout, got %q", got)
	}
}

// -o json still emits the whole execution result, so machine consumers that
// want the envelope keep working.
func TestPageEvalJsJSONOutputKeepsTheEnvelope(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	requireSuccess(t, runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--session-id", sessionID))

	result := runCLIWithTimeout(t, 120*time.Second, "page", "eval-js", "document.title", "--session-id", sessionID)
	requireSuccess(t, result)

	var envelope struct {
		Success bool `json:"success"`
		Data    *struct {
			Markdown string `json:"markdown"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &envelope); err != nil {
		t.Fatalf("expected a JSON envelope on stdout (%v): %q", err, result.Stdout)
	}
	if !envelope.Success {
		t.Errorf("expected success=true, got %+v", envelope)
	}
	if envelope.Data == nil || envelope.Data.Markdown != "Example Domain" {
		t.Errorf("expected the title in data.markdown, got %+v", envelope.Data)
	}
}

// A failing script must exit non-zero and name the actual JavaScript error,
// rather than the generic user-facing sentence the API serializes by default.
func TestPageEvalJsFailureReportsTheJavaScriptError(t *testing.T) {
	sessionID := startTestSession(t)
	defer cleanupSession(t, sessionID)

	requireSuccess(t, runCLIWithTimeout(t, 120*time.Second, "page", "goto", "https://example.com", "--session-id", sessionID))

	result := evalJs(t, sessionID, "notAFunction()")
	requireFailure(t, result)

	combined := result.Stdout + result.Stderr
	if !containsString(combined, "notAFunction") {
		t.Errorf("expected the JavaScript error in the output, got stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
	if containsString(combined, "Sorry, this action cannot be executed") {
		t.Errorf("generic user message leaked instead of the real reason: %q", combined)
	}
}
