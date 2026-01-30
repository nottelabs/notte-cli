package cmd

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

func scrapeSessionJSON() string {
	return `{"session_id":"sess_123","status":"ACTIVE","created_at":"2020-01-01T00:00:00Z","last_accessed_at":"2020-01-01T00:00:00Z","timeout_minutes":0}`
}

func TestRunScrape(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/scrape", 200, `{"markdown":"hello","structured":{},"session":`+scrapeSessionJSON()+`}`)

	origInstructions := scrapeInstructions
	origOnlyMain := scrapeOnlyMain
	t.Cleanup(func() {
		scrapeInstructions = origInstructions
		scrapeOnlyMain = origOnlyMain
	})
	scrapeInstructions = "extract"
	scrapeOnlyMain = true

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runScrape(cmd, []string{"https://example.com"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunScrapeHtml(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/scrape_from_html", 200, `{"model_schema":{"success":true,"model_schema":{}},"scrape":{}}`)

	tmpFile, err := os.CreateTemp("", "page-*.html")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString("<html><body>hi</body></html>"); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })

	origFile := scrapeHtmlFile
	origInstructions := scrapeHtmlInstructions
	t.Cleanup(func() {
		scrapeHtmlFile = origFile
		scrapeHtmlInstructions = origInstructions
	})
	scrapeHtmlFile = tmpFile.Name()
	scrapeHtmlInstructions = "extract"

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runScrapeHtml(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.TrimSpace(stdout) == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunScrapeHtmlMissingFile(t *testing.T) {
	origFile := scrapeHtmlFile
	t.Cleanup(func() { scrapeHtmlFile = origFile })
	scrapeHtmlFile = "does-not-exist.html"

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runScrapeHtml(cmd, nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "failed to read HTML file") {
		t.Fatalf("unexpected error: %v", err)
	}
}
