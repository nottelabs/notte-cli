//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the real runner's redirect and disconnect behavior through CLI
// subprocesses. A quiet function takes longer than the 10-second HTTP timeout
// without launching a browser or touching an external website.
func TestFunctionRunPolling(t *testing.T) {
	t.Setenv("NOTTE_CONFIG_DIR", t.TempDir())
	const marker = "function-polling-integration-complete"
	path := filepath.Join(t.TempDir(), "quiet.py")
	code := "import time\n\ndef run() -> str:\n    time.sleep(20)\n    print('" + marker + "')\n    return '" + marker + "'\n"
	if err := os.WriteFile(path, []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}
	created := runCLI(t, "functions", "create", "--file", path, "--name", "cli-polling-integration")
	requireSuccess(t, created)
	var function struct {
		ID string `json:"function_id"`
	}
	if err := json.Unmarshal([]byte(created.Stdout), &function); err != nil || function.ID == "" {
		t.Fatalf("invalid create response: %s (%v)", created.Stdout, err)
	}
	defer cleanupFunction(t, function.ID)

	type runMetadata struct {
		FunctionID string `json:"function_id"`
		RunID      string `json:"function_run_id"`
		Status     string `json:"status"`
		Result     string `json:"result"`
	}
	decodeRun := func(t *testing.T, result CLIResult) runMetadata {
		t.Helper()
		requireSuccess(t, result)
		var run runMetadata
		if err := json.Unmarshal([]byte(result.Stdout), &run); err != nil {
			t.Fatalf("stdout must contain exactly one JSON document: %s (%v)", result.Stdout, err)
		}
		if run.FunctionID != function.ID || run.RunID == "" {
			t.Fatalf("missing or mismatched run identity: %+v", run)
		}
		return run
	}
	assertCompleted := func(t *testing.T, result CLIResult) runMetadata {
		t.Helper()
		run := decodeRun(t, result)
		if run.Status != "closed" || run.Result != marker {
			t.Fatalf("function did not complete successfully after disconnect: %+v", run)
		}
		if strings.Contains(result.Stderr, marker) {
			t.Fatalf("--no-stream printed function logs: %s", result.Stderr)
		}
		return run
	}

	t.Run("no-wait then resume after wait timeout", func(t *testing.T) {
		started := runCLI(t, "functions", "run", "--function-id", function.ID, "--no-wait")
		run := decodeRun(t, started)
		completed := false
		defer func() {
			if !completed {
				_ = runCLI(t, "functions", "run-stop", "--function-id", function.ID, "--run-id", run.RunID)
			}
		}()
		metadata := decodeRun(t, runCLI(t, "functions", "run-metadata", "--function-id", function.ID, "--run-id", run.RunID))
		if metadata.Status != "active" {
			t.Fatalf("--no-wait waited for completion: %+v", metadata)
		}
		timedOut := runCLI(t, "functions", "run-metadata", "--function-id", function.ID, "--run-id", run.RunID,
			"--wait", "--no-stream", "--wait-timeout", "1s")
		requireFailure(t, timedOut)
		if !strings.Contains(timedOut.Stderr, run.RunID) || !strings.Contains(timedOut.Stderr, "CLI did not stop") {
			t.Fatalf("timeout lost recovery instructions: %s", timedOut.Stderr)
		}
		result := runCLIWithTimeout(t, 2*time.Minute, "functions", "run-metadata", "--function-id", function.ID,
			"--run-id", run.RunID, "--wait", "--no-stream", "--timeout", "10", "--wait-timeout", "90s")
		final := assertCompleted(t, result)
		completed = true
		if final.RunID != run.RunID {
			t.Fatalf("resumed a different run: %s != %s", final.RunID, run.RunID)
		}
	})

	t.Run("no-stream outlasts request timeout", func(t *testing.T) {
		start := time.Now()
		result := runCLIWithTimeout(t, 2*time.Minute, "functions", "run", "--function-id", function.ID,
			"--no-stream", "--timeout", "10", "--wait-timeout", "90s")
		_ = assertCompleted(t, result)
		if elapsed := time.Since(start); elapsed < 10*time.Second {
			t.Fatalf("run did not exercise execution beyond the request timeout: %s", elapsed)
		}
	})

	// Starting and resuming must not create duplicate server runs.
	listed := runCLI(t, "functions", "runs", "--function-id", function.ID)
	requireSuccess(t, listed)
	var runs []runMetadata
	if err := json.Unmarshal([]byte(listed.Stdout), &runs); err != nil || len(runs) != 2 {
		t.Fatalf("expected exactly two runs across start/wait/resume commands: %s (%v)", listed.Stdout, err)
	}
}
