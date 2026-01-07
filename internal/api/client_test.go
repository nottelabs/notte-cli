package api

import (
	"testing"
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
