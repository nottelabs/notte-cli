package pyenv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// realVenv builds an environment from the captured staging report once per
// test binary. Slow, networked and skipped under -short.
func realVenv(t *testing.T) (string, *Health) {
	t.Helper()
	if testing.Short() {
		t.Skip("network")
	}
	tc := toolchain(t)
	h := loadFixture(t, "health_ok.json")

	// Shared across tests in this package run; building it per test would add
	// minutes for no extra coverage.
	venv := filepath.Join(os.TempDir(), "notte-pyenv-test-venv")
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	if _, err := Sync(ctx, tc, SyncRequest{
		VenvDir: venv, Health: h,
		Imports: []string{"requests", "pydantic", "notte_sdk", "httpcloak"},
	}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	return venv, h
}

func validate(t *testing.T, venv string, h *Health, src string) *Verdict {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	v, err := Validate(ctx, venv, h, src)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	return v
}

// The whole reason the allow list is injected. The published SDK rejects
// httpcloak, and ~333 deployed functions import it. With the runtime's list
// substituted, a real production function must pass.
func TestValidateAcceptsARealProductionFunction(t *testing.T) {
	venv, h := realVenv(t)

	src, err := os.ReadFile(filepath.Join(os.Getenv("HOME"),
		"Desktop/projects/anything-api/marketplace/99.co/list_condos_by_letter.py"))
	if err != nil {
		t.Skipf("marketplace checkout not available: %v", err)
	}

	v := validate(t, venv, h, string(src))
	if !v.OK {
		t.Fatalf("a deployed, serving function was rejected: %v", v.Errors)
	}
	if len(v.Variables) == 0 {
		t.Fatal("run() parameters should be extracted as invocation variables")
	}
}

// Same source, unpatched SDK list: this is the bug the injection avoids.
func TestUnpatchedSDKWouldRejectThatSameFunction(t *testing.T) {
	venv, _ := realVenv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// A report whose only allowed imports are the SDK's own stale list.
	stale := &Health{
		Status: StatusOK, PythonVersion: "3.12.0", RuntimeDigest: "sha256:x",
		StdlibModules: []string{"typing"},
		Packages:      []Package{{ImportName: "pydantic"}},
	}
	v, err := Validate(ctx, venv, stale, "import httpcloak\ndef run():\n    return 1\n")
	if err != nil {
		t.Fatal(err)
	}
	if v.OK {
		t.Fatal("with httpcloak absent from the allow list the validator must reject it — " +
			"if this passes, the injection is not taking effect")
	}
}

func TestValidateRejectsStructuralProblems(t *testing.T) {
	venv, h := realVenv(t)

	cases := map[string]string{
		"no run()":        "def other():\n    return 1\n",
		"relative import": "from .util import x\ndef run():\n    return 1\n",
		"denied stdlib":   "import subprocess\ndef run():\n    return 1\n",
		"forbidden call":  "def run():\n    exec(\"x=1\")\n    return 1\n",
		"unknown package": "import pandas\ndef run():\n    return 1\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if v := validate(t, venv, h, src); v.OK {
				t.Fatalf("%s should have been rejected", name)
			}
		})
	}
}

// The one gap in the SDK validator: it accepts two top-level run() definitions
// where the server rejects them, so the bridge checks it directly.
func TestValidateRejectsTwoRunDefinitions(t *testing.T) {
	venv, h := realVenv(t)
	v := validate(t, venv, h, "def run():\n    return 1\n\n\ndef run():\n    return 2\n")
	if v.OK {
		t.Fatal("two run() definitions must be rejected")
	}
	joined := strings.Join(v.Errors, " ")
	if !strings.Contains(joined, "multiple top-level run()") {
		t.Fatalf("error should name the problem, got %v", v.Errors)
	}
}

// A rejected script is a verdict, not a crash: a non-zero exit would be
// indistinguishable from the interpreter or the SDK being broken.
func TestValidateReportsSyntaxErrorsAsAVerdict(t *testing.T) {
	venv, h := realVenv(t)
	v := validate(t, venv, h, "def run(:\n")
	if v.OK {
		t.Fatal("a syntax error must not pass")
	}
	if v.Stage != "syntax" {
		t.Fatalf("stage = %q, want syntax", v.Stage)
	}
}

func TestValidateRefusesADegradedReport(t *testing.T) {
	_, err := Validate(context.Background(), t.TempDir(), loadFixture(t, "health_degraded.json"), "def run(): pass")
	if err == nil {
		t.Fatal("a degraded report carries no import list, so validation cannot be authoritative")
	}
}
