package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/99designs/keyring"

	"github.com/nottelabs/notte-cli/internal/config"
)

// isolatedSystemStore points the system store at a throwaway location: a
// temporary keychain file on macOS, a unique service name elsewhere.
func isolatedSystemStore(t *testing.T) {
	t.Helper()
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	service := "notte-cli-test-" + hex.EncodeToString(suffix)

	keychain := ""
	if runtime.GOOS == "darwin" {
		keychain = filepath.Join(t.TempDir(), "notte-test")
		if out, err := exec.Command("security", "create-keychain", "-p", "test", keychain+".keychain").CombinedOutput(); err != nil {
			t.Fatalf("create-keychain: %v: %s", err, out)
		}
		t.Cleanup(func() { _ = exec.Command("security", "delete-keychain", keychain+".keychain").Run() })
	}

	orig := keyringConfig
	keyringConfig = func(dir string) keyring.Config {
		cfg := orig(dir)
		cfg.ServiceName = service
		cfg.KeychainName = keychain
		cfg.KeychainPasswordFunc = keyring.FixedStringPrompt("test")
		return cfg
	}
	t.Cleanup(func() { keyringConfig = orig })
}

// TestPlatformStore_MigratesFileEntry runs the upgrade path against the real
// system store of this platform: macOS Keychain or Windows Credential Manager.
func TestPlatformStore_MigratesFileEntry(t *testing.T) {
	if os.Getenv("NOTTE_KEYRING_TEST") == "" {
		t.Skip("set NOTTE_KEYRING_TEST=1 to run against the system credential store")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("covers the macOS Keychain and the Windows Credential Manager")
	}
	isolatedSystemStore(t)
	config.SetTestConfigDir(t.TempDir())
	t.Cleanup(func() { config.SetTestConfigDir("") })
	ResetKeyring()

	system, err := openSystemKeyring()
	if err != nil || system == nil {
		t.Fatalf("no system store: %v", err)
	}
	file, err := openFileKeyring()
	if err != nil {
		t.Fatal(err)
	}
	const key = "api_key:prod"
	t.Cleanup(func() { _ = system.Remove(key); _ = file.Remove(key) })

	// A key saved by an older version, in the file store only.
	if err := file.Set(keyring.Item{Key: key, Data: []byte("file-key")}); err != nil {
		t.Fatal(err)
	}
	got, err := GetKeyringSecret(key)
	if err != nil || got != "file-key" {
		t.Fatalf("Get = (%q, %v), want file-key", got, err)
	}
	if item, err := system.Get(key); err != nil || string(item.Data) != "file-key" {
		t.Fatalf("system store after read = (%q, %v), want the migrated key", item.Data, err)
	}
	if _, err := file.Get(key); err != nil {
		t.Fatalf("file entry should be kept after migration: %v", err)
	}

	if err := SetKeyringSecret(key, "new-key"); err != nil {
		t.Fatal(err)
	}
	if item, err := system.Get(key); err != nil || string(item.Data) != "new-key" {
		t.Fatalf("system store after login = (%q, %v), want new-key", item.Data, err)
	}
	if _, err := file.Get(key); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("file entry after login: %v, want ErrKeyNotFound", err)
	}

	if err := DeleteKeyringSecret(key); err != nil {
		t.Fatal(err)
	}
	if _, err := system.Get(key); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("system store after logout: %v, want ErrKeyNotFound", err)
	}
}
