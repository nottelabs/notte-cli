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
	"os/exec"
	"runtime"
	"time"

	"github.com/salmonumbrella/notte-cli/internal/api"
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
	csrfToken     string
}

// NewSetupServer creates a new setup server
func NewSetupServer() *SetupServer {
	tokenBytes := make([]byte, 32)
	_, _ = rand.Read(tokenBytes)

	return &SetupServer{
		result:    make(chan SetupResult, 1),
		shutdown:  make(chan struct{}),
		csrfToken: hex.EncodeToString(tokenBytes),
	}
}

// Start starts the setup server and opens the browser
func (s *SetupServer) Start(ctx context.Context) (*SetupResult, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleSetup)
	mux.HandleFunc("/validate", s.handleValidate)
	mux.HandleFunc("/submit", s.handleSubmit)
	mux.HandleFunc("/success", s.handleSuccess)
	mux.HandleFunc("/complete", s.handleComplete)

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		_ = server.Serve(listener)
	}()

	go func() {
		_ = openBrowser(baseURL)
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
		if s.pendingResult != nil {
			return s.pendingResult, nil
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
		"CSRFToken": s.csrfToken,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
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

	s.pendingResult = &SetupResult{
		APIKey: req.APIKey,
	}

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
	_ = tmpl.Execute(w, nil)
}

func (s *SetupServer) handleComplete(w http.ResponseWriter, r *http.Request) {
	if s.pendingResult != nil {
		s.result <- *s.pendingResult
	}
	close(s.shutdown)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
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
