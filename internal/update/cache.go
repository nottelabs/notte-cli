package update

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	// CacheFileName is the name of the update check cache file.
	CacheFileName = "update_check.json"
	// CheckInterval is how often the CLI checks for updates.
	CheckInterval = 24 * time.Hour
)

// UpdateCache holds the cached result of the latest version check.
type UpdateCache struct {
	LatestVersion  string    `json:"latest_version"`
	CurrentVersion string    `json:"current_version"`
	CheckedAt      time.Time `json:"checked_at"`
	ReleaseURL     string    `json:"release_url,omitempty"`
}

// LoadCache reads the cache file from the config directory.
// Returns nil with no error if the file doesn't exist.
func LoadCache(configDir string) (*UpdateCache, error) {
	path := filepath.Join(configDir, CacheFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var cache UpdateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

// SaveCache writes the cache to disk, creating directories as needed.
func SaveCache(configDir string, cache *UpdateCache) error {
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(configDir, CacheFileName), data, 0o600)
}

// IsStale returns true if the cache is older than CheckInterval or if the
// current version has changed since the last check (e.g. user upgraded).
func (c *UpdateCache) IsStale(currentVersion string) bool {
	if time.Since(c.CheckedAt) > CheckInterval {
		return true
	}
	return c.CurrentVersion != currentVersion
}
