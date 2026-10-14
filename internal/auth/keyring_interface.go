package auth

// KeyringStore defines the interface for credential storage
type KeyringStore interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
}

// defaultKeyring is the package-level keyring used by GetKeyringAPIKey etc.
// Can be overridden for testing via SetKeyring()
var defaultKeyring KeyringStore = &realKeyring{}

// SetKeyring replaces the default keyring (for testing)
func SetKeyring(k KeyringStore) {
	defaultKeyring = k
}

// ResetKeyring restores the real keyring
func ResetKeyring() {
	defaultKeyring = &realKeyring{}
}

// realKeyring wraps the actual 99designs/keyring implementation
type realKeyring struct{}

func (r *realKeyring) Get(key string) (string, error) {
	unlock, err := lockStores()
	if err != nil {
		return "", err
	}
	defer unlock()
	return getFromSystemKeyring(key)
}

func (r *realKeyring) Set(key, value string) error {
	unlock, err := lockStores()
	if err != nil {
		return err
	}
	defer unlock()
	return setInSystemKeyring(key, value)
}

func (r *realKeyring) Delete(key string) error {
	unlock, err := lockStores()
	if err != nil {
		return err
	}
	defer unlock()
	return deleteFromSystemKeyring(key)
}
