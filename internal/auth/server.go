package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/config"
)

// SetupResult contains the result of a browser-based setup
type SetupResult struct {
	APIKey string
	Error  error
}

// SetupServer handles the browser-based authentication flow
type SetupServer struct {
	result        chan SetupResult
	shutdown      chan struct{}
	pendingResult *SetupResult
	mu            sync.Mutex
	csrfToken     string
	oauthState    string
	baseURL       string
}

// NewSetupServer creates a new setup server
func NewSetupServer() *SetupServer {
	csrfBytes := make([]byte, 32)
	_, _ = rand.Read(csrfBytes)

	stateBytes := make([]byte, 32)
	_, _ = rand.Read(stateBytes)

	return &SetupServer{
		result:     make(chan SetupResult, 1),
		shutdown:   make(chan struct{}),
		csrfToken:  hex.EncodeToString(csrfBytes),
		oauthState: hex.EncodeToString(stateBytes),
	}
}

// Start starts the setup server and opens the browser
func (s *SetupServer) Start(ctx context.Context) (*SetupResult, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	s.baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleSetup)
	mux.HandleFunc("/validate", s.handleValidate)
	mux.HandleFunc("/submit", s.handleSubmit)
	mux.HandleFunc("/success", s.handleSuccess)
	mux.HandleFunc("/complete", s.handleComplete)
	mux.HandleFunc("/callback", s.handleCallback)

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		_ = server.Serve(listener)
	}()

	go func() {
		_ = openBrowser(s.baseURL)
	}()

	select {
	case result := <-s.result:
		_ = server.Shutdown(context.Background())
		return &result, nil
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return nil, ctx.Err()
	case <-s.shutdown:
		_ = server.Shutdown(context.Background())
		s.mu.Lock()
		result := s.pendingResult
		s.mu.Unlock()
		if result != nil {
			return result, nil
		}
		return nil, fmt.Errorf("setup cancelled")
	}
}

func (s *SetupServer) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.New("setup").Parse(setupTemplate)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]string{
		"CSRFToken":      s.csrfToken,
		"ConsoleAuthURL": s.GetConsoleAuthURL(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
}

// GetConsoleAuthURL builds the console authentication URL with callback and state
func (s *SetupServer) GetConsoleAuthURL() string {
	consoleURL := config.GetConsoleURL()
	callbackURL := s.baseURL + "/callback"

	authURL, err := url.Parse(consoleURL + "/auth/cli")
	if err != nil {
		return consoleURL + "/apikeys" // Fallback to manual key page
	}
	q := authURL.Query()
	q.Set("callback", callbackURL)
	q.Set("state", s.oauthState)
	authURL.RawQuery = q.Encode()

	return authURL.String()
}

func (s *SetupServer) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("X-CSRF-Token") != s.csrfToken {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		APIKey string `json:"api_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Test the API key by making a health check request
	client, err := api.NewClient(req.APIKey)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Invalid API key: %v", err),
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.Client().HealthCheckWithResponse(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}

	if resp.StatusCode() != 200 {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("API error: %s", resp.Status()),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Connection successful",
	})
}

func (s *SetupServer) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("X-CSRF-Token") != s.csrfToken {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	var req struct {
		APIKey string `json:"api_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Validate the API key first
	client, err := api.NewClient(req.APIKey)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Invalid API key: %v", err),
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.Client().HealthCheckWithResponse(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Connection failed: %v", err),
		})
		return
	}

	if resp.StatusCode() != 200 {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("API error: %s", resp.Status()),
		})
		return
	}

	// Save to keyring
	if err := SetKeyringAPIKey(req.APIKey); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Failed to save credentials: %v", err),
		})
		return
	}

	s.mu.Lock()
	s.pendingResult = &SetupResult{
		APIKey: req.APIKey,
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func (s *SetupServer) handleSuccess(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.New("success").Parse(successTemplate)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
}

func (s *SetupServer) handleComplete(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	result := s.pendingResult
	s.mu.Unlock()
	if result != nil {
		s.result <- *result
	}
	close(s.shutdown)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *SetupServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Serve HTML page that extracts token from URL fragment
		// Fragments (#token=xxx) are never sent to server, so we need JS to extract and POST it
		tmpl, err := template.New("callback").Parse(callbackTemplate)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, map[string]string{
			"ExpectedState": s.oauthState,
		}); err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

	case http.MethodPost:
		// Receive token from the fragment extractor page
		var req struct {
			Token string `json:"token"`
			State string `json:"state"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"success": false,
				"error":   "Invalid request body",
			})
			return
		}

		// Verify state parameter for CSRF protection
		if req.State != s.oauthState {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"success": false,
				"error":   "Invalid state parameter",
			})
			return
		}

		if req.Token == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"success": false,
				"error":   "No token received",
			})
			return
		}

		// Validate the API key
		client, err := api.NewClient(req.Token)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"success": false,
				"error":   fmt.Sprintf("Invalid API key: %v", err),
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		resp, err := client.Client().HealthCheckWithResponse(ctx)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"success": false,
				"error":   fmt.Sprintf("Connection failed: %v", err),
			})
			return
		}

		if resp.StatusCode() != 200 {
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"success": false,
				"error":   fmt.Sprintf("API error: %s", resp.Status()),
			})
			return
		}

		// Save to keyring
		if err := SetKeyringAPIKey(req.Token); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"success": false,
				"error":   fmt.Sprintf("Failed to save credentials: %v", err),
			})
			return
		}

		// Set pending result
		s.mu.Lock()
		s.pendingResult = &SetupResult{
			APIKey: req.Token,
		}
		s.mu.Unlock()

		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
