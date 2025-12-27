// internal/api/idempotency.go
package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
)

// IdempotencyKeyHeader is the header name for idempotency keys
const IdempotencyKeyHeader = "Idempotency-Key"

// GenerateIdempotencyKey generates a cryptographically random idempotency key.
// Returns an error if crypto/rand fails (extremely rare).
func GenerateIdempotencyKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate idempotency key: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// IsMutatingMethod returns true for HTTP methods that modify state
func IsMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// AddIdempotencyKey adds an idempotency key header to mutating requests.
// If key generation fails, the request proceeds without an idempotency key.
func AddIdempotencyKey(req *http.Request) {
	if IsMutatingMethod(req.Method) {
		if req.Header.Get(IdempotencyKeyHeader) == "" {
			if key, err := GenerateIdempotencyKey(); err == nil {
				req.Header.Set(IdempotencyKeyHeader, key)
			}
			// On error, proceed without idempotency key rather than failing
		}
	}
}
