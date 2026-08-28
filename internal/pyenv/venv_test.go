package pyenv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// toolchain skips a test when uv is absent. These tests exercise real
// subprocesses on purpose: the failure modes worth catching here are the ones
// a fake would define away.
func toolchain(t *testing.T) *Toolchain {
	t.Helper()
	tc, err := FindToolchain()
	if err != nil {
		t.Skip("uv not installed")
	}
	return tc
}

func TestFindToolchainErrorExplainsTheFix(t *testing.T) {
	err := ErrNoToolchain{}
	if !strings.Contains(err.Error(), "astral.sh/uv/install.sh") {
		t.Fatalf("error should name the install command, got %v", err)
	}
	if !strings.Contains(err.Error(), "downloads the Python") {
		t.Fatal("error should say uv supplies the interpreter, so users do not go hunting for Python 3.12")
	}
}

// A degraded report has no package list. Building from it would produce an
// empty environment in which every import fails — a confident, wrong answer.
func TestSyncRefusesADegradedReport(t *testing.T) {
	h := loadFixture(t, "health_degraded.json")
	_, err := Sync(context.Background(), &Toolchain{UV: "uv"}, SyncRequest{
		VenvDir: t.TempDir(), Health: h, Imports: []string{"requests"},
	})
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if !strings.Contains(err.Error(), "degraded") {
		t.Fatalf("error should name the status, got %v", err)
	}
}

// Classification happens before any subprocess runs, so it can be asserted
// without uv.
func TestSyncClassifiesImportsBeforeInstalling(t *testing.T) {
	h := loadFixture(t, "health_ok.json")
	install, missing := h.Installable([]string{"requests", "notte", "pandas", "json"})

	if len(install) == 0 {
		t.Fatal("requests should be installable")
	}
	if len(missing) != 1 || missing[0] != "notte" {
		t.Fatalf("allowed-but-absent = %v, want [notte]", missing)
	}
	if got := notAllowed(h, []string{"requests", "pandas", "os", "json"}); len(got) != 2 {
		t.Fatalf("notAllowed = %v, want pandas and os", got)
	}
}

func TestStampMatching(t *testing.T) {
	base := Stamp{RuntimeDigest: "sha256:a", PythonVersion: "3.12.0", Requirements: []string{"requests==1"}}
	if !base.matches(base) {
		t.Fatal("identical stamps should match")
	}
	for _, other := range []Stamp{
		{RuntimeDigest: "sha256:b", PythonVersion: "3.12.0", Requirements: []string{"requests==1"}},
		{RuntimeDigest: "sha256:a", PythonVersion: "3.11.0", Requirements: []string{"requests==1"}},
		{RuntimeDigest: "sha256:a", PythonVersion: "3.12.0", Requirements: []string{"requests==2"}},
		{RuntimeDigest: "sha256:a", PythonVersion: "3.12.0"},
	} {
		if base.matches(other) {
			t.Errorf("should not match: %+v", other)
		}
	}
	// An empty digest is what a degraded report yields; it must never match.
	empty := Stamp{PythonVersion: "3.12.0"}
	if empty.matches(empty) {
		t.Fatal("an empty digest must never satisfy a reuse check")
	}
}

// ty treats an unusable --python as fatal for the entire run, so an absent
// interpreter must be caught before ty is invoked rather than surfacing as a
// wall of unresolved imports.
func TestTypeCheckRefusesAMissingInterpreter(t *testing.T) {
	dir := t.TempDir()
	_, err := TypeCheck(context.Background(), &Toolchain{UV: "uv"}, dir,
		filepath.Join(dir, "nonexistent-venv"), []string{"x.py"})
	if err == nil {
		t.Fatal("a missing interpreter must be reported before ty runs")
	}
	if !strings.Contains(err.Error(), "no interpreter at") {
		t.Fatalf("error should name the path, got %v", err)
	}
}

// The slow path: build a real environment from the real staging report.
func TestSyncBuildsAndReusesARealEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("network")
	}
	tc := toolchain(t)
	h := loadFixture(t, "health_ok.json")
	venv := filepath.Join(t.TempDir(), "venv")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	res, err := Sync(ctx, tc, SyncRequest{VenvDir: venv, Health: h, Imports: []string{"requests", "pydantic"}})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Reused {
		t.Fatal("a fresh directory cannot be a reuse")
	}
	if _, err := os.Stat(PythonPath(venv)); err != nil {
		t.Fatalf("no interpreter in the venv: %v", err)
	}

	// The digest covers contract fields only, so an unchanged runtime reuses.
	again, err := Sync(ctx, tc, SyncRequest{VenvDir: venv, Health: h, Imports: []string{"requests", "pydantic"}})
	if err != nil {
		t.Fatal(err)
	}
	if !again.Reused {
		t.Fatal("an unchanged runtime digest should reuse the environment")
	}

	// A moved runtime must not reuse.
	moved := *h
	moved.RuntimeDigest = "sha256:different"
	third, err := Sync(ctx, tc, SyncRequest{VenvDir: venv, Health: &moved, Imports: []string{"requests", "pydantic"}})
	if err != nil {
		t.Fatal(err)
	}
	if third.Reused {
		t.Fatal("a changed runtime digest must rebuild")
	}
}

// A matching stamp says the environment was built from the same inputs, not
// that it is still intact. Without --force a corrupted venv is unrecoverable
// except by deleting the directory by hand.
func TestSyncForceRebuildsAMatchingEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("network")
	}
	tc := toolchain(t)
	h := loadFixture(t, "health_ok.json")
	venv := filepath.Join(t.TempDir(), "venv")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req := SyncRequest{VenvDir: venv, Health: h, Imports: []string{"requests"}}

	if _, err := Sync(ctx, tc, req); err != nil {
		t.Fatal(err)
	}
	reused, err := Sync(ctx, tc, req)
	if err != nil {
		t.Fatal(err)
	}
	if !reused.Reused {
		t.Fatal("an unchanged runtime should reuse")
	}

	req.Force = true
	forced, err := Sync(ctx, tc, req)
	if err != nil {
		t.Fatal(err)
	}
	if forced.Reused {
		t.Fatal("--force must rebuild even when the stamp matches")
	}
	if _, err := os.Stat(PythonPath(venv)); err != nil {
		t.Fatalf("forced rebuild left no interpreter: %v", err)
	}
}
