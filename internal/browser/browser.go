// Package browser reads cookies out of a local Chromium-based browser profile
// so they can be uploaded into a Notte cloud profile.
//
// Everything here runs against a copy of the browser's own files on the same
// machine, for the user who owns them. It never talks to the network; the
// caller is responsible for deciding which cookies leave the machine.
package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// browserKind distinguishes the two families we support, which discover
// profiles and protect cookies in completely different ways.
type browserKind int

const (
	chromiumKind browserKind = iota // zero value, so existing entries stay Chromium
	firefoxKind
)

// Browser describes a supported browser and where its data (and, for Chromium,
// its encryption key) live on the current OS.
type Browser struct {
	ID          string // stable identifier used on the command line, e.g. "chrome"
	DisplayName string // human name, e.g. "Google Chrome"

	kind browserKind // chromiumKind (default) or firefoxKind

	// macDir / linuxDir are the browser's user-data directory, relative to the
	// per-OS application-support root. On macOS/Linux the profiles sit directly
	// inside this directory (there is no "User Data" subfolder like on Windows).
	macDir   string
	linuxDir string

	// safeStorageLabel is the macOS Keychain service that holds the AES key,
	// by convention "<Product> Safe Storage".
	safeStorageLabel string

	// linuxKeyringApp is the value of the "application" attribute Chrome sets on
	// its libsecret item; used to find the key in the Secret Service.
	linuxKeyringApp string
}

// SupportedBrowsers lists the browsers we know how to read, most common first.
var SupportedBrowsers = []Browser{
	{ID: "chrome", DisplayName: "Google Chrome", macDir: "Google/Chrome", linuxDir: "google-chrome", safeStorageLabel: "Chrome Safe Storage", linuxKeyringApp: "chrome"},
	{ID: "brave", DisplayName: "Brave", macDir: "BraveSoftware/Brave-Browser", linuxDir: "BraveSoftware/Brave-Browser", safeStorageLabel: "Brave Safe Storage", linuxKeyringApp: "brave"},
	{ID: "edge", DisplayName: "Microsoft Edge", macDir: "Microsoft Edge", linuxDir: "microsoft-edge", safeStorageLabel: "Microsoft Edge Safe Storage", linuxKeyringApp: "microsoft-edge"},
	{ID: "chromium", DisplayName: "Chromium", macDir: "Chromium", linuxDir: "chromium", safeStorageLabel: "Chromium Safe Storage", linuxKeyringApp: "chromium"},
	{ID: "firefox", DisplayName: "Firefox", kind: firefoxKind, macDir: "Firefox", linuxDir: "firefox"},
}

// SupportedPlatform reports whether reading local browser cookies is available
// on the current OS. Only macOS and Linux are supported today.
func SupportedPlatform() bool {
	return runtime.GOOS == "darwin" || runtime.GOOS == "linux"
}

// BrowserByID returns the supported browser with the given id.
func BrowserByID(id string) (Browser, bool) {
	for _, b := range SupportedBrowsers {
		if b.ID == id {
			return b, true
		}
	}
	return Browser{}, false
}

// userDataDir returns the absolute path to the browser's user-data directory,
// honouring --user-data-dir-style overrides where the platform supports them.
func (b Browser) userDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", filepath.FromSlash(b.macDir)), nil
	case "linux":
		// Firefox lives under ~/.mozilla and does not honour XDG_CONFIG_HOME. It
		// may also be packaged as a snap or flatpak, each with its own root.
		if b.kind == firefoxKind {
			return firefoxLinuxRoot(home, b.linuxDir), nil
		}
		root := os.Getenv("XDG_CONFIG_HOME")
		if root == "" {
			root = filepath.Join(home, ".config")
		}
		return filepath.Join(root, filepath.FromSlash(b.linuxDir)), nil
	default:
		return "", fmt.Errorf("browser %s is not supported on %s", b.DisplayName, runtime.GOOS)
	}
}

// Installed reports whether this browser's user-data directory exists.
func (b Browser) Installed() bool {
	dir, err := b.userDataDir()
	if err != nil {
		return false
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// Profile is one browser profile (a "person" in Chrome's UI).
type Profile struct {
	Browser Browser
	Dir     string // on-disk directory name, e.g. "Default" or "Profile 1"
	Name    string // display name, e.g. "Work"
	Email   string // signed-in account email, empty if not signed in
	path    string // absolute path to the profile directory
}

// localState is the subset of the browser's Local State file we read.
type localState struct {
	Profile struct {
		InfoCache map[string]struct {
			Name     string `json:"name"`
			UserName string `json:"user_name"`
		} `json:"info_cache"`
		ProfilesOrder []string `json:"profiles_order"`
	} `json:"profile"`
}

// Profiles lists the profiles inside a browser, in the order the browser shows
// them. Profiles whose directory is missing on disk are skipped.
func (b Browser) Profiles() ([]Profile, error) {
	if b.kind == firefoxKind {
		return b.firefoxProfiles()
	}
	dir, err := b.userDataDir()
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(filepath.Join(dir, "Local State"))
	if err != nil {
		return nil, fmt.Errorf("could not read %s profile list: %w", b.DisplayName, err)
	}

	var state localState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("could not parse %s profile list: %w", b.DisplayName, err)
	}

	order := state.Profile.ProfilesOrder
	if len(order) == 0 {
		for name := range state.Profile.InfoCache {
			order = append(order, name)
		}
		sort.Strings(order)
	}

	var profiles []Profile
	seen := make(map[string]bool)
	for _, profileDir := range order {
		if seen[profileDir] {
			continue
		}
		seen[profileDir] = true

		info, ok := state.Profile.InfoCache[profileDir]
		if !ok {
			continue
		}
		path := filepath.Join(dir, profileDir)
		if st, err := os.Stat(path); err != nil || !st.IsDir() {
			continue
		}

		name := info.Name
		if name == "" {
			name = profileDir
		}
		profiles = append(profiles, Profile{
			Browser: b,
			Dir:     profileDir,
			Name:    name,
			Email:   info.UserName,
			path:    path,
		})
	}
	return profiles, nil
}

// InstalledProfiles returns every profile across every installed supported
// browser, so a caller can present one flat list to choose from.
func InstalledProfiles() []Profile {
	var all []Profile
	for _, b := range SupportedBrowsers {
		if !b.Installed() {
			continue
		}
		profiles, err := b.Profiles()
		if err != nil {
			continue
		}
		all = append(all, profiles...)
	}
	return all
}
