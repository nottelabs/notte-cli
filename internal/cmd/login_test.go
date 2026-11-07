package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

// fakeLoginAPI serves the device endpoints and /usage, approving on the
// second poll, and records which keys it was asked to revoke.
type fakeLoginAPI struct {
	mu      sync.Mutex
	polls   int
	revoked []string
	usage   []string
}

func (f *fakeLoginAPI) serve(t *testing.T) *httptest.Server {
	t.Helper()
	write := func(w http.ResponseWriter, status int, body any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/device/code", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, map[string]any{
			"device_code":               "secret-device-code",
			"user_code":                 "HNVS-GHLG",
			"verification_uri":          "https://console.notte.cc/oauth/device",
			"verification_uri_complete": "https://console.notte.cc/oauth/device?user_code=HNVS-GHLG",
			"expires_in":                900,
			"interval":                  5,
		})
	})
	mux.HandleFunc("/auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.polls++
		polls := f.polls
		f.mu.Unlock()
		if polls < 2 {
			write(w, http.StatusBadRequest, map[string]string{"error": "authorization_pending"})
			return
		}
		write(w, http.StatusOK, map[string]any{
			"access_token":   "sk-notte-cli-new",
			"token_type":     "Bearer",
			"key_name":       "CLI · mac · 2026-11-07",
			"email":          "dev@example.com",
			"workspace_name": "Acme",
		})
	})
	mux.HandleFunc("/auth/device/revoke", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.revoked = append(f.revoked, r.PostForm.Get("token"))
		f.mu.Unlock()
		write(w, http.StatusOK, map[string]any{"revoked": strings.HasPrefix(r.PostForm.Get("token"), "sk-notte-cli")})
	})
	mux.HandleFunc("/usage", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.usage = append(f.usage, r.Header.Get("Authorization"))
		f.mu.Unlock()
		write(w, http.StatusOK, map[string]any{})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func setupLogin(t *testing.T, previousKey string) (*fakeLoginAPI, *testutil.TestEnv) {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	api := &fakeLoginAPI{}
	srv := api.serve(t)

	auth.SetKeyring(env.MockStore)
	t.Cleanup(auth.ResetKeyring)
	config.SetTestConfigDir(env.TempDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })
	env.SetEnv("NOTTE_API_URL", srv.URL)
	env.SetEnv(config.EnvConsoleURL, "")
	if previousKey != "" {
		if err := auth.SetKeyringAPIKey(previousKey); err != nil {
			t.Fatal(err)
		}
	}

	origFlow, origNoBrowser, origWithToken, origFormat := newDeviceFlow, loginNoBrowser, loginWithToken, outputFormat
	newDeviceFlow = func(apiURL string) *auth.DeviceFlow {
		flow := auth.NewDeviceFlow(apiURL, "test")
		flow.Sleep = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
		return flow
	}
	loginNoBrowser, loginWithToken, outputFormat = true, false, "text"
	t.Cleanup(func() {
		newDeviceFlow, loginNoBrowser, loginWithToken, outputFormat = origFlow, origNoBrowser, origWithToken, origFormat
	})
	return api, env
}

func commandWithContext() *cobra.Command {
	c := &cobra.Command{}
	c.SetContext(context.Background())
	return c
}

func TestLoginStoresTheApprovedKeyAndRevokesThePreviousOne(t *testing.T) {
	api, _ := setupLogin(t, "sk-notte-cli-old")

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runAuthLogin(commandWithContext(), nil); err != nil {
			t.Fatalf("login: %v", err)
		}
	})

	for _, want := range []string{
		"https://console.notte.cc/oauth/device?user_code=HNVS-GHLG",
		"HNVS-GHLG",
		`Logged in as dev@example.com (Acme). API key "CLI · mac · 2026-11-07" stored.`,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output missing %q:\n%s", want, stdout)
		}
	}
	key, err := auth.GetKeyringAPIKey()
	if err != nil || key != "sk-notte-cli-new" {
		t.Fatalf("stored key = %q, %v", key, err)
	}
	if len(api.usage) != 1 || api.usage[0] != "Bearer sk-notte-cli-new" {
		t.Errorf("validation calls = %v", api.usage)
	}
	if len(api.revoked) != 1 || api.revoked[0] != "sk-notte-cli-old" {
		t.Errorf("revoked = %v, want the previous key", api.revoked)
	}
}

func TestLoginWithTokenStoresAPastedKey(t *testing.T) {
	api, _ := setupLogin(t, "")
	loginWithToken = true
	orig := stdinReader
	stdinReader = strings.NewReader("sk-notte-pasted\n")
	t.Cleanup(func() { stdinReader = orig })

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runAuthLogin(commandWithContext(), nil); err != nil {
			t.Fatalf("login --with-token: %v", err)
		}
	})
	if !strings.Contains(stdout, "API key stored successfully.") {
		t.Errorf("output = %q", stdout)
	}
	if key, _ := auth.GetKeyringAPIKey(); key != "sk-notte-pasted" {
		t.Errorf("stored key = %q", key)
	}
	if api.polls != 0 {
		t.Errorf("--with-token polled the device endpoint %d times", api.polls)
	}
}

func TestLoginWithTokenRequiresInput(t *testing.T) {
	setupLogin(t, "")
	loginWithToken = true
	orig := stdinReader
	stdinReader = strings.NewReader("")
	t.Cleanup(func() { stdinReader = orig })

	err := runAuthLogin(commandWithContext(), nil)
	if err == nil || !strings.Contains(err.Error(), "no API key on standard input") {
		t.Fatalf("error = %v", err)
	}
}

func TestLogoutRevokesACLIKey(t *testing.T) {
	api, _ := setupLogin(t, "sk-notte-cli-current")

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runAuthLogout(commandWithContext(), nil); err != nil {
			t.Fatalf("logout: %v", err)
		}
	})
	if !strings.Contains(stdout, "API key revoked and removed.") {
		t.Errorf("output = %q", stdout)
	}
	if len(api.revoked) != 1 || api.revoked[0] != "sk-notte-cli-current" {
		t.Errorf("revoked = %v", api.revoked)
	}
	if key, _ := auth.GetKeyringAPIKey(); key != "" {
		t.Errorf("key still stored: %q", key)
	}
}

func TestLogoutOnlyRemovesAPastedKey(t *testing.T) {
	_, _ = setupLogin(t, "sk-notte-pasted")

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runAuthLogout(commandWithContext(), nil); err != nil {
			t.Fatalf("logout: %v", err)
		}
	})
	if !strings.Contains(stdout, "API key removed.") || strings.Contains(stdout, "revoked") {
		t.Errorf("output = %q", stdout)
	}
}

func TestTopLevelAuthCommandsExist(t *testing.T) {
	for _, name := range []string{"login", "logout", "whoami"} {
		found, _, err := rootCmd.Find([]string{name})
		if err != nil || found.Name() != name {
			t.Errorf("notte %s not found: %v", name, err)
		}
	}
	for _, name := range []string{"login", "logout", "status"} {
		found, _, err := rootCmd.Find([]string{"auth", name})
		if err != nil || found.Name() != name {
			t.Errorf("notte auth %s not found: %v", name, err)
		}
	}
}
