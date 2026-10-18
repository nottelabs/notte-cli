package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImportProfileStorageOnly(t *testing.T) {
	for _, live := range []bool{false, true} {
		t.Run(fmt.Sprint(live), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer synthetic-key" {
					t.Error("missing workspace authentication")
				}
				if r.URL.Path != "/profiles/notte-profile-0123456789abcdef/state" {
					t.Error("wrong import route")
				}
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if _, ok := body["session_storage"]; ok != live {
					t.Error("uncaptured session storage must be omitted; captured empty state must be explicit")
				}
				_, _ = w.Write([]byte(`{"success":true,"cookies_count":0,"local_storage_origins":1,"session_storage_origins":0}`))
			}))
			defer server.Close()
			client, err := NewClientWithURL("synthetic-key", server.URL, "")
			if err != nil {
				t.Fatal(err)
			}
			state := BrowserStateUploadRequest{Cookies: []Cookie{}, LocalStorage: map[string]map[string]string{"https://bsky.app": {"BSKY_STORAGE": "synthetic-private-token"}}, Mode: "replace"}
			if live {
				sessions := map[string]map[string]string{}
				state.SessionStorage = &sessions
			}
			response, status, err := client.ProfileStateSetWithResponse(context.Background(), "notte-profile-0123456789abcdef", state)
			if err != nil || status != 200 || response.LocalStorageOrigins != 1 {
				t.Fatalf("import failed: %v", err)
			}
		})
	}
}

func TestImportProfileStateNeverLeaksBodyOrFollowsRedirects(t *testing.T) {
	for _, code := range []int{307, 308, 400, 401, 403, 404, 405, 422, 500} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			forwarded := false
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true; w.WriteHeader(200) }))
			defer target.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(code)
				_, _ = w.Write([]byte("synthetic-private-token"))
			}))
			defer server.Close()
			client, err := NewClientWithURL("synthetic-key", server.URL, "")
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = client.ProfileStateSetWithResponse(context.Background(), "profile", BrowserStateUploadRequest{LocalStorage: map[string]map[string]string{"https://bsky.app": {"token": "synthetic-private-token"}}})
			if err == nil || strings.Contains(err.Error(), "synthetic-private-token") || forwarded {
				t.Fatal("import failure leaked or forwarded browser credentials")
			}
		})
	}
}
