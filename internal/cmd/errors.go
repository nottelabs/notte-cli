package cmd

import (
	"fmt"
	"net/http"

	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/errors"
)

// The API names the class of an error in this response header. These three
// classes are answers to an access token; other refusals (an invalid API key,
// a plan limit) carry other classes.
const (
	errorClassHeader                = "X-Error-Class"
	errorClassAccessTokenExpired    = "NotteApiAccessTokenExpiredError"
	errorClassAccessTokenInvalid    = "NotteApiAccessTokenInvalidError"
	errorClassAccessTokenNotAllowed = "NotteApiAccessTokenNotAllowedError"
)

// HandleAPIResponse checks if the response is successful and returns an appropriate error.
// The body parameter should contain the already-read response body bytes
// (from the generated client's resp.Body field).
// Returns nil if status code is 2xx, otherwise returns a typed error.
func HandleAPIResponse(resp *http.Response, body []byte) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	err := errors.ParseAPIError(resp, body)
	return withAccessTokenHint(err, resp.Header.Get(errorClassHeader))
}

// withAccessTokenHint adds what to do to an error the API classed as an
// access-token refusal, which calls for a different fix than a rejected API
// key. The API's message is kept as is. Errors without one of those classes,
// including every error from a server that does not send the header, are
// returned unchanged.
func withAccessTokenHint(err error, class string) error {
	authErr, ok := err.(*errors.AuthError)
	if !ok {
		return err
	}
	hinted := *authErr
	switch class {
	case errorClassAccessTokenExpired:
		hinted.Reason = "expired"
		hinted.Hint = replaceTokenHint("has expired")
	case errorClassAccessTokenInvalid:
		hinted.Hint = replaceTokenHint("is unknown or revoked")
	case errorClassAccessTokenNotAllowed:
		hinted.Hint = "this command needs an API key; access tokens are not accepted on its route"
	default:
		return err
	}
	return &hinted
}

// replaceTokenHint says the access token needs replacing. When it came from
// NOTTE_API_KEY_FILE, the hint names the file and leaves the new token to
// whatever writes it: the agent calling the CLI usually cannot mint one.
func replaceTokenHint(state string) string {
	if path := auth.APIKeyFile(); path != "" {
		return fmt.Sprintf("the access token in %s (%s) %s; whatever writes that file must replace it",
			auth.EnvAPIKeyFile, path, state)
	}
	return "the access token " + state + "; use a new one"
}
