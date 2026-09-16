package browser

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func firefoxBrowser(t *testing.T) Browser {
	t.Helper()
	b, ok := BrowserByID("firefox")
	if !ok {
		t.Fatal("firefox browser not registered")
	}
	return b
}

// buildFirefoxCookieDB writes a minimal moz_cookies database (plaintext values,
// no encryption) and returns a profile pointing at it.
func buildFirefoxCookieDB(t *testing.T) Profile {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cookies.sqlite")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`CREATE TABLE moz_cookies (
		id INTEGER PRIMARY KEY, originAttributes TEXT NOT NULL DEFAULT '',
		name TEXT, value TEXT, host TEXT, path TEXT, expiry INTEGER,
		isSecure INTEGER, isHttpOnly INTEGER, sameSite INTEGER)`); err != nil {
		t.Fatal(err)
	}

	insert := func(origin, host, name, value string, expiry int64, sameSite int) {
		if _, err := db.Exec(
			`INSERT INTO moz_cookies (originAttributes, host, name, value, path, expiry, isSecure, isHttpOnly, sameSite)
			 VALUES (?, ?, ?, ?, '/', ?, 1, 1, ?)`,
			origin, host, name, value, expiry, sameSite); err != nil {
			t.Fatal(err)
		}
	}

	insert("", "github.com", "a", "gh", 2_000_000_000_000, 1)                              // ms expiry, Lax
	insert("", ".github.com", "b", "sub", 2_000_000_000, 2)                                // seconds expiry, Strict, subdomain
	insert("", "example.com", "c", "ex", 2_000_000_000_000, 0)                             // unrelated, sameSite unset
	insert("", "github.com", "d", "ss256", 2_000_000_000_000, 256)                         // unknown sameSite -> omit
	insert("^userContextId=1", "github.com", "e", "container", 2_000_000_000_000, 1)       // non-default container -> skip
	insert("^partitionKey=(https,x.com)", "github.com", "f", "part", 2_000_000_000_000, 1) // partitioned -> skip

	return Profile{Browser: firefoxBrowser(t), Dir: "test", Name: "Test", path: dir}
}

func TestReadFirefoxCookiesRoundTrip(t *testing.T) {
	profile := buildFirefoxCookieDB(t)

	res, err := profile.readFirefoxCookies(nil)
	if err != nil {
		t.Fatalf("readFirefoxCookies: %v", err)
	}
	if res.DecryptFailures != 0 {
		t.Errorf("DecryptFailures = %d, want 0 (Firefox is plaintext)", res.DecryptFailures)
	}
	// Container and partitioned cookies are dropped; 4 default-jar cookies remain.
	if len(res.Cookies) != 4 {
		t.Fatalf("got %d cookies, want 4", len(res.Cookies))
	}

	byName := map[string]Cookie{}
	for _, c := range res.Cookies {
		byName[c.Name] = c
	}
	if _, ok := byName["e"]; ok {
		t.Error("container cookie should be filtered out")
	}
	if _, ok := byName["f"]; ok {
		t.Error("partitioned cookie should be filtered out")
	}

	if a := byName["a"]; a.Value != "gh" || a.SameSite != "Lax" || a.Session {
		t.Errorf("cookie a mapped wrong: %+v", a)
	}
	if a := byName["a"]; a.Expires != 2_000_000_000 {
		t.Errorf("ms expiry not converted: got %v, want 2000000000", a.Expires)
	}
	if b := byName["b"]; b.Expires != 2_000_000_000 || b.SameSite != "Strict" {
		t.Errorf("seconds expiry / samesite wrong: %+v", b)
	}
	if d := byName["d"]; d.SameSite != "" {
		t.Errorf("unknown sameSite should be omitted, got %q", d.SameSite)
	}
}

func TestReadFirefoxCookiesDomainFilter(t *testing.T) {
	profile := buildFirefoxCookieDB(t)

	res, err := profile.readFirefoxCookies([]string{"github.com"})
	if err != nil {
		t.Fatalf("readFirefoxCookies: %v", err)
	}
	// a (github.com), b (.github.com), d (github.com); container/partitioned excluded.
	if len(res.Cookies) != 3 {
		t.Fatalf("got %d github cookies, want 3", len(res.Cookies))
	}
	for _, c := range res.Cookies {
		if c.Domain != "github.com" && c.Domain != ".github.com" {
			t.Errorf("unexpected domain %q past the filter", c.Domain)
		}
	}
}

func TestFirefoxExpiryUnix(t *testing.T) {
	cases := map[int64]float64{
		2_000_000_000:     2_000_000_000, // already seconds
		2_000_000_000_000: 2_000_000_000, // milliseconds -> seconds
		0:                 0,
	}
	for in, want := range cases {
		if got := firefoxExpiryUnix(in); got != want {
			t.Errorf("firefoxExpiryUnix(%d) = %v, want %v", in, got, want)
		}
	}
}

func TestFirefoxSameSite(t *testing.T) {
	cases := map[int]string{0: "", 1: "Lax", 2: "Strict", 256: "", -1: ""}
	for in, want := range cases {
		if got := firefoxSameSite(in); got != want {
			t.Errorf("firefoxSameSite(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestParseFirefoxProfiles(t *testing.T) {
	root := t.TempDir()
	// Two profiles exist on disk; the install default is the second one.
	for _, d := range []string{"Profiles/aaaa.default", "Profiles/bbbb.default-release"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ini := `[Profile0]
Name=default
IsRelative=1
Path=Profiles/aaaa.default
Default=1

[Profile1]
Name=default-release
IsRelative=1
Path=Profiles/bbbb.default-release

[Install ABC123]
Default=Profiles/bbbb.default-release
Locked=1
`
	profiles := parseFirefoxProfiles(firefoxBrowser(t), root, ini)
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles, want 2", len(profiles))
	}
	// The install default must be surfaced first, even though Profile0 has Default=1.
	if profiles[0].Name != "default-release" {
		t.Errorf("first profile = %q, want default-release (the install default)", profiles[0].Name)
	}
	if profiles[0].Dir != "bbbb.default-release" {
		t.Errorf("Dir = %q, want bbbb.default-release", profiles[0].Dir)
	}
}

func TestParseFirefoxProfilesSkipsMissingDirs(t *testing.T) {
	root := t.TempDir()
	ini := `[Profile0]
Name=ghost
IsRelative=1
Path=Profiles/does-not-exist
`
	if got := parseFirefoxProfiles(firefoxBrowser(t), root, ini); len(got) != 0 {
		t.Errorf("expected no profiles for a missing dir, got %d", len(got))
	}
}
