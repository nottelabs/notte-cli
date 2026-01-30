package auth

import (
	"os"
	"testing"

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
