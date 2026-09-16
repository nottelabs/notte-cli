package browser

import (
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// encryptV10 is the inverse of the AES-128-CBC path in decryptValue, so tests
// can build a realistic encrypted_value blob from a known key.
func encryptV10(t *testing.T, key []byte, value string, version int) []byte {
	t.Helper()
	plain := []byte(value)
	if version >= 24 {
		// The real browser prepends a 32-byte domain hash; the exact bytes do
		// not matter here, only that decryptValue strips 32 of them.
		plain = append(make([]byte, 32), plain...)
	}

	pad := aes.BlockSize - len(plain)%aes.BlockSize
	for i := 0; i < pad; i++ {
		plain = append(plain, byte(pad))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	iv := make([]byte, aes.BlockSize)
	for i := range iv {
		iv[i] = ' '
	}
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, plain)
	return append([]byte("v10"), out...)
}

// buildCookieDB writes a minimal Chrome-shaped cookie database and returns a
// profile pointing at it.
func buildCookieDB(t *testing.T, key []byte, version int) Profile {
	t.Helper()
	dir := t.TempDir()
	netDir := filepath.Join(dir, "Network")
	if err := os.MkdirAll(netDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(netDir, "Cookies")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	stmts := []string{
		`CREATE TABLE meta (key TEXT NOT NULL UNIQUE, value LONGVARCHAR)`,
		fmt.Sprintf(`INSERT INTO meta (key, value) VALUES ('version', '%d')`, version),
		`CREATE TABLE cookies (
			host_key TEXT, name TEXT, value TEXT, encrypted_value BLOB,
			path TEXT, expires_utc INTEGER, is_secure INTEGER, is_httponly INTEGER,
			has_expires INTEGER, samesite INTEGER, top_frame_site_key TEXT DEFAULT '')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}

	insert := func(host, name, value string, enc []byte, expiresUTC int64, hasExpires, samesite int, topFrame string) {
		if _, err := db.Exec(
			`INSERT INTO cookies (host_key, name, value, encrypted_value, path, expires_utc, is_secure, is_httponly, has_expires, samesite, top_frame_site_key)
			 VALUES (?, ?, ?, ?, '/', ?, 1, 1, ?, ?, ?)`,
			host, name, value, enc, expiresUTC, hasExpires, samesite, topFrame); err != nil {
			t.Fatal(err)
		}
	}

	// unix 2000000000 -> WebKit micros
	const expiresUnix = 2000000000
	expiresUTC := int64((expiresUnix + chromeEpochOffset) * 1_000_000)

	insert("github.com", "session_id", "", encryptV10(t, key, "secret-value", version), expiresUTC, 1, 1, "")                   // persistent, Lax
	insert(".github.com", "sub", "", encryptV10(t, key, "sub-value", version), 0, 0, 2, "")                                     // session cookie, Strict, subdomain
	insert("example.com", "other", "", encryptV10(t, key, "nope", version), expiresUTC, 1, 0, "")                               // unrelated domain
	insert("plain.com", "legacy", "plaintext-value", nil, expiresUTC, 1, 0, "")                                                 // pre-encryption plaintext, None
	insert("github.com", "partitioned", "", encryptV10(t, key, "part-value", version), expiresUTC, 1, 1, "https://ads.example") // CHIPS partition -> skipped

	return Profile{Dir: "Default", Name: "Test", path: dir}
}

func TestReadCookiesRoundTrip(t *testing.T) {
	key := deriveTestKey()
	profile := buildCookieDB(t, key, 24)
	keys := keyset{v10: key}

	res, err := profile.readCookies(keys, nil)
	if err != nil {
		t.Fatalf("readCookies: %v", err)
	}
	if res.DBVersion != 24 {
		t.Errorf("DBVersion = %d, want 24", res.DBVersion)
	}
	if res.DecryptFailures != 0 {
		t.Errorf("DecryptFailures = %d, want 0", res.DecryptFailures)
	}
	if len(res.Cookies) != 4 {
		t.Fatalf("got %d cookies, want 4 (partitioned cookie excluded)", len(res.Cookies))
	}

	byName := map[string]Cookie{}
	for _, c := range res.Cookies {
		byName[c.Name] = c
	}
	if _, ok := byName["partitioned"]; ok {
		t.Error("partitioned (CHIPS) cookie should be skipped")
	}

	if got := byName["session_id"]; got.Value != "secret-value" {
		t.Errorf("session_id value = %q, want %q", got.Value, "secret-value")
	}
	if got := byName["session_id"]; got.Session || got.Expires != 2000000000 {
		t.Errorf("session_id expiry = (session=%v, exp=%v), want (false, 2000000000)", got.Session, got.Expires)
	}
	if got := byName["session_id"]; got.SameSite != "Lax" {
		t.Errorf("session_id samesite = %q, want Lax", got.SameSite)
	}
	if got := byName["sub"]; !got.Session || got.SameSite != "Strict" {
		t.Errorf("sub = (session=%v, samesite=%q), want (true, Strict)", got.Session, got.SameSite)
	}
	if got := byName["legacy"]; got.Value != "plaintext-value" || got.SameSite != "None" {
		t.Errorf("legacy = (value=%q, samesite=%q), want (plaintext-value, None)", got.Value, got.SameSite)
	}
}

func TestReadCookiesDomainFilter(t *testing.T) {
	key := deriveTestKey()
	profile := buildCookieDB(t, key, 24)
	keys := keyset{v10: key}

	res, err := profile.readCookies(keys, []string{"github.com"})
	if err != nil {
		t.Fatalf("readCookies: %v", err)
	}
	if len(res.Cookies) != 2 {
		t.Fatalf("got %d cookies for github.com, want 2 (apex + subdomain)", len(res.Cookies))
	}
	for _, c := range res.Cookies {
		if c.Domain != "github.com" && c.Domain != ".github.com" {
			t.Errorf("unexpected domain %q leaked through the filter", c.Domain)
		}
	}
}

func TestReadCookiesWrongKeyFailsSoftly(t *testing.T) {
	profile := buildCookieDB(t, deriveTestKey(), 24)
	wrong := make([]byte, 16) // all-zero key

	res, err := profile.readCookies(keyset{v10: wrong}, nil)
	if err != nil {
		t.Fatalf("readCookies should not hard-fail on bad decrypts: %v", err)
	}
	// Chrome's CBC scheme is unauthenticated, so a wrong key can occasionally
	// pass PKCS7 padding and yield garbage rather than a clean failure. The
	// invariants that always hold: the real encrypted values never come back,
	// and the unencrypted legacy cookie is unaffected.
	secrets := map[string]bool{"secret-value": true, "sub-value": true, "nope": true}
	legacyOK := false
	for _, c := range res.Cookies {
		if secrets[c.Value] {
			t.Errorf("wrong key recovered a real value %q", c.Value)
		}
		if c.Name == "legacy" {
			legacyOK = c.Value == "plaintext-value"
		}
	}
	if !legacyOK {
		t.Error("plaintext legacy cookie should survive a wrong key")
	}
}

// TestCopyDBSnapshotWithWAL covers the fallback path used when VACUUM INTO is
// unavailable: the sidecars must be copied so uncheckpointed rows survive.
func TestCopyDBSnapshotWithWAL(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "Cookies")

	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	for _, s := range []string{
		`PRAGMA journal_mode=WAL`,
		`CREATE TABLE t (v TEXT)`,
		`INSERT INTO t VALUES ('kept-in-wal')`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	// The connection stays open so the row lives in -wal, not yet checkpointed
	// into the main file.
	if _, err := os.Stat(dbPath + "-wal"); err != nil {
		t.Skip("sqlite driver did not produce a -wal sidecar")
	}

	snap, err := copyDBSnapshot(dbPath, t.TempDir())
	if err != nil {
		t.Fatalf("copyDBSnapshot: %v", err)
	}
	sdb, err := sql.Open("sqlite", "file:"+snap+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sdb.Close() }()
	var got string
	if err := sdb.QueryRow(`SELECT v FROM t`).Scan(&got); err != nil {
		t.Fatalf("reading snapshot: %v", err)
	}
	if got != "kept-in-wal" {
		t.Errorf("snapshot lost the WAL row: got %q", got)
	}
}

func TestSameSiteString(t *testing.T) {
	cases := map[int]string{-1: "", 0: "None", 1: "Lax", 2: "Strict", 99: ""}
	for in, want := range cases {
		if got := sameSiteString(in); got != want {
			t.Errorf("sameSiteString(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestDomainMatches(t *testing.T) {
	filters := normalizeDomains([]string{".GitHub.com", "example.com"})
	cases := map[string]bool{
		"github.com":          true,
		".github.com":         true,
		"api.github.com":      true,
		"notgithub.com":       false,
		"example.com":         true,
		"sub.example.com":     true,
		"example.com.evil.io": false,
	}
	for host, want := range cases {
		if got := domainMatches(host, filters); got != want {
			t.Errorf("domainMatches(%q) = %v, want %v", host, got, want)
		}
	}
	if !domainMatches("anything.com", nil) {
		t.Error("empty filter should match everything")
	}
}

// deriveTestKey returns a fixed 16-byte AES key for tests.
func deriveTestKey() []byte {
	return []byte("0123456789abcdef")
}
