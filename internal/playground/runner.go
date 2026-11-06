// Package playground previews the real CLI against synthetic API responses.
package playground

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

// Capture keeps the two output streams separate, exactly as the command wrote them.
type Capture struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// Example contains the same scenario rendered in each supported output mode.
type Example struct {
	Scenario
	Variants map[string]Capture `json:"variants"`
}

// Preview is also the portable data format consumed by the static HTML export.
type Preview struct {
	Revision  string    `json:"revision"`
	BuiltAt   string    `json:"builtAt"`
	Version   string    `json:"version"`
	Building  bool      `json:"building"`
	Error     string    `json:"error"`
	Scenarios []Example `json:"scenarios"`
}

// Runner owns a disposable CLI binary. It never uses the user's account or state.
type Runner struct {
	root string
	dir  string
	mu   sync.Mutex
}

func NewRunner(root string) (*Runner, error) {
	dir, err := os.MkdirTemp("", "notte-ui-preview-*")
	if err != nil {
		return nil, err
	}
	return &Runner{root: root, dir: dir}, nil
}

func (r *Runner) Close() error { return os.RemoveAll(r.dir) }

// Render builds the CLI and replays every scenario through its actual entrypoint.
func (r *Runner) Render(ctx context.Context, scenarios []Scenario) ([]Example, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	binary := filepath.Join(r.dir, "notte")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	buildCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, "./cmd/notte")
	build.Dir = r.root
	if output, err := build.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("build CLI: %w\n%s", err, output)
	}

	examples := make([]Example, 0, len(scenarios))
	for _, scenario := range scenarios {
		example := Example{Scenario: scenario, Variants: make(map[string]Capture)}
		for _, mode := range []string{"text", "color", "json"} {
			capture, err := r.capture(ctx, binary, scenario, mode)
			if err != nil {
				return nil, fmt.Errorf("%s (%s): %w", scenario.ID, mode, err)
			}
			example.Variants[mode] = capture
		}
		examples = append(examples, example)
	}
	return examples, nil
}

func (r *Runner) capture(ctx context.Context, binary string, scenario Scenario, mode string) (Capture, error) {
	result, requests, err := r.execute(ctx, binary, scenario.Args, scenario.Responses, mode, false)
	if err != nil {
		return result, err
	}
	if result.ExitCode != scenario.ExpectedExit {
		return result, fmt.Errorf("exit %d, expected %d\n%s%s", result.ExitCode, scenario.ExpectedExit, result.Stdout, result.Stderr)
	}
	paths := make(map[string]bool)
	for _, response := range scenario.Responses {
		paths[response.Path] = true
	}
	if len(requests) == 0 && len(scenario.Responses) > 0 {
		return result, fmt.Errorf("command did not reach the fixture API")
	}
	for path := range requests {
		if !paths[path] {
			return result, fmt.Errorf("unexpected API path %q; add its synthetic response to the scenario", path)
		}
	}
	return result, nil
}

// RunCommand executes an allowlisted Notte command, never a shell command. Each
// request gets fresh synthetic session state, so command history can be replayed
// after source edits without touching a user's account or filesystem.
func (r *Runner) RunCommand(ctx context.Context, args []string, scenarioID string, scenarios []Scenario) (Capture, error) {
	id, err := commandScenario(args)
	if err != nil {
		return Capture{}, err
	}
	if scenarioID != "" {
		id = scenarioID
	}
	var responses []Response
	found := false
	for _, scenario := range scenarios {
		if scenario.ID == id {
			responses, found = scenario.Responses, true
			break
		}
	}
	if !found {
		return Capture{}, fmt.Errorf("no fixture for %q", id)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	binary := filepath.Join(r.dir, "notte")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	result, _, err := r.execute(ctx, binary, args, responses, "interactive", true)
	return result, err
}

// commandScenario restricts execution to commands backed by this playground's
// fixtures. In particular, auth, upgrades, installers, and file commands never run.
func commandScenario(args []string) (string, error) {
	words := make([]string, 0, 2)
	for i := 0; i < len(args) && len(words) < 2; i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			words = append(words, arg)
			continue
		}
		switch arg {
		case "--help", "-h":
			// Cobra resolves commands before registering help flags. Keeping
			// help last prevents it consuming a command word during resolution.
			if i != len(args)-1 {
				return "", fmt.Errorf("place --help or -h at the end of the command")
			}
		case "--output", "-o", "--timeout":
			i++
		case "--yes", "-y", "--no-color", "--verbose", "-v":
		default:
			if !strings.HasPrefix(arg, "--output=") && !strings.HasPrefix(arg, "--timeout=") {
				return "", fmt.Errorf("unsupported playground option %q; type notte for CLI help", arg)
			}
		}
	}
	if len(words) == 0 || words[0] == "help" {
		return "help", nil
	}
	if len(words) == 1 {
		switch words[0] {
		case "sessions", "page":
			return words[0] + "-help", nil
		case "version":
			return "version", nil
		}
	}
	if len(words) == 2 {
		key := words[0] + "-" + words[1]
		switch key {
		case "sessions-start", "sessions-list", "sessions-status", "sessions-stop", "page-observe":
			return key, nil
		case "page-scrape":
			for _, arg := range args {
				if arg == "--instructions" || strings.HasPrefix(arg, "--instructions=") {
					return "page-scrape-structured", nil
				}
			}
			return key, nil
		}
	}
	return "", fmt.Errorf("this playground supports notte help, version, sessions start/list/status/stop, and page observe/scrape")
}

func (r *Runner) execute(ctx context.Context, binary string, commandArgs []string, responses []Response, mode string, interactive bool) (Capture, map[string][]testutil.RecordedRequest, error) {
	server := testutil.NewMockServer()
	defer server.Close()
	for _, response := range responses {
		server.AddResponse(response.Path, response.Status, string(response.Body))
	}
	dir, err := os.MkdirTemp(r.dir, "state-*")
	if err != nil {
		return Capture{}, nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	args := append([]string(nil), commandArgs...)
	args = append(args, "--yes")
	switch mode {
	case "json":
		args = append(args, "--output", "json", "--no-color")
	case "text":
		args = append(args, "--no-color")
	}
	commandCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	command := exec.CommandContext(commandCtx, binary, args...)
	command.Dir = dir
	command.Env = captureEnv(dir, server.URL(), mode == "color" || interactive)
	if interactive {
		id, _ := commandScenario(commandArgs)
		if id != "sessions-start" {
			command.Env = append(command.Env, "NOTTE_SESSION_ID=demo-session-001")
		}
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err = command.Run()
	result := Capture{Stdout: stdout.String(), Stderr: stderr.String()}
	if commandCtx.Err() != nil {
		return result, nil, fmt.Errorf("command interrupted: %w", commandCtx.Err())
	}
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return result, nil, err
		}
		result.ExitCode = exitErr.ExitCode()
	}
	return result, server.AllRequests(), nil
}

// Allowlist environment entries instead of passing through saved credentials,
// session IDs, proxies, or user config. Go builds use the normal development env;
// only the fixture CLI subprocess gets this disposable environment.
func captureEnv(dir, apiURL string, color bool) []string {
	env := []string{
		"HOME=" + dir, "USERPROFILE=" + dir,
		"XDG_CONFIG_HOME=" + dir, "XDG_CACHE_HOME=" + dir,
		"TMPDIR=" + dir, "TMP=" + dir, "TEMP=" + dir,
		"NOTTE_CONFIG_DIR=" + dir,
		"NOTTE_API_KEY=playground-synthetic-key",
		"NOTTE_API_URL=" + apiURL,
		"NOTTE_CONSOLE_URL=https://console.example.com",
		"NOTTE_NO_UPDATE_CHECK=1",
		"TERM=xterm-256color",
	}
	for _, key := range []string{"PATH", "SystemRoot", "WINDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	if color {
		env = append(env, "CLICOLOR_FORCE=1")
	} else {
		env = append(env, "NO_COLOR=1")
	}
	return env
}
