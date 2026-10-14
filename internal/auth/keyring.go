package auth

import (
	"errors"
	"fmt"
	"os"

	"github.com/99designs/keyring"

	"github.com/nottelabs/notte-cli/internal/config"
)

const (
	KeyringService = "notte-cli"
	KeyringKey     = "api_key"

	// filePassword encrypts the file store. It is fixed and public, so the file
	// store protects keys no better than a file only the current user can read.
	filePassword = "notte-cli-keyring"
)

// systemBackends are the platform credential stores, in order of preference.
// Only those available on the running platform are tried: the macOS Keychain
// (when built with cgo), the Windows Credential Manager, or on Linux a Secret
// Service, KWallet or pass.
var systemBackends = []keyring.BackendType{
	keyring.KeychainBackend,
	keyring.WinCredBackend,
	keyring.SecretServiceBackend,
	keyring.KWalletBackend,
	keyring.PassBackend,
}

// keyringConfig returns the configuration for both stores. Tests replace it to
// use isolated stores.
var keyringConfig = func(dir string) keyring.Config {
	return keyring.Config{
		ServiceName:      KeyringService,
		FileDir:          dir,
		FilePasswordFunc: keyring.FixedStringPrompt(filePassword),
		// Lets this binary read the Keychain items it creates without a prompt.
		KeychainTrustApplication: true,
	}
}

func configDir() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}
	return dir, nil
}

// openSystemKeyring opens the platform credential store. It returns nil, and
// no error, when the platform has none (for example a headless Linux host).
var openSystemKeyring = func() (keyring.Keyring, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	cfg := keyringConfig(dir)
	cfg.AllowedBackends = systemBackends
	ring, err := keyring.Open(cfg)
	if errors.Is(err, keyring.ErrNoAvailImpl) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open system keyring: %w", err)
	}
	return ring, nil
}

// openFileKeyring opens the encrypted file store in the config directory. It
// is used when no system store is available, and holds keys saved by older
// versions, which only used a system store on Linux.
var openFileKeyring = func() (keyring.Keyring, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	cfg := keyringConfig(dir)
	cfg.AllowedBackends = []keyring.BackendType{keyring.FileBackend}
	ring, err := keyring.Open(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to open keyring (dir=%s): %w", dir, err)
	}
	return ring, nil
}

func isNotFound(err error) bool {
	return errors.Is(err, keyring.ErrKeyNotFound) || errors.Is(err, os.ErrNotExist)
}

// fileCurrentMarker is the Description of file store entries written by this
// version. Such an entry is known to be current: a key saved while the system
// store was unusable, or an identical copy of a migrated key. An entry without
// it was written by an older version and may be older than the system store's.
const fileCurrentMarker = "notte-cli:current"

// getFromSystemKeyring reads a key. A file entry marked current wins and the
// system store is synced to it. Otherwise the system store wins, and an older
// file entry is only used when the system store lacks the key (or cannot be
// read): it is then copied into the system store and marked current. The file
// entry is kept for now because older SDK versions only read the file; the
// next login or logout removes it.
func getFromSystemKeyring(key string) (string, error) {
	system, err := openSystemKeyring()
	if err != nil {
		return "", err
	}
	file, err := openFileKeyring()
	if err != nil {
		return "", err
	}

	fileItem, fileErr := file.Get(key)
	if fileErr == nil && fileItem.Description == fileCurrentMarker {
		if system != nil {
			current, err := system.Get(key)
			// Leave a store we could not read (a locked Keychain over SSH, say) alone.
			if isNotFound(err) || (err == nil && string(current.Data) != string(fileItem.Data)) {
				_ = system.Set(keyring.Item{Key: key, Data: fileItem.Data})
			}
		}
		return string(fileItem.Data), nil
	}

	systemMissing := system == nil
	if system != nil {
		item, err := system.Get(key)
		if err == nil {
			return string(item.Data), nil
		}
		systemMissing = isNotFound(err)
		if fileErr != nil && !systemMissing {
			return "", fmt.Errorf("failed to get key from keyring: %w", err)
		}
	}
	if fileErr != nil {
		return "", fmt.Errorf("failed to get key from keyring: %w", fileErr)
	}

	// An entry from an older version, and the system store does not have the key.
	if system != nil && systemMissing {
		if err := system.Set(keyring.Item{Key: key, Data: fileItem.Data}); err == nil {
			_ = file.Set(keyring.Item{Key: key, Data: fileItem.Data, Description: fileCurrentMarker})
		}
	}
	return string(fileItem.Data), nil
}

// setInSystemKeyring stores a key in the system store, or in the file store
// when there is no usable system store. Only one store keeps the key; the file
// entry is marked current, so on read it wins over an older entry the system
// store may still hold.
func setInSystemKeyring(key, value string) error {
	system, err := openSystemKeyring()
	if err != nil {
		return err
	}
	file, err := openFileKeyring()
	if err != nil {
		return err
	}
	item := keyring.Item{Key: key, Data: []byte(value)}

	if system != nil {
		if err := system.Set(item); err == nil {
			if err := file.Remove(key); err != nil && !isNotFound(err) {
				return fmt.Errorf("stored key in system keyring but failed to remove the old copy from the file keyring: %w", err)
			}
			return nil
		}
	}

	item.Description = fileCurrentMarker
	if err := file.Set(item); err != nil {
		return fmt.Errorf("failed to set key in keyring: %w", err)
	}
	if system != nil {
		_ = system.Remove(key)
	}
	return nil
}

// deleteFromSystemKeyring removes a key from both stores, attempting both even
// when one fails. It reports a keyring.ErrKeyNotFound error when neither store
// had it.
func deleteFromSystemKeyring(key string) error {
	system, err := openSystemKeyring()
	if err != nil {
		return err
	}
	file, err := openFileKeyring()
	if err != nil {
		return err
	}

	removed := false
	var errs []error
	for _, ring := range []keyring.Keyring{system, file} {
		if ring == nil {
			continue
		}
		err := ring.Remove(key)
		switch {
		case err == nil:
			removed = true
		case !isNotFound(err):
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to remove key from keyring: %w", errors.Join(errs...))
	}
	if !removed {
		return fmt.Errorf("failed to remove key from keyring: %w", keyring.ErrKeyNotFound)
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

// GetKeyringSecret retrieves a named secret, such as a tunnel's OAuth client
// secret, from the same store as the API key. Unlike the API key it is not
// qualified by environment.
func GetKeyringSecret(key string) (string, error) {
	return defaultKeyring.Get(key)
}

// SetKeyringSecret stores a named secret alongside the API key.
func SetKeyringSecret(key, value string) error {
	return defaultKeyring.Set(key, value)
}

// DeleteKeyringSecret removes a named secret. A secret that is already gone
// counts as removed; any other failure is returned.
func DeleteKeyringSecret(key string) error {
	err := defaultKeyring.Delete(key)
	if isNotFound(err) {
		return nil
	}
	return err
}
