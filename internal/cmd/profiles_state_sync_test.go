package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

func liveSyncFixture(t *testing.T, empty bool) string {
	t.Helper()
	u := websocket.Upgrader{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := u.Upgrade(w, r, nil)
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
			var result any = map[string]any{"cookies": []any{}}
			if request.Method == "Runtime.evaluate" {
				local := map[string]string{}
				if !empty {
					local["BSKY_STORAGE"] = "synthetic-private-token"
				}
				snapshot, _ := json.Marshal(map[string]any{"origin": "https://bsky.app", "local": local, "session": map[string]string{}})
				result = map[string]any{"result": map[string]any{"value": string(snapshot)}}
			}
			if err := conn.WriteJSON(map[string]any{"id": request.ID, "result": result}); err != nil {
				return
			}
		}
	}))
	t.Cleanup(s.Close)
	return strings.Replace(s.URL, "http:", "ws:", 1)
}

func configureLiveSyncTest(t *testing.T, endpoint string) {
	t.Helper()
	oldID, oldCDP, oldBrowser, oldLocal := profileID, syncCDPURL, syncBrowser, syncChromeProfile
	oldDomains, oldMode, oldName := syncDomains, syncMode, syncProfileName
	oldConfirm, oldOutput := skipConfirmation, outputFormat
	t.Cleanup(func() {
		profileID, syncCDPURL, syncBrowser, syncChromeProfile = oldID, oldCDP, oldBrowser, oldLocal
		syncDomains, syncMode, syncProfileName = oldDomains, oldMode, oldName
		skipConfirmation, outputFormat = oldConfirm, oldOutput
	})
	profileID, syncCDPURL, syncBrowser, syncChromeProfile = "", endpoint, "", ""
	syncDomains, syncMode, syncProfileName = []string{"bsky.app"}, "replace", "Synthetic"
	skipConfirmation, outputFormat = true, "json"
}

func TestLiveSyncCommandStorageOnlyAndFailureCleanup(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, status := range []int{200, 404, 422, 500} {
			t.Run(fmt.Sprintf("existing-%t-status-%d", existing, status), func(t *testing.T) {
				env := testutil.SetupTestEnv(t)
				env.SetEnv("NOTTE_API_KEY", "test-key")
				configureLiveSyncTest(t, liveSyncFixture(t, false))
				if existing {
					profileID = profileIDTest
				}
				creates, uploads, deletes := 0, 0, 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Authorization") != "Bearer test-key" {
						t.Error("missing authentication")
					}
					w.Header().Set("Content-Type", "application/json")
					switch {
					case r.URL.Path == "/profiles/create":
						creates++
						_, _ = w.Write([]byte(profileJSON()))
					case strings.HasSuffix(r.URL.Path, "/state"):
						uploads++
						var body map[string]json.RawMessage
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						}
						if _, ok := body["session_storage"]; !ok {
							t.Error("live sync must capture session storage explicitly")
						}
						w.WriteHeader(status)
						if status == 200 {
							_, _ = w.Write([]byte(`{"success":true,"cookies_count":0,"local_storage_origins":1,"session_storage_origins":1}`))
						} else {
							_, _ = w.Write([]byte(`{"detail":"synthetic-private-token"}`))
						}
					case r.Method == "DELETE":
						deletes++
						_, _ = w.Write([]byte(`{"success":true}`))
					default:
						t.Error("unexpected API request", r.Method, r.URL.Path)
					}
				}))
				defer server.Close()
				env.SetEnv("NOTTE_API_URL", server.URL)
				cmd := &cobra.Command{}
				cmd.SetContext(context.Background())
				var runErr error
				stdout, stderr := testutil.CaptureOutput(func() { runErr = runProfilesSync(cmd, nil) })
				if status == 200 {
					if runErr != nil {
						t.Fatal(runErr)
					}
					var metadata map[string]any
					if err := json.Unmarshal([]byte(stdout), &metadata); err != nil {
						t.Fatal(err)
					}
					if metadata["profile_id"] != profileIDTest || metadata["cookies_count"] != float64(0) {
						t.Fatal("invalid metadata")
					}
				} else if runErr == nil || strings.Contains(runErr.Error(), "synthetic-private-token") {
					t.Fatal("failed import reported success or leaked values")
				}
				if strings.Contains(stdout+stderr, "synthetic-private-token") {
					t.Fatal("browser credentials appeared in output")
				}
				wantCreates, wantDeletes := 0, 0
				if !existing {
					wantCreates = 1
					if status >= 400 && status < 500 {
						wantDeletes = 1
					}
				}
				if creates != wantCreates || deletes != wantDeletes || uploads != 1 {
					t.Fatalf("creates=%d deletes=%d uploads=%d", creates, deletes, uploads)
				}
			})
		}
	}
}

func TestEmptyLiveSyncNeverCreatesProfile(t *testing.T) {
	configureLiveSyncTest(t, liveSyncFixture(t, true))
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runProfilesSync(cmd, nil); err == nil || !strings.Contains(err.Error(), "no cookies or web storage") {
		t.Fatal("empty snapshot must fail before accessing the API")
	}
}
