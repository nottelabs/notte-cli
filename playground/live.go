package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

// Live runs the real CLI built from this checkout against the real Notte API.
// Only the notte binary is executed, with the arguments as an argv array:
// nothing goes through a shell. It has its own home and config directory, so
// it is signed in separately from an installed CLI, and the recording build's
// overlay keeps its key out of the OS keychain.
type Live struct {
	runner *Runner
	// fixtures is the scenarios.json that fixture runs read on every run.
	fixtures string
	home     string
	mu       sync.Mutex
	runs     map[string]io.WriteCloser
}

const maxLiveFile = 8 << 20

func NewLive(runner *Runner) (*Live, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	home := filepath.Join(cache, "notte-cli-playground", "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, err
	}
	fixtures := filepath.Join(runner.root, "playground", "scenarios.json")
	return &Live{runner: runner, fixtures: fixtures, home: home, runs: make(map[string]io.WriteCloser)}, nil
}

type liveRequest struct {
	Args  []string          `json:"args"`
	Env   map[string]string `json:"env"`
	Files map[string]string `json:"files"`
	// Binary holds the files that are not text, base64-encoded.
	Binary map[string]string `json:"binary"`
	Stdin  string            `json:"stdin"`
	TTY    bool              `json:"tty"`
	// Scenario runs against that fixture's fake API instead of the real one.
	Scenario string `json:"scenario"`
	// Pinned is the fixture run's current session, kept by the page between
	// runs because each one starts from a fresh config.
	Pinned string `json:"pinned"`
}

// liveFile is a file the command created or changed in its working directory.
type liveFile struct {
	Name   string  `json:"name"`
	Text   *string `json:"text,omitempty"`
	Base64 string  `json:"base64,omitempty"`
	Size   int     `json:"size"`
}

type liveEvent struct {
	Type  string     `json:"type"`
	ID    string     `json:"id,omitempty"`
	Data  string     `json:"data,omitempty"`
	Code  int        `json:"code"`
	Files []liveFile `json:"files,omitempty"`
	// Pinned is the current session after a fixture run ("" if none).
	Pinned *string `json:"pinned,omitempty"`
}

func sameOrigin(r *http.Request) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	return err == nil && origin.Host == r.Host && (origin.Scheme == "http" || origin.Scheme == "https")
}

// Run streams one CLI invocation as newline-delimited JSON events. Closing the
// request (Ctrl+C in the page) kills the process.
func (l *Live) Run(w http.ResponseWriter, r *http.Request) {
	// Any web page can send requests to localhost; only this page may run
	// commands with the account signed in here.
	if !sameOrigin(r) {
		http.Error(w, "commands must come from the playground page", http.StatusForbidden)
		return
	}
	var request liveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&request); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is unavailable", http.StatusInternalServerError)
		return
	}
	work, binary, err := l.prepare(r.Context(), request.Files, request.Binary)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = os.RemoveAll(work) }()
	before := snapshot(work)

	env := l.env(request)
	pinnedFile := ""
	if request.Scenario != "" {
		fixtureEnv, pinned, cleanup, err := l.fixtureEnv(request, work)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer cleanup()
		env, pinnedFile = fixtureEnv, pinned
	}
	if request.TTY && request.Stdin == "" {
		// The page sends each key press of interactive menus right away.
		env = append(env, "NOTTE_FORCE_INTERACTIVE=1")
	}
	command := exec.CommandContext(r.Context(), binary, request.Args...)
	command.Dir = work
	command.Env = env
	stdin, err := command.StdinPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	events := make(chan liveEvent, 64)
	stream := func(kind string) io.Writer { return eventWriter{kind: kind, events: events} }
	command.Stdout, command.Stderr = stream("out"), stream("err")

	w.Header().Set("Content-Type", "application/x-ndjson")
	encoder := json.NewEncoder(w)
	if err := command.Start(); err != nil {
		_ = encoder.Encode(liveEvent{Type: "err", Data: "playground: could not start the CLI: " + err.Error() + "\n"})
		_ = encoder.Encode(liveEvent{Type: "exit", Code: 1})
		return
	}
	id := randomID()
	if request.Stdin != "" {
		// Piped input: write it all and close, like a shell pipe.
		go func() { _, _ = io.WriteString(stdin, request.Stdin); _ = stdin.Close() }()
	} else {
		l.mu.Lock()
		l.runs[id] = stdin
		l.mu.Unlock()
	}
	defer func() {
		l.mu.Lock()
		delete(l.runs, id)
		l.mu.Unlock()
	}()
	_ = encoder.Encode(liveEvent{Type: "start", ID: id})
	flusher.Flush()

	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	for {
		select {
		case event := <-events:
			_ = encoder.Encode(event)
			flusher.Flush()
			continue
		case err := <-done:
			for len(events) > 0 {
				_ = encoder.Encode(<-events)
			}
			code := 0
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else if err != nil {
				code = 1
			}
			exit := liveEvent{Type: "exit", Code: code, Files: changedFiles(work, before)}
			if pinnedFile != "" {
				data, _ := os.ReadFile(pinnedFile)
				pinned := strings.TrimSpace(string(data))
				exit.Pinned = &pinned
			}
			_ = encoder.Encode(exit)
			flusher.Flush()
			return
		}
	}
}

// Input forwards a line typed while a command runs, for its prompts.
func (l *Live) Input(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "input must come from the playground page", http.StatusForbidden)
		return
	}
	var request struct {
		ID   string `json:"id"`
		Data string `json:"data"`
		EOF  bool   `json:"eof"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	l.mu.Lock()
	stdin := l.runs[request.ID]
	l.mu.Unlock()
	if stdin == nil {
		http.Error(w, "the command has finished", http.StatusNotFound)
		return
	}
	_, _ = io.WriteString(stdin, request.Data)
	if request.EOF {
		_ = stdin.Close()
	}
	w.WriteHeader(http.StatusNoContent)
}

// prepare copies the current build into a fresh working directory, so a
// rebuild or "notte upgrade" never replaces a binary that is running, and
// writes the page's files for the command to read.
func (l *Live) prepare(ctx context.Context, files, binary map[string]string) (string, string, error) {
	work, err := os.MkdirTemp(l.runner.dir, "live-*")
	if err != nil {
		return "", "", err
	}
	executable, err := l.runner.copyBinary(ctx, filepath.Join(work, ".notte-bin"))
	if err != nil {
		_ = os.RemoveAll(work)
		return "", "", err
	}
	write := func(name string, data []byte) {
		clean := filepath.Clean(name)
		if !filepath.IsLocal(clean) || strings.HasPrefix(clean, ".notte-bin") {
			return
		}
		path := filepath.Join(work, clean)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return
		}
		_ = os.WriteFile(path, data, 0o600)
	}
	for name, content := range files {
		write(name, []byte(content))
	}
	for name, content := range binary {
		if data, err := base64.StdEncoding.DecodeString(content); err == nil {
			write(name, data)
		}
	}
	return work, executable, nil
}

func (l *Live) env(request liveRequest) []string {
	env := []string{
		"HOME=" + l.home, "USERPROFILE=" + l.home,
		"NOTTE_CONFIG_DIR=" + filepath.Join(l.home, ".notte"),
		"NOTTE_NO_UPDATE_CHECK=1",
		"TERM=xterm-256color",
		"COLUMNS=120",
	}
	for _, key := range []string{"PATH", "SystemRoot", "WINDIR", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	// The API records calls from the playground apart from the installed CLI.
	env = append(env, "NOTTE_REQUEST_ORIGIN=cli-playground")
	// Variables exported in the page's shell reach the CLI, e.g.
	// NOTTE_SESSION_ID or NOTTE_API_URL. The host's own NOTTE_* variables do not.
	for key, value := range request.Env {
		if strings.HasPrefix(key, "NOTTE_") && !strings.ContainsAny(key+value, "\x00") {
			env = append(env, key+"="+value)
		}
	}
	if request.TTY {
		// The page renders colors and OSC 8 links like a modern terminal.
		env = append(env, "CLICOLOR_FORCE=1", "FORCE_HYPERLINK=1")
	} else {
		env = append(env, "NO_COLOR=1")
	}
	return env
}

// fixtureEnv starts a fake API answering with the scenario's responses and
// returns the environment of a recording (see captureEnv): a throwaway home
// and config, the scenario's sign-in state, and no route to the real API.
// The scenario is read from scenarios.json on every run, so fixture edits
// apply without a rebuild.
func (l *Live) fixtureEnv(request liveRequest, work string) ([]string, string, func(), error) {
	scenarios, err := LoadScenariosFile(l.fixtures)
	if err != nil {
		return nil, "", nil, err
	}
	var scenario *Scenario
	for i := range scenarios {
		if scenarios[i].ID == request.Scenario {
			scenario = &scenarios[i]
		}
	}
	if scenario == nil {
		return nil, "", nil, fmt.Errorf("no fixture named %q in scenarios.json", request.Scenario)
	}
	if err := scenario.writeFiles(work); err != nil {
		return nil, "", nil, err
	}
	home, err := os.MkdirTemp(l.runner.dir, "fixture-home-*")
	if err != nil {
		return nil, "", nil, err
	}
	// The CLI keeps its current session in a file in its config directory.
	pinned := filepath.Join(home, config.ConfigDirName, config.CurrentSessionFile)
	if request.Pinned != "" {
		if err := os.MkdirAll(filepath.Dir(pinned), 0o700); err != nil {
			_ = os.RemoveAll(home)
			return nil, "", nil, err
		}
		if err := os.WriteFile(pinned, []byte(request.Pinned), 0o600); err != nil {
			_ = os.RemoveAll(home)
			return nil, "", nil, err
		}
	}
	server := testutil.NewMockServer()
	for _, response := range scenario.Responses {
		server.AddResponse(response.Path, response.Status, string(response.Body))
	}
	cleanup := func() { server.Close(); _ = os.RemoveAll(home) }
	env := captureEnv(home, server.URL(), request.TTY, scenario.Auth, len(scenario.Responses) == 0)
	env = append(env, "COLUMNS=120")
	if id := request.Env["NOTTE_SESSION_ID"]; id != "" {
		env = append(env, "NOTTE_SESSION_ID="+id)
	}
	return env, pinned, cleanup, nil
}

type eventWriter struct {
	kind   string
	events chan<- liveEvent
}

func (e eventWriter) Write(p []byte) (int, error) {
	e.events <- liveEvent{Type: e.kind, Data: string(p)}
	return len(p), nil
}

type fileState struct {
	size    int64
	modTime int64
}

func snapshot(dir string) map[string]fileState {
	files := make(map[string]fileState)
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".notte-bin" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err == nil && info.Mode().IsRegular() {
			relative, _ := filepath.Rel(dir, path)
			files[relative] = fileState{info.Size(), info.ModTime().UnixNano()}
		}
		return nil
	})
	return files
}

func changedFiles(dir string, before map[string]fileState) []liveFile {
	var files []liveFile
	for name, state := range snapshot(dir) {
		if previous, ok := before[name]; ok && previous == state {
			continue
		}
		if state.size > maxLiveFile {
			files = append(files, liveFile{Name: filepath.ToSlash(name), Size: int(state.size)})
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		file := liveFile{Name: filepath.ToSlash(name), Size: len(data)}
		if utf8.Valid(data) {
			text := string(data)
			file.Text = &text
		} else {
			file.Base64 = base64.StdEncoding.EncodeToString(data)
		}
		files = append(files, file)
	}
	return files
}

func randomID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// copyBinary waits for any build in progress and copies the latest binary.
func (r *Runner) copyBinary(_ context.Context, dir string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(r.dir, binaryName()))
	if err != nil {
		return "", fmt.Errorf("the CLI has not been built yet")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	target := filepath.Join(dir, binaryName())
	return target, os.WriteFile(target, data, 0o700)
}
