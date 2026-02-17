package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	notteErrors "github.com/nottelabs/notte-cli/internal/errors"
)

const DefaultBaseURL = "https://api.notte.cc"

// NotteClient wraps the generated client with auth and resilience
type NotteClient struct {
	client         *ClientWithResponses
	httpClient     *http.Client
	baseURL        string
	apiKey         string
	retryConfig    *RetryConfig
	circuitBreaker *CircuitBreaker
}

// NotteClientOption configures the NotteClient
type NotteClientOption func(*NotteClient)

// WithRetryConfig sets custom retry configuration
func WithRetryConfig(cfg *RetryConfig) NotteClientOption {
	return func(c *NotteClient) {
		c.retryConfig = cfg
	}
}

// WithCircuitBreaker sets custom circuit breaker
func WithCircuitBreaker(cb *CircuitBreaker) NotteClientOption {
	return func(c *NotteClient) {
		c.circuitBreaker = cb
	}
}

// NewClient creates a new Notte API client
func NewClient(apiKey string, opts ...NotteClientOption) (*NotteClient, error) {
	return NewClientWithURL(apiKey, DefaultBaseURL, "", opts...)
}

// NewClientWithURL creates a client with custom base URL
func NewClientWithURL(apiKey, baseURL, version string, opts ...NotteClientOption) (*NotteClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	nc := &NotteClient{
		baseURL:        baseURL,
		apiKey:         apiKey,
		retryConfig:    DefaultRetryConfig(),
		circuitBreaker: NewCircuitBreaker(5, 30*time.Second),
	}

	// Apply options
	for _, opt := range opts {
		opt(nc)
	}

	// Create HTTP client with TLS 1.2+ and connection pooling
	nc.httpClient = &http.Client{
		Timeout: 45 * time.Second,
		Transport: &resilientTransport{
			apiKey:         apiKey,
			version:        version,
			retryConfig:    nc.retryConfig,
			circuitBreaker: nc.circuitBreaker,
			base: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}

	client, err := NewClientWithResponses(baseURL, WithHTTPClient(nc.httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	nc.client = client
	return nc, nil
}

// resilientTransport wraps http.RoundTripper with auth, retry, and circuit breaker
type resilientTransport struct {
	apiKey         string
	version        string
	retryConfig    *RetryConfig
	circuitBreaker *CircuitBreaker
	base           http.RoundTripper
}

func (t *resilientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Check circuit breaker
	if !t.circuitBreaker.Allow() {
		return nil, &notteErrors.CircuitBreakerError{
			OpenUntil: t.circuitBreaker.OpenUntil(),
		}
	}

	// Add auth header
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	// Add tracking headers
	req.Header.Set("x-notte-request-origin", "cli")
	req.Header.Set("x-notte-sdk-version", t.version)

	// Add idempotency key for mutating requests
	AddIdempotencyKey(req)

	// Execute with retry
	resp, err := t.doWithRetry(req)
	if err != nil {
		t.circuitBreaker.RecordFailure()
		return nil, err
	}

	// Record success/failure for circuit breaker
	if resp.StatusCode >= 500 {
		t.circuitBreaker.RecordFailure()
	} else {
		t.circuitBreaker.RecordSuccess()
	}

	return resp, nil
}

func (t *resilientTransport) doWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= t.retryConfig.MaxRetries; attempt++ {
		// Clone request for each attempt
		reqCopy := cloneRequest(req)

		resp, err = t.base.RoundTrip(reqCopy)
		if err != nil {
			// Network error - retry for idempotent methods
			if !isIdempotent(req.Method) {
				return nil, err
			}
			if attempt < t.retryConfig.MaxRetries {
				time.Sleep(t.retryConfig.Backoff(attempt))
				continue
			}
			return nil, err
		}

		// Check if we should retry based on status
		if !t.retryConfig.ShouldRetry(resp.StatusCode, req.Method, attempt) {
			return resp, nil
		}

		// Close response body before retry
		_ = resp.Body.Close()

		// Sleep before retry
		if attempt < t.retryConfig.MaxRetries {
			time.Sleep(t.retryConfig.Backoff(attempt))
		}
	}

	return resp, err
}

// cloneRequest creates a shallow copy of the request
func cloneRequest(req *http.Request) *http.Request {
	reqCopy := req.Clone(req.Context())
	// Copy headers since Clone doesn't deep copy them
	reqCopy.Header = req.Header.Clone()

	// Fix: If GetBody is available, use it to get a fresh body for the retry
	if reqCopy.GetBody != nil {
		body, err := reqCopy.GetBody()
		if err == nil {
			reqCopy.Body = body
		}
	}

	return reqCopy
}

// Client returns the underlying generated client for direct access
func (c *NotteClient) Client() *ClientWithResponses {
	return c.client
}

// HTTPClient returns the underlying HTTP client for making raw requests
// This client includes authentication, retry logic, and circuit breaker
func (c *NotteClient) HTTPClient() *http.Client {
	return c.httpClient
}

// BaseURL returns the base URL for the API
func (c *NotteClient) BaseURL() string {
	return c.baseURL
}

// APIKey returns the API key used by the client
func (c *NotteClient) APIKey() string {
	return c.apiKey
}

// Context helper for commands
func DefaultContext() context.Context {
	return context.Background()
}
