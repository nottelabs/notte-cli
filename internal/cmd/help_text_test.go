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

// Every command shown under "Get started" in root help must exist: agents copy
// these lines verbatim, and a stale one costs them an "unknown command" error.
func TestRootHelpGetStartedCommandsExist(t *testing.T) {
	inSection := false
	for _, line := range strings.Split(rootCmd.Long, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Get started:" {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if trimmed == "" {
			break
		}
		cmdPart := strings.TrimSpace(strings.SplitN(trimmed, "#", 2)[0])
		var words []string
		for _, w := range strings.Fields(cmdPart)[1:] {
			if strings.HasPrefix(w, "<") || strings.HasPrefix(w, "-") {
				break
			}
			words = append(words, w)
		}
		found, _, err := rootCmd.Find(words)
		if err != nil || found == rootCmd || found.Name() != words[len(words)-1] {
			t.Errorf("root help lists %q, which is not a command", cmdPart)
		}
	}
}
