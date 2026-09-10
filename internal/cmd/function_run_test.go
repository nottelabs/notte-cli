package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

func setupFunctionRunTest(t *testing.T) *cobra.Command {
	t.Helper()
	setupFunctionTest(t)
	origFormat, origTimeout := outputFormat, requestTimeout
	origNoWait, origNoStream := functionRunNoWait, functionRunNoStream
	origVars, origJSON := functionRunVariables, functionRunVariablesJSON
	outputFormat, requestTimeout = "json", 1
	functionRunNoWait, functionRunNoStream = false, true
	functionRunVariables, functionRunVariablesJSON = nil, `{"nested":{"ok":true}}`
	t.Cleanup(func() {
		outputFormat, requestTimeout = origFormat, origTimeout
		functionRunNoWait, functionRunNoStream = origNoWait, origNoStream
		functionRunVariables, functionRunVariablesJSON = origVars, origJSON
	})
	cmd := &cobra.Command{}
	cmd.Flags().Duration("wait-timeout", 0, "")
	cmd.Flags().Bool("no-stream", true, "")
	cmd.SetContext(context.Background())
	return cmd
}

func writeRunMetadata(w http.ResponseWriter, status string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"function_id":"fn_123","function_run_id":"run_123","status":%q,"logs":["first log"],"result":"{\"ok\":true}"}`, status)
}

// No response is kept open for the function's lifetime. The CLI must close the
// runner stream before polling, and a total duration exceeding --timeout must
// still succeed without ever starting a second run.
func TestFunctionRun_DetachesAndPollsBeyondRequestTimeout(t *testing.T) {
	cmd := setupFunctionRunTest(t)
	var starts, polls atomic.Int32
	disconnected := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/functions/fn_123/runs/start":
			starts.Add(1)
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["stream"] != true || body["workflow_id"] != functionIDTest {
				t.Errorf("invalid startup payload: %v", body)
			}
			if body["variables"].(map[string]any)["nested"].(map[string]any)["ok"] != true {
				t.Error("lost nested variables")
			}
			http.Redirect(w, r, "/runner?function_run_id=run_123", http.StatusTemporaryRedirect)
		case "/runner":
			if r.Method != http.MethodPost || r.Header.Get("x-notte-api-key") != "test-key" {
				t.Error("redirect lost method or authentication")
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprint(w, "data: {\"type\":\"log\",\"message\":\"started\"}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(disconnected)
		case "/functions/fn_123/runs/run_123":
			select {
			case <-disconnected:
			case <-time.After(time.Second):
				t.Error("runner stream still open during polling")
			}
			status := "active"
			if polls.Add(1) == 2 {
				status = "closed"
			}
			writeRunMetadata(w, status)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("NOTTE_API_URL", server.URL)
	var runErr error
	stdout, stderr := testutil.CaptureOutput(func() { runErr = runFunctionRun(cmd, nil) })
	if runErr != nil {
		t.Fatal(runErr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout must contain one JSON document: %s (%v)", stdout, err)
	}
	if result["status"] != "closed" || starts.Load() != 1 || polls.Load() != 2 {
		t.Fatalf("result=%v starts=%d polls=%d", result, starts.Load(), polls.Load())
	}
	if !strings.Contains(stderr, "run_123") || !strings.Contains(stderr, "run-metadata --wait") || strings.Contains(stderr, "first log") {
		t.Fatalf("missing run recovery hint or leaked --no-stream log: %s", stderr)
	}
}

func TestFunctionRun_NoWaitReturnsWithoutPolling(t *testing.T) {
	cmd := setupFunctionRunTest(t)
	functionRunNoWait = true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/functions/fn_123/runs/start":
			http.Redirect(w, r, "/runner?function_run_id=run_123", http.StatusTemporaryRedirect)
		case "/runner":
			w.Header().Set("Content-Type", "text/event-stream")
			// A quiet function sends no log events before completion. Startup
			// must return on HTTP acceptance, without waiting for a body byte.
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		default:
			t.Errorf("--no-wait must not poll: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("NOTTE_API_URL", server.URL)
	var runErr error
	stdout, _ := testutil.CaptureOutput(func() { runErr = runFunctionRun(cmd, nil) })
	if runErr != nil {
		t.Fatal(runErr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil || result["function_run_id"] != functionRunIDTest {
		t.Fatalf("missing --no-wait run ID: %s (%v)", stdout, err)
	}
}

func TestFunctionRun_StartupErrorRetainsRunID(t *testing.T) {
	for _, status := range []int{400, 429, 502} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			cmd := setupFunctionRunTest(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/functions/fn_123/runs/start" {
					http.Redirect(w, r, "/runner?function_run_id=run_123", http.StatusTemporaryRedirect)
					return
				}
				http.Error(w, "startup failed", status)
			}))
			defer server.Close()
			t.Setenv("NOTTE_API_URL", server.URL)
			var runErr error
			testutil.CaptureOutput(func() { runErr = runFunctionRun(cmd, nil) })
			if runErr == nil || !strings.Contains(runErr.Error(), "run_123") || !strings.Contains(runErr.Error(), "before starting another run") {
				t.Fatalf("missing startup recovery: %v", runErr)
			}
		})
	}
}

func TestFunctionRunWait_TimeoutDoesNotStopRun(t *testing.T) {
	cmd := setupFunctionRunTest(t)
	cmd.Flags().Bool("wait", true, "")
	if err := cmd.Flags().Set("wait-timeout", "30ms"); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/functions/fn_123/runs/run_123" {
			t.Errorf("wait must only read the existing run: %s %s", r.Method, r.URL.Path)
		}
		writeRunMetadata(w, "active")
	}))
	defer server.Close()
	t.Setenv("NOTTE_API_URL", server.URL)
	err := runFunctionRunMetadata(cmd, nil)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "CLI did not stop") || !strings.Contains(err.Error(), "run-metadata --wait --function-id fn_123 --run-id run_123") {
		t.Fatalf("missing wait recovery: %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d", requests.Load())
	}
}

func TestFunctionRunWait_TerminalAndInvalidResponses(t *testing.T) {
	for _, status := range []string{"closed", "failed", "unexpected"} {
		t.Run(status, func(t *testing.T) {
			setupFunctionRunTest(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeRunMetadata(w, status) }))
			defer server.Close()
			client, err := api.NewClientWithURL("test-key", server.URL, "test")
			if err != nil {
				t.Fatal(err)
			}
			var runErr error
			stdout, stderr := testutil.CaptureOutput(func() {
				runErr = waitFunction(context.Background(), client, functionIDTest, functionRunIDTest, false, time.Millisecond)
			})
			if status == "unexpected" {
				if runErr == nil || !strings.Contains(runErr.Error(), "unrecognized run status") {
					t.Fatalf("unexpected status accepted: %v", runErr)
				}
				return
			}
			if runErr != nil || !strings.Contains(stdout, status) || strings.Count(stderr, "first log") != 1 {
				t.Fatalf("stdout=%s stderr=%s err=%v", stdout, stderr, runErr)
			}
		})
	}
}

func TestGetClient_HonorsRequestTimeout(t *testing.T) {
	setupFunctionRunTest(t)
	requestTimeout = 1800
	client, err := GetClient()
	if err != nil {
		t.Fatal(err)
	}
	if client.HTTPClient().Timeout != 30*time.Minute {
		t.Fatalf("HTTP timeout = %s", client.HTTPClient().Timeout)
	}
}
