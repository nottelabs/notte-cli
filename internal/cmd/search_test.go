package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

const (
	searchResultsBody = `{"results":[` +
		`{"name":"Home Anthropic","url":"https://www.anthropic.com/","content":"AI safety company.","favicon":"https://x/y","type":"text"},` +
		`{"name":"Anthropic - Wikipedia","url":"https://en.wikipedia.org/wiki/Anthropic","content":"Anthropic is an American AI company.","favicon":"https://x/z","type":"text"}` +
		`]}`

	searchSourcedAnswerBody = `{"answer":"Anthropic is an AI safety company.","sources":[` +
		`{"name":"Home Anthropic","url":"https://www.anthropic.com/","snippet":"Anthropic is an AI safety company."},` +
		`{"name":"Wikipedia","url":"https://en.wikipedia.org/wiki/Anthropic","snippet":"American AI firm."}` +
		`]}`

	// Mirrors the wire format the live API returns: HTML entities like
	// &#x27; (apostrophe) and &amp; (ampersand) inside titles and snippets.
	searchEntitiesBody = `{"results":[` +
		`{"name":"Anthropic &amp; Claude","url":"https://example.com/a","content":"Anthropic&#x27;s mission is AI safety."}` +
		`]}`

	searchEntitiesAnswerBody = `{"answer":"Anthropic&#x27;s mission &amp; values.","sources":[` +
		`{"name":"Source &amp; co","url":"https://example.com/b","snippet":"It&#x27;s great."}` +
		`]}`

	searchEmptyResultsBody = `{"results":[]}`
)

// setupSearchTest configures an isolated env with a mock server, a test API key,
// and disables colors. It restores all globals it touches.
func setupSearchTest(t *testing.T) *testutil.MockServer {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	t.Cleanup(func() { server.Close() })
	env.SetEnv("NOTTE_API_URL", server.URL())

	origNoColor := noColor
	noColor = true
	t.Cleanup(func() { noColor = origNoColor })

	origDepth := searchDepth
	origOutputType := searchOutputType
	searchDepth = ""
	searchOutputType = ""
	t.Cleanup(func() {
		searchDepth = origDepth
		searchOutputType = origOutputType
	})

	return server
}

// withFormat temporarily overrides the global outputFormat for a test.
func withFormat(t *testing.T, format string) {
	t.Helper()
	orig := outputFormat
	outputFormat = format
	t.Cleanup(func() { outputFormat = orig })
}

func TestRunSearch_Results_TextMode(t *testing.T) {
	server := setupSearchTest(t)
	server.AddResponse("/search", 200, searchResultsBody)
	withFormat(t, "text")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runSearch(cmd, []string{"what", "is", "anthropic"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Search results for") {
		t.Errorf("missing results header, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"what is anthropic"`) {
		t.Errorf("expected query echoed back, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Home Anthropic") {
		t.Errorf("expected first result title, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "https://www.anthropic.com/") {
		t.Errorf("expected first result URL, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "AI safety company.") {
		t.Errorf("expected first result snippet, got:\n%s", stdout)
	}
}

func TestRunSearch_Results_EmptyList(t *testing.T) {
	server := setupSearchTest(t)
	server.AddResponse("/search", 200, searchEmptyResultsBody)
	withFormat(t, "text")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runSearch(cmd, []string{"obscure-query"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "No results") {
		t.Errorf("expected empty-results message, got:\n%s", stdout)
	}
}

func TestRunSearch_SourcedAnswer_TextMode(t *testing.T) {
	server := setupSearchTest(t)
	server.AddResponse("/search", 200, searchSourcedAnswerBody)
	withFormat(t, "text")

	searchOutputType = "sourcedAnswer"

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runSearch(cmd, []string{"what is anthropic"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Answer for") {
		t.Errorf("expected answer header, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Anthropic is an AI safety company.") {
		t.Errorf("expected answer body, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Sources:") {
		t.Errorf("expected sources header, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "https://en.wikipedia.org/wiki/Anthropic") {
		t.Errorf("expected source URL, got:\n%s", stdout)
	}
}

func TestRunSearch_JSONMode(t *testing.T) {
	server := setupSearchTest(t)
	server.AddResponse("/search", 200, searchResultsBody)
	withFormat(t, "json")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runSearch(cmd, []string{"anthropic"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", stdout, err)
	}
	results, ok := parsed["results"].([]any)
	if !ok {
		t.Fatalf("expected results array in JSON output, got %v", parsed)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestRunSearch_APIError(t *testing.T) {
	server := setupSearchTest(t)
	server.AddResponse("/search", 422, `{"detail":"validation error"}`)
	withFormat(t, "text")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runSearch(cmd, []string{"oops"})
	if err == nil {
		t.Fatal("expected error for non-2xx response, got nil")
	}
}

func TestRunSearch_NoAPIKey(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	auth.SetKeyring(env.MockStore)
	t.Cleanup(auth.ResetKeyring)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	if err := runSearch(cmd, []string{"anthropic"}); err == nil {
		t.Error("expected error when no API key is configured")
	}
}

func TestRunSearch_EmptyQueryAfterTrim(t *testing.T) {
	setupSearchTest(t)
	withFormat(t, "text")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runSearch(cmd, []string{"   "})
	if err == nil {
		t.Fatal("expected error for whitespace-only query")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error, got: %v", err)
	}
}

func TestSearchCmd_RequiresArgs(t *testing.T) {
	if err := searchCmd.Args(searchCmd, nil); err == nil {
		t.Error("expected error when called with no args")
	}
	if err := searchCmd.Args(searchCmd, []string{"query"}); err != nil {
		t.Errorf("expected no error for valid args, got: %v", err)
	}
}

func TestPrintSearchResponse_FallsBackForUnknownShape(t *testing.T) {
	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	body := []byte(`{"unexpected":"payload","other":42}`)
	stdout, _ := testutil.CaptureOutput(func() {
		if err := printSearchResponse(body, "q"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(stdout, "unexpected") || !strings.Contains(stdout, "payload") {
		t.Errorf("expected formatter fallback to print fields, got:\n%s", stdout)
	}
}

func TestPrintSearchResponse_NonJSONBody(t *testing.T) {
	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	body := []byte("not json at all")
	stdout, _ := testutil.CaptureOutput(func() {
		if err := printSearchResponse(body, "q"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(stdout, "not json at all") {
		t.Errorf("expected raw body in output, got:\n%s", stdout)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"short stays", "hello", 10, "hello"},
		{"trims whitespace", "  hello  ", 10, "hello"},
		{"collapses newlines", "hello\n\nworld", 20, "hello world"},
		{"truncates with ellipsis", "abcdefghij", 5, "abcde..."},
		{"decodes html entities", "Anthropic&#x27;s &amp; Claude", 50, "Anthropic's & Claude"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.in, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}

func TestRunSearch_DecodesHTMLEntitiesInTextMode(t *testing.T) {
	server := setupSearchTest(t)
	server.AddResponse("/search", 200, searchEntitiesBody)
	withFormat(t, "text")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runSearch(cmd, []string{"anthropic"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Contains(stdout, "&#x27;") || strings.Contains(stdout, "&amp;") {
		t.Errorf("expected HTML entities to be decoded in text output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Anthropic & Claude") {
		t.Errorf("expected decoded title 'Anthropic & Claude', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Anthropic's mission is AI safety.") {
		t.Errorf("expected decoded snippet, got:\n%s", stdout)
	}
}

func TestRunSearch_DecodesHTMLEntitiesInSourcedAnswer(t *testing.T) {
	server := setupSearchTest(t)
	server.AddResponse("/search", 200, searchEntitiesAnswerBody)
	withFormat(t, "text")
	searchOutputType = "sourcedAnswer"

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runSearch(cmd, []string{"anthropic"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Contains(stdout, "&#x27;") || strings.Contains(stdout, "&amp;") {
		t.Errorf("expected HTML entities to be decoded in answer/sources, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Anthropic's mission & values.") {
		t.Errorf("expected decoded answer, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Source & co") {
		t.Errorf("expected decoded source title, got:\n%s", stdout)
	}
}

func TestRunSearch_PreservesEntitiesInJSONMode(t *testing.T) {
	server := setupSearchTest(t)
	server.AddResponse("/search", 200, searchEntitiesBody)
	withFormat(t, "json")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runSearch(cmd, []string{"anthropic"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", stdout, err)
	}
	results, _ := parsed["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	first, _ := results[0].(map[string]any)
	if name, _ := first["name"].(string); name != "Anthropic &amp; Claude" {
		t.Errorf("JSON mode should preserve raw API entities, got %q", name)
	}
	if content, _ := first["content"].(string); content != "Anthropic&#x27;s mission is AI safety." {
		t.Errorf("JSON mode should preserve raw API entities, got %q", content)
	}
}

func TestStringField(t *testing.T) {
	m := map[string]any{
		"title":   "",
		"name":    "Hello",
		"url":     "https://example.com",
		"missing": 42, // wrong type, should be ignored
	}
	if got := stringField(m, "title", "name"); got != "Hello" {
		t.Errorf("expected fallback to 'name', got %q", got)
	}
	if got := stringField(m, "url"); got != "https://example.com" {
		t.Errorf("expected url, got %q", got)
	}
	if got := stringField(m, "missing", "absent"); got != "" {
		t.Errorf("expected empty for non-string/missing keys, got %q", got)
	}
}
