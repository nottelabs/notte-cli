package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/output"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

const sessionIDTest = "sess_123"

func setupSessionTest(t *testing.T) *testutil.MockServer {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	t.Cleanup(func() { server.Close() })
	env.SetEnv("NOTTE_API_URL", server.URL())

	origID := sessionID
	sessionID = sessionIDTest
	t.Cleanup(func() { sessionID = origID })

	return server
}

func sessionJSON() string {
	return `{"session_id":"` + sessionIDTest + `","status":"ACTIVE","created_at":"2020-01-01T00:00:00Z","last_accessed_at":"2020-01-01T00:00:00Z","timeout_minutes":0}`
}

func TestRunSessionsList_Success(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/sessions", 200, `{"items": [{"session_id": "sess_123", "status": "ACTIVE"}]}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionsList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionsList_Empty(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/sessions", 200, `{"items": []}`)

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionsList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout != "No active sessions (◡︵◡)\nNext: notte sessions list --all\n" {
		t.Errorf("expected empty message, got %q", stdout)
	}
}

func TestRunSessionsStart(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/sessions/start", 200, `{"session_id":"sess_123","status":"ACTIVE","created_at":"2020-01-01T00:00:00Z","last_accessed_at":"2020-01-01T00:00:00Z","timeout_minutes":5}`)

	origBrowser := SessionStartBrowserType
	origTimeout := SessionStartIdleTimeoutMinutes
	origProxies := sessionsStartProxy
	origSolve := SessionStartSolveCaptchas
	origVW := SessionStartViewportWidth
	origVH := SessionStartViewportHeight
	origUA := SessionStartUserAgent
	origCDP := SessionStartCdpUrl
	t.Cleanup(func() {
		SessionStartBrowserType = origBrowser
		SessionStartIdleTimeoutMinutes = origTimeout
		sessionsStartProxy = origProxies
		SessionStartSolveCaptchas = origSolve
		SessionStartViewportWidth = origVW
		SessionStartViewportHeight = origVH
		SessionStartUserAgent = origUA
		SessionStartCdpUrl = origCDP
	})

	SessionStartBrowserType = "chrome"
	SessionStartIdleTimeoutMinutes = 5
	sessionsStartProxy = true
	SessionStartSolveCaptchas = true
	SessionStartViewportWidth = 1280
	SessionStartViewportHeight = 720
	SessionStartUserAgent = "test-agent"
	SessionStartCdpUrl = "ws://cdp"

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&sessionsStartProxy, "proxy", false, "")
	cmd.Flags().BoolVar(&SessionStartSolveCaptchas, "solve-captchas", false, "")
	_ = cmd.Flags().Set("proxy", "true")
	_ = cmd.Flags().Set("solve-captchas", "true")
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionsStart(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionsStart_Minimal(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/sessions/start", 200, `{"session_id":"sess_456","status":"ACTIVE","created_at":"2020-01-01T00:00:00Z","last_accessed_at":"2020-01-01T00:00:00Z","timeout_minutes":3}`)

	origBrowser := SessionStartBrowserType
	origTimeout := SessionStartIdleTimeoutMinutes
	origProxies := sessionsStartProxy
	origSolve := SessionStartSolveCaptchas
	origVW := SessionStartViewportWidth
	origVH := SessionStartViewportHeight
	origUA := SessionStartUserAgent
	origCDP := SessionStartCdpUrl
	t.Cleanup(func() {
		SessionStartBrowserType = origBrowser
		SessionStartIdleTimeoutMinutes = origTimeout
		sessionsStartProxy = origProxies
		SessionStartSolveCaptchas = origSolve
		SessionStartViewportWidth = origVW
		SessionStartViewportHeight = origVH
		SessionStartUserAgent = origUA
		SessionStartCdpUrl = origCDP
	})

	SessionStartBrowserType = ""
	SessionStartIdleTimeoutMinutes = 0
	sessionsStartProxy = false
	SessionStartSolveCaptchas = false
	SessionStartViewportWidth = 0
	SessionStartViewportHeight = 0
	SessionStartUserAgent = ""
	SessionStartCdpUrl = ""

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&sessionsStartProxy, "proxy", false, "")
	cmd.Flags().BoolVar(&SessionStartSolveCaptchas, "solve-captchas", false, "")
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionsStart(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionStatus(t *testing.T) {
	server := setupSessionTest(t)
	server.AddResponse("/sessions/"+sessionIDTest, 200, sessionJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionStatus(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionStop(t *testing.T) {
	server := setupSessionTest(t)
	server.AddResponse("/sessions/"+sessionIDTest+"/stop", 200, sessionJSON())

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionStop(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Stopped session ") {
		t.Errorf("expected stop message, got %q", stdout)
	}
}

func TestRunSessionStopCancelled(t *testing.T) {
	_ = setupSessionTest(t)

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

	// The answer comes through a pipe, as a front end relaying typed input does.
	t.Setenv("NOTTE_FORCE_INTERACTIVE", "1")
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origStdin
		_ = r.Close()
	})

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionStop(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Cancelled.") {
		t.Errorf("expected cancel message, got %q", stdout)
	}
}

func TestRunSessionObserve(t *testing.T) {
	server := setupSessionTest(t)
	observeResp := fmt.Sprintf(`{"metadata":{"tabs":[{"tab_id":1,"title":"Tab","url":"https://example.com"}],"title":"Tab","url":"https://example.com"},"screenshot":{"raw":"aGVsbG8="},"session":%s,"space":{"category":"page","description":"desc","interaction_actions":[]}}`, sessionJSON())
	server.AddResponse("/sessions/"+sessionIDTest+"/page/observe", 200, observeResp)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionObserve(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionExecute(t *testing.T) {
	server := setupSessionTest(t)
	execResp := fmt.Sprintf(`{"action":{"type":"noop"},"data":{},"message":"ok","session":%s,"success":true}`, sessionJSON())
	server.AddResponse("/sessions/"+sessionIDTest+"/page/execute", 200, execResp)

	origAction := sessionExecuteAction
	sessionExecuteAction = `{"type":"noop"}`
	t.Cleanup(func() { sessionExecuteAction = origAction })

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionExecute(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionExecute_InvalidJSON(t *testing.T) {
	_ = setupSessionTest(t)

	origAction := sessionExecuteAction
	sessionExecuteAction = "{"
	t.Cleanup(func() { sessionExecuteAction = origAction })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runSessionExecute(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid action JSON") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSessionScrape(t *testing.T) {
	server := setupSessionTest(t)
	scrapeResp := fmt.Sprintf(`{"markdown":"hi","structured":{"data":{"result":"hi"},"success":true},"session":%s}`, sessionJSON())
	server.AddResponse("/sessions/"+sessionIDTest+"/page/scrape", 200, scrapeResp)

	origInstructions := sessionScrapeInstructions
	origOnlyMain := sessionScrapeOnlyMain
	sessionScrapeInstructions = "extract"
	sessionScrapeOnlyMain = true
	t.Cleanup(func() {
		sessionScrapeInstructions = origInstructions
		sessionScrapeOnlyMain = origOnlyMain
	})

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionScrape(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionScrape_Defaults(t *testing.T) {
	server := setupSessionTest(t)
	scrapeResp := fmt.Sprintf(`{"markdown":"hi","structured":{},"session":%s}`, sessionJSON())
	server.AddResponse("/sessions/"+sessionIDTest+"/page/scrape", 200, scrapeResp)

	origInstructions := sessionScrapeInstructions
	origOnlyMain := sessionScrapeOnlyMain
	sessionScrapeInstructions = ""
	sessionScrapeOnlyMain = false
	t.Cleanup(func() {
		sessionScrapeInstructions = origInstructions
		sessionScrapeOnlyMain = origOnlyMain
	})

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionScrape(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionScrape_InstructionsOnly(t *testing.T) {
	server := setupSessionTest(t)
	scrapeResp := fmt.Sprintf(`{"markdown":"content","structured":{"data":{"title":"Extracted Title","count":2},"success":true},"session":%s}`, sessionJSON())
	server.AddResponse("/sessions/"+sessionIDTest+"/page/scrape", 200, scrapeResp)

	origInstructions := sessionScrapeInstructions
	origOnlyMain := sessionScrapeOnlyMain
	sessionScrapeInstructions = "extract data"
	sessionScrapeOnlyMain = false
	t.Cleanup(func() {
		sessionScrapeInstructions = origInstructions
		sessionScrapeOnlyMain = origOnlyMain
	})

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionScrape(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", stdout, err)
	}
	if _, ok := parsed["markdown"]; ok {
		t.Fatalf("expected structured data only, got markdown key in %v", parsed)
	}
	if parsed["title"] != "Extracted Title" {
		t.Fatalf("expected structured title, got %v", parsed)
	}
	if parsed["count"] != float64(2) {
		t.Fatalf("expected structured count, got %v", parsed)
	}
}

func TestRunSessionScrape_OnlyMainContentOnly(t *testing.T) {
	server := setupSessionTest(t)
	scrapeResp := fmt.Sprintf(`{"markdown":"main content only","structured":{},"session":%s}`, sessionJSON())
	server.AddResponse("/sessions/"+sessionIDTest+"/page/scrape", 200, scrapeResp)

	origInstructions := sessionScrapeInstructions
	origOnlyMain := sessionScrapeOnlyMain
	sessionScrapeInstructions = ""
	sessionScrapeOnlyMain = true
	t.Cleanup(func() {
		sessionScrapeInstructions = origInstructions
		sessionScrapeOnlyMain = origOnlyMain
	})

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionScrape(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", stdout, err)
	}
	if parsed["markdown"] != "main content only" {
		t.Fatalf("expected full scrape response with markdown, got %v", parsed)
	}
}

func TestRunSessionScrape_TextOutput(t *testing.T) {
	server := setupSessionTest(t)
	scrapeResp := fmt.Sprintf(`{"markdown":"# Test Content\n\nSome markdown text","structured":{},"session":%s}`, sessionJSON())
	server.AddResponse("/sessions/"+sessionIDTest+"/page/scrape", 200, scrapeResp)

	origInstructions := sessionScrapeInstructions
	origOnlyMain := sessionScrapeOnlyMain
	sessionScrapeInstructions = ""
	sessionScrapeOnlyMain = false
	t.Cleanup(func() {
		sessionScrapeInstructions = origInstructions
		sessionScrapeOnlyMain = origOnlyMain
	})

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionScrape(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Test Content") {
		t.Errorf("expected markdown content in output, got: %s", stdout)
	}
}

func TestRunSessionScrape_TextOutputWithInstructions(t *testing.T) {
	server := setupSessionTest(t)
	scrapeResp := fmt.Sprintf(`{"markdown":"# Test","structured":{"data":{"title":"Extracted Title","content":"Extracted Data"},"success":true},"session":%s}`, sessionJSON())
	server.AddResponse("/sessions/"+sessionIDTest+"/page/scrape", 200, scrapeResp)

	origInstructions := sessionScrapeInstructions
	origOnlyMain := sessionScrapeOnlyMain
	sessionScrapeInstructions = "extract structured data"
	sessionScrapeOnlyMain = false
	t.Cleanup(func() {
		sessionScrapeInstructions = origInstructions
		sessionScrapeOnlyMain = origOnlyMain
	})

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionScrape(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// When instructions are provided, structured data should be shown
	if !strings.Contains(stdout, "Extracted") {
		t.Errorf("expected structured data in output, got: %s", stdout)
	}
}

func TestRunSessionCookies(t *testing.T) {
	server := setupSessionTest(t)
	server.AddResponse("/sessions/"+sessionIDTest+"/cookies", 200, `{"cookies":[{"domain":"example.com","httpOnly":true,"name":"a","path":"/","value":"b"}]}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionCookies(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionSaveProfile(t *testing.T) {
	cases := []struct {
		name     string
		id       string
		newName  string
		wantBody string
	}{
		{name: "existing profile", id: "notte-profile-0123456789abcdef", wantBody: `{"profile_id":"notte-profile-0123456789abcdef"}`},
		{name: "new profile", newName: "github-login", wantBody: `{"name":"github-login"}`},
		{name: "start profile", wantBody: `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := setupSessionTest(t)
			path := "/sessions/" + sessionIDTest + "/profile/save"
			server.AddResponse(path, 200, `{"profile_id":"notte-profile-0123456789abcdef","name":"github-login","persisted_domains":["github.com"],"created_at":"2026-10-13T00:00:00Z","updated_at":"2026-10-13T00:00:00Z"}`)

			origID, origName := sessionSaveProfileID, sessionSaveProfileName
			sessionSaveProfileID, sessionSaveProfileName = tc.id, tc.newName
			t.Cleanup(func() { sessionSaveProfileID, sessionSaveProfileName = origID, origName })

			origFormat := outputFormat
			outputFormat = "json"
			t.Cleanup(func() { outputFormat = origFormat })

			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())

			stdout, _ := testutil.CaptureOutput(func() {
				if err := runSessionSaveProfile(cmd, nil); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			requests := server.Requests(path)
			if len(requests) != 1 {
				t.Fatalf("expected 1 request to %s, got %d", path, len(requests))
			}
			if requests[0].Method != "POST" {
				t.Errorf("expected POST, got %s", requests[0].Method)
			}
			if got := strings.TrimSpace(requests[0].Body); got != tc.wantBody {
				t.Errorf("request body = %s, want %s", got, tc.wantBody)
			}
			if !strings.Contains(stdout, "github.com") {
				t.Errorf("expected persisted domains in output, got %q", stdout)
			}
		})
	}
}

func TestRunSessionSaveProfileRejectsBothTargets(t *testing.T) {
	server := setupSessionTest(t)
	path := "/sessions/" + sessionIDTest + "/profile/save"

	origID, origName := sessionSaveProfileID, sessionSaveProfileName
	sessionSaveProfileID, sessionSaveProfileName = "notte-profile-0123456789abcdef", "github-login"
	t.Cleanup(func() { sessionSaveProfileID, sessionSaveProfileName = origID, origName })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	if err := runSessionSaveProfile(cmd, nil); err == nil {
		t.Fatal("expected an error when both --profile-id and --name are set")
	}
	if n := len(server.Requests(path)); n != 0 {
		t.Errorf("expected no request, got %d", n)
	}
}

func TestRunSessionSaveProfileRejectsEmptyTargets(t *testing.T) {
	for _, flag := range []string{"profile-id", "name"} {
		t.Run(flag, func(t *testing.T) {
			server := setupSessionTest(t)
			path := "/sessions/" + sessionIDTest + "/profile/save"

			origID, origName := sessionSaveProfileID, sessionSaveProfileName
			t.Cleanup(func() { sessionSaveProfileID, sessionSaveProfileName = origID, origName })

			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			cmd.Flags().StringVar(&sessionSaveProfileID, "profile-id", "", "")
			cmd.Flags().StringVar(&sessionSaveProfileName, "name", "", "")
			if err := cmd.Flags().Set(flag, " "); err != nil {
				t.Fatalf("set flag: %v", err)
			}

			if err := runSessionSaveProfile(cmd, nil); err == nil {
				t.Fatalf("expected an error for an empty --%s", flag)
			}
			if n := len(server.Requests(path)); n != 0 {
				t.Errorf("expected no request, got %d", n)
			}
		})
	}
}

func TestRunSessionCookiesSet(t *testing.T) {
	server := setupSessionTest(t)
	server.AddResponse("/sessions/"+sessionIDTest+"/cookies", 200, `{"message":"ok","success":true}`)

	tmpFile, err := os.CreateTemp("", "cookies-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString(`{"cookies":[{"domain":"example.com","httpOnly":true,"name":"a","path":"/","value":"b"}]}`); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })

	origFile := sessionCookiesSetFile
	sessionCookiesSetFile = tmpFile.Name()
	t.Cleanup(func() { sessionCookiesSetFile = origFile })

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionCookiesSet(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionCookiesSet_MissingFile(t *testing.T) {
	_ = setupSessionTest(t)

	origFile := sessionCookiesSetFile
	sessionCookiesSetFile = "missing-cookies.json"
	t.Cleanup(func() { sessionCookiesSetFile = origFile })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runSessionCookiesSet(cmd, nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "failed to read cookies file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSessionCookiesSet_InvalidJSON(t *testing.T) {
	_ = setupSessionTest(t)

	tmpFile, err := os.CreateTemp("", "cookies-invalid-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString("{"); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })

	origFile := sessionCookiesSetFile
	sessionCookiesSetFile = tmpFile.Name()
	t.Cleanup(func() { sessionCookiesSetFile = origFile })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err = runSessionCookiesSet(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse cookies JSON") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSessionDebug(t *testing.T) {
	server := setupSessionTest(t)
	server.AddResponse("/sessions/"+sessionIDTest+"/debug", 200, `{"debug_url":"http://debug","tabs":[{"debug_url":"http://debug/tab","ws_url":"ws://tab","metadata":{"tab_id":1,"title":"t","url":"u"}}],"ws":{"cdp":"ws://cdp","logs":"ws://logs","recording":"ws://rec"}}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionDebug(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionNetwork(t *testing.T) {
	server := setupSessionTest(t)
	server.AddResponse("/sessions/"+sessionIDTest+"/network/logs", 200, `{"requests":[],"responses":[],"session_id":"`+sessionIDTest+`","total_count":0}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionNetwork(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionReplay(t *testing.T) {
	server := setupSessionTest(t)
	// Mock the mp4 download endpoint
	server.AddResponseWithHeaders("/replay-video.mp4", 200, "fake-video-data", map[string]string{"Content-Type": "video/mp4"})
	// Return ReplayResponse JSON with mp4_url pointing to mock server
	replayJSON := fmt.Sprintf(`{"mp4_url":"%s/replay-video.mp4","expires_at":"2099-01-01T00:00:00Z"}`, server.URL())
	server.AddResponse("/sessions/"+sessionIDTest+"/replay", 200, replayJSON)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionReplay(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionOffset(t *testing.T) {
	server := setupSessionTest(t)
	server.AddResponse("/sessions/"+sessionIDTest+"/offset", 200, `{"offset":3}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionOffset(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionWorkflowCode(t *testing.T) {
	server := setupSessionTest(t)
	server.AddResponse("/sessions/"+sessionIDTest+"/workflow/code", 200, `{"json_actions":[{"type":"noop"}],"python_script":"print('hi')"}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionWorkflowCode(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

// Tests for session ID resolution (file-based tracking)

func setupSessionFileTest(t *testing.T) string {
	t.Helper()

	// Create a temporary config directory
	tmpDir := t.TempDir()
	config.SetTestConfigDir(tmpDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })

	return tmpDir
}

func TestGetCurrentSessionID_FromFlag(t *testing.T) {
	origID := sessionID
	sessionID = "flag_session"
	t.Cleanup(func() { sessionID = origID })

	got := GetCurrentSessionID()
	if got != "flag_session" {
		t.Errorf("GetCurrentSessionID() = %q, want %q", got, "flag_session")
	}
}

func TestGetCurrentSessionID_FromEnvVar(t *testing.T) {
	origID := sessionID
	sessionID = ""
	t.Cleanup(func() { sessionID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_SESSION_ID", "env_session")

	got := GetCurrentSessionID()
	if got != "env_session" {
		t.Errorf("GetCurrentSessionID() = %q, want %q", got, "env_session")
	}
}

func TestGetCurrentSessionID_FromFile(t *testing.T) {
	origID := sessionID
	sessionID = ""
	t.Cleanup(func() { sessionID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_SESSION_ID", "") // Ensure env var is empty

	// Create temp config dir
	tmpDir := setupSessionFileTest(t)

	// Write session file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	sessionFile := filepath.Join(configDir, config.CurrentSessionFile)
	if err := os.WriteFile(sessionFile, []byte("file_session"), 0o600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	got := GetCurrentSessionID()
	if got != "file_session" {
		t.Errorf("GetCurrentSessionID() = %q, want %q", got, "file_session")
	}
}

func TestGetCurrentSessionID_Priority(t *testing.T) {
	origID := sessionID
	t.Cleanup(func() { sessionID = origID })

	env := testutil.SetupTestEnv(t)
	tmpDir := setupSessionFileTest(t)

	// Create session file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	sessionFile := filepath.Join(configDir, config.CurrentSessionFile)
	if err := os.WriteFile(sessionFile, []byte("file_session"), 0o600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	// Test: flag > env > file
	sessionID = "flag_session"
	env.SetEnv("NOTTE_SESSION_ID", "env_session")

	got := GetCurrentSessionID()
	if got != "flag_session" {
		t.Errorf("flag should have highest priority: got %q, want %q", got, "flag_session")
	}

	// Test: env > file
	sessionID = ""
	got = GetCurrentSessionID()
	if got != "env_session" {
		t.Errorf("env should have priority over file: got %q, want %q", got, "env_session")
	}

	// Test: file as fallback
	env.SetEnv("NOTTE_SESSION_ID", "")
	got = GetCurrentSessionID()
	if got != "file_session" {
		t.Errorf("file should be fallback: got %q, want %q", got, "file_session")
	}
}

func TestSetCurrentSession(t *testing.T) {
	tmpDir := setupSessionFileTest(t)

	err := setCurrentSession("test_session_id")
	if err != nil {
		t.Fatalf("setCurrentSession() error = %v", err)
	}

	// Verify file was created
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	sessionFile := filepath.Join(configDir, config.CurrentSessionFile)

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("failed to read session file: %v", err)
	}

	if string(data) != "test_session_id" {
		t.Errorf("session file content = %q, want %q", string(data), "test_session_id")
	}
}

func TestClearCurrentSession(t *testing.T) {
	tmpDir := setupSessionFileTest(t)

	// First create a session file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	sessionFile := filepath.Join(configDir, config.CurrentSessionFile)
	if err := os.WriteFile(sessionFile, []byte("test_session"), 0o600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	// Clear it
	err := clearCurrentSession()
	if err != nil {
		t.Fatalf("clearCurrentSession() error = %v", err)
	}

	// Verify file was removed
	if _, err := os.Stat(sessionFile); !os.IsNotExist(err) {
		t.Error("session file should have been removed")
	}
}

func TestClearCurrentSession_NoFile(t *testing.T) {
	_ = setupSessionFileTest(t)

	// Should not error when file doesn't exist
	err := clearCurrentSession()
	if err != nil {
		t.Errorf("clearCurrentSession() should not error when file doesn't exist: %v", err)
	}
}

func TestRequireSessionID_NoSession(t *testing.T) {
	origID := sessionID
	sessionID = ""
	t.Cleanup(func() { sessionID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_SESSION_ID", "")
	_ = setupSessionFileTest(t)

	err := RequireSessionID()
	if err == nil {
		t.Fatal("RequireSessionID() should error when no session ID available")
	}

	expectedMsg := "session ID required"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("error message should contain %q, got %q", expectedMsg, err.Error())
	}
}

func TestRequireSessionID_FromFile(t *testing.T) {
	origID := sessionID
	sessionID = ""
	t.Cleanup(func() { sessionID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_SESSION_ID", "")
	tmpDir := setupSessionFileTest(t)

	// Create session file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	sessionFile := filepath.Join(configDir, config.CurrentSessionFile)
	if err := os.WriteFile(sessionFile, []byte("file_session"), 0o600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	err := RequireSessionID()
	if err != nil {
		t.Fatalf("RequireSessionID() error = %v", err)
	}

	if sessionID != "file_session" {
		t.Errorf("sessionID = %q, want %q", sessionID, "file_session")
	}
}

func TestSessionsStart_SetsCurrentSession(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupSessionFileTest(t)

	server.AddResponse("/sessions/start", 200, `{"session_id":"sess_new_123","status":"ACTIVE","created_at":"2020-01-01T00:00:00Z","last_accessed_at":"2020-01-01T00:00:00Z","timeout_minutes":5}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&sessionsStartProxy, "proxy", false, "")
	cmd.Flags().BoolVar(&SessionStartSolveCaptchas, "solve-captchas", false, "")
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		err := runSessionsStart(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify session was saved to file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	sessionFile := filepath.Join(configDir, config.CurrentSessionFile)

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("failed to read session file: %v", err)
	}

	if string(data) != "sess_new_123" {
		t.Errorf("session file content = %q, want %q", string(data), "sess_new_123")
	}
}

func TestSessionsStart_TextOutput(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	setupSessionFileTest(t)

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())
	server.AddResponse("/sessions/start", 200, `{"session_id":"sess_new_123","status":"active","browser_type":"chromium","headless":true,"proxies":true,"solve_captchas":true,"idle_timeout_minutes":3,"max_duration_minutes":15,"created_at":"2020-01-01T00:00:00Z","last_accessed_at":"2020-01-01T00:00:00Z","timeout_minutes":3,"user_agent":"Mozilla/5.0","viewer_url":"https://console.example.com/viewer?token=abc","cdp_url":"wss://cdp.example.com/s/token"}`)

	origFormat, origNoColor := outputFormat, noColor
	outputFormat, noColor = "text", true
	t.Cleanup(func() { outputFormat, noColor = origFormat, origNoColor })

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&sessionsStartProxy, "proxy", false, "")
	cmd.SetContext(context.Background())

	for _, tc := range []struct {
		force  string
		viewer string
	}{
		// A terminal without clickable links gets the whole URL.
		{"0", "https://console.example.com/viewer?token=abc"},
		{"1", "\x1b]8;;https://console.example.com/viewer?token=abc\x1b\\open live viewer ↗\x1b]8;;\x1b\\"},
	} {
		env.SetEnv("FORCE_HYPERLINK", tc.force)
		stdout, _ := testutil.CaptureOutput(func() {
			if err := runSessionsStart(cmd, nil); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
		// The spin-up time varies; only its presence and format are checked.
		stdout = spinUpPattern.ReplaceAllString(stdout, " in <time>\n")
		want := "Started session sess_new_123 in <time>\n" +
			"  Browser   chromium, headless · proxies · captcha solving\n" +
			"  Stops     after 3 min idle, 15 min max\n" +
			"  Viewer    " + tc.viewer + "\n"
		if stdout != want {
			t.Errorf("FORCE_HYPERLINK=%s:\n got: %q\nwant: %q", tc.force, stdout, want)
		}
	}
}

func TestSessionStop_ClearsCurrentSession(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupSessionFileTest(t)

	// Create session file first
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	sessionFile := filepath.Join(configDir, config.CurrentSessionFile)
	if err := os.WriteFile(sessionFile, []byte(sessionIDTest), 0o600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	server.AddResponse("/sessions/"+sessionIDTest+"/stop", 200, sessionJSON())

	origID := sessionID
	sessionID = sessionIDTest
	t.Cleanup(func() { sessionID = origID })

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		err := runSessionStop(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify session file was cleared
	if _, err := os.Stat(sessionFile); !os.IsNotExist(err) {
		t.Error("session file should have been removed after stop")
	}
}

func TestSessionStop_DifferentSession_DoesNotClearCurrentSession(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupSessionFileTest(t)

	// Create session file with "sess_current"
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	sessionFile := filepath.Join(configDir, config.CurrentSessionFile)
	if err := os.WriteFile(sessionFile, []byte("sess_current"), 0o600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	// Stop a different session "sess_different"
	server.AddResponse("/sessions/sess_different/stop", 200, `{"session_id":"sess_different","status":"STOPPED"}`)

	origID := sessionID
	sessionID = "sess_different"
	t.Cleanup(func() { sessionID = origID })

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		err := runSessionStop(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify session file still contains "sess_current"
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("session file should still exist: %v", err)
	}
	if strings.TrimSpace(string(data)) != "sess_current" {
		t.Errorf("session file content = %q, want %q", string(data), "sess_current")
	}
}

// Tests for session expiry helpers and auto-clear logic

func TestSetAndGetCurrentSessionExpiry(t *testing.T) {
	_ = setupSessionFileTest(t)

	expiry := time.Date(2025, 6, 15, 12, 30, 0, 0, time.UTC)
	if err := setCurrentSessionExpiry(expiry); err != nil {
		t.Fatalf("setCurrentSessionExpiry() error = %v", err)
	}

	got, err := getCurrentSessionExpiry()
	if err != nil {
		t.Fatalf("getCurrentSessionExpiry() error = %v", err)
	}

	if !got.Equal(expiry) {
		t.Errorf("getCurrentSessionExpiry() = %v, want %v", got, expiry)
	}
}

func TestGetCurrentSessionExpiry_MissingFile(t *testing.T) {
	_ = setupSessionFileTest(t)

	_, err := getCurrentSessionExpiry()
	if err == nil {
		t.Fatal("getCurrentSessionExpiry() should error when file doesn't exist")
	}
}

func TestClearCurrentSessionExpiry(t *testing.T) {
	tmpDir := setupSessionFileTest(t)

	// Create expiry file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	expiryFile := filepath.Join(configDir, config.CurrentSessionExpiryFile)
	if err := os.WriteFile(expiryFile, []byte("2025-06-15T12:30:00Z"), 0o600); err != nil {
		t.Fatalf("failed to write expiry file: %v", err)
	}

	if err := clearCurrentSessionExpiry(); err != nil {
		t.Fatalf("clearCurrentSessionExpiry() error = %v", err)
	}

	if _, err := os.Stat(expiryFile); !os.IsNotExist(err) {
		t.Error("expiry file should have been removed")
	}
}

func TestClearCurrentSessionExpiry_NoFile(t *testing.T) {
	_ = setupSessionFileTest(t)

	err := clearCurrentSessionExpiry()
	if err != nil {
		t.Errorf("clearCurrentSessionExpiry() should not error when file doesn't exist: %v", err)
	}
}

func TestSessionsStart_SavesExpiry(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupSessionFileTest(t)

	// Response with max_duration_minutes set
	server.AddResponse("/sessions/start", 200, `{"session_id":"sess_exp","status":"ACTIVE","created_at":"2025-06-15T12:00:00Z","last_accessed_at":"2025-06-15T12:00:00Z","timeout_minutes":5,"max_duration_minutes":30}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&sessionsStartProxy, "proxy", false, "")
	cmd.Flags().BoolVar(&SessionStartSolveCaptchas, "solve-captchas", false, "")
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		err := runSessionsStart(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify expiry was saved
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	expiryFile := filepath.Join(configDir, config.CurrentSessionExpiryFile)

	data, err := os.ReadFile(expiryFile)
	if err != nil {
		t.Fatalf("expiry file should exist: %v", err)
	}

	got, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("failed to parse expiry: %v", err)
	}

	// created_at (12:00) + 30 minutes = 12:30
	expected := time.Date(2025, 6, 15, 12, 30, 0, 0, time.UTC)
	if !got.Equal(expected) {
		t.Errorf("expiry = %v, want %v", got, expected)
	}
}

func TestSessionsStart_AutoClearsExpiredSession(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupSessionFileTest(t)

	// Set up an existing session with an expiry in the past
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, config.CurrentSessionFile), []byte("sess_old"), 0o600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}
	// Set expiry to the past
	pastExpiry := time.Now().UTC().Add(-10 * time.Minute)
	if err := os.WriteFile(filepath.Join(configDir, config.CurrentSessionExpiryFile), []byte(pastExpiry.Format(time.RFC3339)), 0o600); err != nil {
		t.Fatalf("failed to write expiry file: %v", err)
	}

	// The start endpoint should be called (no stop for the expired session)
	server.AddResponse("/sessions/start", 200, `{"session_id":"sess_new","status":"ACTIVE","created_at":"2025-06-15T12:00:00Z","last_accessed_at":"2025-06-15T12:00:00Z","timeout_minutes":5}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	// Reset sessionID flag so file-based resolution is used
	origID := sessionID
	sessionID = ""
	t.Cleanup(func() { sessionID = origID })
	env.SetEnv("NOTTE_SESSION_ID", "")

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&sessionsStartProxy, "proxy", false, "")
	cmd.Flags().BoolVar(&SessionStartSolveCaptchas, "solve-captchas", false, "")
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		err := runSessionsStart(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify old session was cleared and new one was saved
	data, err := os.ReadFile(filepath.Join(configDir, config.CurrentSessionFile))
	if err != nil {
		t.Fatalf("session file should exist: %v", err)
	}
	if string(data) != "sess_new" {
		t.Errorf("session file = %q, want %q", string(data), "sess_new")
	}

	// Old expiry file should have been cleared (no new one since no max_duration_minutes in response)
	if _, err := os.Stat(filepath.Join(configDir, config.CurrentSessionExpiryFile)); !os.IsNotExist(err) {
		t.Error("expiry file should have been cleared for expired session")
	}
}

func TestSessionsStart_DoesNotAutoClearNonExpiredSession(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupSessionFileTest(t)

	// Set up an existing session with an expiry in the future
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, config.CurrentSessionFile), []byte("sess_active"), 0o600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}
	futureExpiry := time.Now().UTC().Add(30 * time.Minute)
	if err := os.WriteFile(filepath.Join(configDir, config.CurrentSessionExpiryFile), []byte(futureExpiry.Format(time.RFC3339)), 0o600); err != nil {
		t.Fatalf("failed to write expiry file: %v", err)
	}

	// Reset sessionID flag so file-based resolution is used
	origID := sessionID
	sessionID = ""
	t.Cleanup(func() { sessionID = origID })
	env.SetEnv("NOTTE_SESSION_ID", "")

	// Set up stop endpoint (for the confirmation path) and start endpoint
	server.AddResponse("/sessions/sess_active/stop", 200, `{"session_id":"sess_active","status":"STOPPED"}`)
	server.AddResponse("/sessions/start", 200, `{"session_id":"sess_new","status":"ACTIVE","created_at":"2025-06-15T12:00:00Z","last_accessed_at":"2025-06-15T12:00:00Z","timeout_minutes":5}`)

	// Skip confirmation to auto-confirm the replace prompt
	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&sessionsStartProxy, "proxy", false, "")
	cmd.Flags().BoolVar(&SessionStartSolveCaptchas, "solve-captchas", false, "")
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		err := runSessionsStart(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify the new session was saved (confirmation path was followed, not auto-clear)
	data, err := os.ReadFile(filepath.Join(configDir, config.CurrentSessionFile))
	if err != nil {
		t.Fatalf("session file should exist: %v", err)
	}
	if string(data) != "sess_new" {
		t.Errorf("session file = %q, want %q", string(data), "sess_new")
	}
}

func TestSessionStatus_UsesCurrentSession(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupSessionFileTest(t)

	// Create session file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	sessionFile := filepath.Join(configDir, config.CurrentSessionFile)
	if err := os.WriteFile(sessionFile, []byte(sessionIDTest), 0o600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	server.AddResponse("/sessions/"+sessionIDTest, 200, sessionJSON())

	// Clear sessionID to test file-based resolution
	origID := sessionID
	sessionID = ""
	t.Cleanup(func() { sessionID = origID })

	// Clear env var too
	env.SetEnv("NOTTE_SESSION_ID", "")

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionStatus(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunSessionStatusAuth(t *testing.T) {
	server := setupSessionTest(t)
	server.AddResponse("/sessions/"+sessionIDTest+"/auth", 200, `{"session_id":"sess_123","status":"authenticating","operations":[]}`)
	oldAuth, oldFormat := sessionStatusAuth, outputFormat
	sessionStatusAuth, outputFormat = true, "json"
	t.Cleanup(func() { sessionStatusAuth, outputFormat = oldAuth, oldFormat })
	command := &cobra.Command{}
	command.SetContext(context.Background())
	stdout, _ := testutil.CaptureOutput(func() {
		if err := runSessionStatus(command, nil); err != nil {
			t.Fatal(err)
		}
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "authenticating" {
		t.Fatalf("unexpected readiness: %s", stdout)
	}
}

func TestSessionsStart_KeepsPreviousSessionRunning(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupSessionFileTest(t)

	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	sessionFile := filepath.Join(configDir, config.CurrentSessionFile)
	if err := os.WriteFile(sessionFile, []byte("sess_old_123"), 0o600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}

	server.AddResponse("/sessions/start", 200, `{"session_id":"sess_new_456","status":"ACTIVE","created_at":"2020-01-01T00:00:00Z","last_accessed_at":"2020-01-01T00:00:00Z","timeout_minutes":5}`)

	origID := sessionID
	sessionID = ""
	t.Cleanup(func() { sessionID = origID })

	// Neither a prompt nor --yes stops the previous session.
	for _, skip := range []bool{false, true} {
		SetSkipConfirmation(skip)
		t.Cleanup(func() { SetSkipConfirmation(false) })
		if err := os.WriteFile(sessionFile, []byte("sess_old_123"), 0o600); err != nil {
			t.Fatal(err)
		}

		origFormat := outputFormat
		outputFormat = "text"
		t.Cleanup(func() { outputFormat = origFormat })

		cmd := &cobra.Command{}
		cmd.Flags().BoolVar(&sessionsStartProxy, "proxy", false, "")
		cmd.SetContext(context.Background())

		stdout, _ := testutil.CaptureOutput(func() {
			if err := runSessionsStart(cmd, nil); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if requests := server.Requests("/sessions/sess_old_123/stop"); len(requests) != 0 {
			t.Fatalf("--yes=%v: the previous session was stopped", skip)
		}
		if strings.Contains(stdout, "[y/N]") {
			t.Errorf("--yes=%v: unexpected prompt: %q", skip, stdout)
		}
		if current := GetCurrentSessionID(); current != "sess_new_456" {
			t.Errorf("--yes=%v: current session is %q, want the new one", skip, current)
		}
	}
}

func TestPrintSessionList(t *testing.T) {
	t.Setenv("FORCE_HYPERLINK", "0")
	t.Setenv("NOTTE_CONSOLE_URL", "https://console.example.com")
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	at := func(s string) api.FlexibleTime {
		parsed, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return api.FlexibleTime{Time: parsed}
	}
	viewer := "https://console.example.com/static/viewer?s=1"
	sessions := []api.SessionResponse{
		{SessionId: "sess_1", Status: "active", CreatedAt: at("2026-01-15T11:58:00Z"), LastAccessedAt: at("2026-01-15T11:59:40Z"), ViewerUrl: &viewer},
		{SessionId: "sess_long_id_2", Status: "timed_out", CreatedAt: at("2026-01-12T09:00:00Z"), LastAccessedAt: at("2026-01-12T09:20:00Z")},
		{SessionId: "sess_3", Status: "closed", CreatedAt: at("2026-01-15T09:00:00Z"), LastAccessedAt: at("2026-01-15T09:30:00Z"), ViewerUrl: &viewer},
	}
	var buf bytes.Buffer
	text := &output.TextFormatter{Writer: &buf, NoColor: true}
	if err := printSessionList(text, sessions, false, "sess_1", now); err != nil {
		t.Fatal(err)
	}
	// Running sessions link to the live viewer, stopped ones to the console.
	want := "    BROWSER SESSION (3)   STATUS      LAST ACTIVE    CREATED        VIEWER\n" +
		"*   sess_1                active      just now       2 min ago      https://console.example.com/static/viewer?s=1\n" +
		"    sess_long_id_2        timed out   Jan 12 09:20   Jan 12 09:00   https://console.example.com/sessions/sess_long_id_2\n" +
		"    sess_3                closed      2 h ago        3 h ago        https://console.example.com/sessions/sess_3\n" +
		"Next: notte sessions status --session-id <id>\n"
	if buf.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestSessionsAliases(t *testing.T) {
	for _, alias := range []string{"browser", "browsers"} {
		found, _, err := rootCmd.Find([]string{alias, "list"})
		if err != nil || found != sessionsListCmd {
			t.Errorf("notte %s list resolved to %v (%v), want sessions list", alias, found.CommandPath(), err)
		}
	}
}

func TestRunSessionsList_FetchesViewerLinks(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	env.SetEnv("FORCE_HYPERLINK", "0")
	setupSessionFileTest(t)

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	// The list has no viewer links; each active session's status has one.
	server.AddResponse("/sessions", 200, `{"items": [
		{"session_id": "sess_a", "status": "active", "created_at": "2026-01-15T10:00:00Z", "last_accessed_at": "2026-01-15T10:00:00Z", "idle_timeout_minutes": 3},
		{"session_id": "sess_b", "status": "active", "created_at": "2026-01-15T10:00:00Z", "last_accessed_at": "2026-01-15T10:00:00Z", "idle_timeout_minutes": 3},
		{"session_id": "sess_c", "status": "closed", "created_at": "2026-01-15T10:00:00Z", "last_accessed_at": "2026-01-15T10:00:00Z", "idle_timeout_minutes": 3}], "has_next": true}`)
	server.AddResponse("/sessions/sess_a", 200, `{"session_id": "sess_a", "status": "active", "viewer_url": "https://console.example.com/viewer/a", "created_at": "2026-01-15T10:00:00Z", "last_accessed_at": "2026-01-15T10:00:00Z", "idle_timeout_minutes": 3}`)
	server.AddResponse("/sessions/sess_b", 200, `{"session_id": "sess_b", "status": "active", "viewer_url": "https://console.example.com/viewer/b", "created_at": "2026-01-15T10:00:00Z", "last_accessed_at": "2026-01-15T10:00:00Z", "idle_timeout_minutes": 3}`)

	origFormat, origNoColor := outputFormat, noColor
	outputFormat, noColor = "text", true
	t.Cleanup(func() { outputFormat, noColor = origFormat, origNoColor })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	stdout, _ := testutil.CaptureOutput(func() {
		if err := runSessionsList(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"BROWSER SESSION (3+)", "https://console.example.com/viewer/a", "https://console.example.com/viewer/b"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing viewer link %s in:\n%s", want, stdout)
		}
	}
	if requests := server.Requests("/sessions/sess_c"); len(requests) != 0 {
		t.Error("fetched the status of a closed session")
	}
}

func TestSessionsStart_Count(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	env.SetEnv("FORCE_HYPERLINK", "0")
	setupSessionFileTest(t)

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())
	server.AddResponse("/sessions/start", 200, `{"session_id":"sess_many","status":"active","created_at":"2020-01-01T00:00:00Z","last_accessed_at":"2020-01-01T00:00:00Z","idle_timeout_minutes":3}`)

	origFormat, origNoColor := outputFormat, noColor
	outputFormat, noColor = "text", true
	t.Cleanup(func() { outputFormat, noColor = origFormat, origNoColor })

	newCmd := func(count string) *cobra.Command {
		cmd := &cobra.Command{}
		cmd.Flags().BoolVar(&sessionsStartProxy, "proxy", false, "")
		cmd.Flags().IntVarP(&sessionsStartCount, "count", "n", 1, "")
		if err := cmd.Flags().Set("count", count); err != nil {
			t.Fatal(err)
		}
		cmd.SetContext(context.Background())
		return cmd
	}

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runSessionsStart(newCmd("3"), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if got := len(server.Requests("/sessions/start")); got != 3 {
		t.Errorf("sent %d start requests, want 3", got)
	}
	if !spinUpPattern.MatchString(strings.SplitN(stdout, "\n", 2)[0]+"\n") || !strings.HasPrefix(stdout, "Started 3 sessions in ") || strings.Count(stdout, "\n") != 5 || !strings.Contains(stdout, "BROWSER SESSION (3)") || !strings.Contains(stdout, "SPIN-UP") { // title, header, 3 rows
		t.Errorf("unexpected output:\n%s", stdout)
	}
	if current := GetCurrentSessionID(); current != "sess_many" {
		t.Errorf("current session is %q", current)
	}

	for _, bad := range []string{"0", "21"} {
		if err := runSessionsStart(newCmd(bad), nil); err == nil || !strings.Contains(err.Error(), "--count must be between 1 and 20") {
			t.Errorf("--count %s: got %v", bad, err)
		}
	}

	server.AddResponse("/sessions/start", 429, `{"detail":"too many concurrent sessions"}`)
	testutil.CaptureOutput(func() {
		if err := runSessionsStart(newCmd("2"), nil); err == nil {
			t.Error("expected an error when every start fails")
		}
	})
}

var spinUpPattern = regexp.MustCompile(` in (\d+ms|\d+\.\ds)\n`)

func TestFormatSpinUp(t *testing.T) {
	for d, want := range map[time.Duration]string{
		850 * time.Millisecond:  "850ms",
		1840 * time.Millisecond: "1.8s",
		12 * time.Second:        "12.0s",
	} {
		if got := formatSpinUp(d); got != want {
			t.Errorf("formatSpinUp(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestSessionsList_ShortLimitFlag(t *testing.T) {
	flag := sessionsListCmd.Flags().ShorthandLookup("n")
	if flag == nil || flag.Name != "page-size" {
		t.Fatalf("-n on sessions list is %v, want --page-size", flag)
	}
}
