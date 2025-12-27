// internal/api/retry.go
package api

import (
	"context"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries     int           // Maximum number of retries
	InitialBackoff time.Duration // Initial backoff duration
	MaxBackoff     time.Duration // Maximum backoff duration
	Jitter         bool          // Add random jitter to backoff
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: time.Second,
		MaxBackoff:     30 * time.Second,
		Jitter:         true,
	}
}

// ShouldRetry determines if a request should be retried
func (c *RetryConfig) ShouldRetry(statusCode int, method string, attempt int) bool {
	// Don't retry if we've exceeded max attempts
	if attempt >= c.MaxRetries {
		return false
	}

	// Always retry rate limits
	if statusCode == http.StatusTooManyRequests {
		return true
	}

	// Only retry 5xx for idempotent methods
	if statusCode >= 500 && statusCode < 600 {
		return isIdempotent(method)
	}

	return false
}

// Backoff calculates the backoff duration for an attempt
func (c *RetryConfig) Backoff(attempt int) time.Duration {
	// Exponential backoff: initial * 2^attempt
	backoff := float64(c.InitialBackoff) * math.Pow(2, float64(attempt))

	// Add jitter (±25%) before capping
	if c.Jitter {
		jitter := backoff * 0.25 * (rand.Float64()*2 - 1)
		backoff += jitter
	}

	// Cap at max after jitter
	if backoff > float64(c.MaxBackoff) {
		backoff = float64(c.MaxBackoff)
	}

	return time.Duration(backoff)
}

// isIdempotent returns true for HTTP methods that are safe to retry
func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// DoWithRetry executes an HTTP request with retry logic
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, cfg *RetryConfig) (*http.Response, error) {
	if cfg == nil {
		cfg = DefaultRetryConfig()
	}

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Check context before making request
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Clone request for retry (body needs to be re-readable)
		reqCopy := req.Clone(ctx)

		resp, err = client.Do(reqCopy)
		if err != nil {
			// Network error - retry for idempotent methods
			if !isIdempotent(req.Method) {
				return nil, err
			}
			if attempt < cfg.MaxRetries {
				sleepWithContext(ctx, cfg.Backoff(attempt))
				continue
			}
			return nil, err
		}

		// Check if we should retry based on status
		if !cfg.ShouldRetry(resp.StatusCode, req.Method, attempt) {
			return resp, nil
		}

		// Close response body before retry
		_ = resp.Body.Close()

		// Sleep before retry
		if attempt < cfg.MaxRetries {
			sleepWithContext(ctx, cfg.Backoff(attempt))
		}
	}

	return resp, err
}

// sleepWithContext sleeps for duration or until context is canceled
func sleepWithContext(ctx context.Context, d time.Duration) {
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}
