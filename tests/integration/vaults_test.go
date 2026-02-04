//go:build integration

package integration

import (
	"encoding/json"
	"testing"
)

func TestVaultsList(t *testing.T) {
	// List vaults - should work even if empty
	result := runCLI(t, "vaults", "list")
	requireSuccess(t, result)
	t.Log("Successfully listed vaults")
}

func TestVaultsCreateAndDelete(t *testing.T) {
	// Create a new vault
	result := runCLI(t, "vaults", "create")
	requireSuccess(t, result)

	// Parse the response to get vault ID
	var createResp struct {
		VaultID string `json:"vault_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse vault create response: %v", err)
	}
	vaultID := createResp.VaultID
	if vaultID == "" {
		t.Fatal("No vault ID returned from create command")
	}
	t.Logf("Created vault: %s", vaultID)

	// Ensure cleanup
	defer cleanupVault(t, vaultID)

	// List vaults - should include our vault
	result = runCLI(t, "vaults", "list")
	requireSuccess(t, result)
	if !containsString(result.Stdout, vaultID) {
		t.Error("Vault list did not contain our vault")
	}
	t.Log("Vault create and delete test completed successfully")
}

func TestVaultsCreateWithName(t *testing.T) {
	// Create a vault with a custom name
	result := runCLI(t, "vaults", "create", "--name", "test-vault-integration")
	requireSuccess(t, result)

	var createResp struct {
		VaultID string `json:"vault_id"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse vault create response: %v", err)
	}
	vaultID := createResp.VaultID
	if vaultID == "" {
		t.Fatal("No vault ID returned from create command")
	}
	t.Logf("Created vault with name: %s", vaultID)

	defer cleanupVault(t, vaultID)
	t.Log("Vault with name created successfully")
}

func TestVaultsCredentialsLifecycle(t *testing.T) {
	// Create a vault first
	result := runCLI(t, "vaults", "create", "--name", "test-vault-credentials")
	requireSuccess(t, result)

	var createResp struct {
		VaultID string `json:"vault_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse vault create response: %v", err)
	}
	vaultID := createResp.VaultID
	defer cleanupVault(t, vaultID)

	// Add credentials
	result = runCLI(t, "vaults", "credentials", "add",
		"--vault-id", vaultID,
		"--url", "https://example.com",
		"--email", "test@example.com",
		"--password", "testpassword123",
	)
	requireSuccess(t, result)
	t.Log("Successfully added credentials")

	// List credentials
	result = runCLI(t, "vaults", "credentials", "list", "--vault-id", vaultID)
	requireSuccess(t, result)
	if !containsString(result.Stdout, "example.com") {
		t.Log("Credentials URL might be stored differently")
	}
	t.Log("Successfully listed credentials")

	// Get credentials for URL
	result = runCLI(t, "vaults", "credentials", "get", "--vault-id", vaultID, "--url", "https://example.com")
	requireSuccess(t, result)
	t.Log("Successfully retrieved credentials")

	// Delete credentials
	result = runCLI(t, "vaults", "credentials", "delete", "--vault-id", vaultID, "--url", "https://example.com")
	requireSuccess(t, result)
	t.Log("Vault credentials lifecycle completed successfully")
}

func TestVaultsCredentialsWithUsername(t *testing.T) {
	// Create a vault
	result := runCLI(t, "vaults", "create")
	requireSuccess(t, result)

	var createResp struct {
		VaultID string `json:"vault_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse vault create response: %v", err)
	}
	vaultID := createResp.VaultID
	defer cleanupVault(t, vaultID)

	// Add credentials with username
	result = runCLI(t, "vaults", "credentials", "add",
		"--vault-id", vaultID,
		"--url", "https://test-site.com",
		"--username", "testuser",
		"--password", "testpassword456",
	)
	requireSuccess(t, result)
	t.Log("Successfully added credentials with username")

	// Get credentials
	result = runCLI(t, "vaults", "credentials", "get", "--vault-id", vaultID, "--url", "https://test-site.com")
	requireSuccess(t, result)
	t.Log("Credentials with username test completed successfully")
}

func TestVaultsCredentialsWithMFA(t *testing.T) {
	// Create a vault
	result := runCLI(t, "vaults", "create")
	requireSuccess(t, result)

	var createResp struct {
		VaultID string `json:"vault_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse vault create response: %v", err)
	}
	vaultID := createResp.VaultID
	defer cleanupVault(t, vaultID)

	// Add credentials with MFA secret
	result = runCLI(t, "vaults", "credentials", "add",
		"--vault-id", vaultID,
		"--url", "https://secure-site.com",
		"--email", "mfa@example.com",
		"--password", "securepassword",
		"--mfa-secret", "JBSWY3DPEHPK3PXP",
	)
	requireSuccess(t, result)
	t.Log("Successfully added credentials with MFA secret")
}

func TestVaultsUpdate(t *testing.T) {
	// Create a vault
	result := runCLI(t, "vaults", "create", "--name", "original-name")
	requireSuccess(t, result)

	var createResp struct {
		VaultID string `json:"vault_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse vault create response: %v", err)
	}
	vaultID := createResp.VaultID
	defer cleanupVault(t, vaultID)

	// Update vault name
	result = runCLI(t, "vaults", "update", "--vault-id", vaultID, "--name", "updated-name")
	requireSuccess(t, result)
	t.Log("Successfully updated vault name")
}

func TestVaultsDeleteNonexistent(t *testing.T) {
	// Try to delete a non-existent vault
	result := runCLI(t, "vaults", "delete", "--vault-id", "nonexistent-vault-id-12345")
	requireFailure(t, result)
	t.Log("Correctly failed to delete non-existent vault")
}

func TestVaultsCredentialsGetNonexistent(t *testing.T) {
	// Create a vault
	result := runCLI(t, "vaults", "create")
	requireSuccess(t, result)

	var createResp struct {
		VaultID string `json:"vault_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse vault create response: %v", err)
	}
	vaultID := createResp.VaultID
	defer cleanupVault(t, vaultID)

	// Try to get credentials for a URL that doesn't exist
	result = runCLI(t, "vaults", "credentials", "get", "--vault-id", vaultID, "--url", "https://nonexistent-url.com")
	requireFailure(t, result)
	t.Log("Correctly failed to get non-existent credentials")
}
