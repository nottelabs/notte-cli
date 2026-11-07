package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nottelabs/notte-cli/internal/config"
)

// `notte login` uses the OAuth 2.0 device authorization grant (RFC 8628).
//
// The CLI asks the API for a code pair, prints a console link with the short
// user code, and polls the API until the user has approved the code in the
// console. The API then creates an API key named after this machine and
// returns it once. Nothing listens on this machine, so the browser can be on
// another device: over SSH, in a container or in an agent sandbox.
//
// The endpoints live on the API, not the console: the console's bot
// protection answers non-browser requests with HTTP 429.

const (
	deviceCodePath   = "/auth/device/code"
	deviceTokenPath  = "/auth/device/token"
	deviceRevokePath = "/auth/device/revoke"

	deviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"

	deviceRequestTimeout = 15 * time.Second
	// RFC 8628 section 3.5: add 5 seconds to the interval on every slow_down.
	slowDownStep = 5 * time.Second
)

var (
	// ErrLoginDenied means the user pressed Cancel in the console.
	ErrLoginDenied = errors.New("the login was cancelled in the console")
	// ErrLoginExpired means the code was not approved in time.
	ErrLoginExpired = errors.New("the login code expired before it was approved; run `notte login` again")
)

// DeviceClientInfo is what the console shows about this machine, labelled
// there as reported by the device and not verified.
type DeviceClientInfo struct {
	Hostname string `json:"client_hostname,omitempty"`
	OS       string `json:"client_os,omitempty"`
	Version  string `json:"client_version,omitempty"`
}

// DeviceCode is the API's answer to a login request (RFC 8628 section 3.2).
type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// DeviceToken is the API key issued once the login was approved.
type DeviceToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	// KeyName is the name the key has on the console's API keys page.
	KeyName string `json:"key_name"`
	Email   string `json:"email"`
	// WorkspaceName is empty for the personal workspace.
	WorkspaceName string `json:"workspace_name"`
}

type oauthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *oauthError) Error() string {
	if e.Description != "" {
		return e.Description
	}
	return e.Code
}

// DeviceFlow talks to the API's device endpoints. The zero value is not
// usable; build one with NewDeviceFlow.
type DeviceFlow struct {
	APIURL     string
	HTTPClient *http.Client
	UserAgent  string
	// Sleep waits between polls. Tests replace it to run without waiting.
	Sleep func(ctx context.Context, d time.Duration) error
}

// NewDeviceFlow targets apiURL with the default HTTP client and real waits.
func NewDeviceFlow(apiURL, version string) *DeviceFlow {
	return &DeviceFlow{
		APIURL:     strings.TrimRight(apiURL, "/"),
		HTTPClient: &http.Client{Timeout: deviceRequestTimeout},
		UserAgent:  "notte-cli/" + version,
		Sleep:      sleepContext,
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (f *DeviceFlow) post(ctx context.Context, path, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.APIURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	if f.UserAgent != "" {
		req.Header.Set("User-Agent", f.UserAgent)
	}
	return f.HTTPClient.Do(req)
}

// RequestCode starts a login and returns the codes to show the user.
func (f *DeviceFlow) RequestCode(ctx context.Context, info DeviceClientInfo) (*DeviceCode, error) {
	payload, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}
	resp, err := f.post(ctx, deviceCodePath, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("could not reach %s: %w", f.APIURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, responseError(resp)
	}
	var code DeviceCode
	if err := json.NewDecoder(resp.Body).Decode(&code); err != nil {
		return nil, fmt.Errorf("unexpected login response: %w", err)
	}
	if code.DeviceCode == "" || code.UserCode == "" || code.VerificationURI == "" {
		return nil, errors.New("unexpected login response: missing codes")
	}
	return &code, nil
}

// Poll waits for the user to approve code and returns the issued key.
//
// It follows RFC 8628 section 3.5: authorization_pending keeps polling,
// slow_down adds 5 seconds to the interval, access_denied and expired_token
// end the login. Transient failures (a dropped connection, a 5xx) are retried
// until the code expires, since the user may still be approving it.
func (f *DeviceFlow) Poll(ctx context.Context, code *DeviceCode) (*DeviceToken, error) {
	interval := time.Duration(code.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	expiresIn := time.Duration(code.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, expiresIn)
	defer cancel()

	for {
		if err := f.Sleep(ctx, interval); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, ErrLoginExpired
			}
			return nil, err
		}

		token, oerr, err := f.pollOnce(ctx, code.DeviceCode)
		var permanent *permanentPollError
		switch {
		case errors.As(err, &permanent):
			return nil, fmt.Errorf("login failed: %w", permanent.err)
		case err != nil:
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, ErrLoginExpired
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue // Transient: try again at the next interval.
		case token != nil:
			return token, nil
		}

		switch oerr.Code {
		case "authorization_pending":
		case "slow_down":
			interval += slowDownStep
		case "access_denied":
			return nil, ErrLoginDenied
		case "expired_token":
			return nil, ErrLoginExpired
		default:
			return nil, fmt.Errorf("login failed: %w", oerr)
		}
	}
}

// permanentPollError is a poll failure that waiting will not fix: a 4xx that is
// not an OAuth answer (a proxy's 403, a route that does not exist), or a 200
// whose key could not be read. The latter cannot be retried either: the API
// hands a key out once.
type permanentPollError struct{ err error }

func (e *permanentPollError) Error() string { return e.err.Error() }

// pollOnce returns the token, an OAuth error to act on, a transient error
// (connection failure, 5xx, 429), or a *permanentPollError.
func (f *DeviceFlow) pollOnce(ctx context.Context, deviceCode string) (*DeviceToken, *oauthError, error) {
	form := url.Values{"grant_type": {deviceCodeGrantType}, "device_code": {deviceCode}}
	resp, err := f.post(ctx, deviceTokenPath, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		var token DeviceToken
		if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
			return nil, nil, &permanentPollError{fmt.Errorf("unexpected token response: %w", err)}
		}
		if token.AccessToken == "" {
			return nil, nil, &permanentPollError{errors.New("unexpected token response: no key")}
		}
		return &token, nil, nil
	}

	var oerr oauthError
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if json.Unmarshal(body, &oerr) == nil && oerr.Code != "" {
		// A 5xx with an OAuth body (server_error) is still transient.
		if resp.StatusCode >= 500 {
			return nil, nil, &oerr
		}
		return nil, &oerr, nil
	}
	switch {
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
		return nil, nil, fmt.Errorf("unexpected status %s", resp.Status)
	case resp.StatusCode == http.StatusNotFound:
		return nil, nil, &permanentPollError{errors.New("this API does not support `notte login` yet (HTTP 404)")}
	default:
		return nil, nil, &permanentPollError{fmt.Errorf("unexpected status %s", resp.Status)}
	}
}

// Revoke asks the API to revoke a key that a CLI login created. It reports
// false, without error, for any other key (one pasted in by hand, or a
// workspace default key): those are not the CLI's to revoke.
func (f *DeviceFlow) Revoke(ctx context.Context, apiKey string) (bool, error) {
	form := url.Values{"token": {apiKey}}
	resp, err := f.post(ctx, deviceRevokePath, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return false, fmt.Errorf("could not reach %s: %w", f.APIURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, responseError(resp)
	}
	var body struct {
		Revoked bool `json:"revoked"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, fmt.Errorf("unexpected revoke response: %w", err)
	}
	return body.Revoked, nil
}

func responseError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var oerr oauthError
	if json.Unmarshal(body, &oerr) == nil && oerr.Code != "" {
		return fmt.Errorf("%s (HTTP %d)", oerr.Error(), resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("this API does not support `notte login` yet (HTTP 404)")
	}
	return fmt.Errorf("unexpected status %s", resp.Status)
}

// VerificationURL is the link to show the user. The API names the console
// that shares its database; NOTTE_CONSOLE_URL replaces that origin, for a
// local or preview console the API cannot know about.
func VerificationURL(fromAPI string) string {
	override := strings.TrimRight(os.Getenv(config.EnvConsoleURL), "/")
	if override == "" {
		return fromAPI
	}
	target, err := url.Parse(fromAPI)
	if err != nil {
		return fromAPI
	}
	base, err := url.Parse(override)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return fromAPI
	}
	target.Scheme = base.Scheme
	target.Host = base.Host
	target.Path = strings.TrimRight(base.Path, "/") + target.Path
	return target.String()
}
