//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// A local CDP fixture supplies synthetic browser state; profile creation,
// authenticated state import and cleanup use the real integration API.
func TestProfilesSyncStorageOnlyAndMixed(t *testing.T) {
	for _, withCookie := range []bool{false, true} {
		name := "storage_only"
		if withCookie {
			name = "cookies_and_storage"
		}
		t.Run(name, func(t *testing.T) {
			created := runCLI(t, "profiles", "create", "--name", "test-profile-state-sync")
			requireSuccess(t, created)
			var profile struct {
				ID string `json:"profile_id"`
			}
			if err := json.Unmarshal([]byte(created.Stdout), &profile); err != nil || profile.ID == "" {
				t.Fatal("profile creation returned no usable profile ID")
			}
			t.Cleanup(func() { cleanupProfile(t, profile.ID) })

			cookies := []any{}
			if withCookie {
				cookies = append(cookies, map[string]any{
					"name": "login", "value": "synthetic-cookie-token", "domain": "example.com",
					"path": "/", "secure": true, "httpOnly": true, "sameSite": "Lax", "expires": 1893456000,
				})
			}
			snapshot, err := json.Marshal(map[string]any{
				"origin": "https://example.com",
				"local":  map[string]string{"login": "synthetic-local-token"},
				"session": map[string]string{
					"login": "synthetic-session-token",
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			upgrader := websocket.Upgrader{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer func() { _ = conn.Close() }()
				for {
					var request struct {
						ID     int    `json:"id"`
						Method string `json:"method"`
					}
					if err := conn.ReadJSON(&request); err != nil {
						return
					}
					var result any
					switch request.Method {
					case "Network.getAllCookies":
						result = map[string]any{"cookies": cookies}
					case "Runtime.evaluate":
						result = map[string]any{"result": map[string]any{"value": string(snapshot)}}
					default:
						t.Errorf("unexpected browser operation %q", request.Method)
						return
					}
					if err := conn.WriteJSON(map[string]any{"id": request.ID, "result": result}); err != nil {
						return
					}
				}
			}))
			t.Cleanup(server.Close)
			result := runCLI(t, "profiles", "sync", "--profile-id", profile.ID,
				"--cdp-url", strings.Replace(server.URL, "http:", "ws:", 1), "--domain", "example.com")
			for _, token := range []string{"synthetic-cookie-token", "synthetic-local-token", "synthetic-session-token"} {
				if strings.Contains(result.Stdout+result.Stderr, token) {
					t.Fatal("browser values were printed during sync")
				}
			}
			requireSuccess(t, result)
			var synced struct {
				ProfileID      string `json:"profile_id"`
				Created        bool   `json:"created"`
				Cookies        int    `json:"cookies_count"`
				LocalOrigins   int    `json:"local_storage_origins"`
				SessionOrigins int    `json:"session_storage_origins"`
			}
			if err := json.Unmarshal([]byte(result.Stdout), &synced); err != nil {
				t.Fatal("sync returned invalid JSON")
			}
			if synced.ProfileID != profile.ID || synced.Created || synced.Cookies != len(cookies) || synced.LocalOrigins != 1 || synced.SessionOrigins != 1 {
				t.Fatalf("unexpected imported state counts: %+v", synced)
			}
		})
	}
}
