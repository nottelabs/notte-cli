package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

func TestResolveEnvLabel(t *testing.T) {
	tests := []struct {
		name   string
		apiURL string
		want   string
	}{
		// Known prod aliases
		{"default prod", "https://api.notte.cc", "prod"},
		{"us-prod", "https://us-prod.notte.cc", "prod"},
		{"prod with path", "https://api.notte.cc/v1/foo", "prod"},
		{"eu", "https://eu-api.notte.cc", "prod"},

		// Staging
		{"staging", "https://us-staging.notte.cc", "staging"},
		{"staging with path", "https://us-staging.notte.cc/v1", "staging"},

		// Dev aliases
		{"dev", "https://us-dev.notte.cc", "dev"},
		{"dev-test", "https://us-dev-test.notte.cc", "dev"},

		// Unknown hosts -> hostname as label
		{"localhost", "http://localhost:8080", "localhost"},
		{"custom host", "https://my-api.example.com", "my-api.example.com"},

		// Empty URL -> prod
		{"empty URL", "", "prod"},

		// Invalid URL -> prod
		{"invalid URL", "://bad", "prod"},

		// Port stripping
		{"prod with port", "https://api.notte.cc:443", "prod"},
		{"localhost with port", "http://localhost:3000", "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveEnvLabel(tt.apiURL)
			if got != tt.want {
				t.Errorf("ResolveEnvLabel(%q) = %q, want %q", tt.apiURL, got, tt.want)
			}
		})
	}
}

func TestKeyringKeyForEnv(t *testing.T) {
	tests := []struct {
		label string
		want  string
	}{
		{"prod", "api_key:prod"},
		{"staging", "api_key:staging"},
		{"dev", "api_key:dev"},
		{"localhost", "api_key:localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := KeyringKeyForEnv(tt.label)
			if got != tt.want {
				t.Errorf("KeyringKeyForEnv(%q) = %q, want %q", tt.label, got, tt.want)
			}
		})
	}
}

// withConfigDir isolates the config file, optionally writing content to it.
func withConfigDir(t *testing.T, env *testutil.TestEnv, content string) {
	t.Helper()
	config.SetTestConfigDir(env.TempDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })
	if content == "" {
		return
	}
	path, err := config.DefaultConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAPIURLForKey_DefaultsByRegion(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	withConfigDir(t, env, "")

	tests := []struct {
		key  string
		want string
	}{
		{"", "https://api.notte.cc"},
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
			if got := APIURLForKey(tt.key); got != tt.want {
				t.Errorf("APIURLForKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
			got, err := ResolveAPIURL(tt.key)
			if err != nil || got != tt.want {
				t.Errorf("ResolveAPIURL(%q) = %q, %v, want %q", tt.key, got, err, tt.want)
			}
		})
	}

	// Before a key is known there is no region to pick.
	if got := GetCurrentAPIURL(); got != "https://api.notte.cc" {
		t.Errorf("GetCurrentAPIURL() = %q, want the default API", got)
	}
}

func TestAPIURLForKey_EnvVarWins(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	withConfigDir(t, env, `{"api_url": "https://from-config.example.com"}`)
	env.SetEnv(config.EnvAPIURL, "https://api.notte.cc")

	for _, key := range []string{"sk-notte-abc", "sk-notte-eu-abc", "sk-notte-org-eu-abc", "nat_eu_abc"} {
		if got := APIURLForKey(key); got != "https://api.notte.cc" {
			t.Errorf("APIURLForKey(%q) = %q, want NOTTE_API_URL", key, got)
		}
		if got, err := ResolveAPIURL(key); err != nil || got != "https://api.notte.cc" {
			t.Errorf("ResolveAPIURL(%q) = %q, %v, want NOTTE_API_URL", key, got, err)
		}
	}
}

func TestAPIURLForKey_ConfigFileWins(t *testing.T) {
	for _, configured := range []string{"https://us-staging.notte.cc", "https://api.notte.cc"} {
		t.Run(configured, func(t *testing.T) {
			env := testutil.SetupTestEnv(t)
			withConfigDir(t, env, `{"api_url": "`+configured+`"}`)

			for _, key := range []string{"sk-notte-abc", "sk-notte-eu-abc", "nat_eu_abc"} {
				if got := APIURLForKey(key); got != configured {
					t.Errorf("APIURLForKey(%q) = %q, want the config file's %q", key, got, configured)
				}
				if got, err := ResolveAPIURL(key); err != nil || got != configured {
					t.Errorf("ResolveAPIURL(%q) = %q, %v, want %q", key, got, err, configured)
				}
			}
			if got := GetCurrentAPIURL(); got != configured {
				t.Errorf("GetCurrentAPIURL() = %q, want %q", got, configured)
			}
		})
	}
}

func TestResolveAPIURL_ReportsAnUnreadableConfigFile(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	withConfigDir(t, env, `{not json`)

	if _, err := ResolveAPIURL("sk-notte-eu-abc"); err == nil {
		t.Error("ResolveAPIURL() = nil error, want the config file's parse error")
	}
	// The lenient form falls back to the key's region, as before.
	if got := APIURLForKey("sk-notte-eu-abc"); got != "https://eu-api.notte.cc" {
		t.Errorf("APIURLForKey() = %q, want the EU API", got)
	}
}
