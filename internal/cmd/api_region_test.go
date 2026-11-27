package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

// withSavedDefaultKey stores key as `notte login` would with no API URL set.
func withSavedDefaultKey(t *testing.T, env *testutil.TestEnv, key string) {
	t.Helper()
	auth.SetKeyring(env.MockStore)
	t.Cleanup(auth.ResetKeyring)
	config.SetTestConfigDir(env.TempDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })
	if err := auth.SetKeyringAPIKey(key); err != nil {
		t.Fatal(err)
	}
}

func TestGetClient_PicksTheAPIOfTheKeysRegion(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"sk-notte-abc", "https://api.notte.cc"},
		{"sk-notte-org-abc", "https://api.notte.cc"},
		{"nat_abc", "https://api.notte.cc"},
		{"sk-notte-europe-abc", "https://api.notte.cc"},
		{"sk-notte-eu-abc", "https://eu-api.notte.cc"},
		{"sk-notte-org-eu-abc", "https://eu-api.notte.cc"},
		{"nat_eu_abc", "https://eu-api.notte.cc"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			env := testutil.SetupTestEnv(t)
			withSavedDefaultKey(t, env, tt.key)

			client, err := GetClient()
			if err != nil {
				t.Fatalf("GetClient: %v", err)
			}
			if got := client.BaseURL(); got != tt.want {
				t.Errorf("BaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetClient_EnvKeyPicksTheAPIOfItsRegion(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	config.SetTestConfigDir(env.TempDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })
	env.SetEnv("NOTTE_API_KEY", "nat_eu_token")

	client, err := GetClient()
	if err != nil {
		t.Fatalf("GetClient: %v", err)
	}
	if got := client.BaseURL(); got != "https://eu-api.notte.cc" {
		t.Errorf("BaseURL() = %q, want the EU API", got)
	}
}

func TestGetClient_ExplicitAPIURLWinsOverTheKeysRegion(t *testing.T) {
	t.Run("env var", func(t *testing.T) {
		env := testutil.SetupTestEnv(t)
		withSavedKey(t, env, "https://us-staging.notte.cc", "sk-notte-eu-abc")

		client, err := GetClient()
		if err != nil {
			t.Fatalf("GetClient: %v", err)
		}
		if got := client.BaseURL(); got != "https://us-staging.notte.cc" {
			t.Errorf("BaseURL() = %q, want NOTTE_API_URL", got)
		}
	})
	t.Run("config file", func(t *testing.T) {
		env := testutil.SetupTestEnv(t)
		config.SetTestConfigDir(env.TempDir)
		t.Cleanup(func() { config.SetTestConfigDir("") })
		cfg := &config.Config{APIURL: "https://api.notte.cc"}
		if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}
		env.SetEnv("NOTTE_API_KEY", "sk-notte-org-eu-abc")

		client, err := GetClient()
		if err != nil {
			t.Fatalf("GetClient: %v", err)
		}
		if got := client.BaseURL(); got != "https://api.notte.cc" {
			t.Errorf("BaseURL() = %q, want the config file's api_url", got)
		}
	})
}

func TestLogoutRevokesAnEUKeyAtTheEUAPI(t *testing.T) {
	// No API URL set: the saved EU key is revoked where it is used, the EU
	// API, not the default one.
	env := testutil.SetupTestEnv(t)
	withSavedDefaultKey(t, env, "sk-notte-eu-cli-current")

	var mu sync.Mutex
	var revoked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		revoked = append(revoked, r.PostForm.Get("token"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"revoked": true}`))
	}))
	t.Cleanup(srv.Close)

	var targets []string
	origFlow, origFormat := newDeviceFlow, outputFormat
	newDeviceFlow = func(apiURL string) *auth.DeviceFlow {
		targets = append(targets, apiURL)
		flow := auth.NewDeviceFlow(srv.URL, "test")
		flow.Sleep = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
		return flow
	}
	outputFormat = "text"
	t.Cleanup(func() { newDeviceFlow, outputFormat = origFlow, origFormat })

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runAuthLogout(commandWithContext(), nil); err != nil {
			t.Fatalf("logout: %v", err)
		}
	})
	if !strings.Contains(stdout, "API key revoked and removed.") {
		t.Errorf("output = %q", stdout)
	}
	if len(targets) != 1 || targets[0] != "https://eu-api.notte.cc" {
		t.Errorf("revoke went to %v, want the EU API", targets)
	}
	if len(revoked) != 1 || revoked[0] != "sk-notte-eu-cli-current" {
		t.Errorf("revoked = %v", revoked)
	}
}
