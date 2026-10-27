package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// BrowserStateUploadRequest uses the cloud profile's existing browser-state
// format. Domain-scoped replace leaves unrelated cloud logins unchanged.
type BrowserStateUploadRequest struct {
	Cookies        []Cookie                      `json:"cookies"`
	LocalStorage   map[string]map[string]string  `json:"local_storage"`
	SessionStorage *map[string]map[string]string `json:"session_storage,omitempty"`
	Domains        []string                      `json:"domains,omitempty"`
	Mode           string                        `json:"mode"`
	StorageScope   string                        `json:"storage_scope,omitempty"`
}

type BrowserStateUploadResponse struct {
	Success               bool `json:"success"`
	CookiesCount          int  `json:"cookies_count"`
	LocalStorageOrigins   int  `json:"local_storage_origins"`
	SessionStorageOrigins int  `json:"session_storage_origins"`
}

// Validate checks the final wire representation before a destination is created.
func (state BrowserStateUploadRequest) Validate() error {
	body, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("cannot encode browser state")
	}
	if len(body) > 8*1024*1024 {
		return fmt.Errorf("browser state exceeds the 8 MiB import limit")
	}
	return nil
}

func (c *NotteClient) ProfileStateSetWithResponse(ctx context.Context, id string, state BrowserStateUploadRequest) (*BrowserStateUploadResponse, int, error) {
	if err := state.Validate(); err != nil {
		return nil, 0, err
	}
	body, _ := json.Marshal(state)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL(), "/")+"/profiles/"+url.PathEscape(id)+"/state", bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	// Redirects must never forward a body containing browser credentials.
	httpClient := *c.HTTPClient()
	httpClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("profile state import could not be confirmed; inspect the profile before retrying")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return nil, resp.StatusCode, fmt.Errorf("this API does not support browser-state import yet; update the server before syncing storage-based logins")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("profile state import failed (HTTP %d); browser values were not printed", resp.StatusCode)
	}
	var result BrowserStateUploadResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&result); err != nil || !result.Success {
		return nil, resp.StatusCode, fmt.Errorf("profile state import returned an invalid confirmation")
	}
	return &result, resp.StatusCode, nil
}
