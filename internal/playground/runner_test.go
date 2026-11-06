package playground

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
	root, err := filepath.Abs(filepath.Join("..", ".."))
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
	examples, err := runner.Render(ctx, scenarios)
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) != len(scenarios) {
		t.Fatalf("rendered %d scenarios, want %d", len(examples), len(scenarios))
	}
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	expectedText := map[string][]string{
		"help":                   {"Usage:", "Available Commands:", "notte sessions start"},
		"sessions-help":          {"Usage:", "notte sessions", "start", "status"},
		"page-help":              {"Usage:", "notte page", "observe", "scrape"},
		"version":                {"notte version dev"},
		"sessions-start":         {"demo-session-001", "wss://example.com/cdp/"},
		"sessions-status":        {"demo-session-001", "goto", "observation", "click"},
		"sessions-list":          {"demo-session-001", "demo-session-002", "firefox"},
		"sessions-empty":         {"No active sessions."},
		"sessions-long-values":   {"demo-session-with-a-deliberately-long-identifier-001", "Chrome/130.0.0.0"},
		"sessions-error":         {"404", "demo-session-missing"},
		"sessions-stop":          {"demo-session-001", "stopped"},
		"page-observe":           {"Example Shop", "[I1]", "[B2]"},
		"page-scrape":            {"# Example Shop", "café white", "https://example.com/products/ceramic-mug"},
		"page-scrape-structured": {"Example Shop", "Ceramic mug", "Café white"},
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
		scenario.ExpectedExit = 1
		scenario.Responses = []Response{{Path: "/unused-fixture", Status: 200, Body: json.RawMessage(`{}`)}}
		_, err := runner.capture(ctx, binary, scenario, "text")
		if err == nil || !strings.Contains(err.Error(), "unexpected API path") || !strings.Contains(err.Error(), startScenario.Responses[0].Path) {
			t.Fatalf("expected missing fixture path error, got %v", err)
		}
	})
	for _, tc := range []struct {
		name       string
		args       []string
		scenarioID string
		want       string
		exit       int
		json       bool
	}{
		{name: "root help", want: "Usage:"},
		{name: "session help", args: []string{"sessions"}, want: "notte sessions"},
		{name: "page help", args: []string{"page"}, want: "notte page"},
		{name: "version", args: []string{"version"}, want: "notte version dev"},
		{name: "list", args: []string{"sessions", "list"}, want: "demo-session-002"},
		{name: "status uses synthetic current session", args: []string{"sessions", "status"}, want: "demo-session-001"},
		{name: "observe", args: []string{"page", "observe"}, want: "Example Shop"},
		{name: "scrape", args: []string{"page", "scrape"}, want: "# Example Shop"},
		{name: "structured scrape", args: []string{"page", "scrape", "--instructions", "Extract products"}, want: "Ceramic mug"},
		{name: "JSON root option", args: []string{"--output", "json", "sessions", "list"}, want: "demo-session-002", json: true},
		{name: "JSON status", args: []string{"sessions", "status", "--output=json"}, want: "demo-session-001", json: true},
		{name: "unknown flag", args: []string{"sessions", "list", "--does-not-exist"}, want: "unknown flag", exit: 1},
		{name: "selected empty fixture", args: []string{"sessions", "list"}, scenarioID: "sessions-empty", want: "No active sessions."},
		{name: "selected API error", args: []string{"sessions", "status", "--session-id", "demo-session-missing"}, scenarioID: "sessions-error", want: "404", exit: 1},
	} {
		t.Run("typed command/"+tc.name, func(t *testing.T) {
			capture, err := runner.RunCommand(ctx, tc.args, tc.scenarioID, scenarios)
			if err != nil {
				t.Fatal(err)
			}
			if capture.ExitCode != tc.exit || !strings.Contains(capture.Stdout+capture.Stderr, tc.want) {
				t.Fatalf("unexpected typed command result: %+v", capture)
			}
			if tc.json && !json.Valid([]byte(capture.Stdout)) {
				t.Errorf("typed command did not return JSON: %+v", capture)
			}
		})
	}
	for _, args := range [][]string{{"auth", "login"}, {"upgrade"}, {"skills", "add"}, {"files", "list"}, {"--help", "auth", "login", "--help=false"}} {
		t.Run("blocked typed command/"+strings.Join(args, " "), func(t *testing.T) {
			if _, err := runner.RunCommand(ctx, args, "", scenarios); err == nil {
				t.Fatal("unsupported command reached the fixture runner")
			}
		})
	}
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

func TestCommandScenario(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "help"},
		{[]string{"--help"}, "help"},
		{[]string{"sessions"}, "sessions-help"},
		{[]string{"page"}, "page-help"},
		{[]string{"version"}, "version"},
		{[]string{"-o", "json", "sessions", "status"}, "sessions-status"},
		{[]string{"--timeout=3", "--no-color", "page", "observe"}, "page-observe"},
		{[]string{"--verbose", "--output=json", "sessions", "list"}, "sessions-list"},
		{[]string{"page", "scrape", "--instructions", "Extract prices"}, "page-scrape-structured"},
		{[]string{"page", "scrape", "--instructions=Extract prices"}, "page-scrape-structured"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			got, err := commandScenario(tc.args)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	for _, args := range [][]string{
		{"auth", "login"},
		{"upgrade"},
		{"skills", "add"},
		{"files", "list"},
		{"profiles", "list"},
		{"page", "goto", "https://example.com"},
		{"sessions", "replay"},
		{"completion", "bash"},
		{"--help", "auth", "login", "--help=false"},
		{"--", "auth", "login"},
		{"--help", "help", "auth", "logout", "--help=false"},
		{"--help", "sessions", "--help", "start", "auth", "logout", "--help=false"},
	} {
		t.Run("blocked/"+strings.Join(args, " "), func(t *testing.T) {
			if _, err := commandScenario(args); err == nil {
				t.Fatal("unsupported command was accepted")
			}
		})
	}
}
