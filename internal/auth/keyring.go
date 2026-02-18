package auth

import (
	"fmt"
	"os"
	"sync"

	"github.com/99designs/keyring"

	"github.com/nottelabs/notte-cli/internal/config"
)

const (
	KeyringService = "notte-cli"
	KeyringKey     = "api_key"
	KeychainName   = "notte-api-key"
)

var (
	onceMkdir sync.Once
	mkdirErr  error
)

// openKeyring initializes and returns a keyring instance
func openKeyring() (keyring.Keyring, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	onceMkdir.Do(func() {
		mkdirErr = os.MkdirAll(dir, 0o700)
	})
	if mkdirErr != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", mkdirErr)
	}

	// Note: FixedStringPrompt is used for the file backend fallback (when no system keyring is available).
	// The file backend provides basic protection for stored credentials on systems without keyring support.
	ring, err := keyring.Open(keyring.Config{
		ServiceName:              KeyringService,
		KeychainName:             KeychainName,
		FileDir:                  dir,
		AllowedBackends:          []keyring.BackendType{keyring.SecretServiceBackend, keyring.KWalletBackend, keyring.PassBackend, keyring.FileBackend},
		FilePasswordFunc:         keyring.FixedStringPrompt("notte-cli-keyring"),
		KeychainTrustApplication: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open keyring (dir=%s): %w", dir, err)
	}
	return ring, nil
}

// getFromSystemKeyring reads from the real OS keyring
func getFromSystemKeyring(key string) (string, error) {
	ring, err := openKeyring()
	if err != nil {
		return "", err
	}

	item, err := ring.Get(key)
	if err != nil {
		return "", fmt.Errorf("failed to get key from keyring: %w", err)
	}

	return string(item.Data), nil
}

// setInSystemKeyring writes to the real OS keyring
func setInSystemKeyring(key, value string) error {
	ring, err := openKeyring()
	if err != nil {
		return err
	}

	if err := ring.Set(keyring.Item{
		Key:  key,
		Data: []byte(value),
	}); err != nil {
		return fmt.Errorf("failed to set key in keyring: %w", err)
	}

	return nil
}

// deleteFromSystemKeyring removes from the real OS keyring
func deleteFromSystemKeyring(key string) error {
	ring, err := openKeyring()
	if err != nil {
		return err
	}

	if err := ring.Remove(key); err != nil {
		return fmt.Errorf("failed to remove key from keyring: %w", err)
	}

	return nil
}

// GetKeyringAPIKey retrieves API key from OS keychain for the current environment.
// On first read after upgrade, it falls back to the legacy "api_key" entry and
// auto-migrates it to the env-qualified key.
func GetKeyringAPIKey() (string, error) {
	envLabel := ResolveEnvLabel(GetCurrentAPIURL())
	envKey := KeyringKeyForEnv(envLabel)

	// Try env-qualified key first
	if val, err := defaultKeyring.Get(envKey); err == nil {
		return val, nil
	}

	// Fall back to legacy key and auto-migrate to prod.
	// The legacy entry was always associated with the default (prod) environment,
	// so always migrate to "api_key:prod" regardless of current NOTTE_API_URL.
	val, err := defaultKeyring.Get(KeyringKey)
	if err != nil {
		return "", err
	}

	prodKey := KeyringKeyForEnv("prod")
	_ = defaultKeyring.Set(prodKey, val)
	_ = defaultKeyring.Delete(KeyringKey)

	// Only return the value if the current env is actually prod
	if envLabel != "prod" {
		return "", fmt.Errorf("failed to get key from keyring: legacy key migrated to prod, but current environment is %s", envLabel)
	}

	return val, nil
}

// SetKeyringAPIKey stores API key in OS keychain for the current environment.
func SetKeyringAPIKey(apiKey string) error {
	envLabel := ResolveEnvLabel(GetCurrentAPIURL())
	return defaultKeyring.Set(KeyringKeyForEnv(envLabel), apiKey)
}

// DeleteKeyringAPIKey removes API key from OS keychain for the current environment.
func DeleteKeyringAPIKey() error {
	envLabel := ResolveEnvLabel(GetCurrentAPIURL())
	return defaultKeyring.Delete(KeyringKeyForEnv(envLabel))
}
