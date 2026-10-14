package cmd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

// usageServer counts requests and records the bearer token of the last one.
func usageServer(t *testing.T) (*httptest.Server, *atomic.Int32, *atomic.Value) {
	t.Helper()
	var hits atomic.Int32
	var token atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		token.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits, &token
}

// withSavedKey stores key for apiURL in a mock keyring, as `notte auth login` would.
func withSavedKey(t *testing.T, env *testutil.TestEnv, apiURL, key string) {
	t.Helper()
	auth.SetKeyring(env.MockStore)
	t.Cleanup(auth.ResetKeyring)
	config.SetTestConfigDir(env.TempDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })
	env.SetEnv("NOTTE_API_URL", apiURL)
	if err := auth.SetKeyringAPIKey(key); err != nil {
		t.Fatal(err)
	}
}

func TestGetClient_SendsSavedKeyToLoopbackOverHTTP(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	srv, hits, token := usageServer(t)
	withSavedKey(t, env, srv.URL, "saved-key")

	client, err := GetClient()
	if err != nil {
		t.Fatalf("GetClient: %v", err)
	}
	if _, err := client.Client().GetUsageWithResponse(context.Background(), &api.GetUsageParams{}); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 || token.Load() != "Bearer saved-key" {
		t.Fatalf("hits=%d token=%v, want one request with the saved key", hits.Load(), token.Load())
	}
}

func TestGetClient_RefusesSavedKeyOverPlainHTTP(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	srv, hits, _ := usageServer(t)
	// Same server, but named as a host that is not loopback by name.
	apiURL := strings.Replace(srv.URL, "127.0.0.1", "localhost.", 1)
	withSavedKey(t, env, apiURL, "saved-key")

	_, err := GetClient()
	if !errors.Is(err, auth.ErrInsecureAPIURL) {
		t.Fatalf("GetClient error = %v, want ErrInsecureAPIURL", err)
	}
	if !strings.Contains(err.Error(), apiURL) {
		t.Errorf("error %q should name the API URL", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("the server received %d requests, want none", hits.Load())
	}
}

func TestGetClient_RefusesConfigFileKeyOverPlainHTTP(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	auth.SetKeyring(env.MockStore)
	t.Cleanup(auth.ResetKeyring)
	config.SetTestConfigDir(env.TempDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })
	dir, err := config.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := `{"api_key": "config-key", "api_url": "http://api.notte.cc"}`
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := GetClient(); !errors.Is(err, auth.ErrInsecureAPIURL) {
		t.Fatalf("GetClient error = %v, want ErrInsecureAPIURL", err)
	}
}

func TestGetClient_EnvKeyIsNotRestricted(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "env-key")
	env.SetEnv("NOTTE_API_URL", "http://api.example.com")

	if _, err := GetClient(); err != nil {
		t.Fatalf("GetClient with NOTTE_API_KEY: %v", err)
	}
}

func TestRunAuthLogin_RefusesPlainHTTP(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_URL", "http://api.example.com")
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	if err := runAuthLogin(cmd, nil); !errors.Is(err, auth.ErrInsecureAPIURL) {
		t.Fatalf("runAuthLogin error = %v, want ErrInsecureAPIURL", err)
	}
}
