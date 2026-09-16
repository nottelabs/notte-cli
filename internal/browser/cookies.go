package browser

import (
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo
)

// Cookie is a single decrypted cookie in a browser-neutral shape. The command
// layer maps it onto the API's cookie type.
type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Secure   bool
	HTTPOnly bool
	SameSite string  // "" (unspecified), "None", "Lax" or "Strict"
	Expires  float64 // Unix seconds; 0 for a session cookie
	Session  bool
}

// ReadResult carries the cookies plus enough context to report honestly on what
// happened (some cookies can fail to decrypt without the whole read failing).
type ReadResult struct {
	Cookies         []Cookie
	DecryptFailures int
	DBVersion       int
}

// chromeEpochOffset converts Chrome's WebKit timestamp (microseconds since
// 1601-01-01) to a Unix timestamp: unix = micros/1e6 - 11644473600.
const chromeEpochOffset = 11644473600

// errPlaintext marks a value that was never encrypted (older Chrome stored some
// cookies in the plaintext `value` column).
var errPlaintext = errors.New("value is not encrypted")

// ReadCookies decrypts the cookies in the profile. If domains is non-empty, only
// cookies for those domains (and their subdomains) are returned; everything else
// is dropped before it ever leaves this function.
func (p Profile) ReadCookies(domains []string) (ReadResult, error) {
	// Firefox stores cookies in the clear, so it skips the key step entirely and
	// never touches the Chromium-only keyset seam.
	if p.Browser.kind == firefoxKind {
		return p.readFirefoxCookies(domains)
	}
	keys, err := p.Browser.keys()
	if err != nil {
		return ReadResult{}, err
	}
	return p.readCookies(keys, domains)
}

// readCookies is the key-independent core, split out so it can be exercised in
// tests with a supplied keyset instead of the OS keystore.
func (p Profile) readCookies(keys keyset, domains []string) (ReadResult, error) {
	dbPath := p.cookieDBPath()
	if dbPath == "" {
		return ReadResult{}, fmt.Errorf("no cookie database found for profile %q", p.Name)
	}

	tmpDir, err := os.MkdirTemp("", "notte-cookies-")
	if err != nil {
		return ReadResult{}, err
	}
	// The copy holds live session tokens; remove it as soon as we are done.
	defer func() { _ = os.RemoveAll(tmpDir) }()

	copyPath, err := copyDBSnapshot(dbPath, tmpDir)
	if err != nil {
		return ReadResult{}, err
	}

	db, err := sql.Open("sqlite", "file:"+copyPath+"?mode=ro")
	if err != nil {
		return ReadResult{}, err
	}
	defer func() { _ = db.Close() }()

	version := metaVersion(db)
	filters := normalizeDomains(domains)

	rows, err := db.Query(`SELECT host_key, name, value, encrypted_value, path,
		expires_utc, is_secure, is_httponly, has_expires, samesite FROM cookies`)
	if err != nil {
		return ReadResult{}, fmt.Errorf("could not read cookies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := ReadResult{DBVersion: version}
	for rows.Next() {
		var (
			host, name, plainValue, path string
			encrypted                    []byte
			expiresUTC                   int64
			isSecure, isHTTPOnly         int
			hasExpires, sameSite         int
		)
		if err := rows.Scan(&host, &name, &plainValue, &encrypted, &path,
			&expiresUTC, &isSecure, &isHTTPOnly, &hasExpires, &sameSite); err != nil {
			return ReadResult{}, err
		}

		if !domainMatches(host, filters) {
			continue
		}

		value, err := decryptValue(keys, encrypted, plainValue, version)
		if err != nil {
			result.DecryptFailures++
			continue
		}

		cookie := Cookie{
			Name:     name,
			Value:    value,
			Domain:   host,
			Path:     path,
			Secure:   isSecure != 0,
			HTTPOnly: isHTTPOnly != 0,
			SameSite: sameSiteString(sameSite),
		}
		if hasExpires != 0 && expiresUTC != 0 {
			cookie.Expires = float64(expiresUTC)/1_000_000 - chromeEpochOffset
		} else {
			cookie.Session = true
		}
		result.Cookies = append(result.Cookies, cookie)
	}
	return result, rows.Err()
}

// cookieDBPath returns the cookie database path, preferring the modern location
// (Network/Cookies, since Chrome 96) and falling back to the legacy one.
func (p Profile) cookieDBPath() string {
	candidates := []string{
		filepath.Join(p.path, "Network", "Cookies"),
		filepath.Join(p.path, "Cookies"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// copyDBSnapshot copies the cookie DB and its WAL sidecars to tmpDir so we can
// read a consistent snapshot while the browser is still running. Copying only
// the main file (and not -wal/-shm) is the classic mistake that yields stale or
// inconsistent reads, so all three are copied when present.
func copyDBSnapshot(dbPath, tmpDir string) (string, error) {
	dst := filepath.Join(tmpDir, "Cookies")
	if err := copyFile(dbPath, dst); err != nil {
		return "", err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		src := dbPath + suffix
		if _, err := os.Stat(src); err != nil {
			continue // sidecar absent (rollback-journal mode); nothing to copy
		}
		if err := copyFile(src, dst+suffix); err != nil {
			return "", err
		}
	}
	return dst, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// metaVersion reads the schema version, which tells us whether cookie plaintext
// carries the 32-byte domain-hash prefix (added at version 24). A missing value
// is treated as 0.
func metaVersion(db *sql.DB) int {
	var v int
	_ = db.QueryRow(`SELECT value FROM meta WHERE key = 'version'`).Scan(&v)
	return v
}

// decryptValue turns an encrypted_value blob into the cleartext cookie value.
// It handles the AES-128-CBC scheme Chrome uses on macOS and Linux, the older
// plaintext column, and the domain-bound 32-byte prefix on newer databases.
func decryptValue(keys keyset, encrypted []byte, plain string, version int) (string, error) {
	if len(encrypted) == 0 {
		return plain, nil
	}
	if len(encrypted) < 3 {
		return "", errPlaintext
	}

	prefix := string(encrypted[:3])
	var key []byte
	switch prefix {
	case "v10":
		key = keys.v10
	case "v11":
		key = keys.v11
	default:
		// No known version tag: an unencrypted legacy value.
		return plain, nil
	}
	if len(key) == 0 {
		return "", fmt.Errorf("no decryption key available for %s cookies", prefix)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	body := encrypted[3:]
	if len(body) == 0 || len(body)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext is not a whole number of blocks")
	}

	iv := make([]byte, aes.BlockSize)
	for i := range iv {
		iv[i] = ' '
	}
	decrypted := make([]byte, len(body))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(decrypted, body)

	decrypted, err = stripPKCS7(decrypted)
	if err != nil {
		return "", err
	}
	// Chrome >= 130 (schema version 24+) prepends the SHA-256 of the cookie's
	// eTLD+1 to the plaintext. Drop it to recover the real value.
	if version >= 24 && len(decrypted) >= 32 {
		decrypted = decrypted[32:]
	}
	return string(decrypted), nil
}

func stripPKCS7(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("empty plaintext")
	}
	pad := int(b[len(b)-1])
	if pad == 0 || pad > aes.BlockSize || pad > len(b) {
		return nil, fmt.Errorf("invalid padding")
	}
	return b[:len(b)-pad], nil
}

// sameSiteString maps Chrome's integer samesite to the API's string form.
// -1 (unspecified) becomes "" so the field is omitted and the server default
// applies.
func sameSiteString(v int) string {
	switch v {
	case 0:
		return "None"
	case 1:
		return "Lax"
	case 2:
		return "Strict"
	default:
		return ""
	}
}

func normalizeDomains(domains []string) []string {
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		d = strings.TrimPrefix(d, ".")
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}

// domainMatches reports whether a cookie host matches one of the requested
// domains, treating a request for "example.com" as also covering its
// subdomains. An empty filter matches everything.
func domainMatches(host string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	host = strings.TrimPrefix(strings.ToLower(host), ".")
	for _, d := range filters {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}
