package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nottelabs/notte-cli/internal/config"
)

func TestRunnerRendersFixtures(t *testing.T) {
	// Keep the existing module/build caches when HOME becomes a synthetic user.
	goEnv, err := exec.Command("go", "env", "-json", "GOCACHE", "GOPATH", "GOMODCACHE").Output()
	if err != nil {
		t.Fatal(err)
	}
	var caches map[string]string
	if err := json.Unmarshal(goEnv, &caches); err != nil {
		t.Fatal(err)
	}
	for key, value := range caches {
		t.Setenv(key, value)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, inheritedConfig := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(config.EnvConfigDir, inheritedConfig)
	t.Setenv(config.EnvSessionID, "parent-session-sentinel")
	t.Setenv("NOTTE_API_KEY", "parent-synthetic-key")
	t.Setenv("NO_COLOR", "1")
	var inheritedRequests atomic.Int32
	inheritedAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		inheritedRequests.Add(1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"inherited API must not be used"}`))
	}))
	t.Cleanup(inheritedAPI.Close)
	t.Setenv(config.EnvAPIURL, inheritedAPI.URL)

	sentinels := map[string]string{}
	for _, base := range []string{home, inheritedConfig} {
		dir := filepath.Join(base, config.ConfigDirName)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for name, content := range map[string]string{
			config.ConfigFileName:           `{"api_key":"saved-synthetic-key","api_url":"https://example.invalid"}`,
			config.CurrentSessionFile:       "saved-session-sentinel",
			config.CurrentViewerURLFile:     "https://example.com/saved-session-sentinel",
			config.CurrentSessionExpiryFile: "2099-01-01T00:00:00Z",
		} {
			path := filepath.Join(dir, name)
			sentinels[path] = content
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	scenarios, err := LoadScenarios()
	if err != nil {
		t.Fatal(err)
	}
	runner, err := NewRunner(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := runner.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	recording, err := runner.Render(ctx, scenarios, true)
	if err != nil {
		t.Fatal(err)
	}
	examples, help := recording.Scenarios, recording.Help
	for _, path := range []string{"", "sessions", "sessions start", "page scrape", "functions list"} {
		if !strings.Contains(help[path].Stdout, "Usage:") {
			t.Errorf("help for %q was not recorded: %q", path, help[path].Stdout)
		}
	}
	if len(examples) != len(scenarios) {
		t.Fatalf("rendered %d scenarios, want %d", len(examples), len(scenarios))
	}
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	expectedText := map[string][]string{
		"help":                      {"Usage:", "Available Commands:", "notte sessions start"},
		"sessions-help":             {"Usage:", "notte sessions", "start", "status"},
		"page-help":                 {"Usage:", "notte page", "observe", "scrape"},
		"version":                   {"notte version "},
		"sessions-start":            {"Started session demo-session-001", "https://example.com/sessions/demo-session-001"},
		"sessions-status":           {"demo-session-001", "goto", "observation", "click"},
		"sessions-list":             {"BROWSER SESSION (2)", "demo-session-001", "demo-session-002"},
		"sessions-empty":            {"No active sessions (◡︵◡)"},
		"sessions-long-values":      {"demo-session-with-a-deliberately-long-identifier-001", "LAST ACTIVE"},
		"sessions-error":            {"404", "demo-session-missing"},
		"sessions-stop":             {"Stopped session demo-session-001", "Next: notte sessions status"},
		"functions-delete":          {"Deleted function demo-function-001", "Next: notte functions list"},
		"functions-create":          {"Created function demo-function-003", "Console", "Next: notte functions run"},
		"functions-run-wait-failed": {"failed", "Error: function run demo-run-001 failed"},
		"functions-secrets-get":     {"demo-secret-value"},
		"vaults-credentials-add":    {"Saved credentials for https://example.com in vault demo-vault-001", "Next: notte vaults credentials list"},
		"payment-status":            {"Payment 00000000-0000-4000-8000-000000000001", "awaiting approval", "Next: notte payment wait"},
		"files-upload":              {"Uploaded products.csv as file demo-file-003", "Next: notte files list"},
		"page-observe":              {"Example Shop", "[I1]", "[B2]"},
		"page-scrape":               {"# Example Shop", "café white", "https://example.com/products/ceramic-mug"},
		"page-scrape-structured":    {"Example Shop", "Ceramic mug", "Café white"},
	}
	for _, example := range examples {
		t.Run(example.ID, func(t *testing.T) {
			if filepath.IsAbs(example.Source) || strings.HasPrefix(filepath.Clean(example.Source), "..") {
				t.Fatalf("renderer source must be relative to the repo: %q", example.Source)
			}
			if info, err := os.Stat(filepath.Join(root, example.Source)); err != nil || info.IsDir() {
				t.Fatalf("renderer source %q is not a file: %v", example.Source, err)
			}
			for _, mode := range []string{"text", "color", "json"} {
				capture, ok := example.Variants[mode]
				if !ok || capture.ExitCode != example.ExpectedExit {
					t.Fatalf("%s capture missing or has unexpected exit: %+v", mode, capture)
				}
				if mode != "color" && ansi.MatchString(capture.Stdout+capture.Stderr) {
					t.Errorf("%s output contains ANSI color codes", mode)
				}
				// Cobra help remains text regardless of the output format flag.
				if mode == "json" && example.Group != "Help" {
					stream, other := capture.Stdout, capture.Stderr
					if capture.ExitCode != 0 {
						stream, other = capture.Stderr, capture.Stdout
					}
					// Waiting for a run streams its logs to stderr, and a failed
					// run is data on stdout followed by the error on stderr.
					if example.Source == "internal/cmd/function_run.go" {
						lines := strings.Split(strings.TrimSpace(capture.Stderr), "\n")
						if !json.Valid([]byte(capture.Stdout)) || (capture.ExitCode != 0 && !json.Valid([]byte(lines[len(lines)-1]))) {
							t.Errorf("a waited run must print JSON on stdout and a JSON error last on stderr: %+v", capture)
						}
						continue
					}
					if !json.Valid([]byte(stream)) || strings.TrimSpace(other) != "" {
						t.Errorf("JSON result must occupy only its expected stream: %+v", capture)
					}
				}
			}
			plain := example.Variants["text"]
			for _, fragment := range expectedText[example.ID] {
				if !strings.Contains(plain.Stdout+plain.Stderr, fragment) {
					t.Errorf("text output lost expected content %q", fragment)
				}
			}
			if example.ID == "sessions-start" && !ansi.MatchString(example.Variants["color"].Stdout) {
				t.Error("common text formatter did not produce color when forced")
			}
		})
	}

	binary := filepath.Join(runner.dir, "notte")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	var startScenario Scenario
	for _, scenario := range scenarios {
		if scenario.ID == "sessions-start" {
			startScenario = scenario
			break
		}
	}
	t.Run("JSON output keeps its shape", func(t *testing.T) { checkJSONShapes(t, examples) })
	if startScenario.ID == "" {
		t.Fatal("sessions-start fixture is required to verify API failures")
	}
	t.Run("unexpected exit remains visible", func(t *testing.T) {
		scenario := startScenario
		scenario.ExpectedExit = 1
		capture, err := runner.capture(ctx, binary, scenario, "text")
		if err == nil || !strings.Contains(err.Error(), "exit 0, expected 1") || capture.Stdout == "" {
			t.Fatalf("expected exit mismatch with preserved output, got %+v, %v", capture, err)
		}
	})
	t.Run("unexpected API path remains visible", func(t *testing.T) {
		scenario := startScenario
		scenario.ExpectedExit = 4 // the fake API answers an unlisted path with 404
		scenario.Responses = []Response{{Path: "/unused-fixture", Status: 200, Body: json.RawMessage(`{}`)}}
		_, err := runner.capture(ctx, binary, scenario, "text")
		if err == nil || !strings.Contains(err.Error(), "unexpected API path") || !strings.Contains(err.Error(), startScenario.Responses[0].Path) {
			t.Fatalf("expected missing fixture path error, got %v", err)
		}
	})
	if inheritedRequests.Load() != 0 {
		t.Fatal("fixture runner contacted the inherited API")
	}
	for _, base := range []string{home, inheritedConfig} {
		err := filepath.WalkDir(filepath.Join(base, config.ConfigDirName), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				if _, ok := sentinels[path]; !ok {
					t.Errorf("runner added a file to inherited config: %s", entry.Name())
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for path, original := range sentinels {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != original {
			t.Errorf("runner changed inherited config %s: %v", filepath.Base(path), err)
		}
	}
}

// jsonShapesFile records, per fixture, the top-level fields of the command's
// JSON output (for a list, those of its first item). Scripts and agents read
// those fields, so they may be added to but must not be renamed or dropped by
// work on the text output. After a deliberate change, regenerate the file:
//
//	UPDATE_JSON_SHAPES=1 go test ./playground -run TestRunnerRendersFixtures
const jsonShapesFile = "testdata/json_shapes.json"

func jsonShape(output string) []string {
	var value any
	if err := json.Unmarshal([]byte(output), &value); err != nil {
		return []string{"(not JSON)"}
	}
	prefix := ""
	if list, ok := value.([]any); ok {
		prefix = "[]."
		if len(list) == 0 {
			return []string{"[]"}
		}
		value = list[0]
	}
	object, ok := value.(map[string]any)
	if !ok {
		return []string{prefix + "(value)"}
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, prefix+key)
	}
	sort.Strings(keys)
	return keys
}

func checkJSONShapes(t *testing.T, examples []Example) {
	got := map[string][]string{}
	for _, example := range examples {
		capture := example.Variants["json"]
		output := capture.Stdout
		if strings.TrimSpace(output) == "" {
			output = capture.Stderr
		}
		// Help pages are text in every mode.
		if shape := jsonShape(output); shape[0] != "(not JSON)" {
			got[example.ID] = shape
		}
	}
	if os.Getenv("UPDATE_JSON_SHAPES") != "" {
		data, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(jsonShapesFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(jsonShapesFile, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(jsonShapesFile)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	for id, shape := range got {
		recorded, ok := want[id]
		if !ok {
			t.Errorf("%s: no recorded JSON shape; regenerate %s", id, jsonShapesFile)
			continue
		}
		have := map[string]bool{}
		for _, key := range shape {
			have[key] = true
		}
		for _, key := range recorded {
			if !have[key] {
				t.Errorf("%s: JSON output lost %q (now %v)", id, key, shape)
			}
		}
		if len(shape) != len(recorded) {
			t.Errorf("%s: JSON output gained fields (now %v); regenerate %s", id, shape, jsonShapesFile)
		}
	}
}
