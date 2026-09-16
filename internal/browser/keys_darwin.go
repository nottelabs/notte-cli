//go:build darwin

package browser

import (
	"crypto/pbkdf2"
	"crypto/sha1"
	"fmt"
	"os/exec"
	"strings"
)

// keyset holds the AES keys for the cookie encryption schemes seen on this OS.
// On macOS every modern cookie is "v10", derived from one Keychain secret.
type keyset struct {
	v10 []byte
	v11 []byte
}

// keys derives the AES-128 key from the browser's "Safe Storage" Keychain item.
// The first read triggers the macOS permission dialog; choosing "Always Allow"
// keeps later reads silent for a stably signed binary.
func (b Browser) keys() (keyset, error) {
	secret, err := b.safeStoragePassword()
	if err != nil {
		return keyset{}, err
	}
	// Chrome on macOS: PBKDF2-HMAC-SHA1, salt "saltysalt", 1003 iterations, 16-byte key.
	key, err := pbkdf2.Key(sha1.New, secret, []byte("saltysalt"), 1003, 16)
	if err != nil {
		return keyset{}, err
	}
	return keyset{v10: key}, nil
}

// safeStoragePassword reads the browser's encryption secret from the login
// Keychain via the `security` tool, so we never link a Keychain framework.
func (b Browser) safeStoragePassword() (string, error) {
	account := strings.TrimSuffix(b.safeStorageLabel, " Safe Storage")

	// Prefer the exact (service, account) pair; fall back to service-only for
	// installs where the account name differs.
	for _, args := range [][]string{
		{"find-generic-password", "-w", "-s", b.safeStorageLabel, "-a", account},
		{"find-generic-password", "-w", "-s", b.safeStorageLabel},
	} {
		out, err := exec.Command("/usr/bin/security", args...).Output()
		if err == nil {
			return strings.TrimRight(string(out), "\r\n"), nil
		}
	}
	return "", fmt.Errorf("could not read the %q key from your Keychain (was the permission dialog dismissed?)", b.safeStorageLabel)
}
