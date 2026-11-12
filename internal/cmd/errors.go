package cmd

import (
	"net/http"
	"strings"

	"github.com/nottelabs/notte-cli/internal/auth"
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
	err := errors.ParseAPIError(resp, body)
	if sentAccessToken(resp) {
		return accessTokenError(err)
	}
	return err
}

// sentAccessToken reports whether the request behind resp carried an access
// token rather than an API key. resp.Request is the request as sent, with the
// Authorization header the client's transport set.
func sentAccessToken(resp *http.Response) bool {
	if resp.Request == nil {
		return false
	}
	token, ok := strings.CutPrefix(resp.Request.Header.Get("Authorization"), "Bearer ")
	return ok && auth.IsAccessToken(token)
}

// accessTokenError explains a 401 or 403 answered to an access token, which
// call for a different fix than a rejected API key. Other errors are returned
// unchanged.
func accessTokenError(err error) error {
	authErr, ok := err.(*errors.AuthError)
	if !ok {
		return err
	}
	switch authErr.StatusCode {
	case http.StatusUnauthorized:
		const mint = "mint a new one"
		msg := authErr.Message
		switch {
		case msg == "":
			msg = "the access token is expired or revoked; " + mint
		case !strings.Contains(msg, "mint a new"):
			msg += "; the access token is expired or revoked, " + mint
		}
		return &errors.AuthError{Reason: authErr.Reason, Message: msg, StatusCode: authErr.StatusCode}
	case http.StatusForbidden:
		msg := "this command needs an API key: access tokens are not accepted on this route"
		// A refusal for another reason (a plan limit, a missing permission)
		// is kept, so the explanation does not hide it.
		if authErr.Message != "" && !strings.Contains(authErr.Message, "access token") {
			msg += " (" + authErr.Message + ")"
		}
		return &errors.AuthError{Reason: authErr.Reason, Message: msg, StatusCode: authErr.StatusCode}
	}
	return err
}
