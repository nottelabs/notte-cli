package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// AacConnection is the user's password manager connection over Agent Access. It never carries the token.
type AacConnection struct {
	Provider  *string `json:"provider"`
	Connected bool    `json:"connected"`
	// Paired: a fill paired Notte's aac device with the listener; until then the token pairs on the next fill.
	Paired bool `json:"paired"`
}

// aacConnectRequest sends the reusable PSK token of `aac listen --reusable-psk`.
type aacConnectRequest struct {
	Provider string `json:"provider"`
	Token    string `json:"token"`
}

// ConnectAac connects the user's password manager: sessions started with aac_vault fill logins from it.
// These endpoints stay out of the API's OpenAPI schema while Agent Access is in alpha, so they are
// kept separate from the periodically regenerated client.
func (c *NotteClient) ConnectAac(ctx context.Context, provider, token string) (*AacConnection, *http.Response, []byte, error) {
	body, err := json.Marshal(aacConnectRequest{Provider: provider, Token: token})
	if err != nil {
		return nil, nil, nil, err
	}
	return c.aacRequest(ctx, http.MethodPost, "/aac/connect", body)
}

// DisconnectAac deletes the user's password manager connection. Safe to repeat.
func (c *NotteClient) DisconnectAac(ctx context.Context) (*AacConnection, *http.Response, []byte, error) {
	return c.aacRequest(ctx, http.MethodDelete, "/aac/connection", nil)
}

func (c *NotteClient) aacRequest(ctx context.Context, method, path string, body []byte) (*AacConnection, *http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL(), "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Redirects must never forward a body containing the listener token.
	httpClient := *c.HTTPClient()
	httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp, raw, nil
	}
	var result AacConnection
	err = json.Unmarshal(raw, &result)
	return &result, resp, raw, err
}
