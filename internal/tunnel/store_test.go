package tunnel

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

func setupStore(t *testing.T) *testutil.MockKeyring {
	t.Helper()
	config.SetTestConfigDir(t.TempDir())
	t.Cleanup(func() { config.SetTestConfigDir("") })
	ring := testutil.NewMockKeyring()
	auth.SetKeyring(ring)
	t.Cleanup(auth.ResetKeyring)
	return ring
}

func TestStore_RoundTripKeepsSecretOutOfConfigFile(t *testing.T) {
	setupStore(t)
	home := Tunnel{Name: "home-mac", ExitNode: "home-mac.tail1234.ts.net", OAuthClientID: "client-1"}
	if err := Save(home, "s3cret"); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Get("home-mac")
	if err != nil || *got != home {
		t.Fatalf("Get() = %+v, %v; want %+v", got, err, home)
	}
	if secret, err := Secret("home-mac"); err != nil || secret != "s3cret" {
		t.Errorf("Secret() = %q, %v", secret, err)
	}

	dir, _ := config.Dir()
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "s3cret") {
		t.Errorf("secret written to %s: %s", FileName, data)
	}
}

func TestStore_EmptySecretKeepsStoredOne(t *testing.T) {
	setupStore(t)
	if err := Save(Tunnel{Name: "home-mac", ExitNode: "a.ts.net", OAuthClientID: "c"}, "first"); err != nil {
		t.Fatal(err)
	}
	if err := Save(Tunnel{Name: "home-mac", ExitNode: "b.ts.net", OAuthClientID: "c"}, ""); err != nil {
		t.Fatal(err)
	}
	if secret, _ := Secret("home-mac"); secret != "first" {
		t.Errorf("Secret() = %q, want first", secret)
	}
	if got, _ := Get("home-mac"); got.ExitNode != "b.ts.net" {
		t.Errorf("ExitNode = %q, want b.ts.net", got.ExitNode)
	}
}

func TestStore_RemoveDeletesTunnelAndSecret(t *testing.T) {
	ring := setupStore(t)
	if err := Save(Tunnel{Name: "home-mac", ExitNode: "a.ts.net", OAuthClientID: "c"}, "s"); err != nil {
		t.Fatal(err)
	}
	if err := Remove("home-mac"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := Get("home-mac"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() after Remove = %v, want ErrNotFound", err)
	}
	if _, err := ring.Get(secretKey("home-mac")); err == nil {
		t.Error("secret still in keyring after Remove")
	}
	if err := Remove("home-mac"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Remove() = %v, want ErrNotFound", err)
	}
}

func TestStore_ListIsSortedAndEmptyWhenMissing(t *testing.T) {
	setupStore(t)
	if tunnels, err := List(); err != nil || len(tunnels) != 0 {
		t.Fatalf("List() on empty store = %v, %v", tunnels, err)
	}
	for _, n := range []string{"work", "home"} {
		if err := Save(Tunnel{Name: n, ExitNode: n + ".ts.net", OAuthClientID: "c"}, "s"); err != nil {
			t.Fatal(err)
		}
	}
	tunnels, err := List()
	if err != nil || len(tunnels) != 2 || tunnels[0].Name != "home" || tunnels[1].Name != "work" {
		t.Errorf("List() = %+v, %v", tunnels, err)
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"home-mac", "lucas.mbp", "a", "x_1"} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("ValidateName(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Home", "-lead", "has space", "../etc", strings.Repeat("a", 64)} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", bad)
		}
	}
}

// failingDeleteKeyring refuses every delete, like a locked or unreachable keyring.
type failingDeleteKeyring struct{ *testutil.MockKeyring }

func (failingDeleteKeyring) Delete(string) error { return errors.New("keyring locked") }

func TestStore_RemoveKeepsTunnelWhenSecretDeleteFails(t *testing.T) {
	ring := setupStore(t)
	if err := Save(Tunnel{Name: "home-mac", ExitNode: "a.ts.net", OAuthClientID: "c"}, "s"); err != nil {
		t.Fatal(err)
	}
	auth.SetKeyring(failingDeleteKeyring{ring})

	if err := Remove("home-mac"); err == nil || !strings.Contains(err.Error(), "keyring locked") {
		t.Fatalf("Remove() = %v, want the keyring error", err)
	}
	if _, err := Get("home-mac"); err != nil {
		t.Errorf("tunnel was dropped despite the failed secret delete: %v", err)
	}
}

func TestStore_SaveRestoresPreviousSecretWhenWriteFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	setupStore(t)
	if err := Save(Tunnel{Name: "home-mac", ExitNode: "a.ts.net", OAuthClientID: "old-client"}, "old-secret"); err != nil {
		t.Fatal(err)
	}
	dir, _ := config.Dir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := Save(Tunnel{Name: "home-mac", ExitNode: "a.ts.net", OAuthClientID: "new-client"}, "new-secret"); err == nil {
		t.Fatal("Save() into a read-only directory succeeded")
	}
	got, _ := Get("home-mac")
	secret, _ := Secret("home-mac")
	if got.OAuthClientID != "old-client" || secret != "old-secret" {
		t.Errorf("after failed save: client %q with secret %q, want old-client with old-secret", got.OAuthClientID, secret)
	}
}

func TestStore_WriteLeavesNoTempFiles(t *testing.T) {
	setupStore(t)
	if err := Save(Tunnel{Name: "home-mac", ExitNode: "a.ts.net", OAuthClientID: "c"}, "s"); err != nil {
		t.Fatal(err)
	}
	dir, _ := config.Dir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
}

func TestStore_RemoveRestoresSecretWhenWriteFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	setupStore(t)
	if err := Save(Tunnel{Name: "home-mac", ExitNode: "a.ts.net", OAuthClientID: "c"}, "s3cret"); err != nil {
		t.Fatal(err)
	}
	dir, _ := config.Dir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := Remove("home-mac"); err == nil {
		t.Fatal("Remove() in a read-only directory succeeded")
	}
	if _, err := Get("home-mac"); err != nil {
		t.Errorf("tunnel no longer listed: %v", err)
	}
	if secret, err := Secret("home-mac"); err != nil || secret != "s3cret" {
		t.Errorf("Secret() = %q, %v; want the original secret restored", secret, err)
	}
}
