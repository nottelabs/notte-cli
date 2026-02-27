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
	// Use existing persona with phone number (monorepo)
	personaID := "7abb4f37-25a1-4409-98d9-c4c916918254"

	// List SMS messages for the persona
	result := runCLI(t, "personas", "sms", "--persona-id", personaID)
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

func TestZZZ_CleanupPersonas(t *testing.T) {
	// Important personas that should never be deleted
	importantPersonas := map[string]bool{
		// Front end tests
		"f2e2834b-a054-4a96-a388-a447c37756ff": true,
		"131a21e1-8c8e-4016-80b9-765c0ce4fb5c": true,
		"ee3da1f5-e53c-4159-839d-e8db16bbe2e7": true,
		"46d0649e-1d13-47be-a21f-703ce4cf02ea": true,
		// Monorepo
		"7abb4f37-25a1-4409-98d9-c4c916918254": true,
		// Others
		"23ae78af-93b4-4aeb-ba21-d18e1496bdd9": true,
		"4e9faffa-ae3e-4a86-a87f-584bf77794e0": true,
	}

	result := runCLI(t, "personas", "list", "--page-size", "100")
	requireSuccess(t, result)

	var personas []struct {
		PersonaID string `json:"persona_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &personas); err != nil {
		t.Fatalf("Failed to parse personas list: %v", err)
	}

	deleted := 0
	for _, p := range personas {
		if importantPersonas[p.PersonaID] {
			continue
		}
		r := runCLI(t, "personas", "delete", "--persona-id", p.PersonaID)
		if r.ExitCode == 0 {
			deleted++
		} else {
			t.Logf("Warning: failed to delete persona %s: %s", p.PersonaID, r.Stderr)
		}
	}
	t.Logf("Cleanup complete: deleted %d personas, kept %d important", deleted, len(personas)-deleted)
}
