package auth

import (
	"testing"
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
