package auth

import (
	"fmt"
	"os"
	"path/filepath"
)

// storeLockFile serializes reads and writes of saved secrets across notte
// processes. Reading can migrate a key between stores while another process
// logs in; without the lock, the migration could put back a key the login just
// replaced.
const storeLockFile = "keyring.lock"

// lockStores takes an exclusive lock on the config directory's lock file and
// returns the function that releases it.
func lockStores() (func(), error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, storeLockFile), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to open keyring lock: %w", err)
	}
	if err := lockFile(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to lock keyring: %w", err)
	}
	return func() {
		_ = unlockFile(f)
		_ = f.Close()
	}, nil
}
