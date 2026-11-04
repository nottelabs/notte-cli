package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// VaultConnection is a session's password manager connection. It never carries the token.
type VaultConnection struct {
	SessionID string  `json:"session_id"`
	Provider  *string `json:"provider"`
	Connected bool    `json:"connected"`
}

// vaultConnectionRequest connects a password manager to a session.
type vaultConnectionRequest struct {
	Provider string `json:"provider"`
	Token    string `json:"token"`
}

// ConnectVault connects a password manager to a running session: its login fills ask it first.
// These endpoints are kept separate from the periodically regenerated client.
func (c *NotteClient) ConnectVault(ctx context.Context, sessionID, provider, token string) (*VaultConnection, *http.Response, []byte, error) {
	body, err := json.Marshal(vaultConnectionRequest{Provider: provider, Token: token})
	if err != nil {
		return nil, nil, nil, err
	}
	return c.vaultConnection(ctx, http.MethodPost, sessionID, body)
}

// DisconnectVault disconnects the session's password manager. Safe to repeat.
func (c *NotteClient) DisconnectVault(ctx context.Context, sessionID string) (*VaultConnection, *http.Response, []byte, error) {
	return c.vaultConnection(ctx, http.MethodDelete, sessionID, nil)
}

func (c *NotteClient) vaultConnection(ctx context.Context, method, sessionID string, body []byte) (*VaultConnection, *http.Response, []byte, error) {
	path := "/sessions/" + url.PathEscape(sessionID) + "/vault/connection"
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL(), "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient().Do(req)
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
	var result VaultConnection
	err = json.Unmarshal(raw, &result)
	return &result, resp, raw, err
}
