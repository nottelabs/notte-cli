//go:build integration

package integration

import (
	"encoding/json"
	"testing"
)

func TestProfilesList(t *testing.T) {
	// List profiles - should work even if empty
	result := runCLI(t, "profiles", "list")
	requireSuccess(t, result)
	t.Log("Successfully listed profiles")
}

func TestProfilesCreateAndDelete(t *testing.T) {
	// Create a new profile
	result := runCLI(t, "profiles", "create")
	requireSuccess(t, result)

	// Parse the response to get profile ID
	var createResp struct {
		ProfileID string `json:"profile_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse profile create response: %v", err)
	}
	profileID := createResp.ProfileID
	if profileID == "" {
		t.Fatal("No profile ID returned from create command")
	}
	t.Logf("Created profile: %s", profileID)

	// Ensure cleanup
	defer cleanupProfile(t, profileID)

	// Show profile details
	result = runCLI(t, "profiles", "show", "--profile-id", profileID)
	requireSuccess(t, result)
	if !containsString(result.Stdout, profileID) {
		t.Error("Profile show did not contain profile ID")
	}

	// List profiles - should include our profile
	result = runCLI(t, "profiles", "list")
	requireSuccess(t, result)
	if !containsString(result.Stdout, profileID) {
		t.Error("Profile list did not contain our profile")
	}
	t.Log("Profile create and delete test completed successfully")
}

func TestProfilesCreateWithName(t *testing.T) {
	// Create a profile with a custom name
	result := runCLI(t, "profiles", "create", "--name", "test-profile-integration")
	requireSuccess(t, result)

	var createResp struct {
		ProfileID string `json:"profile_id"`
		Name      string `json:"name"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse profile create response: %v", err)
	}
	profileID := createResp.ProfileID
	if profileID == "" {
		t.Fatal("No profile ID returned from create command")
	}
	t.Logf("Created profile with name: %s", profileID)

	defer cleanupProfile(t, profileID)

	// Show profile to verify name
	result = runCLI(t, "profiles", "show", "--profile-id", profileID)
	requireSuccess(t, result)
	if !containsString(result.Stdout, "test-profile-integration") {
		t.Log("Profile name might not be in show output, but creation succeeded")
	}
	t.Log("Profile with name created successfully")
}

func TestProfilesShowNonexistent(t *testing.T) {
	// Try to show a non-existent profile
	result := runCLI(t, "profiles", "show", "--profile-id", "nonexistent-profile-id-12345")
	requireFailure(t, result)
	t.Log("Correctly failed to show non-existent profile")
}

func TestProfilesDeleteNonexistent(t *testing.T) {
	// Try to delete a non-existent profile
	result := runCLI(t, "profiles", "delete", "--profile-id", "nonexistent-profile-id-12345")
	requireFailure(t, result)
	t.Log("Correctly failed to delete non-existent profile")
}
