package auth

import (
	"fmt"
	"os"
	"testing"

	"github.com/99designs/keyring"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

func TestKeyringServiceName(t *testing.T) {
	if KeyringService != "notte-cli" {
		t.Errorf("expected service name 'notte-cli', got %q", KeyringService)
	}
}

// Integration test - only run if keyring available
func TestKeyring_SetGetDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping keyring integration test")
	}
	if os.Getenv("NOTTE_KEYRING_TEST") == "" {
		t.Skip("set NOTTE_KEYRING_TEST=1 to run keyring integration test")
	}

	testKey := "test_api_key_12345"

	// Set
	if err := SetKeyringAPIKey(testKey); err != nil {
		t.Fatalf("SetKeyringAPIKey failed: %v", err)
	}

	// Get
	got, err := GetKeyringAPIKey()
	if err != nil {
		t.Fatalf("GetKeyringAPIKey failed: %v", err)
	}
	if got != testKey {
		t.Errorf("got %q, want %q", got, testKey)
	}

	// Delete
	if err := DeleteKeyringAPIKey(); err != nil {
		t.Fatalf("DeleteKeyringAPIKey failed: %v", err)
	}

	// Verify deleted
	_, err = GetKeyringAPIKey()
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestKeyring_SetAndGet(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	SetKeyring(env.MockStore)
	defer ResetKeyring()

	// Test Set
	err := SetKeyringAPIKey("test-key-123")
	if err != nil {
		t.Fatalf("SetKeyringAPIKey failed: %v", err)
	}

	// Test Get
	key, err := GetKeyringAPIKey()
	if err != nil {
		t.Fatalf("GetKeyringAPIKey failed: %v", err)
	}
	if key != "test-key-123" {
		t.Errorf("got %q, want %q", key, "test-key-123")
	}
}

func TestKeyring_Delete(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	SetKeyring(env.MockStore)
	defer ResetKeyring()

	// Set then delete
	if err := SetKeyringAPIKey("test-key"); err != nil {
		t.Fatalf("SetKeyringAPIKey failed: %v", err)
	}
	err := DeleteKeyringAPIKey()
	if err != nil {
		t.Fatalf("DeleteKeyringAPIKey failed: %v", err)
	}

	// Should be gone
	key, err := GetKeyringAPIKey()
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
	if key != "" {
		t.Errorf("expected empty key after delete, got %q", key)
	}
}

func TestKeyring_MultipleEnvironments(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	SetKeyring(env.MockStore)
	defer ResetKeyring()

	// Store key for prod (default, NOTTE_API_URL unset)
	err := SetKeyringAPIKey("prod-key-123")
	if err != nil {
		t.Fatalf("SetKeyringAPIKey (prod) failed: %v", err)
	}

	// Store key for staging
	env.SetEnv("NOTTE_API_URL", "https://us-staging.notte.cc")
	err = SetKeyringAPIKey("staging-key-456")
	if err != nil {
		t.Fatalf("SetKeyringAPIKey (staging) failed: %v", err)
	}

	// Retrieve staging key
	key, err := GetKeyringAPIKey()
	if err != nil {
		t.Fatalf("GetKeyringAPIKey (staging) failed: %v", err)
	}
	if key != "staging-key-456" {
		t.Errorf("staging key = %q, want %q", key, "staging-key-456")
	}

	// Switch back to prod
	_ = os.Unsetenv("NOTTE_API_URL")
	key, err = GetKeyringAPIKey()
	if err != nil {
		t.Fatalf("GetKeyringAPIKey (prod) failed: %v", err)
	}
	if key != "prod-key-123" {
		t.Errorf("prod key = %q, want %q", key, "prod-key-123")
	}
}

func TestKeyring_LegacyMigration(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	SetKeyring(env.MockStore)
	defer ResetKeyring()

	// Simulate pre-upgrade: legacy key exists
	_ = env.MockStore.Set("api_key", "legacy-key")

	// Get should find legacy key and migrate it
	key, err := GetKeyringAPIKey()
	if err != nil {
		t.Fatalf("GetKeyringAPIKey failed: %v", err)
	}
	if key != "legacy-key" {
		t.Errorf("got %q, want %q", key, "legacy-key")
	}

	// Verify migration happened
	migrated, err := env.MockStore.Get("api_key:prod")
	if err != nil {
		t.Fatalf("expected migrated key: %v", err)
	}
	if migrated != "legacy-key" {
		t.Errorf("migrated = %q, want %q", migrated, "legacy-key")
	}

	// Legacy key should be gone
	_, err = env.MockStore.Get("api_key")
	if err == nil {
		t.Error("expected legacy key to be deleted")
	}
}

func TestKeyring_LegacyMigration_NonProdEnv(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	SetKeyring(env.MockStore)
	defer ResetKeyring()

	// Simulate pre-upgrade: legacy key exists
	_ = env.MockStore.Set("api_key", "legacy-key")

	// Set current env to staging
	env.SetEnv("NOTTE_API_URL", "https://us-staging.notte.cc")

	// Get should NOT return the legacy key for staging
	_, err := GetKeyringAPIKey()
	if err == nil {
		t.Fatal("expected error when legacy key is accessed from non-prod env")
	}

	// But the legacy key should still be migrated to prod
	migrated, err := env.MockStore.Get("api_key:prod")
	if err != nil {
		t.Fatalf("expected legacy key migrated to prod: %v", err)
	}
	if migrated != "legacy-key" {
		t.Errorf("migrated = %q, want %q", migrated, "legacy-key")
	}

	// Legacy key should be deleted
	_, err = env.MockStore.Get("api_key")
	if err == nil {
		t.Error("expected legacy key to be deleted after migration")
	}
}

func TestKeyring_LegacyMigration_KeepsExistingProdKey(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	SetKeyring(env.MockStore)
	defer ResetKeyring()

	// A stale legacy key next to a newer prod key, e.g. a macOS Keychain item
	// from an old version alongside a key migrated from the file store.
	_ = env.MockStore.Set("api_key", "legacy-key")
	_ = env.MockStore.Set("api_key:prod", "prod-key")

	// An environment without a key reaches the legacy fallback.
	env.SetEnv("NOTTE_API_URL", "https://us-staging.notte.cc")
	if _, err := GetKeyringAPIKey(); err == nil {
		t.Fatal("expected error for staging without a key")
	}

	prod, err := env.MockStore.Get("api_key:prod")
	if err != nil {
		t.Fatalf("expected prod key to remain: %v", err)
	}
	if prod != "prod-key" {
		t.Errorf("prod key = %q, want %q", prod, "prod-key")
	}

	// The superseded legacy key is dropped.
	if _, err := env.MockStore.Get("api_key"); err == nil {
		t.Error("expected legacy key to be deleted")
	}
}

func TestKeyring_GetWhenEmpty(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	SetKeyring(env.MockStore)
	defer ResetKeyring()

	key, err := GetKeyringAPIKey()
	if err == nil {
		t.Error("expected error for empty keyring, got nil")
	}
	if key != "" {
		t.Errorf("expected empty key, got %q", key)
	}
}

// missingKeyring reports every key as missing, like the real keyring.
type missingKeyring struct{}

func (missingKeyring) Get(string) (string, error) { return "", keyring.ErrKeyNotFound }
func (missingKeyring) Set(string, string) error   { return nil }
func (missingKeyring) Delete(string) error {
	return fmt.Errorf("failed to remove key from keyring: %w", keyring.ErrKeyNotFound)
}

func TestDeleteKeyringSecret_MissingSecretIsRemoved(t *testing.T) {
	SetKeyring(missingKeyring{})
	t.Cleanup(ResetKeyring)
	if err := DeleteKeyringSecret("tunnel:gone:oauth_client_secret"); err != nil {
		t.Errorf("DeleteKeyringSecret() on a missing key = %v, want nil", err)
	}
}

func TestKeyring_EUKeyWithDefaultAPI(t *testing.T) {
	// An EU key is sent to the EU API when no API URL is set, but it is saved
	// and found under the same production entry as any other production key,
	// so keys saved before regions existed are still found, and naming either
	// production host explicitly finds the same key.
	env := testutil.SetupTestEnv(t)
	SetKeyring(env.MockStore)
	defer ResetKeyring()
	withConfigDir(t, env, "")

	const euKey = "sk-notte-org-eu-abc123"
	if err := SetKeyringAPIKey(euKey); err != nil {
		t.Fatalf("SetKeyringAPIKey failed: %v", err)
	}
	if stored, err := env.MockStore.Get("api_key:prod"); err != nil || stored != euKey {
		t.Fatalf("api_key:prod = %q, %v, want the EU key", stored, err)
	}

	key, source, err := GetAPIKey("")
	if err != nil || key != euKey || source != SourceKeyring {
		t.Fatalf("GetAPIKey() = %q, %q, %v, want the EU key from the keyring", key, source, err)
	}
	if got := APIURLForKey(key); got != "https://eu-api.notte.cc" {
		t.Errorf("APIURLForKey() = %q, want the EU API", got)
	}

	for _, explicit := range []string{"https://eu-api.notte.cc", "https://api.notte.cc"} {
		env.SetEnv("NOTTE_API_URL", explicit)
		if got, err := GetKeyringAPIKey(); err != nil || got != euKey {
			t.Errorf("NOTTE_API_URL=%s: GetKeyringAPIKey() = %q, %v, want the EU key", explicit, got, err)
		}
		if got := APIURLForKey(euKey); got != explicit {
			t.Errorf("NOTTE_API_URL=%s: APIURLForKey() = %q, want the explicit URL", explicit, got)
		}
	}

	_ = os.Unsetenv("NOTTE_API_URL")
	if err := DeleteKeyringAPIKey(); err != nil {
		t.Fatalf("DeleteKeyringAPIKey failed: %v", err)
	}
	if _, err := env.MockStore.Get("api_key:prod"); err == nil {
		t.Error("api_key:prod still set after delete")
	}
}

func TestKeyring_ExistingProdKeyKeepsTheDefaultAPI(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	SetKeyring(env.MockStore)
	defer ResetKeyring()
	withConfigDir(t, env, "")

	_ = env.MockStore.Set("api_key:prod", "sk-notte-existing")
	key, err := GetKeyringAPIKey()
	if err != nil || key != "sk-notte-existing" {
		t.Fatalf("GetKeyringAPIKey() = %q, %v", key, err)
	}
	if got := APIURLForKey(key); got != "https://api.notte.cc" {
		t.Errorf("APIURLForKey() = %q, want the default API", got)
	}
}
