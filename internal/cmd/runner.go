package cmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

// APIFunc is a function that calls the API and returns a response and error.
// T is the result type returned by the API call.
// The function returns the result, HTTP response, response body bytes, and error.
type APIFunc[T any] func(ctx context.Context, client *api.ClientWithResponses) (*T, *http.Response, []byte, error)

// RunAPICommand handles the common boilerplate for API commands:
//   - Creates authenticated client
//   - Sets up context with timeout
//   - Calls the API function
//   - Handles response errors
//   - Prints the result
func RunAPICommand[T any](cmd *cobra.Command, apiFn APIFunc[T]) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	result, httpResp, body, err := apiFn(ctx, client.Client())
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(httpResp, body); err != nil {
		return err
	}

	return GetFormatter().Print(result)
}
