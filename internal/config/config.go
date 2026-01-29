package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	DefaultAPIURL      = "https://api.notte.cc"
	DefaultConsoleURL  = "https://console.notte.cc"
	ConfigDirName      = "notte"
	ConfigFileName     = "config.json"
	CurrentSessionFile = "current_session"
	EnvAPIURL          = "NOTTE_API_URL"
	EnvConsoleURL      = "NOTTE_CONSOLE_URL"
	EnvSessionID       = "NOTTE_SESSION_ID"
)

// testConfigDir allows overriding the config directory for testing.
// If empty, the default os.UserConfigDir() path is used.
var testConfigDir string

// SetTestConfigDir sets a custom config directory for testing.
// Pass empty string to restore default behavior.
func SetTestConfigDir(dir string) {
	testConfigDir = dir
}

// Config holds CLI configuration
type Config struct {
	APIKey string `json:"api_key,omitempty"`
	APIURL string `json:"api_url,omitempty"`
}

// Dir returns the notte config directory path (~/.config/notte or ~/Library/Application Support/notte on macOS)
func Dir() (string, error) {
	if testConfigDir != "" {
		return filepath.Join(testConfigDir, ConfigDirName), nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, ConfigDirName), nil
}

// DefaultConfigPath returns ~/.config/notte/config.json
func DefaultConfigPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
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

// GetConsoleURL returns the console URL from env var or default
func GetConsoleURL() string {
	if url := os.Getenv(EnvConsoleURL); url != "" {
		return url
	}
	return DefaultConsoleURL
}
