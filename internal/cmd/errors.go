package cmd

import (
	"net/http"

	"github.com/salmonumbrella/notte-cli/internal/errors"
)

// HandleAPIResponse checks the response status and returns an appropriate error.
// Returns nil for successful responses (2xx status codes).
func HandleAPIResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return errors.ParseAPIError(resp)
}
