package update

import (
	"bytes"
	"strings"
	"testing"
)

func TestDetectInstallMethod(t *testing.T) {
	// This test verifies the function runs without panic.
	// The result depends on the local environment (whether brew is installed).
	method := DetectInstallMethod()
	if method != UpgradeHomebrew && method != UpgradeManual {
		t.Errorf("unexpected install method: %d", method)
	}
}

func TestRunUpgrade_Manual(t *testing.T) {
	var buf bytes.Buffer
	err := RunUpgrade(&buf, UpgradeManual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "To upgrade manually") {
		t.Error("expected manual upgrade instructions")
	}
	if !strings.Contains(output, "brew install") {
		t.Error("expected brew install instruction")
	}
	if !strings.Contains(output, "go install") {
		t.Error("expected go install instruction")
	}
	if !strings.Contains(output, "github.com/nottelabs/notte-cli") {
		t.Error("expected GitHub URL in instructions")
	}
}

func TestUpgradeMethod_Constants(t *testing.T) {
	// Verify the constants have distinct values
	if UpgradeHomebrew == UpgradeManual {
		t.Error("UpgradeHomebrew and UpgradeManual should be distinct")
	}
}
