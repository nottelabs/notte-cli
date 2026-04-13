package update

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewChecker_DevVersion(t *testing.T) {
	checker := NewChecker("dev")
	if checker != nil {
		t.Fatal("expected nil checker for dev version")
	}
}

func TestNewChecker_EnvDisabled(t *testing.T) {
	t.Setenv("NOTTE_NO_UPDATE_CHECK", "1")
	checker := NewChecker("0.0.10")
	if checker != nil {
		t.Fatal("expected nil checker when NOTTE_NO_UPDATE_CHECK is set")
	}
}

func TestNewChecker_ValidVersion(t *testing.T) {
	// Set to empty string so the check is not disabled
	t.Setenv("NOTTE_NO_UPDATE_CHECK", "")

	checker := NewChecker("0.0.10")
	if checker == nil {
		t.Fatal("expected non-nil checker for valid version")
	}
	if checker.currentVersion != "0.0.10" {
		t.Errorf("currentVersion = %q, want %q", checker.currentVersion, "0.0.10")
	}
}

func TestPrintUpdateNotification_JSONMode(t *testing.T) {
	var buf bytes.Buffer
	result := &Result{
		CurrentVersion:  "0.0.10",
		LatestVersion:   "v0.0.12",
		ReleaseURL:      "https://github.com/nottelabs/notte-cli/releases/tag/v0.0.12",
		UpdateAvailable: true,
	}

	PrintUpdateNotification(result, &buf, strings.NewReader(""), false, true, false)

	if buf.Len() > 0 {
		t.Errorf("expected no output in JSON mode, got %q", buf.String())
	}
}

func TestPrintUpdateNotification_NilResult(t *testing.T) {
	var buf bytes.Buffer
	PrintUpdateNotification(nil, &buf, strings.NewReader(""), false, false, false)
	if buf.Len() > 0 {
		t.Errorf("expected no output for nil result, got %q", buf.String())
	}
}

func TestPrintUpdateNotification_NotAvailable(t *testing.T) {
	var buf bytes.Buffer
	result := &Result{
		CurrentVersion:  "0.0.12",
		LatestVersion:   "v0.0.12",
		UpdateAvailable: false,
	}

	PrintUpdateNotification(result, &buf, strings.NewReader(""), false, false, false)
	if buf.Len() > 0 {
		t.Errorf("expected no output when no update available, got %q", buf.String())
	}
}

func TestPrintUpdateNotification_NonInteractive(t *testing.T) {
	var buf bytes.Buffer
	result := &Result{
		CurrentVersion:  "0.0.10",
		LatestVersion:   "v0.0.12",
		ReleaseURL:      "https://github.com/nottelabs/notte-cli/releases/tag/v0.0.12",
		UpdateAvailable: true,
	}

	// strings.NewReader is not a terminal, so prompt should be skipped
	PrintUpdateNotification(result, &buf, strings.NewReader(""), false, false, true)

	output := buf.String()
	if !strings.Contains(output, "Update available") {
		t.Error("expected update notification in output")
	}
	if !strings.Contains(output, "v0.0.10") {
		t.Error("expected current version in output")
	}
	if !strings.Contains(output, "v0.0.12") {
		t.Error("expected latest version in output")
	}
	if !strings.Contains(output, "Changelog:") {
		t.Error("expected changelog link in output")
	}
	// Should NOT contain the prompt since stdin is not a terminal
	if strings.Contains(output, "Would you like to upgrade now?") {
		t.Error("should not prompt in non-interactive mode")
	}
}

func TestPrintUpdateNotification_NoColor(t *testing.T) {
	var buf bytes.Buffer
	result := &Result{
		CurrentVersion:  "0.0.10",
		LatestVersion:   "v0.0.12",
		ReleaseURL:      "https://github.com/nottelabs/notte-cli/releases/tag/v0.0.12",
		UpdateAvailable: true,
	}

	PrintUpdateNotification(result, &buf, strings.NewReader(""), false, false, true)

	output := buf.String()
	// Should not contain ANSI escape codes
	if strings.Contains(output, "\033[") {
		t.Error("expected no ANSI escape codes with noColor=true")
	}
	if !strings.Contains(output, "Update available for Notte CLI") {
		t.Error("expected update message in output")
	}
}

func TestPrintUpdateNotification_DeclineUpgrade(t *testing.T) {
	var buf bytes.Buffer
	result := &Result{
		CurrentVersion:  "0.0.10",
		LatestVersion:   "v0.0.12",
		ReleaseURL:      "https://github.com/nottelabs/notte-cli/releases/tag/v0.0.12",
		UpdateAvailable: true,
	}

	// Simulate typing "n" - but since strings.NewReader is not a terminal,
	// the prompt won't appear. Test that skipConfirm=false + non-terminal = no prompt.
	PrintUpdateNotification(result, &buf, strings.NewReader("n\n"), false, false, true)

	output := buf.String()
	if strings.Contains(output, "Upgrading") {
		t.Error("should not upgrade when user declines")
	}
}

func TestResult_Fields(t *testing.T) {
	r := &Result{
		CurrentVersion:  "0.0.10",
		LatestVersion:   "v0.0.12",
		ReleaseURL:      "https://example.com/release",
		UpdateAvailable: true,
	}

	if r.CurrentVersion != "0.0.10" {
		t.Errorf("unexpected CurrentVersion: %q", r.CurrentVersion)
	}
	if r.LatestVersion != "v0.0.12" {
		t.Errorf("unexpected LatestVersion: %q", r.LatestVersion)
	}
	if r.ReleaseURL != "https://example.com/release" {
		t.Errorf("unexpected ReleaseURL: %q", r.ReleaseURL)
	}
	if !r.UpdateAvailable {
		t.Error("expected UpdateAvailable to be true")
	}
}
