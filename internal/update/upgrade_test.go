package update

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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
