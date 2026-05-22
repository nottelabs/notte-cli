package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/config"
)

var (
	apiKeyScopeSuffixOnce  sync.Once
	apiKeyScopeSuffixCache string

	// apiKeyScopeOverride, when non-nil, short-circuits the cached lookup.
	// Tests use SetAPIKeyScopeForTesting to deterministically pin the scope.
	apiKeyScopeOverride *string
)

// apiKeyScopeSuffix returns a per-API-key suffix (e.g. ".abc12345") used to
// scope CLI state files such as current_session / current_agent so that two
// different accounts on the same machine never collide on the same shared
// file. When no API key is configured (e.g. during `notte auth login` itself),
// it returns "" - state files keep their legacy unscoped names.
func apiKeyScopeSuffix() string {
	if apiKeyScopeOverride != nil {
		return *apiKeyScopeOverride
	}
	apiKeyScopeSuffixOnce.Do(func() {
		key, _, err := auth.GetAPIKey("")
		if err != nil || key == "" {
			apiKeyScopeSuffixCache = ""
			return
		}
		sum := sha256.Sum256([]byte(key))
		apiKeyScopeSuffixCache = "." + hex.EncodeToString(sum[:])[:8]
	})
	return apiKeyScopeSuffixCache
}

// SetAPIKeyScopeForTesting pins the API-key scope suffix to a deterministic
// value for the duration of the caller's test. Restores the previous override
// (typically nil) via t.Cleanup. Tests only.
func SetAPIKeyScopeForTesting(suffix string) func() {
	prev := apiKeyScopeOverride
	override := suffix
	apiKeyScopeOverride = &override
	return func() { apiKeyScopeOverride = prev }
}

// stateFilePath returns the per-API-key path for a CLI state file. If no API
// key is configured the suffix is empty and the path matches the legacy one.
func stateFilePath(name string) (string, error) {
	configDir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, name+apiKeyScopeSuffix()), nil
}

// readStateFile returns the trimmed contents of a state file, trying the
// scoped path first and falling back to the legacy unscoped path for users
// upgrading from a version that didn't scope by API key. Returns ("", nil)
// when neither file exists, mirroring the previous best-effort read behavior.
func readStateFile(name string) (string, error) {
	configDir, err := config.Dir()
	if err != nil {
		return "", nil
	}
	scoped := filepath.Join(configDir, name+apiKeyScopeSuffix())
	legacy := filepath.Join(configDir, name)
	for _, p := range dedupePaths(scoped, legacy) {
		data, err := os.ReadFile(p)
		if err == nil {
			return strings.TrimSpace(string(data)), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
	}
	return "", nil
}

// writeStateFile writes content to the scoped state file path, creating the
// config directory if needed.
func writeStateFile(name, content string) error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, name+apiKeyScopeSuffix()), []byte(content), 0o600)
}

// clearStateFile removes the scoped state file and, when distinct, the legacy
// unscoped file too. Cleanup is best-effort: a non-existent file is not an error.
func clearStateFile(name string) error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	scoped := filepath.Join(configDir, name+apiKeyScopeSuffix())
	legacy := filepath.Join(configDir, name)
	for _, p := range dedupePaths(scoped, legacy) {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func dedupePaths(a, b string) []string {
	if a == b {
		return []string{a}
	}
	return []string{a, b}
}
