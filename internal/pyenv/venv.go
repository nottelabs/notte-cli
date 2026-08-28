package pyenv

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// StampName records which runtime a venv was built for, so a rebuild happens
// when the runtime moves and not merely when time passes.
const StampName = ".notte-runtime.json"

// Toolchain is the external tooling the stack commands need.
type Toolchain struct {
	// UV is the path to uv, which supplies the interpreter as well as the
	// packages — so the requirement is "have uv", not "have Python 3.12".
	UV string
}

// ErrNoToolchain is returned when uv is absent.
type ErrNoToolchain struct{}

func (ErrNoToolchain) Error() string {
	return "uv is required by `notte stack` and was not found on PATH\n" +
		"  install it with:  curl -LsSf https://astral.sh/uv/install.sh | sh\n" +
		"  (uv downloads the Python the runtime uses, so nothing else is needed)"
}

// FindToolchain locates uv.
func FindToolchain() (*Toolchain, error) {
	path, err := exec.LookPath("uv")
	if err != nil {
		return nil, ErrNoToolchain{}
	}
	return &Toolchain{UV: path}, nil
}

// Stamp is written into the venv to record what it was built from.
type Stamp struct {
	RuntimeDigest string   `json:"runtime_digest"`
	PythonVersion string   `json:"python_version"`
	Requirements  []string `json:"requirements"`
}

// AlwaysInstall are put into every environment regardless of what the
// functions currently import.
//
// notte_core carries ScriptValidator, so an environment without it can build a
// perfectly good artifact and then fail to check it.
//
// notte_sdk is what people actually write against — it is the single most
// common import in real stacks, and someone typing `from notte_sdk import
// NotteClient` into a new function wants completion before they have saved and
// re-synced. Measured at 1 MB on top of notte_core's 46 MB, in a third of a
// second, with no browser dependencies, so the mirroring argument wins easily.
//
// Neither loosens the mirror: both are in the runtime image regardless, so
// installing them unconditionally makes the venv a closer match, not a looser
// one.
var AlwaysInstall = []string{"notte_core", "notte_sdk"}

// SyncRequest describes the environment to build.
type SyncRequest struct {
	// VenvDir is where the environment lives, normally .notte/venv.
	VenvDir string
	// Health is the runtime's report. Must be Complete().
	Health *Health
	// Imports are the non-relative module names the functions use. Only the
	// intersection with what the runtime ships is installed: the allowlist is
	// closed, so this is a set intersection rather than dependency resolution.
	Imports []string
	// Force rebuilds even when the stamp matches.
	//
	// The stamp records what an environment was built *from*, which is not the
	// same as it still being intact — a half-finished install or a deleted
	// site-packages leaves a venv that reuse happily accepts. Without this
	// there is no way to recover except deleting the directory by hand.
	Force bool
}

// SyncResult describes what was built.
type SyncResult struct {
	VenvDir string
	Python  string
	// Installed are the packages put into the environment.
	Installed []Package
	// AllowedButMissing are imports the runtime permits but does not ship.
	// They pass upload validation and fail at run time, so they are surfaced
	// rather than silently skipped.
	AllowedButMissing []string
	// NotAllowed are imports the runtime rejects outright.
	NotAllowed []string
	// Reused reports that an existing venv already matched the runtime digest.
	Reused bool
}

// PythonPath is the interpreter inside a venv.
func PythonPath(venvDir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvDir, "Scripts", "python.exe")
	}
	return filepath.Join(venvDir, "bin", "python")
}

// Sync builds an environment matching the runtime.
//
// It refuses to work from a partial report. A degraded response carries no
// packages and no stdlib list, so building from it would produce an empty
// environment in which every import fails — a confident, wrong answer.
func Sync(ctx context.Context, tc *Toolchain, req SyncRequest) (*SyncResult, error) {
	if !req.Health.Complete() {
		return nil, fmt.Errorf("cannot build an environment from a %s runtime report: it carries no package list", req.Health.Status)
	}

	install, missing := req.Health.Installable(append(append([]string{}, AlwaysInstall...), req.Imports...))
	res := &SyncResult{
		VenvDir:           req.VenvDir,
		Python:            req.Health.PythonVersion,
		Installed:         install,
		AllowedButMissing: missing,
		NotAllowed:        notAllowed(req.Health, req.Imports),
	}

	requirements := make([]string, 0, len(install))
	for _, p := range install {
		requirements = append(requirements, p.Requirement())
	}
	want := Stamp{
		RuntimeDigest: req.Health.RuntimeDigest,
		PythonVersion: req.Health.PythonVersion,
		Requirements:  requirements,
	}

	// The digest covers the contract fields only, so it moves if and only if an
	// environment built against the previous answer would now be wrong. A
	// rebuild that changes nothing observable does not invalidate this venv.
	if have, err := readStamp(req.VenvDir); !req.Force && err == nil && have.matches(want) {
		res.Reused = true
		return res, nil
	}

	if err := os.MkdirAll(filepath.Dir(req.VenvDir), 0o755); err != nil {
		return nil, err
	}
	// Reuse was already ruled out, so anything here is stale or a half-built
	// environment from an interrupted run. uv will not reliably recreate over
	// one, and a partial venv that looks present is worse than none.
	if err := os.RemoveAll(req.VenvDir); err != nil {
		return nil, err
	}
	if err := run(ctx, tc.UV, "venv", "--quiet", "--python", req.Health.PythonVersion, req.VenvDir); err != nil {
		return nil, fmt.Errorf("create venv: %w", err)
	}
	if len(requirements) > 0 {
		args := append([]string{"pip", "install", "--quiet", "--python", req.VenvDir}, requirements...)
		if err := run(ctx, tc.UV, args...); err != nil {
			return nil, fmt.Errorf("install runtime packages: %w", err)
		}
	}
	if err := writeStamp(req.VenvDir, want); err != nil {
		return nil, err
	}
	return res, nil
}

// notAllowed reports imports the runtime rejects outright, so the caller can
// fail with the file and line rather than let ty report a bare
// unresolved-import.
func notAllowed(h *Health, imports []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, name := range imports {
		if h.Allows(name) || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func (s Stamp) matches(other Stamp) bool {
	if s.RuntimeDigest == "" || s.RuntimeDigest != other.RuntimeDigest {
		return false
	}
	if s.PythonVersion != other.PythonVersion || len(s.Requirements) != len(other.Requirements) {
		return false
	}
	for i := range s.Requirements {
		if s.Requirements[i] != other.Requirements[i] {
			return false
		}
	}
	return true
}

func readStamp(venvDir string) (Stamp, error) {
	raw, err := os.ReadFile(filepath.Join(venvDir, StampName))
	if err != nil {
		return Stamp{}, err
	}
	var s Stamp
	if err := json.Unmarshal(raw, &s); err != nil {
		return Stamp{}, err
	}
	// A venv whose interpreter has been removed is not reusable regardless of
	// what the stamp claims.
	if _, err := os.Stat(PythonPath(venvDir)); err != nil {
		return Stamp{}, err
	}
	return s, nil
}

func writeStamp(venvDir string, s Stamp) error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(venvDir, StampName), append(raw, '\n'), 0o644)
}

func run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return nil
}
