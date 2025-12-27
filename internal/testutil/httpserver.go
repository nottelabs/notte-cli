// internal/testutil/httpserver.go
package testutil

import (
	"net/http"
	"net/http/httptest"
	"sync"
)

// MockResponse represents a canned response
type MockResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

// RecordedRequest stores request details
type RecordedRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    string
}

// MockServer provides a test HTTP server with canned responses
type MockServer struct {
	server    *httptest.Server
	mu        sync.RWMutex
	responses map[string]MockResponse
	requests  map[string][]RecordedRequest
}

// NewMockServer creates a new mock HTTP server
func NewMockServer() *MockServer {
	ms := &MockServer{
		responses: make(map[string]MockResponse),
		requests:  make(map[string][]RecordedRequest),
	}

	ms.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms.recordRequest(r)

		ms.mu.RLock()
		resp, ok := ms.responses[r.URL.Path]
		ms.mu.RUnlock()

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "not found"}`))
			return
		}

		for key, val := range resp.Headers {
			w.Header().Set(key, val)
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write([]byte(resp.Body))
	}))

	return ms
}

func (ms *MockServer) recordRequest(r *http.Request) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	rec := RecordedRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: r.Header.Clone(),
	}

	ms.requests[r.URL.Path] = append(ms.requests[r.URL.Path], rec)
}

// AddResponse adds a canned response for a path
func (ms *MockServer) AddResponse(path string, statusCode int, body string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.responses[path] = MockResponse{
		StatusCode: statusCode,
		Body:       body,
		Headers:    map[string]string{"Content-Type": "application/json"},
	}
}

// AddResponseWithHeaders adds a response with custom headers
func (ms *MockServer) AddResponseWithHeaders(path string, statusCode int, body string, headers map[string]string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.responses[path] = MockResponse{
		StatusCode: statusCode,
		Body:       body,
		Headers:    headers,
	}
}

// URL returns the server's base URL
func (ms *MockServer) URL() string {
	return ms.server.URL
}

// Requests returns recorded requests for a path
func (ms *MockServer) Requests(path string) []RecordedRequest {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return ms.requests[path]
}

// AllRequests returns all recorded requests
func (ms *MockServer) AllRequests() map[string][]RecordedRequest {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	result := make(map[string][]RecordedRequest)
	for k, v := range ms.requests {
		result[k] = v
	}
	return result
}

// Reset clears all responses and recorded requests
func (ms *MockServer) Reset() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.responses = make(map[string]MockResponse)
	ms.requests = make(map[string][]RecordedRequest)
}

// Close shuts down the server
func (ms *MockServer) Close() {
	ms.server.Close()
}
