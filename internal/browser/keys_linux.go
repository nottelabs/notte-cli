//go:build linux

package browser

import (
	"crypto/pbkdf2"
	"crypto/sha1"
	"os/exec"
	"strings"
)

// keyset holds the AES keys for the cookie encryption schemes seen on this OS.
// On Linux "v11" cookies use a secret from the desktop keyring while "v10"
// cookies use a well-known constant, so both keys are derived up front.
type keyset struct {
	v10 []byte
	v11 []byte
}

// keys derives the Linux AES-128 keys. The "v10" key comes from the constant
// "peanuts" password Chrome uses when no keyring is available; the "v11" key,
// when present, comes from the browser's Secret Service item.
func (b Browser) keys() (keyset, error) {
	ks := keyset{v10: deriveLinuxKey("peanuts")}
	if secret, ok := b.keyringSecret(); ok {
		ks.v11 = deriveLinuxKey(secret)
	}
	return ks, nil
}

// deriveLinuxKey applies Chrome's Linux KDF: PBKDF2-HMAC-SHA1, salt "saltysalt",
// a single iteration, 16-byte key.
func deriveLinuxKey(password string) []byte {
	key, err := pbkdf2.Key(sha1.New, password, []byte("saltysalt"), 1, 16)
	if err != nil {
		// The parameters are fixed and valid, so this cannot fail in practice.
		return nil
	}
	return key
}

// keyringSecret looks up the browser's encryption password in the Secret
// Service using secret-tool. If it is not installed or the item is absent we
// return false and fall back to the "v10" path, which is what Chrome itself
// does when it cannot reach a keyring.
func (b Browser) keyringSecret() (string, bool) {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return "", false
	}
	out, err := exec.Command("secret-tool", "lookup", "application", b.linuxKeyringApp).Output()
	if err != nil {
		return "", false
	}
	secret := strings.TrimRight(string(out), "\r\n")
	if secret == "" {
		return "", false
	}
	return secret, true
}
