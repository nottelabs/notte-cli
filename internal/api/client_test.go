package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient_RequiresAPIKey(t *testing.T) {
	_, err := NewClient("")
	if err == nil {
		t.Error("expected error for empty API key")
	}
}

func TestNewClient_Success(t *testing.T) {
	client, err := NewClient("test-api-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client")
	}
	if client.apiKey != "test-api-key" {
		t.Errorf("got apiKey %q, want %q", client.apiKey, "test-api-key")
	}
	if client.baseURL != DefaultBaseURL {
		t.Errorf("got baseURL %q, want %q", client.baseURL, DefaultBaseURL)
	}
}

func TestNewClientWithURL_CustomURL(t *testing.T) {
	client, err := NewClientWithURL("test-key", "https://custom.api.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.baseURL != "https://custom.api.com" {
		t.Errorf("got baseURL %q, want %q", client.baseURL, "https://custom.api.com")
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	customRetry := &RetryConfig{MaxRetries: 10}
	client, err := NewClient("test-key", WithRetryConfig(customRetry))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.retryConfig.MaxRetries != 10 {
		t.Errorf("got MaxRetries %d, want 10", client.retryConfig.MaxRetries)
	}
}

func TestResilientTransport_AddsAuthHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			t.Errorf("got Authorization %q, want %q", auth, "Bearer test-api-key")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, err := NewClientWithURL("test-api-key", server.URL)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want 200", resp.StatusCode)
	}
}

func TestResilientTransport_RecordsFailureOn5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cb := NewCircuitBreaker(2, time.Second) // Opens after 2 failures
	// Use fast retry config to avoid slow test execution
	fastRetry := &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Jitter:         false,
	}
	client, err := NewClientWithURL("test-key", server.URL,
		WithCircuitBreaker(cb),
		WithRetryConfig(fastRetry))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Make requests until circuit opens
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", server.URL+"/test", nil)
		_, _ = client.httpClient.Do(req)
	}

	// Circuit should be open now
	if cb.Allow() {
		t.Error("circuit breaker should be open after failures")
	}
}
