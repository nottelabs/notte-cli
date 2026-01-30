package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

// mockKeyringAdapter adapts testutil.MockKeyring to auth.KeyringStore
type mockKeyringAdapter struct {
	mock *testutil.MockKeyring
}

func (m *mockKeyringAdapter) Get(key string) (string, error) {
	return m.mock.Get(key)
}

func (m *mockKeyringAdapter) Set(key, value string) error {
	return m.mock.Set(key, value)
}

func (m *mockKeyringAdapter) Delete(key string) error {
	return m.mock.Delete(key)
}

func setupTestAuth(t *testing.T) (*testutil.TestEnv, func()) {
	t.Helper()
	env := testutil.SetupTestEnv(t)

	// Install mock keyring
	adapter := &mockKeyringAdapter{mock: env.MockStore}
	SetKeyring(adapter)

	cleanup := func() {
		ResetKeyring()
	}

	return env, cleanup
}

func TestGetAPIKey_EnvVar(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()

	env.SetEnv(EnvAPIKey, "env_test_key")

	key, source, err := GetAPIKey("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "env_test_key" {
		t.Errorf("got %q, want 'env_test_key'", key)
	}
	if source != SourceEnv {
		t.Errorf("got source %q, want %q", source, SourceEnv)
	}
}

func TestGetAPIKey_Keyring(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()

	// Store key in mock keyring
	_ = env.MockStore.Set("api_key", "keyring_test_key")

	key, source, err := GetAPIKey("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "keyring_test_key" {
		t.Errorf("got %q, want 'keyring_test_key'", key)
	}
	if source != SourceKeyring {
		t.Errorf("got source %q, want %q", source, SourceKeyring)
	}
}

func TestGetAPIKey_ConfigFile(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()

	// No env var, no keyring - should fall through to config
	cfgPath := filepath.Join(env.TempDir, "config.json")
	content := `{"api_key": "config_test_key"}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	key, source, err := GetAPIKey(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "config_test_key" {
		t.Errorf("got %q, want 'config_test_key'", key)
	}
	if source != SourceConfig {
		t.Errorf("got source %q, want %q", source, SourceConfig)
	}
}

func TestGetAPIKey_NotFound(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()

	cfgPath := filepath.Join(env.TempDir, "config.json")

	_, _, err := GetAPIKey(cfgPath)
	if err == nil {
		t.Error("expected error when no API key found")
	}
	if err != ErrNoAPIKey {
		t.Errorf("got error %v, want ErrNoAPIKey", err)
	}
}

func TestGetAPIKey_Priority(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()

	// Set all three sources
	env.SetEnv(EnvAPIKey, "env_key")
	_ = env.MockStore.Set("api_key", "keyring_key")

	cfgPath := filepath.Join(env.TempDir, "config.json")
	_ = os.WriteFile(cfgPath, []byte(`{"api_key": "config_key"}`), 0o600)

	// Env should win
	key, source, _ := GetAPIKey(cfgPath)
	if key != "env_key" || source != SourceEnv {
		t.Errorf("env should have priority: got %q from %q", key, source)
	}

	// Remove env, keyring should win
	_ = os.Unsetenv(EnvAPIKey)
	key, source, _ = GetAPIKey(cfgPath)
	if key != "keyring_key" || source != SourceKeyring {
		t.Errorf("keyring should have priority over config: got %q from %q", key, source)
	}

	// Remove keyring, config should win
	_ = env.MockStore.Delete("api_key")
	key, source, _ = GetAPIKey(cfgPath)
	if key != "config_key" || source != SourceConfig {
		t.Errorf("config should be fallback: got %q from %q", key, source)
	}
}
