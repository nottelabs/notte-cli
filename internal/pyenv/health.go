// Package pyenv turns the runtime's self-description into a local Python
// environment that matches it, and runs the checks that need an interpreter.
//
// Everything here follows from one fact: the CLI must not carry its own copy of
// the runtime's rules. Earlier drafts vendored the import allowlist, a denylist
// and a stdlib set generated from a pinned CPython, and all three drifted —
// one of them shipping a list from CPython 3.14 that would have rejected
// modules the 3.12 runner actually has. GET /functions/health makes the runner
// authoritative instead.
package pyenv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// Status values reported by GET /functions/health.
const (
	// StatusOK means the runner answered and the report is complete.
	StatusOK = "ok"
	// StatusDegraded means the API is healthy but the runner did not answer
	// the contract question — normally because an API deploy has landed and
	// the runner image has not been rebuilt yet. It is a routine state, not
	// an outage, and must never block a deploy.
	StatusDegraded = "degraded"
	// StatusUnreachable means the runner could not be reached at all.
	StatusUnreachable = "unreachable"
)

// Health is the runtime's description of itself.
type Health struct {
	Status    string  `json:"status"`
	Reachable bool    `json:"reachable"`
	LatencyMS float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`

	// PythonVersion is the interpreter the runner executes, e.g. "3.12.0".
	// Empty when degraded.
	PythonVersion string `json:"python_version"`
	// Packages are the non-stdlib imports the runner allows. Empty when
	// degraded.
	Packages []Package `json:"packages"`
	// StdlibModules are the standard-library imports the runner allows,
	// post-patch — so tempfile, which upload accepts and the runner discards,
	// is absent. Empty when degraded.
	StdlibModules []string `json:"stdlib_modules"`
	// ReservedEnvNames are function_env names the API refuses to store. This
	// is the API's own rule rather than the runner's, so it is populated even
	// when degraded, and `secrets` validation keeps working through the window.
	ReservedEnvNames []string `json:"reserved_env_names"`
	// RuntimeDigest covers the contract fields only, so it moves if and only
	// if an environment built against the previous answer would now be wrong.
	// Null when the report is partial: a validator for "no answer" is worse
	// than none.
	RuntimeDigest string `json:"runtime_digest"`
}

// Package is one importable non-stdlib name.
type Package struct {
	// ImportName is what appears in an import statement, e.g. "bs4".
	ImportName string `json:"import_name"`
	// Package is the distribution name, e.g. "beautifulsoup4".
	Package string `json:"package"`
	Version string `json:"version"`
	// Installed reports whether the image actually ships it. A name can be
	// allowed and absent, which passes upload validation and then dies on
	// ModuleNotFoundError mid-run.
	Installed bool `json:"installed"`
	// Source is the PEP 610 direct_url of the install when it did not come
	// from an index — the runner installs notte-sdk and notte-core from git
	// SHAs, so for those a version is not an identity. Empty for index
	// installs, where the version is the whole identity.
	Source string `json:"source"`
}

// Complete reports whether the report carries the contract fields. A degraded
// report has reserved names and nothing else usable.
func (h *Health) Complete() bool {
	return h.Status == StatusOK && h.PythonVersion != "" && h.RuntimeDigest != ""
}

// Allows reports whether an import is permitted by the runtime.
//
// Submodules follow their root, which is how safe_import resolves them: a
// denied root denies its children, an allowed root allows them.
func (h *Health) Allows(module string) bool {
	root := module
	if i := strings.IndexByte(module, '.'); i >= 0 {
		root = module[:i]
	}
	for _, m := range h.StdlibModules {
		if m == root || m == module {
			return true
		}
	}
	for _, p := range h.Packages {
		if p.ImportName == root {
			return true
		}
	}
	return false
}

// Package returns the entry for an import name.
func (h *Health) Package(importName string) (Package, bool) {
	for _, p := range h.Packages {
		if p.ImportName == importName {
			return p, true
		}
	}
	return Package{}, false
}

// Installable is the subset of wanted imports that the runner both allows and
// actually ships, plus the names it allows but does not ship.
//
// The second return is the case worth surfacing: allowed-but-absent passes
// upload validation and fails at run time, and nothing else makes it visible.
func (h *Health) Installable(wanted []string) (install []Package, allowedButMissing []string) {
	seen := map[string]bool{}
	for _, name := range wanted {
		root := name
		if i := strings.IndexByte(name, '.'); i >= 0 {
			root = name[:i]
		}
		if seen[root] {
			continue
		}
		seen[root] = true

		p, ok := h.Package(root)
		if !ok {
			continue // stdlib, or not allowed at all — the caller reports that
		}
		if p.Installed {
			install = append(install, p)
		} else {
			allowedButMissing = append(allowedButMissing, root)
		}
	}
	sort.Slice(install, func(i, j int) bool { return install[i].ImportName < install[j].ImportName })
	sort.Strings(allowedButMissing)
	return install, allowedButMissing
}

// Requirement renders a package as a uv/pip install argument.
//
// Source wins over version when present. The runner installs notte-sdk and
// notte-core from git, and the published package under the same version number
// is different code — a version-only install produces a near-miss environment
// that reports confident, wrong answers.
func (p Package) Requirement() string {
	name := p.Package
	if name == "" {
		name = p.ImportName
	}
	if p.Source != "" {
		// PEP 508 direct reference. The git+ prefix is required — uv rejects a
		// bare https URL outright — and naming the distribution keeps the
		// resolver's messages legible.
		return name + " @ " + p.Source
	}
	if p.Version == "" {
		return name
	}
	return name + "==" + p.Version
}

// FetchHealth reads GET /functions/health.
//
// The endpoint always answers 200 and carries the answer in `status`, so a
// caller that branches on the HTTP code learns nothing. Only transport and
// decoding failures surface as errors here.
func FetchHealth(ctx context.Context, client *http.Client, baseURL, apiKey string) (*Health, error) {
	url := strings.TrimSuffix(baseURL, "/") + "/functions/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET /functions/health: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("GET /functions/health: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		// The route is matched by GET /functions/{function_id} on an API old
		// enough to lack it, so the error talks about a function called
		// "health". Say what it means instead.
		return nil, fmt.Errorf("this Notte API does not have GET /functions/health yet; upgrade it or use an environment that does")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /functions/health: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var h Health
	if err := json.Unmarshal(body, &h); err != nil {
		return nil, fmt.Errorf("GET /functions/health: %w", err)
	}
	return &h, nil
}
