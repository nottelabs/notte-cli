package pyenv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestModuleFromUnresolvedMessage(t *testing.T) {
	cases := map[string]string{
		"Cannot resolve imported module `httpcloak`":       "httpcloak",
		"Cannot resolve imported module `notte_sdk.types`": "notte_sdk",
		"something else entirely":                          "",
	}
	for message, want := range cases {
		if got := moduleFromUnresolved(message); got != want {
			t.Errorf("%q -> %q, want %q", message, got, want)
		}
	}
}

// An unresolved import of something the runtime says it ships means the venv
// or ty.toml wiring is broken, not the user's code. Reporting it as a code
// error sends someone to fix a file that is fine.
func TestMisconfiguredDistinguishesWiringFromUserError(t *testing.T) {
	h := loadFixture(t, "health_ok.json")
	res := &TypeCheckResult{Unresolved: []string{"requests", "pandas", "notte"}}

	broken := res.Misconfigured(h)
	if len(broken) != 1 || broken[0] != "requests" {
		t.Fatalf("misconfigured = %v, want [requests]", broken)
	}
	// pandas is not allowed at all — a genuine user error.
	// notte is allowed but installed:false, so it is expected to be missing
	// and must not be blamed on the environment.
}

func TestTypeCheckWithNoTargetsIsClean(t *testing.T) {
	res, err := TypeCheck(context.Background(), &Toolchain{UV: "uv"}, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() {
		t.Fatal("no targets should be clean")
	}
}

// The real thing: ty against a venv built from the real staging report.
func TestTypeCheckAgainstARealEnvironment(t *testing.T) {
	venv, h := realVenv(t)
	tc := toolchain(t)

	dir := t.TempDir()
	if err := WriteTyConfig(dir, venv); err != nil {
		t.Fatalf("ty config: %v", err)
	}

	// requests and pydantic are in the venv; nonexistent_pkg is not.
	src := `import requests
from pydantic import BaseModel
import nonexistent_pkg


class Response(BaseModel):
    ok: bool


def run() -> Response:
    return Response(ok=bool(requests) and bool(nonexistent_pkg))
`
	if err := os.WriteFile(filepath.Join(dir, "artifact.py"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	res, err := TypeCheck(ctx, tc, dir, []string{"artifact.py"})
	if err != nil {
		t.Fatalf("typecheck: %v", err)
	}

	// The point of WriteTyConfig: packages that ARE installed must resolve.
	// If these come back unresolved, ty is pointed at the wrong interpreter
	// and every check it performs is meaningless.
	for _, module := range res.Unresolved {
		if module == "requests" || module == "pydantic" {
			t.Fatalf("ty could not resolve %q from the venv — the interpreter is not configured, "+
				"which is the failure that lets a mandatory type check pass while checking nothing", module)
		}
	}
	if len(res.Misconfigured(h)) != 0 {
		t.Fatalf("environment reported broken: %v", res.Misconfigured(h))
	}

	// And a genuinely missing package must be reported.
	found := false
	for _, module := range res.Unresolved {
		if module == "nonexistent_pkg" {
			found = true
		}
	}
	if !found {
		t.Fatalf("an import of a package not in the venv should be unresolved; got %v", res.Unresolved)
	}
	if res.OK() {
		t.Fatal("a file with an unresolvable import is not clean")
	}
}

// A type error unrelated to imports must be reported, which is what makes ty
// worth running at all: it catches the redefinitions a flattener could
// silently introduce.
func TestTypeCheckReportsRealTypeErrors(t *testing.T) {
	venv, _ := realVenv(t)
	tc := toolchain(t)

	dir := t.TempDir()
	if err := WriteTyConfig(dir, venv); err != nil {
		t.Fatal(err)
	}
	src := "def run() -> int:\n    return \"not an int\"\n"
	if err := os.WriteFile(filepath.Join(dir, "artifact.py"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	res, err := TypeCheck(ctx, tc, dir, []string{"artifact.py"})
	if err != nil {
		t.Fatal(err)
	}
	if res.OK() {
		t.Fatal("a wrong return type should be reported")
	}
	joined := strings.Join(diagnosticStrings(res), " ")
	if !strings.Contains(joined, "invalid-return-type") {
		t.Fatalf("expected invalid-return-type, got %v", diagnosticStrings(res))
	}
	if res.Diagnostics[0].Line == 0 {
		t.Fatal("diagnostics must carry a line number")
	}
}

func diagnosticStrings(res *TypeCheckResult) []string {
	out := make([]string, len(res.Diagnostics))
	for i, d := range res.Diagnostics {
		out[i] = d.String()
	}
	return out
}
