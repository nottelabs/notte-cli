package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConnectVaultNeverFollowsARedirectWithTheToken(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef_f00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeed"
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		t.Errorf("the redirect target received %s %s with %d body bytes", r.Method, r.URL.Path, len(body))
	}))
	defer elsewhere.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+r.URL.Path, http.StatusPermanentRedirect)
	}))
	defer api.Close()
	client, err := NewClientWithURL("test-key", api.URL, "")
	if err != nil {
		t.Fatal(err)
	}

	result, resp, body, err := client.ConnectVault(context.Background(), "session", "bitwarden", token)

	if err != nil || result != nil || resp == nil || resp.StatusCode != http.StatusPermanentRedirect {
		t.Fatalf("got %v, %v, %v; want the redirect itself, unfollowed", result, resp, err)
	}
	if strings.Contains(string(body), token) {
		t.Fatal("the token must not be echoed")
	}
}
