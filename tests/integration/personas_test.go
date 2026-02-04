//go:build integration

package integration

import (
	"encoding/json"
	"testing"
)

func TestPersonasList(t *testing.T) {
	// List personas - should work even if empty
	result := runCLI(t, "personas", "list")
	requireSuccess(t, result)
	t.Log("Successfully listed personas")
}

func TestPersonasCreateAndDelete(t *testing.T) {
	// Create a new persona
	result := runCLI(t, "personas", "create")
	requireSuccess(t, result)

	// Parse the response to get persona ID
	var createResp struct {
		PersonaID string `json:"persona_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse persona create response: %v", err)
	}
	personaID := createResp.PersonaID
	if personaID == "" {
		t.Fatal("No persona ID returned from create command")
	}
	t.Logf("Created persona: %s", personaID)

	// Ensure cleanup
	defer cleanupPersona(t, personaID)

	// Show persona details
	result = runCLI(t, "personas", "show", "--persona-id", personaID)
	requireSuccess(t, result)
	if !containsString(result.Stdout, personaID) {
		t.Error("Persona show did not contain persona ID")
	}

	// List personas - should include our persona
	result = runCLI(t, "personas", "list")
	requireSuccess(t, result)
	if !containsString(result.Stdout, personaID) {
		t.Error("Persona list did not contain our persona")
	}
	t.Log("Persona create and delete test completed successfully")
}

func TestPersonasCreateWithVault(t *testing.T) {
	// Create a persona with a vault
	result := runCLI(t, "personas", "create", "--create-vault")
	requireSuccess(t, result)

	var createResp struct {
		PersonaID string `json:"persona_id"`
		VaultID   string `json:"vault_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse persona create response: %v", err)
	}
	personaID := createResp.PersonaID
	if personaID == "" {
		t.Fatal("No persona ID returned from create command")
	}
	t.Logf("Created persona with vault: %s", personaID)

	defer cleanupPersona(t, personaID)

	// Show persona details
	result = runCLI(t, "personas", "show", "--persona-id", personaID)
	requireSuccess(t, result)
	t.Log("Persona with vault created successfully")
}

func TestPersonasEmails(t *testing.T) {
	// Create a persona first
	result := runCLI(t, "personas", "create")
	requireSuccess(t, result)

	var createResp struct {
		PersonaID string `json:"persona_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse persona create response: %v", err)
	}
	personaID := createResp.PersonaID
	defer cleanupPersona(t, personaID)

	// List emails for the persona
	result = runCLI(t, "personas", "emails", "--persona-id", personaID)
	requireSuccess(t, result)
	t.Log("Successfully listed persona emails")
}

func TestPersonasSms(t *testing.T) {
	// Create a persona with phone number
	result := runCLI(t, "personas", "create", "--create-phone-number")
	requireSuccess(t, result)

	var createResp struct {
		PersonaID string `json:"persona_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse persona create response: %v", err)
	}
	personaID := createResp.PersonaID
	defer cleanupPersona(t, personaID)

	// List SMS messages for the persona
	result = runCLI(t, "personas", "sms", "--persona-id", personaID)
	requireSuccess(t, result)
	t.Log("Successfully listed persona SMS messages")
}

func TestPersonasShowNonexistent(t *testing.T) {
	// Try to show a non-existent persona
	result := runCLI(t, "personas", "show", "--persona-id", "nonexistent-persona-id-12345")
	requireFailure(t, result)
	t.Log("Correctly failed to show non-existent persona")
}

func TestPersonasDeleteNonexistent(t *testing.T) {
	// Try to delete a non-existent persona
	result := runCLI(t, "personas", "delete", "--persona-id", "nonexistent-persona-id-12345")
	requireFailure(t, result)
	t.Log("Correctly failed to delete non-existent persona")
}
