// Package aac drives the Agent Access CLI (aac, https://github.com/bitwarden/agent-access)
// on the machine that holds the user's password manager.
//
// `aac listen --reusable-psk` waits for credential requests and asks the user to
// approve each one. A remote client pairs with it using its token:
// `<psk hex>_<listener identity fingerprint hex>`. A reusable PSK keeps working
// across restarts, which Notte needs: every credential lookup in a cloud session
// pairs again from whichever API pod serves it.
//
// The token is read from aac's own files rather than from its screen: the PSK
// from its reusable PSK store, which `aac listen --reusable-psk` loads (it takes
// the first stored PSK and creates one when there is none), and the fingerprint
// from `aac connections list`. The formats are those of aac v0.11.0.
package aac

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"
)

// Version is the aac release Notte's API pairs with.
const Version = "v0.11.0"

// InstallHint tells the user where to get aac.
const InstallHint = "install aac " + Version + " from https://github.com/bitwarden/agent-access/releases/tag/" + Version

// pskLength is the size of an aac PSK in bytes.
const pskLength = 32

// pskStoreFile is where `aac listen --reusable-psk` keeps its PSKs, under ~/.access-protocol.
const pskStoreFile = "psk_store_user_client.json"

// identityLine is how `aac connections list --client-type user` prints the listener's fingerprint.
var identityLine = regexp.MustCompile(`Your identity: ([0-9a-f]{64})`)

// ErrNoIdentity means the listener has no identity yet: `aac listen` creates it when it starts.
var ErrNoIdentity = errors.New("aac has no listener identity yet")

// Find returns the path of the aac binary on PATH.
func Find() (string, error) {
	path, err := exec.LookPath("aac")
	if err != nil {
		return "", fmt.Errorf("aac not found on PATH: %s", InstallHint)
	}
	return path, nil
}

// pskRecord is one entry of aac's reusable PSK store.
type pskRecord struct {
	PskID     string  `json:"psk_id"`
	PskHex    string  `json:"psk_hex"`
	Name      *string `json:"name"`
	CreatedAt int64   `json:"created_at"`
}

type pskStore struct {
	Psks []pskRecord `json:"psks"`
}

// storeDir is aac's directory: it uses the home directory of the user running it.
func storeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find the home directory aac uses: %w", err)
	}
	return filepath.Join(home, ".access-protocol"), nil
}

// ReusablePSK returns the PSK `aac listen --reusable-psk` will accept, as hex: the
// first one in its store, or a new one this writes there for it to load.
func ReusablePSK() (string, error) {
	dir, err := storeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, pskStoreFile)
	var store pskStore
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &store); err != nil {
			return "", fmt.Errorf("unreadable aac PSK store %s: %w", path, err)
		}
		// aac loads the first stored PSK.
		if len(store.Psks) > 0 {
			psk := store.Psks[0].PskHex
			if raw, err := hex.DecodeString(psk); err != nil || len(raw) != pskLength {
				return "", fmt.Errorf("aac PSK store %s holds an invalid PSK", path)
			}
			return psk, nil
		}
	case !errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("could not read the aac PSK store %s: %w", path, err)
	}

	raw := make([]byte, pskLength)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("could not generate a PSK: %w", err)
	}
	digest := sha256.Sum256(raw)
	name := "notte"
	store.Psks = []pskRecord{{
		// aac's PSK id: the first 8 bytes of the PSK's SHA-256, as hex.
		PskID:     hex.EncodeToString(digest[:8]),
		PskHex:    hex.EncodeToString(raw),
		Name:      &name,
		CreatedAt: time.Now().Unix(),
	}}
	data, err = json.MarshalIndent(store, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("could not create %s: %w", dir, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("could not write the aac PSK store %s: %w", path, err)
	}
	return store.Psks[0].PskHex, nil
}

// Fingerprint returns the listener's identity fingerprint, as hex. ErrNoIdentity
// until `aac listen` has run once.
func Fingerprint(ctx context.Context, binary string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, "connections", "list", "--client-type", "user")
	// Plain text: no ANSI colors around the fingerprint.
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("aac connections list failed: %w", err)
	}
	match := identityLine.FindSubmatch(out)
	if match == nil {
		return "", ErrNoIdentity
	}
	return string(match[1]), nil
}

// WaitForFingerprint polls Fingerprint until the listener has created its
// identity, or the context ends.
func WaitForFingerprint(ctx context.Context, binary string, every time.Duration) (string, error) {
	for {
		fingerprint, err := Fingerprint(ctx, binary)
		if !errors.Is(err, ErrNoIdentity) {
			return fingerprint, err
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("aac listen did not create its identity: %w", ctx.Err())
		case <-time.After(every):
		}
	}
}

// Token is the pairing token of a reusable PSK listener.
func Token(psk, fingerprint string) string {
	return psk + "_" + fingerprint
}

// tokenPattern matches a reusable PSK token.
var tokenPattern = regexp.MustCompile(`^[0-9a-f]{64}_[0-9a-f]{64}$`)

// ValidToken reports whether token has the shape of `aac listen --reusable-psk`'s token.
func ValidToken(token string) bool {
	return tokenPattern.MatchString(token)
}
