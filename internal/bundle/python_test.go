package bundle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// python locates an interpreter, or skips. These tests assert that the artifact
// is real Python rather than merely plausible-looking text, which no amount of
// string matching in Go can establish.
func python(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}
	return p
}

// writeTemp puts code in a file named so tracebacks are legible.
func writeTemp(t *testing.T, code string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "artifact.py")
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Every golden artifact must compile. A bundler that emits a syntax error is
// worse than one that refuses, because the failure surfaces after upload.
func TestGoldenArtifactsCompile(t *testing.T) {
	py := python(t)
	for _, name := range goldenCases {
		t.Run(name, func(t *testing.T) {
			code, err := os.ReadFile(filepath.Join("testdata", name, "want.py"))
			if err != nil {
				t.Skipf("no golden file yet: %v", err)
			}
			path := writeTemp(t, string(code))
			out, err := exec.Command(py, "-m", "py_compile", path).CombinedOutput()
			if err != nil {
				t.Fatalf("artifact does not compile: %v\n%s\n--- code ---\n%s", err, out, code)
			}
		})
	}
}

// runArtifact executes the artifact and evaluates expr against its namespace.
func runArtifact(t *testing.T, code, expr string) string {
	t.Helper()
	py := python(t)
	path := writeTemp(t, code)
	script := "import runpy; ns = runpy.run_path(" + strconv.Quote(path) + "); print(" + expr + ")"
	out, err := exec.Command(py, "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("executing artifact failed: %v\n%s\n--- code ---\n%s", err, out, code)
	}
	return strings.TrimSpace(string(out))
}

// The alias rule, proven rather than asserted: deleting the import without
// recreating the binding raises NameError here, which is exactly the failure
// that would otherwise reach production.
func TestAliasedImportArtifactActuallyRuns(t *testing.T) {
	res := bundleCase(t, "alias-preserved")
	if got := runArtifact(t, res.Code, `ns["run"]("  hi  ")`); got != "['hi']" {
		t.Fatalf("run() returned %q, want %q", got, "['hi']")
	}
}

func TestFlattenedDependencyChainRuns(t *testing.T) {
	res := bundleCase(t, "topo-order")
	if got := runArtifact(t, res.Code, `ns["run"]()`); got != "2" {
		t.Fatalf("run() returned %q, want 2", got)
	}
}

func TestDiamondArtifactRuns(t *testing.T) {
	res := bundleCase(t, "diamond")
	if got := runArtifact(t, res.Code, `ns["run"]()`); got != "1" {
		t.Fatalf("run() returned %q, want 1", got)
	}
}

// __future__ must be the first statement or Python refuses the file outright,
// so this compiles only if the emitter got the ordering right.
func TestFutureAnnotationsArtifactCompiles(t *testing.T) {
	res := bundleCase(t, "future-annotations")
	path := writeTemp(t, res.Code)
	out, err := exec.Command(python(t), "-m", "py_compile", path).CombinedOutput()
	if err != nil {
		t.Fatalf("misplaced __future__ import: %v\n%s\n%s", err, out, res.Code)
	}
}

// Property test: every module in the package must survive into the artifact
// with its top-level definitions intact.
func TestAllGoldenArtifactsDefineRun(t *testing.T) {
	py := python(t)
	for _, name := range goldenCases {
		t.Run(name, func(t *testing.T) {
			res := bundleCase(t, name)
			if strings.Contains(res.Code, "import requests") || strings.Contains(res.Code, "pydantic") {
				t.Skip("artifact needs third-party packages that are not installed for tests")
			}
			path := writeTemp(t, res.Code)
			script := "import runpy; ns = runpy.run_path(" + strconv.Quote(path) + "); assert callable(ns.get('run')), 'run() missing'"
			if out, err := exec.Command(py, "-c", script).CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
		})
	}
}
