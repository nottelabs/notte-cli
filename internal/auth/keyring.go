package auth

import (
	"fmt"
	"os"

	"github.com/99designs/keyring"

	"github.com/nottelabs/notte-cli/internal/config"
)

const (
	KeyringService = "notte-cli"
	KeyringKey     = "api_key"
	KeychainName   = "notte-api-key"
)

// openKeyring initializes and returns a keyring instance
func openKeyring() (keyring.Keyring, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
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

// GetKeyringAPIKey retrieves API key from OS keychain
func GetKeyringAPIKey() (string, error) {
	return defaultKeyring.Get(KeyringKey)
}

// SetKeyringAPIKey stores API key in OS keychain
func SetKeyringAPIKey(apiKey string) error {
	return defaultKeyring.Set(KeyringKey, apiKey)
}

// DeleteKeyringAPIKey removes API key from OS keychain
func DeleteKeyringAPIKey() error {
	return defaultKeyring.Delete(KeyringKey)
}
