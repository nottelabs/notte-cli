package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/nottelabs/notte-cli/internal/api"
)

// ErrInvalidAPIKey means the API answered and rejected the key, as opposed to
// not answering at all. Callers use it to tell a bad credential (which the user
// can fix by getting another key) from an unreachable API (which they cannot).
var ErrInvalidAPIKey = errors.New("the API rejected this key")

const validateTimeout = 10 * time.Second

// ValidateAPIKey reports whether a key can actually be used.
//
// Checked against an endpoint that requires authentication. /health does not:
// it answers 200 with no credential at all, so validating against it only ever
// proved the API was reachable, and any string at all was accepted and stored
// as a working key.
//
// The request goes to the configured API, not the default one. Login stores the
// key under the environment named by NOTTE_API_URL, so validating somewhere else
// can accept a key that does not work where it is about to be saved.
//
// Client options are accepted so callers (and tests) can adjust the retry
// policy; production call sites pass none and get the default.
func ValidateAPIKey(ctx context.Context, apiKey string, opts ...api.NotteClientOption) error {
	client, err := api.NewClientWithURL(apiKey, GetCurrentAPIURL(), "", opts...)
	if err != nil {
		// Only returned for a malformed or empty key, which is a bad
		// credential rather than a transport problem.
		return fmt.Errorf("%w: %v", ErrInvalidAPIKey, err)
	}

	ctx, cancel := context.WithTimeout(ctx, validateTimeout)
	defer cancel()

	resp, err := client.Client().GetUsageWithResponse(ctx, &api.GetUsageParams{})
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	switch code := resp.StatusCode(); code {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrInvalidAPIKey
	default:
		return fmt.Errorf("API error: %s", resp.Status())
	}
}
