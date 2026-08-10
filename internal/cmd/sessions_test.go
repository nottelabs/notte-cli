package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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

	if stdout == "" || !strings.Contains(stdout, "No active sessions.") {
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

	origHeadless := SessionStartHeadless
	origBrowser := SessionStartBrowserType
	origTimeout := SessionStartIdleTimeoutMinutes
	origProxies := sessionsStartProxy
	origSolve := SessionStartSolveCaptchas
	origVW := SessionStartViewportWidth
	origVH := SessionStartViewportHeight
	origUA := SessionStartUserAgent
	origCDP := SessionStartCdpUrl
	origFileStorage := SessionStartUseFileStorage
	t.Cleanup(func() {
		SessionStartHeadless = origHeadless
		SessionStartBrowserType = origBrowser
		SessionStartIdleTimeoutMinutes = origTimeout
		sessionsStartProxy = origProxies
		SessionStartSolveCaptchas = origSolve
		SessionStartViewportWidth = origVW
		SessionStartViewportHeight = origVH
		SessionStartUserAgent = origUA
		SessionStartCdpUrl = origCDP
		SessionStartUseFileStorage = origFileStorage
	})

	SessionStartHeadless = false
	SessionStartBrowserType = "chrome"
	SessionStartIdleTimeoutMinutes = 5
	sessionsStartProxy = true
	SessionStartSolveCaptchas = true
	SessionStartViewportWidth = 1280
	SessionStartViewportHeight = 720
	SessionStartUserAgent = "test-agent"
	SessionStartCdpUrl = "ws://cdp"
	SessionStartUseFileStorage = true

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&SessionStartHeadless, "headless", true, "")
	cmd.Flags().BoolVar(&sessionsStartProxy, "proxy", false, "")
	cmd.Flags().BoolVar(&SessionStartSolveCaptchas, "solve-captchas", false, "")
	cmd.Flags().BoolVar(&SessionStartUseFileStorage, "file-storage", false, "")
	_ = cmd.Flags().Set("headless", "false")
	_ = cmd.Flags().Set("proxy", "true")
	_ = cmd.Flags().Set("solve-captchas", "true")
	_ = cmd.Flags().Set("file-storage", "true")
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

	origHeadless := SessionStartHeadless
	origBrowser := SessionStartBrowserType
	origTimeout := SessionStartIdleTimeoutMinutes
	origProxies := sessionsStartProxy
	origSolve := SessionStartSolveCaptchas
	origVW := SessionStartViewportWidth
	origVH := SessionStartViewportHeight
	origUA := SessionStartUserAgent
	origCDP := SessionStartCdpUrl
	t.Cleanup(func() {
		SessionStartHeadless = origHeadless
		SessionStartBrowserType = origBrowser
		SessionStartIdleTimeoutMinutes = origTimeout
		sessionsStartProxy = origProxies
		SessionStartSolveCaptchas = origSolve
		SessionStartViewportWidth = origVW
		SessionStartViewportHeight = origVH
		SessionStartUserAgent = origUA
		SessionStartCdpUrl = origCDP
	})

	SessionStartHeadless = true
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
	cmd.Flags().BoolVar(&SessionStartHeadless, "headless", true, "")
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

	if !strings.Contains(stdout, "stopped") {
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

func TestRequireSessionID_RequiresExplicitID(t *testing.T) {
	origID := sessionID
	sessionID = ""
	t.Cleanup(func() { sessionID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_SESSION_ID", "env_session")

	err := RequireSessionID()
	if err == nil {
		t.Fatal("RequireSessionID() should reject implicit session IDs")
	}
	if !strings.Contains(err.Error(), "use --session-id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequireSessionID_WithExplicitID(t *testing.T) {
	origID := sessionID
	sessionID = "sess_explicit"
	t.Cleanup(func() { sessionID = origID })

	if err := RequireSessionID(); err != nil {
		t.Fatalf("RequireSessionID() error = %v", err)
	}
}
