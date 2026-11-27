// Command playground runs the notte CLI in a browser terminal: against the
// real API, against synthetic API responses, or compiled to WebAssembly.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	Revision string `json:"revision"`
	BuiltAt  string `json:"builtAt"`
	Version  string `json:"version"`
	Building bool   `json:"building"`
	Error    string `json:"error"`
	// Fixtures changes whenever scenarios.json does.
	Fixtures  string    `json:"fixtures"`
	Scenarios []Example `json:"scenarios"`
	// Help maps a command path ("", "sessions", "sessions start") to its
	// colored --help output. The browser derives the command tree and each
	// command's flags from these pages.
	Help map[string]Capture `json:"help"`
}

// Recording is everything captured from one build of the CLI.
type Recording struct {
	Version   string
	Scenarios []Example
	Help      map[string]Capture
}

// Runner owns a disposable CLI binary. It never uses the user's account or state.
type Runner struct {
	root string
	dir  string
	mu   sync.Mutex
}

func NewRunner(root string) (*Runner, error) {
	dir, err := os.MkdirTemp("", "notte-playground-*")
	if err != nil {
		return nil, err
	}
	return &Runner{root: root, dir: dir}, nil
}

func (r *Runner) Close() error { return os.RemoveAll(r.dir) }

// Render builds the CLI and records its help pages. With
// record set it also replays every scenario through the CLI's actual
// entrypoint, which a static export needs; the live playground runs scenarios
// on demand instead, so its rebuilds stay fast.
func (r *Runner) Render(ctx context.Context, scenarios []Scenario, record bool) (Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	binary := filepath.Join(r.dir, binaryName())
	buildCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	overlay, err := r.credentialOverlay()
	if err != nil {
		return Recording{}, err
	}
	version := r.gitVersion(buildCtx)
	build := exec.CommandContext(buildCtx, "go", "build", "-overlay", overlay,
		"-ldflags", "-X main.version="+strings.TrimPrefix(version, "v"), "-o", binary, "./cmd/notte")
	build.Dir = r.root
	if output, err := build.CombinedOutput(); err != nil {
		return Recording{}, fmt.Errorf("build CLI: %w\n%s", err, output)
	}

	examples := make([]Example, 0, len(scenarios))
	for _, scenario := range scenarios {
		example := Example{Scenario: scenario, Variants: make(map[string]Capture)}
		if !record {
			examples = append(examples, example)
			continue
		}
		for _, mode := range []string{"text", "color", "json"} {
			capture, err := r.capture(ctx, binary, scenario, mode)
			if err != nil {
				return Recording{}, fmt.Errorf("%s (%s): %w", scenario.ID, mode, err)
			}
			example.Variants[mode] = capture
		}
		examples = append(examples, example)
	}
	help, err := r.captureAllHelp(ctx, binary)
	if err != nil {
		return Recording{}, err
	}
	return Recording{Version: version, Scenarios: examples, Help: help}, nil
}

// binaryName is the file name of the CLI build, which Windows only runs with
// its extension.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "notte.exe"
	}
	return "notte"
}

// gitVersion names the build like a release, relative to the latest tag:
// "v0.0.67" on a tag, "v0.0.67-2-gf780035-dirty" two commits after it with
// uncommitted changes. Builds outside a git checkout are "dev".
func (r *Runner) gitVersion(ctx context.Context) string {
	describe := exec.CommandContext(ctx, "git", "describe", "--tags", "--always", "--dirty")
	describe.Dir = r.root
	output, err := describe.Output()
	if version := strings.TrimSpace(string(output)); err == nil && version != "" {
		return version
	}
	return "dev"
}

// BuildWasm compiles the CLI for the browser into directory as notte.wasm,
// next to the loader that ships with the Go toolchain (wasm_exec.js). A static
// export runs this build in the visitor's browser: the same source as the
// released binaries, with nothing executed on a server.
func (r *Runner) BuildWasm(ctx context.Context, directory string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	buildCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	version := strings.TrimPrefix(r.gitVersion(buildCtx), "v")
	build := exec.CommandContext(buildCtx, "go", "build",
		"-ldflags", "-s -w -X main.version="+version, "-o", filepath.Join(directory, "notte.wasm"), "./cmd/notte")
	build.Dir = r.root
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	if output, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build CLI for the browser: %w\n%s", err, output)
	}
	goroot := exec.CommandContext(buildCtx, "go", "env", "GOROOT")
	goroot.Dir = r.root
	output, err := goroot.Output()
	if err != nil {
		return fmt.Errorf("locate the Go toolchain: %w", err)
	}
	loader, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(output)), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		return fmt.Errorf("read the WebAssembly loader: %w", err)
	}
	return os.WriteFile(filepath.Join(directory, "wasm_exec.js"), loader, 0o644)
}

// captureAllHelp records the help page of every command, walking the tree
// from the root through each page's "Available Commands". Pages are captured
// in parallel; there are over a hundred.
func (r *Runner) captureAllHelp(ctx context.Context, binary string) (map[string]Capture, error) {
	var (
		mu       sync.Mutex
		help     = make(map[string]Capture)
		firstErr error
		wg       sync.WaitGroup
		slots    = make(chan struct{}, 2*runtime.NumCPU())
	)
	var visit func(path []string)
	visit = func(path []string) {
		defer wg.Done()
		slots <- struct{}{}
		args := append(append([]string(nil), path...), "--help")
		result, _, err := r.execute(ctx, binary, Scenario{Args: args}, "color")
		<-slots
		key := strings.Join(path, " ")
		if err == nil && result.ExitCode != 0 {
			err = fmt.Errorf("notte %s --help: exit %d\n%s", key, result.ExitCode, result.Stderr)
		}
		mu.Lock()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			mu.Unlock()
			return
		}
		help[key] = result
		mu.Unlock()
		for _, name := range subcommands(result.Stdout) {
			if name == "help" || name == "completion" {
				continue
			}
			wg.Add(1)
			go visit(append(append([]string(nil), path...), name))
		}
	}
	wg.Add(1)
	visit(nil)
	wg.Wait()
	return help, firstErr
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func subcommands(page string) []string {
	var names []string
	listing := false
	for _, line := range strings.Split(ansiPattern.ReplaceAllString(page, ""), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasSuffix(trimmed, "Commands:"):
			listing = true
		case trimmed == "":
			listing = false
		case listing && strings.HasPrefix(line, "  "):
			names = append(names, strings.Fields(trimmed)[0])
		}
	}
	return names
}

// demoAPIKey looks like a real key so recorded output reads naturally.
const demoAPIKey = "sk-notte-demo-7f3a9c21e4b8"

// credentialOverlay adds a file to the auth package of the recording build
// only. It disables the OS credential store, so recordings never read or
// change the developer's Keychain (auth logout would otherwise delete their
// real key), and keys live in the encrypted file store inside each
// recording's throwaway config directory. NOTTE_PLAYGROUND_SAVED_KEY seeds
// that store, which records output as a user who ran "notte auth login" sees it.
func (r *Runner) credentialOverlay() (string, error) {
	store := filepath.Join(r.dir, "playground_store.go")
	const source = `package auth

import (
	"os"

	"github.com/99designs/keyring"
)

func init() {
	openSystemKeyring = func() (keyring.Keyring, error) { return nil, nil }
	if key := os.Getenv("NOTTE_PLAYGROUND_SAVED_KEY"); key != "" {
		if err := SetKeyringAPIKey(key); err != nil {
			panic(err)
		}
	}
}
`
	if err := os.WriteFile(store, []byte(source), 0o600); err != nil {
		return "", err
	}
	data, err := json.Marshal(struct{ Replace map[string]string }{Replace: map[string]string{
		filepath.Join(r.root, "internal", "auth", "zz_playground_store.go"): store,
	}})
	if err != nil {
		return "", err
	}
	path := filepath.Join(r.dir, "overlay.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func (r *Runner) capture(ctx context.Context, binary string, scenario Scenario, mode string) (Capture, error) {
	result, requests, err := r.execute(ctx, binary, scenario, mode)
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

func (r *Runner) execute(ctx context.Context, binary string, scenario Scenario, mode string) (Capture, map[string][]testutil.RecordedRequest, error) {
	commandArgs, responses, auth := scenario.Args, scenario.Responses, scenario.Auth
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
	if err := scenario.writeFiles(dir); err != nil {
		return Capture{}, nil, err
	}

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
	command.Env = captureEnv(dir, server.URL(), mode == "color", auth, len(responses) == 0)
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
func captureEnv(dir, apiURL string, color bool, auth string, offline bool) []string {
	env := []string{
		"HOME=" + dir, "USERPROFILE=" + dir,
		"XDG_CONFIG_HOME=" + dir, "XDG_CACHE_HOME=" + dir,
		"TMPDIR=" + dir, "TMP=" + dir, "TEMP=" + dir,
		"NOTTE_CONFIG_DIR=" + dir,
		"NOTTE_CONSOLE_URL=https://console.example.com",
		"NOTTE_NO_UPDATE_CHECK=1",
		"TERM=xterm-256color",
	}
	if offline {
		// A command without fixture responses must not call any API. Show the
		// production URL, as a user would see it, and send any request that
		// still happens to a closed local port instead of the internet.
		env = append(env, "NOTTE_API_URL=https://api.notte.cc",
			"HTTPS_PROXY=http://127.0.0.1:9", "HTTP_PROXY=http://127.0.0.1:9")
	} else {
		env = append(env, "NOTTE_API_URL="+apiURL)
	}
	switch auth {
	case AuthSaved:
		env = append(env, "NOTTE_PLAYGROUND_SAVED_KEY="+demoAPIKey)
	case AuthNone:
	default:
		env = append(env, "NOTTE_API_KEY="+demoAPIKey)
	}
	for _, key := range []string{"PATH", "SystemRoot", "WINDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	if color {
		env = append(env, "CLICOLOR_FORCE=1", "FORCE_HYPERLINK=1")
	} else {
		env = append(env, "NO_COLOR=1")
	}
	return env
}
