// internal/testutil/httpserver_test.go
package testutil

import (
	"io"
	"net/http"
	"testing"
)

func TestMockServer_ReturnsResponse(t *testing.T) {
	server := NewMockServer()
	defer server.Close()

	server.AddResponse("/test", http.StatusOK, `{"message": "hello"}`)

	resp, err := http.Get(server.URL() + "/test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"message": "hello"}` {
		t.Errorf("got body %q, want %q", string(body), `{"message": "hello"}`)
	}
}

func TestMockServer_TracksRequests(t *testing.T) {
	server := NewMockServer()
	defer server.Close()

	server.AddResponse("/api/sessions", http.StatusOK, `{}`)

	resp1, _ := http.Get(server.URL() + "/api/sessions")
	if resp1 != nil {
		_ = resp1.Body.Close()
	}
	resp2, _ := http.Get(server.URL() + "/api/sessions")
	if resp2 != nil {
		_ = resp2.Body.Close()
	}

	requests := server.Requests("/api/sessions")
	if len(requests) != 2 {
		t.Errorf("got %d requests, want 2", len(requests))
	}
}

func TestMockServer_Returns404ForUnknownPaths(t *testing.T) {
	server := NewMockServer()
	defer server.Close()

	resp, err := http.Get(server.URL() + "/unknown")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
