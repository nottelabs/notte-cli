package cmd

import (
	"strings"
	"testing"
)

// The help text is published as the CLI reference, so it must not steer
// readers wrong.

func TestScreenshotExampleUsesPathFlag(t *testing.T) {
	// --output is the global output format flag, not the file to write.
	if strings.Contains(pageScreenshotCmd.Long, "--output out.jpg") {
		t.Error("page screenshot help passes the file name to --output")
	}
	if !strings.Contains(pageScreenshotCmd.Long, "notte page screenshot --path out.jpg") {
		t.Error("page screenshot help should show --path for the output file")
	}
}

func TestVaultPasswordNoteIsAccurate(t *testing.T) {
	note := strings.Join(strings.Fields(vaultsCredentialsAddCmd.Long), " ")
	// An environment variable keeps the value out of shell history only: the
	// shell expands it into notte's arguments, where ps can still see it.
	if !strings.Contains(note, "keeps the literal value out of your shell history") ||
		!strings.Contains(note, "still passes it to notte as an argument") {
		t.Errorf("vault credentials add help overstates what an environment variable protects: %q", note)
	}
}
