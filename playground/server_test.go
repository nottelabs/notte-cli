package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
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
		"playground/scenarios.json": "[]", "playground/web/index.html": "<html>",
		"go.mod": "module example", "go.sum": "",
	} {
		writeSource(t, root, name, data)
	}
	previous, err := SourceRevision(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"internal/output/text.go", "playground/scenarios.json", "playground/web/index.html", "go.mod"} {
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
	writeSource(t, root, "playground/scenarios.json", "invalid JSON")
	server, err := NewServer(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	server.state = Preview{Revision: "last-good", Scenarios: []Example{{Scenario: Scenario{ID: "saved"}}}}
	updates := make(chan struct{}, 1)
	server.clients[updates] = struct{}{}
	if err := server.Rebuild(context.Background(), "broken", true); err == nil {
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
	writeSource(t, root, "playground/web/index.html", "<html><body>live page</body></html>")
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

func TestLiveRunRejectsOtherOrigins(t *testing.T) {
	server, err := NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	for _, origin := range []string{"", "https://example.com", "http://127.0.0.1:9999"} {
		for _, path := range []string{"/api/run", "/api/input"} {
			request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:4173"+path, strings.NewReader(`{"args":["version"]}`))
			if origin != "" {
				request.Header.Set("Origin", origin)
			}
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Errorf("%s from origin %q: status %d, want 403", path, origin, response.Code)
			}
		}
	}
}

// The page calls the API through a path on its own host, which vercel.json
// forwards to the API. Both must name the same path.
func TestHostForwardsThePagesAPIPath(t *testing.T) {
	data, err := os.ReadFile("vercel.json")
	if err != nil {
		t.Fatal(err)
	}
	var host struct {
		Rewrites []struct{ Source, Destination string }
	}
	if err := json.Unmarshal(data, &host); err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`API_PROXY = '([^']+)'`).FindSubmatch(page)
	if match == nil {
		t.Fatal("the page does not define API_PROXY")
	}
	for _, rewrite := range host.Rewrites {
		if rewrite.Source == string(match[1])+":path*" && rewrite.Destination == "https://api.notte.cc/:path*" {
			return
		}
	}
	t.Fatalf("vercel.json does not forward %s to the API: %+v", match[1], host.Rewrites)
}

// A live command gets the page's files, binary ones decoded, and never a path
// outside its working directory.
func TestLivePrepareWritesPageFiles(t *testing.T) {
	runner, err := NewRunner(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	if err := os.WriteFile(filepath.Join(runner.dir, binaryName()), []byte("build"), 0o700); err != nil {
		t.Fatal(err)
	}
	live := &Live{runner: runner}
	work, binary, err := live.prepare(context.Background(),
		map[string]string{"notes/a.txt": "text", "../escape.txt": "no"},
		map[string]string{"shot.png": "iVBORw0KGgo="})
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(binary); err != nil || string(data) != "build" {
		t.Fatalf("binary not copied: %q, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(work, "notes", "a.txt")); err != nil || string(data) != "text" {
		t.Fatalf("text file: %q, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(work, "shot.png")); err != nil || string(data) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("binary file: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(work), "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("a file escaped the working directory: %v", err)
	}
}
