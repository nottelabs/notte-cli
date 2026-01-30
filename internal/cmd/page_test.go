package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

const pageSessionIDTest = "sess_page_123"

func setupPageTest(t *testing.T) *testutil.MockServer {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	t.Cleanup(func() { server.Close() })
	env.SetEnv("NOTTE_API_URL", server.URL())

	origID := sessionID
	sessionID = pageSessionIDTest
	t.Cleanup(func() { sessionID = origID })

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	return server
}

func pageExecResponse() string {
	return `{"action":{"type":"click"},"data":{},"message":"ok","session":{"session_id":"` + pageSessionIDTest + `","status":"ACTIVE"},"success":true}`
}

// Test parseSelector helper
func TestParseSelector(t *testing.T) {
	tests := []struct {
		input        string
		wantID       string
		wantSelector string
		wantErr      bool
	}{
		// Legacy @-prefix format (backwards compatibility)
		{"@B3", "B3", "", false},
		{"@submit-btn", "submit-btn", "", false},

		// New auto-detected ID format (letter + digits)
		{"B1", "B1", "", false},
		{"B3", "B3", "", false},
		{"I5", "I5", "", false},
		{"L10", "L10", "", false},
		{"F2", "F2", "", false},
		{"O3", "O3", "", false},
		{"M1", "M1", "", false},
		{"B999", "B999", "", false},

		// CSS selectors (not IDs)
		{"#btn", "", "#btn", false},
		{".class", "", ".class", false},
		{"button[type=submit]", "", "button[type=submit]", false},
		{"div > span", "", "div > span", false},

		// Edge cases that should be treated as selectors, not IDs
		{"b1", "", "b1", false},                 // lowercase - not an ID
		{"B", "", "B", false},                   // no digits - not an ID
		{"X1", "", "X1", false},                 // invalid letter - not an ID
		{"BB1", "", "BB1", false},               // two letters - not an ID
		{"B1a", "", "B1a", false},               // ends with letter - not an ID
		{"1B", "", "1B", false},                 // starts with digit - not an ID
		{"submit-btn", "", "submit-btn", false}, // contains hyphen - not an ID

		// Error cases
		{"@", "", "", true}, // @ with nothing after
		{"", "", "", true},  // empty string
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			id, selector, err := parseSelector(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSelector(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if id != tt.wantID {
				t.Errorf("parseSelector(%q) id = %q, want %q", tt.input, id, tt.wantID)
			}
			if selector != tt.wantSelector {
				t.Errorf("parseSelector(%q) selector = %q, want %q", tt.input, selector, tt.wantSelector)
			}
		})
	}
}

// Element Actions Tests

func TestRunPageClick_WithSelector(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageClick(cmd, []string{"#submit-btn"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageClick_WithID(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageClick(cmd, []string{"B3"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageClick_WithIDLegacyPrefix(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageClick(cmd, []string{"@B3"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageClick_WithFlags(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	origTimeout := pageClickTimeout
	origEnter := pageClickEnter
	pageClickTimeout = 5000
	pageClickEnter = true
	t.Cleanup(func() {
		pageClickTimeout = origTimeout
		pageClickEnter = origEnter
	})

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageClick(cmd, []string{"#btn"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageFill(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageFill(cmd, []string{"I1", "hello world"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageFill_WithFlags(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	origClear := pageFillClear
	origEnter := pageFillEnter
	pageFillClear = true
	pageFillEnter = true
	t.Cleanup(func() {
		pageFillClear = origClear
		pageFillEnter = origEnter
	})

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageFill(cmd, []string{"#email", "test@example.com"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageCheck(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageCheck(cmd, []string{"I2"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageSelect(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageSelect(cmd, []string{"#country", "USA"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageDownload(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageDownload(cmd, []string{"L5"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageUpload(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	origFile := pageUploadFile
	pageUploadFile = "/path/to/file.pdf"
	t.Cleanup(func() { pageUploadFile = origFile })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageUpload(cmd, []string{"#file-input"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

// Navigation Actions Tests

func TestRunPageGoto(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageGoto(cmd, []string{"https://example.com"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageNewTab(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageNewTab(cmd, []string{"https://example.com/new"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageBack(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageBack(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageForward(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageForward(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageReload(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageReload(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

// Scroll Actions Tests

func TestRunPageScrollDown(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageScrollDown(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageScrollDown_WithAmount(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageScrollDown(cmd, []string{"500"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageScrollDown_InvalidAmount(t *testing.T) {
	_ = setupPageTest(t)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runPageScrollDown(cmd, []string{"notanumber"})
	if err == nil {
		t.Fatal("expected error for invalid scroll amount")
	}
	if !strings.Contains(err.Error(), "invalid scroll amount") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPageScrollUp(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageScrollUp(cmd, []string{"200"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

// Keyboard Actions Tests

func TestRunPagePress(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPagePress(cmd, []string{"Enter"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

// Tab Management Tests

func TestRunPageSwitchTab(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageSwitchTab(cmd, []string{"1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageSwitchTab_InvalidIndex(t *testing.T) {
	_ = setupPageTest(t)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runPageSwitchTab(cmd, []string{"abc"})
	if err == nil {
		t.Fatal("expected error for invalid tab index")
	}
	if !strings.Contains(err.Error(), "invalid tab index") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPageCloseTab(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageCloseTab(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

// Wait/Utility Tests

func TestRunPageWait(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageWait(cmd, []string{"1000"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageWait_InvalidTime(t *testing.T) {
	_ = setupPageTest(t)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runPageWait(cmd, []string{"invalid"})
	if err == nil {
		t.Fatal("expected error for invalid time value")
	}
	if !strings.Contains(err.Error(), "invalid time value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Data Extraction Tests

// Page scrape tests removed - now covered by sessions_test.go since page scrape calls runSessionScrape

// Page Observe Tests

func pageObserveResponse() string {
	return `{"metadata":{"url":"https://example.com"},"screenshot":{},"session":{"session_id":"` + pageSessionIDTest + `","status":"ACTIVE"},"space":{"description":"A test page with example content","interaction_actions":[]}}`
}

func TestRunPageObserve(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/observe", 200, pageObserveResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionObserve(cmd, []string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageObserve_WithURL(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/observe", 200, pageObserveResponse())

	origURL := sessionObserveURL
	sessionObserveURL = "https://example.com"
	t.Cleanup(func() { sessionObserveURL = origURL })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runSessionObserve(cmd, []string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

// Other Actions Tests

func TestRunPageCaptchaSolve(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageCaptchaSolve(cmd, []string{"recaptcha_v2"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageComplete(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	origSuccess := pageCompleteSuccess
	pageCompleteSuccess = true
	t.Cleanup(func() { pageCompleteSuccess = origSuccess })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageComplete(cmd, []string{"Task completed successfully"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageFormFill(t *testing.T) {
	server := setupPageTest(t)
	server.AddResponse("/sessions/"+pageSessionIDTest+"/page/execute", 200, pageExecResponse())

	origData := pageFormFillData
	pageFormFillData = `{"name": "John", "email": "john@example.com"}`
	t.Cleanup(func() { pageFormFillData = origData })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPageFormFill(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPageFormFill_InvalidJSON(t *testing.T) {
	_ = setupPageTest(t)

	origData := pageFormFillData
	pageFormFillData = `{invalid json}`
	t.Cleanup(func() { pageFormFillData = origData })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runPageFormFill(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid JSON data") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test session ID requirement

func TestPageCommand_NoSessionID(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	env.SetEnv("NOTTE_SESSION_ID", "")

	server := testutil.NewMockServer()
	t.Cleanup(func() { server.Close() })
	env.SetEnv("NOTTE_API_URL", server.URL())

	// Use isolated config directory so no current_session file is found
	config.SetTestConfigDir(env.TempDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })

	// Clear sessionID
	origID := sessionID
	sessionID = ""
	t.Cleanup(func() { sessionID = origID })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runPageClick(cmd, []string{"#btn"})
	if err == nil {
		t.Fatal("expected error when no session ID available")
	}
	if !strings.Contains(err.Error(), "session ID required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
