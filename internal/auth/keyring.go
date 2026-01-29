package auth

import (
	"github.com/99designs/keyring"
)

const (
	KeyringService = "notte-cli"
	KeyringKey     = "api_key"
	KeychainName   = "notte-api-key"
)

// getFromSystemKeyring reads from the real OS keyring
func getFromSystemKeyring(key string) (string, error) {
	ring, err := keyring.Open(keyring.Config{
		ServiceName:  KeyringService,
		KeychainName: KeychainName,
	})
	if err != nil {
		return "", err
	}

	item, err := ring.Get(key)
	if err != nil {
		return "", err
	}

	return string(item.Data), nil
}

// setInSystemKeyring writes to the real OS keyring
func setInSystemKeyring(key, value string) error {
	ring, err := keyring.Open(keyring.Config{
		ServiceName:  KeyringService,
		KeychainName: KeychainName,
	})
	if err != nil {
		return err
	}

	return ring.Set(keyring.Item{
		Key:  key,
		Data: []byte(value),
	})
}

// deleteFromSystemKeyring removes from the real OS keyring
func deleteFromSystemKeyring(key string) error {
	ring, err := keyring.Open(keyring.Config{
		ServiceName:  KeyringService,
		KeychainName: KeychainName,
	})
	if err != nil {
		return err
	}

	return ring.Remove(key)
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
