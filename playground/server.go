package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed web/index.html
var page []byte

// Server watches source files and publishes complete, atomic preview revisions.
type Server struct {
	root      string
	runner    *Runner
	live      *Live
	execution sync.Mutex
	mu        sync.Mutex
	state     Preview
	clients   map[chan struct{}]struct{}
}

func NewServer(root string) (*Server, error) {
	runner, err := NewRunner(root)
	if err != nil {
		return nil, err
	}
	live, err := NewLive(runner)
	if err != nil {
		return nil, err
	}
	return &Server{
		root: root, runner: runner, live: live,
		state:   Preview{Version: "dev", Scenarios: []Example{}},
		clients: make(map[chan struct{}]struct{}),
	}, nil
}

func (s *Server) Close() error { return s.runner.Close() }

func (s *Server) Snapshot() Preview {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Server) publish(update func(*Preview)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	update(&s.state)
	for client := range s.clients {
		select {
		case client <- struct{}{}:
		default:
		}
	}
}

// Rebuild retains the last successful examples on error, marking them stale in
// the API so a broken edit never looks like a successful preview.
func (s *Server) Rebuild(ctx context.Context, revision string, record bool) error {
	s.execution.Lock()
	defer s.execution.Unlock()
	s.publish(func(state *Preview) { state.Building, state.Error = true, "" })
	scenarios, err := LoadScenariosFile(filepath.Join(s.root, "playground", "scenarios.json"))
	var recording Recording
	if err == nil {
		recording, err = s.runner.Render(ctx, scenarios, record)
	}
	s.publish(func(state *Preview) {
		state.Building = false
		if err != nil {
			state.Error = err.Error()
			return
		}
		state.Revision = revision
		state.BuiltAt = time.Now().UTC().Format(time.RFC3339Nano)
		state.Version = recording.Version
		state.Scenarios = recording.Scenarios
		state.Help = recording.Help
	})
	return err
}

// Watch requires no platform-specific file watcher or additional dependency.
// Source hashes catch same-size writes and atomic file replacements as well.
func (s *Server) Watch(ctx context.Context) {
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	var attempted, fixtures string
	for {
		// Only CLI source rebuilds the binary. The page is read from disk on
		// every load, and fixtures are read on every run, so editing either
		// needs no rebuild; a fixture edit only tells the page to re-run.
		revision, err := sourceRevision(s.root, isCLISource)
		if err == nil {
			var current string
			current, err = sourceRevision(s.root, isFixtureFile)
			if err == nil && current != fixtures {
				fixtures = current
				s.reloadFixtures(current)
			}
		}
		if err != nil {
			s.publish(func(state *Preview) { state.Error = err.Error() })
		} else if revision != attempted {
			attempted = revision
			_ = s.Rebuild(ctx, revision, false) // Rebuild publishes the error to the UI.
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) reloadFixtures(revision string) {
	scenarios, err := LoadScenariosFile(s.fixturesPath())
	if err != nil {
		s.publish(func(state *Preview) { state.Error = err.Error() })
		return
	}
	examples := make([]Example, 0, len(scenarios))
	for _, scenario := range scenarios {
		examples = append(examples, Example{Scenario: scenario, Variants: map[string]Capture{}})
	}
	s.publish(func(state *Preview) {
		state.Fixtures = revision
		if !state.Building {
			state.Scenarios = examples
		}
	})
}

func (s *Server) fixturesPath() string {
	return filepath.Join(s.root, "playground", "scenarios.json")
}

func isFixtureFile(relative string) bool {
	return filepath.ToSlash(relative) == "playground/scenarios.json"
}

// isCLISource reports whether a file is part of the CLI binary. The
// playground's own code is not.
func isCLISource(relative string) bool {
	return !strings.HasPrefix(filepath.ToSlash(relative), "playground/")
}

// SourceRevision tracks everything affecting captured output or the HTML page.
func SourceRevision(root string) (string, error) {
	return sourceRevision(root, func(string) bool { return true })
}

func sourceRevision(root string, include func(relative string) bool) (string, error) {
	hash := sha256.New()
	for _, directory := range []string{"cmd", "internal", "playground"} {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			switch filepath.Ext(path) {
			case ".go", ".json", ".html", ".css", ".js":
			default:
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if !include(relative) {
				return nil
			}
			_, _ = fmt.Fprintf(hash, "%s\x00%d\x00", relative, len(data))
			_, _ = hash.Write(data)
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%d\x00", name, len(data))
		_, _ = hash.Write(data)
	}
	return hex.EncodeToString(hash.Sum(nil))[:12], nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join(s.root, "playground", "web", "index.html"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("GET /api/preview", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(s.Snapshot()); err != nil {
			return // The client disconnected; there is no response left to write.
		}
	})
	mux.HandleFunc("GET /events", s.events)
	// The development server runs the real CLI; static exports only replay.
	mux.HandleFunc("POST /api/run", s.live.Run)
	mux.HandleFunc("POST /api/input", s.live.Input)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		host := r.Host
		if parsed, _, err := net.SplitHostPort(host); err == nil {
			host = parsed
		}
		ip := net.ParseIP(strings.Trim(host, "[]"))
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			http.Error(w, "the development playground requires a loopback host", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	client := make(chan struct{}, 1)
	s.mu.Lock()
	s.clients[client] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, client)
		s.mu.Unlock()
	}()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		state := s.Snapshot()
		payload, _ := json.Marshal(map[string]any{
			"revision": state.Revision, "building": state.Building, "error": state.Error, "fixtures": state.Fixtures,
		})
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-client:
		case <-heartbeat.C:
		}
	}
}

// BuildWasm compiles the CLI that a static export runs in the browser.
func (s *Server) BuildWasm(ctx context.Context, directory string) error {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	return s.runner.BuildWasm(ctx, directory)
}

// Export writes the static site: the page and the help pages it builds its
// command list from. Recorded scenarios are left out: the export runs the real
// CLI (see BuildWasm). The host configuration, including the API pass-through
// the page needs, is vercel.json next to this file.
func Export(directory string, preview Preview, html []byte) error {
	if preview.Error != "" || preview.Building || preview.Revision == "" {
		return fmt.Errorf("cannot export an incomplete or failed preview")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	staticPage := strings.Replace(string(html), "<html", "<html data-mode=\"static\"", 1)
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte(staticPage), 0o644); err != nil {
		return err
	}
	preview.Scenarios = []Example{}
	data, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, "preview.json"), append(data, '\n'), 0o644)
}

// Page returns the bundled page for export without depending on a working tree.
func Page() []byte { return append([]byte(nil), page...) }
