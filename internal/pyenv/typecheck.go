package pyenv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// TyVersion pins the type checker.
//
// ty is on 0.0.x with no stable API and breaking changes between releases, so
// a floating version would let an upstream change turn a passing stack red
// without anything local moving. Pinned rather than avoided: it is what
// anything-api already gates its build agent on, and its 10-100x advantage on
// cold checks is exactly the case a CLI hits on every invocation.
const TyVersion = "0.0.75"

// UnresolvedImport is the rule that must never be treated as noise.
//
// anything-api's ty-config.ts records why: with no interpreter configured, ty
// resolved against the first python on PATH, every import came back
// unresolved, and the build agent deployed straight through a mandatory type
// check. A checker that cannot resolve imports does not fail — it goes green.
const UnresolvedImport = "unresolved-import"

// EnvironmentError is a failure of the environment rather than of the code
// being checked.
//
// It carries the rebuild command because the obvious one does not work: a
// plain sync reuses the environment, since the stamp records what it was built
// from and that still matches. Only --force gets past the reuse check.
type EnvironmentError struct {
	VenvDir string
	Detail  string
}

func (e *EnvironmentError) Error() string {
	return fmt.Sprintf("the environment in %s is unusable, so nothing could be checked:\n  %s\n"+
		"  rebuild it with:\n      notte stack sync --force", e.VenvDir, e.Detail)
}

// Diagnostic is one finding.
type Diagnostic struct {
	Rule    string
	Message string
	Path    string
	Line    int
	Column  int
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("%s:%d:%d: %s [%s]", d.Path, d.Line, d.Column, d.Message, d.Rule)
}

// TypeCheckResult is the outcome of a ty run.
type TypeCheckResult struct {
	Diagnostics []Diagnostic
	// Unresolved are the module names ty could not resolve. Split out because
	// their meaning depends on whether the runtime claims to ship them.
	Unresolved []string
}

// OK reports whether the artifact is clean.
func (r *TypeCheckResult) OK() bool { return len(r.Diagnostics) == 0 }

// Misconfigured reports whether ty failed to resolve something the runtime
// says it ships, which means the environment wiring is broken rather than the
// user's code. Reporting that as a code error would send someone to fix a file
// that is fine.
func (r *TypeCheckResult) Misconfigured(h *Health) []string {
	var broken []string
	for _, module := range r.Unresolved {
		if p, ok := h.Package(module); ok && p.Installed {
			broken = append(broken, module)
		}
	}
	sort.Strings(broken)
	return broken
}

// gitlabDiagnostic is ty's GitLab Code Quality output. ty has no plain JSON
// format; this is the structured one, and parsing it beats scraping the
// human-readable lines.
type gitlabDiagnostic struct {
	CheckName   string `json:"check_name"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Location    struct {
		Path      string `json:"path"`
		Positions struct {
			Begin struct {
				Line   int `json:"line"`
				Column int `json:"column"`
			} `json:"begin"`
		} `json:"positions"`
	} `json:"location"`
}

// TypeCheck runs ty over targets, relative to dir, resolving imports against
// the environment in venvDir.
//
// ty is run through uvx rather than installed into the venv on purpose: the
// venv mirrors the runtime image, and putting a package in it that the runtime
// does not have weakens the property that makes the venv the enforcement.
//
// The interpreter is passed explicitly on every invocation. Left to itself ty
// resolves against the first python on PATH, and anything-api records what that
// costs: every import came back unresolved and a mandatory type check went
// green while checking nothing. Passing --python rather than writing a ty.toml
// keeps it at the call site, where it cannot be omitted by a caller that
// forgot to generate the file — and leaves no machine-specific absolute path
// in the user's repository.
func TypeCheck(ctx context.Context, tc *Toolchain, dir, venvDir string, targets []string) (*TypeCheckResult, error) {
	if len(targets) == 0 {
		return &TypeCheckResult{}, nil
	}
	// ty treats an unusable --python as fatal for the whole run, which is worse
	// than the misconfiguration it prevents, so it is checked first.
	if _, err := os.Stat(PythonPath(venvDir)); err != nil {
		return nil, fmt.Errorf("no interpreter at %s: %w", PythonPath(venvDir), err)
	}

	args := append([]string{
		"ty@" + TyVersion, "check",
		"--python", venvDir,
		"--output-format", "gitlab",
		// Diagnostics are read from stdout, never from the exit code — the
		// same reason the health endpoint always answers 200.
		"--exit-zero",
	}, targets...)

	cmd := exec.CommandContext(ctx, tc.UV+"x", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		// --exit-zero means diagnostics never fail the process, so a non-zero
		// exit is ty itself refusing to run — overwhelmingly a broken
		// environment rather than anything about the code. Returning a bare
		// "exit status 2" sends someone hunting through their functions for a
		// problem that is not there.
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, &EnvironmentError{VenvDir: venvDir, Detail: detail}
	}

	var raw []gitlabDiagnostic
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("decode ty output: %w (got %q)", err, truncate(string(out), 200))
	}

	res := &TypeCheckResult{}
	seenUnresolved := map[string]bool{}
	for _, d := range raw {
		message := strings.TrimPrefix(d.Description, d.CheckName+": ")
		res.Diagnostics = append(res.Diagnostics, Diagnostic{
			Rule:    d.CheckName,
			Message: message,
			Path:    d.Location.Path,
			Line:    d.Location.Positions.Begin.Line,
			Column:  d.Location.Positions.Begin.Column,
		})
		if d.CheckName == UnresolvedImport {
			if module := moduleFromUnresolved(message); module != "" && !seenUnresolved[module] {
				seenUnresolved[module] = true
				res.Unresolved = append(res.Unresolved, module)
			}
		}
	}
	sort.Strings(res.Unresolved)
	return res, nil
}

// moduleFromUnresolved pulls the module name out of ty's message, which reads
// "Cannot resolve imported module `foo`".
func moduleFromUnresolved(message string) string {
	start := strings.IndexByte(message, '`')
	if start < 0 {
		return ""
	}
	rest := message[start+1:]
	end := strings.IndexByte(rest, '`')
	if end < 0 {
		return ""
	}
	root := rest[:end]
	if i := strings.IndexByte(root, '.'); i >= 0 {
		root = root[:i]
	}
	return root
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// UvxPath is uv's tool runner, which sits beside uv.
func (t *Toolchain) UvxPath() string { return t.UV + "x" }
