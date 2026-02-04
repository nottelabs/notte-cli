package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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
	// NewClient never returns nil client with nil error, so no need to check
	if client.apiKey != "test-api-key" {
		t.Errorf("got apiKey %q, want %q", client.apiKey, "test-api-key")
	}
	if client.baseURL != DefaultBaseURL {
		t.Errorf("got baseURL %q, want %q", client.baseURL, DefaultBaseURL)
	}
}

func TestNewClientWithURL_CustomURL(t *testing.T) {
	client, err := NewClientWithURL("test-key", "https://custom.api.com", "v1.0.0")
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

	client, err := NewClientWithURL("test-api-key", server.URL, "v1.0.0")
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

func TestResilientTransport_AddsTrackingHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("x-notte-request-origin")
		if origin != "cli" {
			t.Errorf("got x-notte-request-origin %q, want %q", origin, "cli")
		}
		version := r.Header.Get("x-notte-sdk-version")
		if version != "v1.2.3" {
			t.Errorf("got x-notte-sdk-version %q, want %q", version, "v1.2.3")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, err := NewClientWithURL("test-api-key", server.URL, "v1.2.3")
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
	client, err := NewClientWithURL("test-key", server.URL, "v1.0.0",
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

func TestResilientTransport_RoundTrip_CircuitOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, time.Hour)
	cb.RecordFailure()

	rt := &resilientTransport{
		apiKey:         "test-key",
		retryConfig:    &RetryConfig{MaxRetries: 0},
		circuitBreaker: cb,
		base: transportFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatal("base RoundTrip should not be called when circuit is open")
			return nil, nil
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, err := rt.RoundTrip(req)
	if err == nil {
		t.Fatal("expected circuit breaker error")
	}
	if resp != nil {
		t.Errorf("expected nil response, got %#v", resp)
	}
}

func TestResilientTransport_RoundTrip_AddsIdempotencyKey(t *testing.T) {
	cb := NewCircuitBreaker(5, time.Minute)
	rt := &resilientTransport{
		apiKey:         "test-key",
		retryConfig:    &RetryConfig{MaxRetries: 0, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, Jitter: false},
		circuitBreaker: cb,
		base: transportFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("Authorization header = %q", got)
			}
			if req.Method == http.MethodPost {
				key := req.Header.Get(IdempotencyKeyHeader)
				if key == "" {
					t.Fatal("expected idempotency key for POST")
				}
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("{}")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
}

func TestResilientTransport_RoundTrip_RecordsFailureOnError(t *testing.T) {
	cb := NewCircuitBreaker(1, time.Hour)
	rt := &resilientTransport{
		apiKey:         "test-key",
		retryConfig:    &RetryConfig{MaxRetries: 0, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, Jitter: false},
		circuitBreaker: cb,
		base: transportFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("network error")
		}),
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)
	_, err := rt.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if cb.Allow() {
		t.Error("expected circuit breaker to record failure")
	}
}

func TestResilientTransport_RoundTrip_RecordsFailureOn5xx(t *testing.T) {
	cb := NewCircuitBreaker(1, time.Hour)
	rt := &resilientTransport{
		apiKey:         "test-key",
		retryConfig:    &RetryConfig{MaxRetries: 0, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, Jitter: false},
		circuitBreaker: cb,
		base: transportFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("{}")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if cb.Allow() {
		t.Error("expected circuit breaker to open after 5xx")
	}
}

func TestResilientTransport_DoWithRetry_RetriesOnStatus(t *testing.T) {
	callCount := 0
	rt := &resilientTransport{
		retryConfig:    &RetryConfig{MaxRetries: 1, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, Jitter: false},
		circuitBreaker: NewCircuitBreaker(5, time.Minute),
		base: transportFunc(func(req *http.Request) (*http.Response, error) {
			callCount++
			status := http.StatusBadGateway
			if callCount > 1 {
				status = http.StatusOK
			}
			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(strings.NewReader("{}")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, err := rt.doWithRetry(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestResilientTransport_DoWithRetry_RetriesOnNetworkError(t *testing.T) {
	callCount := 0
	rt := &resilientTransport{
		retryConfig:    &RetryConfig{MaxRetries: 1, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, Jitter: false},
		circuitBreaker: NewCircuitBreaker(5, time.Minute),
		base: transportFunc(func(req *http.Request) (*http.Response, error) {
			callCount++
			if callCount == 1 {
				return nil, errors.New("network error")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("{}")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, err := rt.doWithRetry(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestResilientTransport_DoWithRetry_NonIdempotentError(t *testing.T) {
	callCount := 0
	rt := &resilientTransport{
		retryConfig:    &RetryConfig{MaxRetries: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, Jitter: false},
		circuitBreaker: NewCircuitBreaker(5, time.Minute),
		base: transportFunc(func(req *http.Request) (*http.Response, error) {
			callCount++
			return nil, errors.New("network error")
		}),
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)
	_, err := rt.doWithRetry(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestNotteClient_Client(t *testing.T) {
	client, err := NewClient("test-key")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	inner := client.Client()
	if inner == nil {
		t.Error("Client() should return non-nil ClientWithResponses")
	}
}

func TestNotteClient_APIKey(t *testing.T) {
	client, err := NewClient("test-api-key-123")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if got := client.APIKey(); got != "test-api-key-123" {
		t.Errorf("APIKey() = %q, want %q", got, "test-api-key-123")
	}
}

func TestDefaultContext(t *testing.T) {
	ctx := DefaultContext()
	if ctx == nil {
		t.Error("DefaultContext() should return non-nil context")
	}
	if ctx.Err() != nil {
		t.Errorf("DefaultContext() should not have error: %v", ctx.Err())
	}
	if _, ok := ctx.Deadline(); ok {
		t.Error("DefaultContext() should not have deadline")
	}
	if ctx != context.Background() {
		t.Error("DefaultContext() should return context.Background()")
	}
}
