package browser

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeChromiumInstall lays out a Local State file and profile directories under
// a temporary home, matching where the browser keeps them on this OS.
func fakeChromiumInstall(t *testing.T, b Browser, localState string, profileDirs ...string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	dir, err := b.userDataDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range profileDirs {
		if err := os.MkdirAll(filepath.Join(dir, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "Local State"), []byte(localState), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestArcHidesSystemProfile(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Arc is supported on macOS only")
	}
	arc, _ := BrowserByID("arc")
	fakeChromiumInstall(t, arc, `{"profile":{
		"info_cache":{
			"Default":{"name":"Personal"},
			"Profile 1":{"name":"__ARC_SYSTEM_PROFILE"}
		},
		"profiles_order":["Default","Profile 1"]
	}}`, "Default", "Profile 1")

	profiles, err := arc.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].Name != "Personal" {
		t.Fatalf("expected only the Personal profile, got %+v", profiles)
	}
}

func TestMacOnlyBrowsersNotInstalledOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("checks the Linux path")
	}
	// Without a Linux directory these must not resolve to ~/.config itself,
	// which exists on nearly every machine and would look "installed".
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".config"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"arc", "helium"} {
		b, _ := BrowserByID(id)
		if b.Installed() {
			t.Errorf("%s reported installed on Linux", id)
		}
		if err := b.CheckPlatform(); err == nil || !strings.Contains(err.Error(), "macOS only") {
			t.Errorf("%s: expected a macOS-only error, got %v", id, err)
		}
	}
}
