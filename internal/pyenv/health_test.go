package pyenv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadFixture reads a payload captured from a live environment. These are real
// responses, not hand-written approximations — the point is to be tested
// against what the API actually sends.
func loadFixture(t *testing.T, name string) *Health {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var h Health
	if err := json.Unmarshal(raw, &h); err != nil {
		t.Fatal(err)
	}
	return &h
}

func TestDecodeRealOKResponse(t *testing.T) {
	h := loadFixture(t, "health_ok.json")
	if h.Status != StatusOK || !h.Reachable {
		t.Fatalf("status=%q reachable=%v", h.Status, h.Reachable)
	}
	if h.PythonVersion != "3.12.0" {
		t.Fatalf("python_version = %q", h.PythonVersion)
	}
	if !strings.HasPrefix(h.RuntimeDigest, "sha256:") {
		t.Fatalf("runtime_digest = %q", h.RuntimeDigest)
	}
	if !h.Complete() {
		t.Fatal("an ok report with a version and a digest should be complete")
	}
	if len(h.Packages) == 0 || len(h.StdlibModules) == 0 {
		t.Fatalf("packages=%d stdlib=%d", len(h.Packages), len(h.StdlibModules))
	}
}

// A degraded report carries reserved names and nothing else usable. Treating it
// as authoritative would build an empty venv and reject every import.
func TestDecodeRealDegradedResponse(t *testing.T) {
	h := loadFixture(t, "health_degraded.json")
	if h.Status != StatusDegraded {
		t.Fatalf("status = %q", h.Status)
	}
	if !h.Reachable {
		t.Fatal("degraded still means the API answered")
	}
	if h.Complete() {
		t.Fatal("a degraded report must not be treated as complete")
	}
	if h.RuntimeDigest != "" {
		t.Fatalf("digest must be null when partial, got %q", h.RuntimeDigest)
	}
	if len(h.ReservedEnvNames) == 0 {
		t.Fatal("reserved_env_names must survive degradation — it is the API's rule, not the runner's")
	}
}

// tempfile is allowed at upload and discarded by the runner. If it leaks back
// into stdlib_modules, the CLI accepts something that dies at run time.
func TestRuntimeStdlibExcludesTempfileAndProcessControl(t *testing.T) {
	h := loadFixture(t, "health_ok.json")
	for _, denied := range []string{"tempfile", "os", "sys", "subprocess", "pathlib", "socket"} {
		if h.Allows(denied) {
			t.Errorf("%q must not be allowed by the runtime", denied)
		}
	}
	for _, allowed := range []string{"json", "re", "datetime", "asyncio"} {
		if !h.Allows(allowed) {
			t.Errorf("%q should be allowed", allowed)
		}
	}
}

func TestAllowsFollowsTheRootOfADottedImport(t *testing.T) {
	h := loadFixture(t, "health_ok.json")
	if !h.Allows("notte_sdk.types") {
		t.Error("a submodule of an allowed package should be allowed")
	}
	if h.Allows("os.path") {
		t.Error("a submodule of a denied root must stay denied")
	}
}

// The runner installs notte-sdk from a git SHA, and the published package under
// the same version number is different code. A version-only install builds a
// near-miss environment that reports confident, wrong answers.
func TestRequirementPrefersSourceOverVersion(t *testing.T) {
	h := loadFixture(t, "health_ok.json")
	sdk, ok := h.Package("notte_sdk")
	if !ok {
		t.Fatal("notte_sdk missing from the report")
	}
	if sdk.Source == "" {
		t.Skip("this capture has no git source for notte_sdk")
	}
	req := sdk.Requirement()
	if !strings.Contains(req, "github.com/nottelabs/notte") {
		t.Fatalf("requirement should install from the git source, got %q", req)
	}
	// uv rejects a bare https URL: the git+ prefix is what marks it a VCS
	// reference. Verified against uv directly.
	if !strings.Contains(req, "git+https://") {
		t.Fatalf("the git+ prefix must survive, got %q", req)
	}
	if !strings.HasPrefix(req, "notte-sdk @ ") {
		t.Fatalf("PEP 508 direct reference should name the distribution, got %q", req)
	}
}

func TestRequirementUsesPinnedVersionForIndexInstalls(t *testing.T) {
	p := Package{ImportName: "bs4", Package: "beautifulsoup4", Version: "4.12.3", Installed: true}
	if got := p.Requirement(); got != "beautifulsoup4==4.12.3" {
		t.Fatalf("got %q", got)
	}
	// The import name is not always the distribution name.
	if p.ImportName == p.Package {
		t.Fatal("fixture should exercise the import-name/package-name split")
	}
}

// Allowed-but-absent is the case that passes upload validation and dies at run
// time. The real staging report has three of them.
func TestInstallableSeparatesAllowedFromShipped(t *testing.T) {
	h := loadFixture(t, "health_ok.json")
	install, missing := h.Installable([]string{"requests", "notte", "notte_sdk", "json", "pandas"})

	var installed []string
	for _, p := range install {
		installed = append(installed, p.ImportName)
	}
	if len(installed) == 0 {
		t.Fatal("expected requests and notte_sdk to be installable")
	}
	for _, want := range []string{"notte_sdk", "requests"} {
		found := false
		for _, got := range installed {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%q should be installable, got %v", want, installed)
		}
	}
	// notte is allowed by the runtime but not shipped in the image.
	found := false
	for _, m := range missing {
		if m == "notte" {
			found = true
		}
	}
	if !found {
		t.Errorf("notte is allowed-but-absent in the real report; got missing=%v", missing)
	}
	// stdlib and unknown names are neither installable nor "missing".
	for _, m := range missing {
		if m == "json" || m == "pandas" {
			t.Errorf("%q should not be reported as allowed-but-missing", m)
		}
	}
}

func TestFetchHealthReadsStatusNotHTTPCode(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "health_degraded.json"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer k" {
			t.Errorf("Authorization = %q", got)
		}
		if r.URL.Path != "/functions/health" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK) // always 200, even when degraded
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	h, err := FetchHealth(context.Background(), srv.Client(), srv.URL, "k")
	if err != nil {
		t.Fatalf("degraded must not surface as a transport error: %v", err)
	}
	if h.Status != StatusDegraded {
		t.Fatalf("status = %q", h.Status)
	}
}

// An API without the route matches it against GET /functions/{function_id},
// producing an error about a function called "health".
func TestFetchHealthExplainsAnOldAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"The function 'health' does not exist or the user does not have access to it"}`))
	}))
	defer srv.Close()

	_, err := FetchHealth(context.Background(), srv.Client(), srv.URL, "k")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "does not have GET /functions/health yet") {
		t.Fatalf("error should explain the real cause, got %v", err)
	}
}

func TestFetchHealthTrimsTrailingSlash(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()
	if _, err := FetchHealth(context.Background(), srv.Client(), srv.URL+"/", "k"); err != nil {
		t.Fatal(err)
	}
	if path != "/functions/health" {
		t.Fatalf("path = %q", path)
	}
}
