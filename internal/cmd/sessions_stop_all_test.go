package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

func stopAllServer(t *testing.T, items string) *testutil.MockServer {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	setupSessionFileTest(t)
	server := testutil.NewMockServer()
	t.Cleanup(server.Close)
	env.SetEnv("NOTTE_API_URL", server.URL())
	server.AddResponse("/sessions", 200, `{"items": [`+items+`], "has_next": false}`)
	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })
	return server
}

const threeActive = `{"session_id": "s1", "status": "active"}, {"session_id": "s2", "status": "active"}, {"session_id": "s3", "status": "active"}`

func TestStopAllSessions(t *testing.T) {
	server := stopAllServer(t, threeActive)
	for _, id := range []string{"s1", "s2", "s3"} {
		server.AddResponse("/sessions/"+id+"/stop", 200, `{}`)
	}
	if err := setCurrentSession("s2"); err != nil {
		t.Fatal(err)
	}

	var prompt bytes.Buffer
	stdout, _ := testutil.CaptureOutput(func() {
		if err := stopAllSessions(context.Background(), strings.NewReader("y\n"), &prompt); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if prompt.String() != "Stop 3 active sessions? [y/N]: " {
		t.Errorf("prompt %q", prompt.String())
	}
	if stdout != "Stopped 3 sessions\nNext: notte sessions list --all\n" {
		t.Errorf("output %q", stdout)
	}
	for _, id := range []string{"s1", "s2", "s3"} {
		if len(server.Requests("/sessions/"+id+"/stop")) != 1 {
			t.Errorf("%s was not stopped", id)
		}
	}
	if current := GetCurrentSessionID(); current != "" {
		t.Errorf("the stopped pinned session is still current: %q", current)
	}
}

func TestStopAllSessions_DefaultIsNo(t *testing.T) {
	server := stopAllServer(t, threeActive)
	stdout, _ := testutil.CaptureOutput(func() {
		if err := stopAllSessions(context.Background(), strings.NewReader("\n"), &bytes.Buffer{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stdout != "Cancelled.\n" || len(server.Requests("/sessions/s1/stop")) != 0 {
		t.Errorf("enter alone must not stop anything: %q", stdout)
	}
}

func TestStopAllSessions_YesSkipsPrompt(t *testing.T) {
	server := stopAllServer(t, `{"session_id": "s1", "status": "active"}`)
	server.AddResponse("/sessions/s1/stop", 200, `{}`)
	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })
	var prompt bytes.Buffer
	stdout, _ := testutil.CaptureOutput(func() {
		if err := stopAllSessions(context.Background(), strings.NewReader(""), &prompt); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if prompt.Len() != 0 || stdout != "Stopped 1 session\nNext: notte sessions list --all\n" {
		t.Errorf("prompt %q, output %q", prompt.String(), stdout)
	}
}

func TestStopAllSessions_NoneActive(t *testing.T) {
	stopAllServer(t, "")
	stdout, _ := testutil.CaptureOutput(func() {
		if err := stopAllSessions(context.Background(), strings.NewReader(""), &bytes.Buffer{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stdout != "No active sessions (◡︵◡)\n" {
		t.Errorf("output %q", stdout)
	}
}

func TestStopAllSessions_ReportsFailures(t *testing.T) {
	server := stopAllServer(t, threeActive)
	server.AddResponse("/sessions/s1/stop", 200, `{}`)
	server.AddResponse("/sessions/s2/stop", 500, `{"detail": "boom"}`)
	server.AddResponse("/sessions/s3/stop", 200, `{}`)
	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })
	var err error
	stdout, _ := testutil.CaptureOutput(func() {
		err = stopAllSessions(context.Background(), strings.NewReader(""), &bytes.Buffer{})
	})
	if err == nil || err.Error() != "stopped 2 of 3 sessions" {
		t.Errorf("error %v", err)
	}
	if !strings.Contains(stdout, "Stopped 2 sessions") || !strings.Contains(stdout, "could not stop session s2") {
		t.Errorf("output %q", stdout)
	}
}

func TestSessionsStop_AllFlag(t *testing.T) {
	flag := sessionsStopCmd.Flags().ShorthandLookup("a")
	if flag == nil || flag.Name != "all" {
		t.Fatalf("-a on sessions stop is %v, want --all", flag)
	}
}

// The rate limit behind a partial failure must still reach the exit code.
func TestStopAllSessions_KeepsFailureCode(t *testing.T) {
	server := stopAllServer(t, `{"session_id": "s1", "status": "active"}`)
	server.AddResponse("/sessions/s1/stop", 429, `{"detail": "slow down"}`)
	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })
	var err error
	_, _ = testutil.CaptureOutput(func() {
		err = stopAllSessions(context.Background(), strings.NewReader(""), &bytes.Buffer{})
	})
	if code, exit := classifyError(err); code != "rate_limited" || exit != ExitRateLimited {
		t.Errorf("got (%q, %d) for %v, want rate_limited", code, exit, err)
	}
}

// Time spent answering the prompt must not count against the request timeout.
func TestStopAllSessions_TimeoutStartsAfterPrompt(t *testing.T) {
	server := stopAllServer(t, `{"session_id": "s1", "status": "active"}`)
	server.AddResponse("/sessions/s1/stop", 200, `{}`)
	origTimeout := requestTimeout
	requestTimeout = 1
	t.Cleanup(func() { requestTimeout = origTimeout })

	slow := io.MultiReader(&delayReader{delay: 1200 * time.Millisecond}, strings.NewReader("y\n"))
	var err error
	_, _ = testutil.CaptureOutput(func() { err = stopAllSessions(context.Background(), slow, &bytes.Buffer{}) })
	if err != nil {
		t.Fatalf("a slow answer made the stop fail: %v", err)
	}
}

type delayReader struct{ delay time.Duration }

func (d *delayReader) Read([]byte) (int, error) { time.Sleep(d.delay); return 0, io.EOF }

// Every page is followed, not just the first 50, and a runaway server is an
// error rather than a quiet truncation.
func TestActiveSessionIDs_FollowsEveryPage(t *testing.T) {
	const pages = 60
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var page int
		_, _ = fmt.Sscan(r.URL.Query().Get("page"), &page)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"items":[{"session_id":"s%d","status":"active"}],"has_next":%t}`, page, page < pages)
	}))
	defer server.Close()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	env.SetEnv("NOTTE_API_URL", server.URL)
	client, err := GetClient()
	if err != nil {
		t.Fatal(err)
	}
	ids, err := activeSessionIDs(context.Background(), client)
	if err != nil || len(ids) != pages {
		t.Fatalf("got %d ids, err %v; want %d", len(ids), err, pages)
	}
}
