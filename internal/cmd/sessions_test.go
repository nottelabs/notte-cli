package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/config"
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

	origHeadless := sessionsStartHeadless
	origBrowser := sessionsStartBrowser
	origTimeout := sessionsStartIdleTimeout
	origProxies := sessionsStartProxies
	origSolve := sessionsStartSolveCaptchas
	origVW := sessionsStartViewportW
	origVH := sessionsStartViewportH
	origUA := sessionsStartUserAgent
	origCDP := sessionsStartCdpURL
	origFileStorage := sessionsStartFileStorage
	t.Cleanup(func() {
		sessionsStartHeadless = origHeadless
		sessionsStartBrowser = origBrowser
		sessionsStartIdleTimeout = origTimeout
		sessionsStartProxies = origProxies
		sessionsStartSolveCaptchas = origSolve
		sessionsStartViewportW = origVW
		sessionsStartViewportH = origVH
		sessionsStartUserAgent = origUA
		sessionsStartCdpURL = origCDP
		sessionsStartFileStorage = origFileStorage
	})

	sessionsStartHeadless = false
	sessionsStartBrowser = "firefox"
	sessionsStartIdleTimeout = 5
	sessionsStartProxies = true
	sessionsStartSolveCaptchas = true
	sessionsStartViewportW = 1280
	sessionsStartViewportH = 720
	sessionsStartUserAgent = "test-agent"
	sessionsStartCdpURL = "ws://cdp"
	sessionsStartFileStorage = true

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&sessionsStartHeadless, "headless", true, "")
	cmd.Flags().BoolVar(&sessionsStartProxies, "proxies", false, "")
	cmd.Flags().BoolVar(&sessionsStartSolveCaptchas, "solve-captchas", false, "")
	cmd.Flags().BoolVar(&sessionsStartFileStorage, "file-storage", false, "")
	_ = cmd.Flags().Set("headless", "false")
	_ = cmd.Flags().Set("proxies", "true")
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

	origHeadless := sessionsStartHeadless
	origBrowser := sessionsStartBrowser
	origTimeout := sessionsStartIdleTimeout
	origProxies := sessionsStartProxies
	origSolve := sessionsStartSolveCaptchas
	origVW := sessionsStartViewportW
	origVH := sessionsStartViewportH
	origUA := sessionsStartUserAgent
	origCDP := sessionsStartCdpURL
	t.Cleanup(func() {
		sessionsStartHeadless = origHeadless
		sessionsStartBrowser = origBrowser
		sessionsStartIdleTimeout = origTimeout
		sessionsStartProxies = origProxies
		sessionsStartSolveCaptchas = origSolve
		sessionsStartViewportW = origVW
		sessionsStartViewportH = origVH
		sessionsStartUserAgent = origUA
		sessionsStartCdpURL = origCDP
	})

	sessionsStartHeadless = true
	sessionsStartBrowser = ""
	sessionsStartIdleTimeout = 0
	sessionsStartProxies = false
	sessionsStartSolveCaptchas = false
	sessionsStartViewportW = 0
	sessionsStartViewportH = 0
	sessionsStartUserAgent = ""
	sessionsStartCdpURL = ""

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&sessionsStartHeadless, "headless", true, "")
	cmd.Flags().BoolVar(&sessionsStartProxies, "proxies", false, "")
	cmd.Flags().BoolVar(&sessionsStartSolveCaptchas, "solve-captchas", false, "")
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

	origURL := sessionObserveURL
	sessionObserveURL = "https://example.com"
	t.Cleanup(func() { sessionObserveURL = origURL })

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

func TestRunSessionObserve_NoURL(t *testing.T) {
	server := setupSessionTest(t)
	observeResp := fmt.Sprintf(`{"metadata":{"tabs":[{"tab_id":1,"title":"Tab","url":"https://example.com"}],"title":"Tab","url":"https://example.com"},"screenshot":{"raw":"aGVsbG8="},"session":%s,"space":{"category":"page","description":"desc","interaction_actions":[]}}`, sessionJSON())
	server.AddResponse("/sessions/"+sessionIDTest+"/page/observe", 200, observeResp)

	origURL := sessionObserveURL
	sessionObserveURL = ""
	t.Cleanup(func() { sessionObserveURL = origURL })

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
	scrapeResp := fmt.Sprintf(`{"markdown":"hi","structured":{},"session":%s}`, sessionJSON())
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
	server.AddResponse("/sessions/"+sessionIDTest+"/replay", 200, `replay-data`)

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
	cmd.Flags().BoolVar(&sessionsStartHeadless, "headless", true, "")
	cmd.Flags().BoolVar(&sessionsStartProxies, "proxies", false, "")
	cmd.Flags().BoolVar(&sessionsStartSolveCaptchas, "solve-captchas", false, "")
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
