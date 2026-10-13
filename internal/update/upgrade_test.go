package update

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectInstallMethod(t *testing.T) {
	// This test verifies the function runs without panic.
	// The result depends on the local environment (whether brew is installed).
	method := DetectInstallMethod()
	if method != UpgradeHomebrew && method != UpgradeScript && method != UpgradeManual {
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
	if UpgradeHomebrew == UpgradeManual || UpgradeScript == UpgradeHomebrew || UpgradeScript == UpgradeManual {
		t.Error("UpgradeHomebrew, UpgradeScript and UpgradeManual should be distinct")
	}
}

func TestMethodForOS(t *testing.T) {
	cases := map[string]UpgradeMethod{
		"darwin":  UpgradeScript,
		"linux":   UpgradeScript,
		"windows": UpgradeManual,
	}
	for goos, want := range cases {
		if got := methodForOS(goos); got != want {
			t.Errorf("methodForOS(%q) = %d, want %d", goos, got, want)
		}
	}
}

func TestDirOverride(t *testing.T) {
	// The script's default directory is left to the script, so it keeps its
	// sudo fallback; anywhere else is pinned so the running binary is replaced.
	if got := dirOverride("/usr/local/bin"); got != "" {
		t.Errorf("dirOverride(/usr/local/bin) = %q, want empty", got)
	}
	if got := dirOverride("/home/me/.local/bin"); got != "/home/me/.local/bin" {
		t.Errorf("dirOverride(~/.local/bin) = %q, want the directory", got)
	}
}

func setInstallScriptURL(t *testing.T, url string) {
	t.Helper()
	original := installScriptURL
	installScriptURL = url
	t.Cleanup(func() { installScriptURL = original })
}

func TestFetchInstallScript(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("#!/bin/sh\necho ok\n"))
	}))
	defer server.Close()
	setInstallScriptURL(t, server.URL)

	script, err := fetchInstallScript(context.Background(), server.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if script != "#!/bin/sh\necho ok\n" {
		t.Errorf("script = %q", script)
	}
}

func TestFetchInstallScript_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	setInstallScriptURL(t, server.URL)

	if _, err := fetchInstallScript(context.Background(), server.Client()); err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

func serveScript(t *testing.T, script string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(script))
	}))
	t.Cleanup(server.Close)
	setInstallScriptURL(t, server.URL)
	return server
}

func TestInstallLatest_RunsTheScriptWithTheChosenDirectory(t *testing.T) {
	// Settings left over from an earlier install must not pin the version or
	// redirect the upgrade.
	t.Setenv("NOTTE_VERSION", "0.0.1")
	t.Setenv("NOTTE_INSTALL_DIR", "/elsewhere")
	server := serveScript(t, `echo "dir=${NOTTE_INSTALL_DIR:-unset} version=${NOTTE_VERSION:-unset}"`)

	var buf bytes.Buffer
	if err := installLatest(server.Client(), &buf, "/target"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "dir=/target version=unset" {
		t.Errorf("installer saw %q", got)
	}
}

func TestInstallLatest_DefaultDirectory(t *testing.T) {
	t.Setenv("NOTTE_INSTALL_DIR", "/elsewhere")
	server := serveScript(t, `echo "dir=${NOTTE_INSTALL_DIR:-unset}"`)

	var buf bytes.Buffer
	if err := installLatest(server.Client(), &buf, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "dir=unset" {
		t.Errorf("installer saw %q", got)
	}
}

func TestInstallLatest_ScriptFailure(t *testing.T) {
	server := serveScript(t, `echo "notte install: download failed" >&2; exit 3`)

	var buf bytes.Buffer
	if err := installLatest(server.Client(), &buf, "/target"); err == nil {
		t.Fatal("expected an error when the install script fails")
	}
	if !strings.Contains(buf.String(), "download failed") {
		t.Errorf("expected the script's stderr to be forwarded, got %q", buf.String())
	}
}

func writeFakeNotte(t *testing.T, printed string) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "notte")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\necho '"+printed+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func TestVerifyVersion(t *testing.T) {
	exe := writeFakeNotte(t, "notte version 0.0.55")
	if err := verifyVersion(exe, "v0.0.55"); err != nil {
		t.Errorf("expected 0.0.55 to verify, got %v", err)
	}
	if err := verifyVersion(exe, "0.0.56"); err == nil {
		t.Error("expected an error when the binary still reports an older version")
	}
}
