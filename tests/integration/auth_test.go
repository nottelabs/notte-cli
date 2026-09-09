//go:build integration

package integration

import (
	"testing"
)

// TestAuth_APIRejectsInvalidKey pins the contract sign-in validation relies on:
// the endpoint it checks a key against must actually reject a bad one.
//
// Validation used to run against /health, which answers 200 with no credential
// at all, so every string validated successfully and was stored as a working
// key. Unit tests can only assert this against a stub. If the real endpoint
// ever stopped requiring authentication, the bug would return with every unit
// test still green - this is the test that would notice.
func TestAuth_APIRejectsInvalidKey(t *testing.T) {
	t.Setenv("NOTTE_API_KEY", "sk-notte-not-a-real-key-000000000000")

	result := runCLI(t, "usage")
	requireFailure(t, result)

	// The failure has to be about the credential. A generic transport error
	// would satisfy requireFailure while telling us nothing about whether the
	// key was checked.
	output := result.Stderr + result.Stdout
	if !containsString(output, "401") &&
		!containsString(output, "auth") &&
		!containsString(output, "Auth") &&
		!containsString(output, "key") {
		t.Errorf("expected an authentication failure, got: %s", output)
	}

	t.Logf("invalid key correctly rejected: %s", output)
}

// TestAuth_ValidKeyIsAccepted is the other half: the same endpoint answers a
// real key, so validation does not reject keys that are fine.
func TestAuth_ValidKeyIsAccepted(t *testing.T) {
	result := runCLI(t, "usage")
	requireSuccess(t, result)
}
