package cmd

import (
	"net/http"

	"github.com/nottelabs/notte-cli/internal/errors"
)

// HandleAPIResponse checks the response status and returns an appropriate error.
// Returns nil for successful responses (2xx status codes).
// The body parameter should contain the already-read response body bytes
// (from the generated client's resp.Body field).
func HandleAPIResponse(resp *http.Response, body []byte) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return errors.ParseAPIError(resp, body)
}
