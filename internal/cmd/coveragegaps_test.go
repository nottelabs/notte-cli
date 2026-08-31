package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

func setupCoverageTest(t *testing.T) *testutil.MockServer {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	t.Cleanup(func() { server.Close() })
	env.SetEnv("NOTTE_API_URL", server.URL())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	return server
}

func TestPersonaUpdate_SendsTheNewName(t *testing.T) {
	server := setupCoverageTest(t)
	server.AddResponse("/personas/"+"p_1", 200, `{"persona_id":"p_1","status":"active"}`)

	origID := personaID
	personaID = "p_1"
	t.Cleanup(func() { personaID = origID; PersonaUpdateName = "" })

	cmd := &cobra.Command{}
	RegisterPersonaUpdateFlags(cmd)
	cmd.SetContext(context.Background())
	if err := cmd.Flags().Set("name", "checkout tester"); err != nil {
		t.Fatalf("setting --name: %v", err)
	}

	testutil.CaptureOutput(func() {
		if err := runPersonaUpdate(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	requests := server.Requests("/personas/p_1")
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(requests))
	}
	if requests[0].Method != "PATCH" {
		t.Errorf("method = %s, want PATCH", requests[0].Method)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(requests[0].Body), &body); err != nil {
		t.Fatalf("parsing body: %v", err)
	}
	if body["name"] != "checkout tester" {
		t.Errorf("name = %v", body["name"])
	}
}

func TestProfileCookies_ReadsTheProfile(t *testing.T) {
	server := setupCoverageTest(t)
	server.AddResponse("/profiles/"+profileIDTest+"/cookies", 200, `{"cookies":[{"name":"a","value":"b"}]}`)

	origID := profileID
	profileID = profileIDTest
	t.Cleanup(func() { profileID = origID })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runProfileCookies(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "cookies") {
		t.Errorf("expected the cookies in the output, got %q", stdout)
	}
}

// Playwright's storageState and the browser extensions export a bare array,
// while the API wants it under a `cookies` key. Making the caller reshape their
// own export first would be a papercut for no reason.
func TestProfileCookiesSet_AcceptsBothFileShapes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "bare array", content: `[{"name":"a","value":"b","domain":"example.com","path":"/"}]`},
		{name: "wrapped object", content: `{"cookies":[{"name":"a","value":"b","domain":"example.com","path":"/"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := setupCoverageTest(t)
			server.AddResponse("/profiles/"+profileIDTest+"/cookies", 200,
				`{"success":true,"message":"ok","cookies_count":1,"mode":"replace"}`)

			path := filepath.Join(t.TempDir(), "cookies.json")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatalf("writing cookies file: %v", err)
			}

			origID, origFile := profileID, profileCookiesFile
			profileID, profileCookiesFile = profileIDTest, path
			t.Cleanup(func() { profileID, profileCookiesFile = origID, origFile })

			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())

			testutil.CaptureOutput(func() {
				if err := runProfileCookiesSet(cmd, nil); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			requests := server.Requests("/profiles/" + profileIDTest + "/cookies")
			if len(requests) != 1 {
				t.Fatalf("got %d requests, want 1", len(requests))
			}
			var body map[string]any
			if err := json.Unmarshal([]byte(requests[0].Body), &body); err != nil {
				t.Fatalf("parsing body: %v", err)
			}
			cookies, ok := body["cookies"].([]any)
			if !ok || len(cookies) != 1 {
				t.Fatalf("expected one cookie under `cookies`, got %v", body)
			}
			// Neither optional field was passed, so neither should be sent.
			if _, present := body["source_format"]; present {
				t.Error("source_format was sent without --source-format")
			}
			if _, present := body["mode"]; present {
				t.Error("mode was sent without --mode")
			}
		})
	}
}

func TestProfileCookiesSet_RejectsAFileWithNoCookies(t *testing.T) {
	setupCoverageTest(t)

	path := filepath.Join(t.TempDir(), "cookies.json")
	if err := os.WriteFile(path, []byte(`{"notCookies":1}`), 0o600); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	origID, origFile := profileID, profileCookiesFile
	profileID, profileCookiesFile = profileIDTest, path
	t.Cleanup(func() { profileID, profileCookiesFile = origID, origFile })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runProfileCookiesSet(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "no cookies") {
		t.Fatalf("expected a no-cookies error, got %v", err)
	}
}

// The filters are sent only when asked, so the API keeps owning its defaults.
func TestUsageLogs_SendsOnlyTheFiltersPassed(t *testing.T) {
	server := setupCoverageTest(t)
	server.AddResponse("/usage/logs", 200,
		`{"items":[{"endpoint":"/sessions/start"}],"page":1,"page_size":10,"has_next":false,"has_previous":false}`)

	origEndpoint := usageLogsEndpoint
	usageLogsEndpoint = "/sessions/start"
	t.Cleanup(func() { usageLogsEndpoint = origEndpoint })

	cmd := &cobra.Command{}
	registerPaginationFlags(cmd)
	cmd.Flags().Bool("only-current-token", false, "")
	cmd.Flags().Bool("include-system", false, "")
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		if err := runUsageLogs(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	requests := server.Requests("/usage/logs")
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(requests))
	}
	query := requests[0].Query
	if !strings.Contains(query, "endpoint=") {
		t.Errorf("expected the endpoint filter in %q", query)
	}
	for _, unwanted := range []string{"only_current_token", "include_system", "only_active"} {
		if strings.Contains(query, unwanted) {
			t.Errorf("%s was sent without being asked for: %q", unwanted, query)
		}
	}
}
