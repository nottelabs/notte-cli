package browser

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo
)

// firefoxLinuxRoot returns the Firefox root directory, probing the native,
// snap and flatpak locations in turn and returning the first that actually
// holds a profiles.ini. The native path is returned as the fallback so error
// messages point somewhere sensible when Firefox is not installed.
func firefoxLinuxRoot(home, linuxDir string) string {
	native := filepath.Join(home, ".mozilla", linuxDir)
	candidates := []string{
		native,
		filepath.Join(home, "snap", "firefox", "common", ".mozilla", linuxDir),
		filepath.Join(home, ".var", "app", "org.mozilla.firefox", ".mozilla", linuxDir),
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "profiles.ini")); err == nil {
			return c
		}
	}
	return native
}

// firefoxProfiles enumerates Firefox profiles from profiles.ini, producing the
// same Profile values the Chromium path does. Unlike Chrome, Firefox has no
// per-profile encryption and its cookies are stored in the clear.
func (b Browser) firefoxProfiles() ([]Profile, error) {
	root, err := b.userDataDir()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(root, "profiles.ini"))
	if err != nil {
		return nil, fmt.Errorf("could not read %s profile list: %w", b.DisplayName, err)
	}
	return parseFirefoxProfiles(b, root, string(raw)), nil
}

// parseFirefoxProfiles hand-parses profiles.ini (there is no INI dependency in
// this module) and returns the profiles whose directory exists on disk. The
// profile the running Firefox actually uses - named by the [Install...]
// section's Default key - is placed first so it is the obvious pick.
func parseFirefoxProfiles(b Browser, root, ini string) []Profile {
	type section struct {
		name  string
		props map[string]string
	}

	var sections []section
	var cur *section
	for _, line := range strings.Split(ini, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			sections = append(sections, section{name: line[1 : len(line)-1], props: map[string]string{}})
			cur = &sections[len(sections)-1]
			continue
		}
		if cur == nil {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			cur.props[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}

	// The install's Default is a profile path like "Profiles/xxxx.default-release".
	installDefault := ""
	for _, s := range sections {
		if strings.HasPrefix(s.name, "Install") {
			if d := s.props["Default"]; d != "" {
				installDefault = d
			}
		}
	}

	var profiles []Profile
	for _, s := range sections {
		if !strings.HasPrefix(s.name, "Profile") {
			continue
		}
		rel := s.props["Path"]
		if rel == "" {
			continue
		}
		path := rel
		if s.props["IsRelative"] != "0" { // default and "1" mean relative to root
			path = filepath.Join(root, filepath.FromSlash(rel))
		}
		if st, err := os.Stat(path); err != nil || !st.IsDir() {
			continue
		}

		name := s.props["Name"]
		if name == "" {
			name = filepath.Base(path)
		}
		p := Profile{
			Browser: b,
			Dir:     filepath.Base(path),
			Name:    name,
			path:    path,
		}
		// Surface the install default first.
		if installDefault != "" && filepath.ToSlash(rel) == installDefault {
			profiles = append([]Profile{p}, profiles...)
		} else {
			profiles = append(profiles, p)
		}
	}
	return profiles
}

// readFirefoxCookies mirrors readCookies but for the moz_cookies schema:
// plaintext values (no key, no decrypt), a different sameSite encoding, and an
// expiry that recent Firefox stores in milliseconds. It reuses the shared
// snapshot and domain-filter helpers.
func (p Profile) readFirefoxCookies(domains []string) (ReadResult, error) {
	dbPath := filepath.Join(p.path, "cookies.sqlite")
	if _, err := os.Stat(dbPath); err != nil {
		return ReadResult{}, fmt.Errorf("no cookie database found for profile %q", p.Name)
	}

	tmpDir, err := os.MkdirTemp("", "notte-cookies-")
	if err != nil {
		return ReadResult{}, err
	}
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

	filters := normalizeDomains(domains)

	rows, err := db.Query(`SELECT host, name, value, path, expiry,
		isSecure, isHttpOnly, sameSite, originAttributes FROM moz_cookies`)
	if err != nil {
		return ReadResult{}, fmt.Errorf("could not read cookies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	now := time.Now().Unix()
	var result ReadResult
	for rows.Next() {
		var (
			host, name, value, path, originAttrs string
			expiry                               int64
			isSecure, isHTTPOnly, sameSite       int
		)
		if err := rows.Scan(&host, &name, &value, &path, &expiry,
			&isSecure, &isHTTPOnly, &sameSite, &originAttrs); err != nil {
			return ReadResult{}, err
		}

		// Only the default cookie jar: skip container (userContextId) and
		// partitioned (partitionKey) cookies, which carry a non-empty
		// originAttributes and are third-party or per-container state, not the
		// normal login cookies.
		if originAttrs != "" {
			continue
		}
		if !domainMatches(host, filters) {
			continue
		}

		cookie := Cookie{
			Name:     name,
			Value:    value,
			Domain:   host,
			Path:     path,
			Secure:   isSecure != 0,
			HTTPOnly: isHTTPOnly != 0,
			SameSite: firefoxSameSite(sameSite),
		}
		// Every persisted Firefox cookie has a real expiry (session cookies are
		// kept in memory, not here). Firefox purges expired rows lazily, so drop
		// any that are already past rather than upload cookies the destination
		// would reject.
		cookie.Expires = firefoxExpiryUnix(expiry)
		if cookie.Expires > 0 && cookie.Expires <= float64(now) {
			continue
		}
		result.Cookies = append(result.Cookies, cookie)
	}
	return result, rows.Err()
}

// firefoxSameSite maps moz_cookies' sameSite integer to the API string form.
// Only the well-known values are mapped; anything else (Firefox has used other
// internal encodings) is omitted so the server applies its default rather than
// receiving a bogus value.
func firefoxSameSite(v int) string {
	switch v {
	case 1:
		return "Lax"
	case 2:
		return "Strict"
	default:
		return ""
	}
}

// firefoxExpiryUnix returns the cookie expiry in Unix seconds. Older Firefox
// stored expiry in seconds and recent versions store milliseconds, so the unit
// is inferred from magnitude: a seconds value is ~1e9, a millisecond value is
// ~1e12 and above.
func firefoxExpiryUnix(expiry int64) float64 {
	if expiry >= 1_000_000_000_000 {
		return float64(expiry) / 1000
	}
	return float64(expiry)
}
