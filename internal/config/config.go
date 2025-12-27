package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	DefaultAPIURL  = "https://api.notte.cc"
	ConfigDirName  = "notte"
	ConfigFileName = "config.json"
)

// Config holds CLI configuration
type Config struct {
	APIKey string `json:"api_key,omitempty"`
	APIURL string `json:"api_url,omitempty"`
}

// DefaultConfigPath returns ~/.config/notte/config.json
func DefaultConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, ConfigDirName, ConfigFileName), nil
}

// Load loads config from default path
func Load() (*Config, error) {
	path, err := DefaultConfigPath()
	if err != nil {
		return nil, err
	}
	return LoadFromPath(path)
}

// LoadFromPath loads config from specific path
func LoadFromPath(path string) (*Config, error) {
	cfg := &Config{
		APIURL: DefaultAPIURL,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil // Return defaults if no config
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Ensure default URL if not set
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPIURL
	}

	return cfg, nil
}

// Save saves config to default path
func (c *Config) Save() error {
	path, err := DefaultConfigPath()
	if err != nil {
		return err
	}
	return c.SaveToPath(path)
}

// SaveToPath saves config to specific path
func (c *Config) SaveToPath(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}
