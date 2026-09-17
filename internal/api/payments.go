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

// SessionPaymentRequest requests spending in currency units, preserving the decimal text.
type SessionPaymentRequest struct {
	Amount       string `json:"amount"`
	Currency     string `json:"currency"`
	MerchantURL  string `json:"merchant_url"`
	MerchantName string `json:"merchant_name"`
	Description  string `json:"description"`
	Mode         string `json:"mode,omitempty"`
}

// PaymentVerificationAction describes verification completed by the wallet owner.
type PaymentVerificationAction struct {
	Type       string  `json:"type"`
	Resolution string  `json:"resolution"`
	ActionURL  *string `json:"action_url,omitempty"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
}

// PaymentStatus contains only the public, sanitized payment fields.
// Ready indicates that credentials are installed, not that a purchase succeeded.
type PaymentStatus struct {
	ID               string                     `json:"id"`
	SessionID        string                     `json:"session_id"`
	Status           string                     `json:"status"`
	Mode             string                     `json:"mode"`
	Amount           json.Number                `json:"amount"`
	Currency         string                     `json:"currency"`
	MerchantName     string                     `json:"merchant_name"`
	MerchantURL      string                     `json:"merchant_url"`
	VaultID          *string                    `json:"vault_id,omitempty"`
	ApprovalURL      *string                    `json:"approval_url,omitempty"`
	ConnectionURL    *string                    `json:"connection_url,omitempty"`
	ConnectionPhrase *string                    `json:"connection_phrase,omitempty"`
	ExpiresAt        *string                    `json:"expires_at,omitempty"`
	ErrorCode        *string                    `json:"error_code,omitempty"`
	NextAction       *PaymentVerificationAction `json:"next_action,omitempty"`
	PurchaseStatus   string                     `json:"purchase_status"`
}

// Payment calls the payment endpoints through the shared authenticated transport.
// These endpoints are kept separate from the periodically regenerated client.
func (c *NotteClient) Payment(ctx context.Context, sessionID, paymentID, key string, payload *SessionPaymentRequest) (*PaymentStatus, *http.Response, []byte, error) {
	method, path := http.MethodGet, "/payments/"+url.PathEscape(paymentID)
	var body []byte
	var err error
	if payload != nil {
		method, path = http.MethodPost, "/sessions/"+url.PathEscape(sessionID)+"/payments"
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL(), "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, nil, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(IdempotencyKeyHeader, key)
	}
	resp, err := c.HTTPClient().Do(req)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp, body, nil
	}
	var result PaymentStatus
	err = json.Unmarshal(body, &result)
	return &result, resp, body, err
}

// WalletConnectionStatus never exposes device credentials or wallet tokens.
type WalletConnectionStatus struct {
	ID               string  `json:"id"`
	Mode             string  `json:"mode"`
	Status           string  `json:"status"`
	ConnectionURL    *string `json:"connection_url,omitempty"`
	ConnectionPhrase *string `json:"connection_phrase,omitempty"`
	ExpiresAt        *string `json:"expires_at,omitempty"`
	ErrorCode        *string `json:"error_code,omitempty"`
}

// ConnectPaymentWallet creates or retrieves the caller's connection for a mode.
func (c *NotteClient) ConnectPaymentWallet(ctx context.Context, mode string) (*WalletConnectionStatus, *http.Response, []byte, error) {
	return c.paymentWalletOperation(ctx, "/payments/connect", mode)
}

// DisconnectPaymentWallet revokes and forgets one mode without affecting the other.
func (c *NotteClient) DisconnectPaymentWallet(ctx context.Context, mode string) (*WalletConnectionStatus, *http.Response, []byte, error) {
	return c.paymentWalletOperation(ctx, "/payments/disconnect", mode)
}

func (c *NotteClient) paymentWalletOperation(ctx context.Context, path, mode string) (*WalletConnectionStatus, *http.Response, []byte, error) {
	payload := struct {
		Mode string `json:"mode,omitempty"`
	}{Mode: mode}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL(), "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
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
	var result WalletConnectionStatus
	err = json.Unmarshal(raw, &result)
	return &result, resp, raw, err
}
