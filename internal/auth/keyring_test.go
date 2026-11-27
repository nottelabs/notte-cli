package auth

import (
	"errors"
	"fmt"
	"os"
	"strings"
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

func TestKeyring_MigratesKeySavedUnderTheEUHostName(t *testing.T) {
	// Before the EU host mapped to "prod", NOTTE_API_URL=https://eu-api.notte.cc
	// saved the key under the hostname. It must still be found, with or without
	// the URL set, and logout must remove it.
	for _, apiURL := range []string{"", "https://eu-api.notte.cc"} {
		t.Run("NOTTE_API_URL="+apiURL, func(t *testing.T) {
			env := testutil.SetupTestEnv(t)
			SetKeyring(env.MockStore)
			defer ResetKeyring()
			withConfigDir(t, env, "")
			if apiURL != "" {
				env.SetEnv("NOTTE_API_URL", apiURL)
			}

			const euKey = "sk-notte-eu-saved"
			_ = env.MockStore.Set("api_key:eu-api.notte.cc", euKey)

			key, err := GetKeyringAPIKey()
			if err != nil || key != euKey {
				t.Fatalf("GetKeyringAPIKey() = %q, %v, want the key saved under the EU host", key, err)
			}
			if got, err := env.MockStore.Get("api_key:prod"); err != nil || got != euKey {
				t.Errorf("api_key:prod = %q, %v, want the migrated key", got, err)
			}
			if _, err := env.MockStore.Get("api_key:eu-api.notte.cc"); err == nil {
				t.Error("legacy EU entry still present after migration")
			}
		})
	}
}

func TestKeyring_DeleteRemovesKeySavedUnderTheEUHostName(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	SetKeyring(env.MockStore)
	defer ResetKeyring()
	withConfigDir(t, env, "")

	// Only the legacy entry: deleting succeeds and removes it.
	_ = env.MockStore.Set("api_key:eu-api.notte.cc", "sk-notte-eu-saved")
	if err := DeleteKeyringAPIKey(); err != nil {
		t.Fatalf("DeleteKeyringAPIKey() = %v, want nil", err)
	}
	if _, err := env.MockStore.Get("api_key:eu-api.notte.cc"); err == nil {
		t.Error("legacy EU entry still present after delete")
	}

	// Both entries: both go.
	_ = env.MockStore.Set("api_key:prod", "sk-notte-prod")
	_ = env.MockStore.Set("api_key:eu-api.notte.cc", "sk-notte-eu-saved")
	if err := DeleteKeyringAPIKey(); err != nil {
		t.Fatalf("DeleteKeyringAPIKey() = %v, want nil", err)
	}
	for _, k := range []string{"api_key:prod", "api_key:eu-api.notte.cc"} {
		if _, err := env.MockStore.Get(k); err == nil {
			t.Errorf("%s still present after delete", k)
		}
	}
}

func TestDeleteKeyringAPIKey_NothingSavedIsNotFound(t *testing.T) {
	// Checking the EU host's entry too must not turn "nothing saved" into success.
	env := testutil.SetupTestEnv(t)
	withConfigDir(t, env, "")
	SetKeyring(missingKeyring{})
	t.Cleanup(ResetKeyring)
	if err := DeleteKeyringAPIKey(); !IsNoSavedKey(err) {
		t.Errorf("DeleteKeyringAPIKey() with nothing saved = %v, want a not-found error", err)
	}
}

// failingDeleteKeyring is a mock keyring whose Delete fails for one key.
type failingDeleteKeyring struct {
	*testutil.MockKeyring
	failKey string
}

func (k failingDeleteKeyring) Delete(key string) error {
	if key == k.failKey {
		return errors.New("keychain is locked")
	}
	return k.MockKeyring.Delete(key)
}

func TestDeleteKeyringAPIKey_ReportsAFailedLegacyEUDelete(t *testing.T) {
	// A key left behind under the EU host's name must not read as signed out.
	for _, prodSaved := range []bool{true, false} {
		t.Run(fmt.Sprintf("prod saved=%v", prodSaved), func(t *testing.T) {
			env := testutil.SetupTestEnv(t)
			withConfigDir(t, env, "")
			_ = env.MockStore.Set("api_key:eu-api.notte.cc", "sk-notte-eu-saved")
			if prodSaved {
				_ = env.MockStore.Set("api_key:prod", "sk-notte-prod")
			}
			SetKeyring(failingDeleteKeyring{env.MockStore, "api_key:eu-api.notte.cc"})
			t.Cleanup(ResetKeyring)

			err := DeleteKeyringAPIKey()
			if err == nil || IsNoSavedKey(err) {
				t.Fatalf("DeleteKeyringAPIKey() = %v, want the legacy delete failure", err)
			}
			if !strings.Contains(err.Error(), "keychain is locked") {
				t.Errorf("DeleteKeyringAPIKey() = %v, want it to name the failure", err)
			}
			if _, err := env.MockStore.Get("api_key:prod"); err == nil {
				t.Error("api_key:prod still present: the prod entry should be removed regardless")
			}
		})
	}
}
