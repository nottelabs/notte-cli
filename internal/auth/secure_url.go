package auth

import (
	"errors"
	"fmt"
	"net/url"
)

// ErrInsecureAPIURL means a saved API key would have been sent over plain HTTP
// to another machine, where anyone on the network path could read it.
var ErrInsecureAPIURL = errors.New("plain HTTP is only allowed for localhost, 127.0.0.1 and ::1; use an https:// API URL")

// loopbackHosts may receive a saved key over plain HTTP: the request does not
// leave this machine.
var loopbackHosts = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}

// IsSecureAPIURL reports whether a saved API key may be sent to apiURL: over
// HTTPS, or to a server on this machine.
func IsSecureAPIURL(apiURL string) bool {
	u, err := url.Parse(apiURL)
	if err != nil {
		return false
	}
	return u.Scheme == "https" || loopbackHosts[u.Hostname()]
}

// CheckSavedKeyURL returns an error unless a key saved by `notte auth login`
// (or in the config file) may be sent to apiURL. A key passed through
// NOTTE_API_KEY is the caller's explicit choice and is not checked.
func CheckSavedKeyURL(apiURL string) error {
	if IsSecureAPIURL(apiURL) {
		return nil
	}
	return fmt.Errorf("refusing to send the saved API key to %s: %w", apiURL, ErrInsecureAPIURL)
}
