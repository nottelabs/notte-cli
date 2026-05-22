package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

// TestMain pins the API-key state-file scope to "" for the entire cmd test
// package so existing tests that hand-roll state-file paths continue to read
// and write the legacy unscoped name. Tests that exercise scoping flip the
// override locally via SetAPIKeyScopeForTesting.
func TestMain(m *testing.M) {
	empty := ""
	apiKeyScopeOverride = &empty
	os.Exit(m.Run())
}

func expectedAPIKeyScopeSuffix(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(apiKey))
	return "." + hex.EncodeToString(sum[:])[:8]
}

// resetAPIKeyScopeCache forces the next apiKeyScopeSuffix() call to re-derive
// the cached value from the current environment. Restores the test-default
// (override pinned to "") via t.Cleanup. Local test helper only.
func resetAPIKeyScopeCache(t *testing.T) {
	t.Helper()
	apiKeyScopeSuffixOnce = sync.Once{}
	apiKeyScopeSuffixCache = ""
	apiKeyScopeOverride = nil
	t.Cleanup(func() {
		empty := ""
		apiKeyScopeSuffixOnce = sync.Once{}
		apiKeyScopeSuffixCache = ""
		apiKeyScopeOverride = &empty
	})
}

// TestAPIKeyScopeSuffix_FromEnvVar verifies the scope suffix is derived from
// NOTTE_API_KEY and is stable across calls.
func TestAPIKeyScopeSuffix_FromEnvVar(t *testing.T) {
	resetAPIKeyScopeCache(t)

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key-12345")

	got := apiKeyScopeSuffix()
	want := expectedAPIKeyScopeSuffix("test-key-12345")
	if got != want {
		t.Fatalf("apiKeyScopeSuffix() = %q, want %q", got, want)
	}

	// Stability across calls (cached via sync.Once).
	if again := apiKeyScopeSuffix(); again != want {
		t.Fatalf("apiKeyScopeSuffix() second call = %q, want %q", again, want)
	}
}

// TestStateFilePath_WithAPIKeyScope verifies writes land at the suffixed path.
func TestStateFilePath_WithAPIKeyScope(t *testing.T) {
	cleanup := SetAPIKeyScopeForTesting(".abc12345")
	t.Cleanup(cleanup)

	tmpDir := t.TempDir()
	config.SetTestConfigDir(tmpDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })

	if err := writeStateFile(config.CurrentSessionFile, "sess_abc"); err != nil {
		t.Fatalf("writeStateFile: %v", err)
	}

	scopedPath := filepath.Join(tmpDir, config.ConfigDirName, config.CurrentSessionFile+".abc12345")
	legacyPath := filepath.Join(tmpDir, config.ConfigDirName, config.CurrentSessionFile)

	if data, err := os.ReadFile(scopedPath); err != nil {
		t.Fatalf("expected scoped file %s to exist: %v", scopedPath, err)
	} else if string(data) != "sess_abc" {
		t.Fatalf("scoped file content = %q, want %q", string(data), "sess_abc")
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy unscoped file must not be written when scope is set; stat err=%v", err)
	}
}

// TestReadStateFile_FallsBackToLegacyUnscopedPath verifies users upgrading
// from a previous CLI version do not lose their existing current_session.
func TestReadStateFile_FallsBackToLegacyUnscopedPath(t *testing.T) {
	cleanup := SetAPIKeyScopeForTesting(".abc12345")
	t.Cleanup(cleanup)

	tmpDir := t.TempDir()
	config.SetTestConfigDir(tmpDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })

	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacyPath := filepath.Join(configDir, config.CurrentSessionFile)
	if err := os.WriteFile(legacyPath, []byte("sess_legacy"), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	got, err := readStateFile(config.CurrentSessionFile)
	if err != nil {
		t.Fatalf("readStateFile: %v", err)
	}
	if got != "sess_legacy" {
		t.Fatalf("readStateFile = %q, want %q (legacy fallback)", got, "sess_legacy")
	}
}

// TestReadStateFile_PrefersScopedOverLegacy verifies the per-API-key file
// wins over the legacy unscoped one when both exist (e.g. a previous CLI
// version left the unscoped one behind).
func TestReadStateFile_PrefersScopedOverLegacy(t *testing.T) {
	cleanup := SetAPIKeyScopeForTesting(".abc12345")
	t.Cleanup(cleanup)

	tmpDir := t.TempDir()
	config.SetTestConfigDir(tmpDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })

	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, config.CurrentSessionFile), []byte("sess_legacy"), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, config.CurrentSessionFile+".abc12345"), []byte("sess_scoped"), 0o600); err != nil {
		t.Fatalf("write scoped: %v", err)
	}

	got, err := readStateFile(config.CurrentSessionFile)
	if err != nil {
		t.Fatalf("readStateFile: %v", err)
	}
	if got != "sess_scoped" {
		t.Fatalf("readStateFile = %q, want %q (scoped takes precedence)", got, "sess_scoped")
	}
}

// TestClearStateFile_RemovesBothScopedAndLegacy verifies the cleanup path
// drops the legacy file alongside the scoped one so users don't see stale
// state after a clear.
func TestClearStateFile_RemovesBothScopedAndLegacy(t *testing.T) {
	cleanup := SetAPIKeyScopeForTesting(".abc12345")
	t.Cleanup(cleanup)

	tmpDir := t.TempDir()
	config.SetTestConfigDir(tmpDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })

	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacyPath := filepath.Join(configDir, config.CurrentSessionFile)
	scopedPath := filepath.Join(configDir, config.CurrentSessionFile+".abc12345")
	if err := os.WriteFile(legacyPath, []byte("sess_legacy"), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	if err := os.WriteFile(scopedPath, []byte("sess_scoped"), 0o600); err != nil {
		t.Fatalf("write scoped: %v", err)
	}

	if err := clearStateFile(config.CurrentSessionFile); err != nil {
		t.Fatalf("clearStateFile: %v", err)
	}

	for _, p := range []string{legacyPath, scopedPath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, stat err=%v", p, err)
		}
	}
}

// TestDifferentAPIKeysIsolated verifies two API keys land on different paths
// so two accounts on the same machine never see each other's sessions.
func TestDifferentAPIKeysIsolated(t *testing.T) {
	tmpDir := t.TempDir()
	config.SetTestConfigDir(tmpDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })

	cleanupA := SetAPIKeyScopeForTesting(".aaaaaaaa")
	if err := writeStateFile(config.CurrentSessionFile, "sess_for_key_A"); err != nil {
		t.Fatalf("writeStateFile A: %v", err)
	}
	cleanupA()

	cleanupB := SetAPIKeyScopeForTesting(".bbbbbbbb")
	if err := writeStateFile(config.CurrentSessionFile, "sess_for_key_B"); err != nil {
		t.Fatalf("writeStateFile B: %v", err)
	}

	got, err := readStateFile(config.CurrentSessionFile)
	if err != nil {
		t.Fatalf("readStateFile (B scope): %v", err)
	}
	if got != "sess_for_key_B" {
		t.Fatalf("readStateFile (B scope) = %q, want %q", got, "sess_for_key_B")
	}
	cleanupB()

	cleanupA2 := SetAPIKeyScopeForTesting(".aaaaaaaa")
	t.Cleanup(cleanupA2)
	got, err = readStateFile(config.CurrentSessionFile)
	if err != nil {
		t.Fatalf("readStateFile (A scope): %v", err)
	}
	if got != "sess_for_key_A" {
		t.Fatalf("readStateFile (A scope) = %q, want %q (account isolation)", got, "sess_for_key_A")
	}
}
