package playground

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSource(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSourceRevisionTracksContent(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string]string{
		"cmd/notte/main.go": "package main", "internal/output/text.go": "before",
		"internal/playground/scenarios.json": "[]", "internal/playground/web/index.html": "<html>",
		"go.mod": "module example", "go.sum": "",
	} {
		writeSource(t, root, name, data)
	}
	previous, err := SourceRevision(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"internal/output/text.go", "internal/playground/scenarios.json", "internal/playground/web/index.html", "go.mod"} {
		// Same-size writes must be noticed as well as new files and timestamps.
		writeSource(t, root, name, "after!")
		current, err := SourceRevision(root)
		if err != nil {
			t.Fatal(err)
		}
		if current == previous {
			t.Fatalf("did not notice a change to %s", name)
		}
		previous = current
	}
}

func TestFailedRebuildKeepsLastGoodOutputAndNotifies(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "internal/playground/scenarios.json", "invalid JSON")
	server, err := NewServer(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	server.state = Preview{Revision: "last-good", Scenarios: []Example{{Scenario: Scenario{ID: "saved"}}}}
	updates := make(chan struct{}, 1)
	server.clients[updates] = struct{}{}
	if err := server.Rebuild(context.Background(), "broken"); err == nil {
		t.Fatal("invalid fixtures should fail the rebuild")
	}
	state := server.Snapshot()
	if state.Error == "" || state.Building || state.Revision != "last-good" || state.Scenarios[0].ID != "saved" {
		t.Fatalf("failure hid or replaced the previous output: %+v", state)
	}
	select {
	case <-updates:
	default:
		t.Fatal("subscribers were not notified")
	}
}

func TestPreviewHTTPAndStaticExport(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "internal/playground/web/index.html", "<html><body>live page</body></html>")
	server, err := NewServer(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	server.state = Preview{Revision: "captured", Scenarios: []Example{{Scenario: Scenario{ID: "sample"}}}}
	handler := server.Handler()
	for _, path := range []string{"/", "/api/preview"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil))
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: status %d, headers %v", path, response.Code, response.Header())
		}
		if path == "/api/preview" {
			var data Preview
			if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil || data.Revision != "captured" {
				t.Fatalf("invalid preview: %s (%v)", response.Body.String(), err)
			}
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "http://localhost/api/preview", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected command submission accepted: %d", response.Code)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://example.com/api/preview", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-loopback Host was accepted: %d", response.Code)
	}

	directory := t.TempDir()
	if err := Export(directory, server.Snapshot(), Page()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "index.html"))
	if err != nil || !strings.Contains(string(data), "data-mode=\"static\"") {
		t.Fatalf("missing static mode: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(directory, "preview.json"))
	if err != nil || !json.Valid(data) {
		t.Fatalf("invalid exported preview: %v", err)
	}
	broken := server.Snapshot()
	broken.Error = "build failed"
	if err := Export(directory, broken, Page()); err == nil {
		t.Fatal("export accepted a failed preview")
	}
}

func TestInteractiveHTTPRejectsInvalidRequests(t *testing.T) {
	server, err := NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	for _, test := range []struct {
		name   string
		body   string
		origin string
		status int
	}{
		{"external origin", `{"args":[]}`, "https://example.com", http.StatusForbidden},
		{"malformed JSON", `{`, "", http.StatusBadRequest},
		{"unknown fields", `{"args":[],"shell":"upgrade"}`, "", http.StatusBadRequest},
		{"unsupported command", `{"args":["auth","login"]}`, "", http.StatusBadRequest},
		{"help override", `{"args":["--help","auth","login","--help=false"]}`, "", http.StatusBadRequest},
		{"not built yet", `{"args":[]}`, "http://localhost", http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://localhost/api/run", strings.NewReader(test.body))
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != test.status || !json.Valid(response.Body.Bytes()) {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
		})
	}
}
