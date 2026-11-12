package auth

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

	// Store key in mock keyring (env-qualified for prod since NOTTE_API_URL is unset)
	_ = env.MockStore.Set("api_key:prod", "keyring_test_key")

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

func TestGetAPIKey_LegacyMigration(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()

	// Store key under legacy "api_key" (simulating pre-upgrade state)
	_ = env.MockStore.Set("api_key", "legacy_key_value")

	// Should find via legacy fallback and migrate
	key, source, err := GetAPIKey("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "legacy_key_value" {
		t.Errorf("got %q, want 'legacy_key_value'", key)
	}
	if source != SourceKeyring {
		t.Errorf("got source %q, want %q", source, SourceKeyring)
	}

	// Verify migration: env-qualified key should now exist
	migrated, err := env.MockStore.Get("api_key:prod")
	if err != nil {
		t.Fatalf("expected migrated key, got error: %v", err)
	}
	if migrated != "legacy_key_value" {
		t.Errorf("migrated key = %q, want 'legacy_key_value'", migrated)
	}

	// Legacy key should be deleted
	_, err = env.MockStore.Get("api_key")
	if err == nil {
		t.Error("expected legacy key to be deleted after migration")
	}
}

func TestGetAPIKey_Priority(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()

	// Set all three sources
	env.SetEnv(EnvAPIKey, "env_key")
	_ = env.MockStore.Set("api_key:prod", "keyring_key")

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
	_ = env.MockStore.Delete("api_key:prod")
	key, source, _ = GetAPIKey(cfgPath)
	if key != "config_key" || source != SourceConfig {
		t.Errorf("config should be fallback: got %q from %q", key, source)
	}
}

// writeKeyFile writes content to a file in the test's temp dir and names it in
// NOTTE_API_KEY_FILE.
func writeKeyFile(t *testing.T, env *testutil.TestEnv, content string) string {
	t.Helper()
	path := filepath.Join(env.TempDir, "token")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	env.SetEnv(EnvAPIKeyFile, path)
	return path
}

func TestGetAPIKey_KeyFileWinsOverEveryOtherSource(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()

	env.SetEnv(EnvAPIKey, "env_key")
	_ = env.MockStore.Set("api_key:prod", "keyring_key")
	cfgPath := filepath.Join(env.TempDir, "config.json")
	_ = os.WriteFile(cfgPath, []byte(`{"api_key": "config_key"}`), 0o600)
	writeKeyFile(t, env, "  nat_file_token\n")

	key, source, err := GetAPIKey(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "nat_file_token" || source != SourceFile {
		t.Errorf("got %q from %q, want the trimmed file content from %q", key, source, SourceFile)
	}
}

func TestGetAPIKey_KeyFileIsReadOnEveryCall(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()

	path := writeKeyFile(t, env, "nat_first")
	if key, _, err := GetAPIKey(""); err != nil || key != "nat_first" {
		t.Fatalf("first read: got %q, %v", key, err)
	}
	if err := os.WriteFile(path, []byte("nat_second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if key, _, err := GetAPIKey(""); err != nil || key != "nat_second" {
		t.Fatalf("after rotation: got %q, %v, want nat_second", key, err)
	}
}

func TestGetAPIKey_KeyFileProblemsDoNotFallBack(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setup   func(t *testing.T, env *testutil.TestEnv) string
		wantMsg string
	}{
		{
			name: "missing",
			setup: func(t *testing.T, env *testutil.TestEnv) string {
				path := filepath.Join(env.TempDir, "does-not-exist")
				env.SetEnv(EnvAPIKeyFile, path)
				return path
			},
			wantMsg: "could not be read",
		},
		{
			name: "empty",
			setup: func(t *testing.T, env *testutil.TestEnv) string {
				return writeKeyFile(t, env, "")
			},
			wantMsg: "is empty",
		},
		{
			name: "whitespace only",
			setup: func(t *testing.T, env *testutil.TestEnv) string {
				return writeKeyFile(t, env, " \n\t\n")
			},
			wantMsg: "is empty",
		},
		{
			name: "unreadable",
			setup: func(t *testing.T, env *testutil.TestEnv) string {
				// A directory cannot be read as a file on any platform.
				path := filepath.Join(env.TempDir, "a-directory")
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				env.SetEnv(EnvAPIKeyFile, path)
				return path
			},
			wantMsg: "could not be read",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, cleanup := setupTestAuth(t)
			defer cleanup()

			// Every other source holds a key that must not be used.
			env.SetEnv(EnvAPIKey, "env_key")
			_ = env.MockStore.Set("api_key:prod", "keyring_key")
			cfgPath := filepath.Join(env.TempDir, "config.json")
			_ = os.WriteFile(cfgPath, []byte(`{"api_key": "config_key"}`), 0o600)
			path := tc.setup(t, env)

			key, source, err := GetAPIKey(cfgPath)
			if err == nil {
				t.Fatalf("got %q from %q, want an error", key, source)
			}
			if key != "" || source != "" {
				t.Errorf("got %q from %q alongside the error, want nothing", key, source)
			}
			var fileErr *APIKeyFileError
			if !errors.As(err, &fileErr) {
				t.Fatalf("error %v is not an APIKeyFileError", err)
			}
			for _, want := range []string{EnvAPIKeyFile, path, tc.wantMsg} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q should contain %q", err, want)
				}
			}
		})
	}
}

func TestGetAPIKey_EmptyKeyFileVariableIsUnset(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()

	env.SetEnv(EnvAPIKeyFile, "")
	env.SetEnv(EnvAPIKey, "env_key")
	key, source, err := GetAPIKey("")
	if err != nil || key != "env_key" || source != SourceEnv {
		t.Fatalf("got %q from %q (%v), want env_key from the environment", key, source, err)
	}
}

// captureWarnings collects what GetAPIKey prints to stderr during the test.
func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	t.Cleanup(SetWarningOutput(&buf))
	return &buf
}

func TestGetAPIKey_NotesIgnoredEnvKey(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()
	warnings := captureWarnings(t)

	writeKeyFile(t, env, "nat_file_token")
	env.SetEnv(EnvAPIKey, "sk-env-secret")
	for range 2 {
		if key, source, err := GetAPIKey(""); err != nil || key != "nat_file_token" || source != SourceFile {
			t.Fatalf("got %q from %q (%v), want the file's token", key, source, err)
		}
	}
	want := "note: NOTTE_API_KEY is ignored because NOTTE_API_KEY_FILE is set\n"
	if warnings.String() != want {
		t.Fatalf("stderr %q, want exactly one line %q", warnings.String(), want)
	}
	for _, secret := range []string{"sk-env-secret", "nat_file_token"} {
		if strings.Contains(warnings.String(), secret) {
			t.Fatalf("stderr %q prints a credential", warnings.String())
		}
	}
}

func TestGetAPIKey_NoNoteWithOnlyKeyFile(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()
	warnings := captureWarnings(t)

	writeKeyFile(t, env, "nat_file_token")
	if _, _, err := GetAPIKey(""); err != nil {
		t.Fatal(err)
	}
	if warnings.Len() != 0 {
		t.Fatalf("stderr %q, want nothing", warnings.String())
	}
}

func TestGetAPIKey_KeyFileSizeCap(t *testing.T) {
	env, cleanup := setupTestAuth(t)
	defer cleanup()
	captureWarnings(t)

	// Exactly at the cap is read.
	atCap := "nat_" + strings.Repeat("a", MaxAPIKeyFileSize-4)
	writeKeyFile(t, env, atCap)
	if key, _, err := GetAPIKey(""); err != nil || key != atCap {
		t.Fatalf("file at the cap: got %d bytes, %v", len(key), err)
	}

	// One byte more is refused, without falling back.
	env.SetEnv(EnvAPIKey, "env_key")
	path := writeKeyFile(t, env, atCap+"a")
	key, source, err := GetAPIKey("")
	if err == nil || key != "" || source != "" {
		t.Fatalf("got %d bytes from %q (%v), want an error", len(key), source, err)
	}
	var fileErr *APIKeyFileError
	if !errors.As(err, &fileErr) || !fileErr.TooLong {
		t.Fatalf("error %v is not a too-large APIKeyFileError", err)
	}
	for _, want := range []string{EnvAPIKeyFile, path, "larger than 64 KiB"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should contain %q", err, want)
		}
	}
}

func TestGetAPIKey_WarnsOnOpenKeyFilePermissions(t *testing.T) {
	cases := []struct {
		mode os.FileMode
		warn bool
	}{
		{0o600, false},
		{0o400, false},
		{0o640, true},
		{0o644, true},
		{0o604, true},
	}
	if runtime.GOOS == "windows" {
		// Windows has no Unix mode bits, so nothing is checked there.
		cases = cases[3:4]
		cases[0].warn = false
	}
	for _, tc := range cases {
		t.Run(tc.mode.String(), func(t *testing.T) {
			env, cleanup := setupTestAuth(t)
			defer cleanup()
			warnings := captureWarnings(t)

			path := writeKeyFile(t, env, "nat_file_token")
			if err := os.Chmod(path, tc.mode); err != nil {
				t.Fatal(err)
			}
			key, _, err := GetAPIKey("")
			if err != nil || key != "nat_file_token" {
				t.Fatalf("got %q, %v; a permission warning must not fail the read", key, err)
			}
			got := warnings.String()
			if !tc.warn {
				if got != "" {
					t.Fatalf("stderr %q, want nothing for mode %v", got, tc.mode)
				}
				return
			}
			for _, want := range []string{"warning:", EnvAPIKeyFile, path, "chmod 600"} {
				if !strings.Contains(got, want) {
					t.Errorf("stderr %q should contain %q", got, want)
				}
			}
			if strings.Contains(got, "nat_file_token") {
				t.Fatalf("stderr %q prints the credential", got)
			}
		})
	}
}
