package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/browser"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

func liveSyncFixture(t *testing.T, empty bool) string {
	value := "synthetic-private-token"
	if empty {
		value = ""
	}
	return liveSyncValueFixture(t, value)
}

func liveSyncValueFixture(t *testing.T, value string) string {
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
				if value != "" {
					local["BSKY_STORAGE"] = value
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
						if string(body["storage_scope"]) != `"origins"` {
							t.Error("live snapshots must preserve storage for closed tabs")
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

func TestOversizedFinalRequestNeverCreatesProfile(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	configureLiveSyncTest(t, liveSyncValueFixture(t, strings.Repeat("x", browser.MaxStateBytes-4096)))
	// The browser snapshot fits; adding the requested domain scope does not.
	syncDomains = make([]string, 1000)
	for i := range syncDomains {
		syncDomains[i] = "bsky.app"
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("oversized request accessed the API", r.URL.Path)
	}))
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL)
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runProfilesSync(cmd, nil); err == nil || !strings.Contains(err.Error(), "import limit") {
		t.Fatal("final wire size must be checked before profile creation", err)
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

func TestCookieOnlyEndpointCompatibilityAndScope(t *testing.T) {
	state := browser.State{LocalStorage: map[string]map[string]string{"https://bsky.app": {}}}
	if needsStateImport(state, nil, false) {
		t.Fatal("empty storage tables must not require a new server for unscoped disk cookie sync")
	}
	if !needsStateImport(state, []string{"bsky.app"}, false) {
		t.Fatal("scoped cookies require the state API to preserve other sites")
	}
	if !needsStateImport(state, nil, true) {
		t.Fatal("live snapshots must replace captured origins without clearing closed ones")
	}
	state.LocalStorage["https://bsky.app"]["token"] = "synthetic"
	if !needsStateImport(state, nil, false) {
		t.Fatal("storage values must never be silently dropped for cookie API compatibility")
	}
}

func TestLegacyCookieSyncRefusesRedirectsAndRedactsErrors(t *testing.T) {
	if !browser.SupportedPlatform() {
		t.Skip("disk browser sync is not supported on this OS")
	}
	for _, status := range []int{200, 307, 308, 422} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			env := testutil.SetupTestEnv(t)
			env.SetEnv("HOME", env.TempDir)
			env.SetEnv("NOTTE_API_KEY", "test-key")
			configureLiveSyncTest(t, "")
			profileID, syncDomains, syncBrowser, syncChromeProfile = profileIDTest, nil, "firefox", "Synthetic"
			root := filepath.Join(env.TempDir, ".mozilla", "firefox")
			if runtime.GOOS == "darwin" {
				root = filepath.Join(env.TempDir, "Library", "Application Support", "Firefox")
			}
			dir := filepath.Join(root, "synthetic")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "profiles.ini"), []byte("[Profile0]\nName=Synthetic\nIsRelative=1\nPath=synthetic\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("sqlite", filepath.Join(dir, "cookies.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			for _, statement := range []string{
				`CREATE TABLE moz_cookies(host TEXT, name TEXT, value TEXT, path TEXT, expiry INTEGER, isSecure INTEGER, isHttpOnly INTEGER, sameSite INTEGER, originAttributes TEXT)`,
				`INSERT INTO moz_cookies VALUES('bsky.app','login','synthetic-private-token','/',1893456000,1,1,1,'')`,
			} {
				if _, err := db.Exec(statement); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("browser credentials were forwarded on redirect")
			}))
			defer target.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/cookies") {
					t.Error("unscoped cookie-only sync did not use the compatible endpoint", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Location", target.URL)
				w.WriteHeader(status)
				if status == 200 {
					_, _ = w.Write([]byte(`{"success":true,"cookies_count":1,"mode":"replace","message":"imported"}`))
				} else {
					_, _ = w.Write([]byte(`{"detail":"synthetic-private-token"}`))
				}
			}))
			defer server.Close()
			env.SetEnv("NOTTE_API_URL", server.URL)
			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			var runErr error
			stdout, stderr := testutil.CaptureOutput(func() { runErr = runProfilesSync(cmd, nil) })
			if (runErr == nil) != (status == 200) {
				t.Fatal("cookie import reported an incorrect result", runErr)
			}
			if strings.Contains(stdout+stderr+fmt.Sprint(runErr), "synthetic-private-token") {
				t.Fatal("cookie values leaked into output")
			}
		})
	}
}
