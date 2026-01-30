package auth

import (
	"errors"
	"os"

	"github.com/nottelabs/notte-cli/internal/config"
)

const (
	EnvAPIKey = "NOTTE_API_KEY"
)

// Source indicates where the API key was found
type Source string

const (
	SourceEnv     Source = "environment"
	SourceKeyring Source = "keyring"
	SourceConfig  Source = "config"
)

var ErrNoAPIKey = errors.New("no API key found. Run 'notte auth login' or set NOTTE_API_KEY")

// GetAPIKey resolves API key from env → keyring → config
// configPath can be empty to use default
func GetAPIKey(configPath string) (string, Source, error) {
	// 1. Check environment variable
	if key := os.Getenv(EnvAPIKey); key != "" {
		return key, SourceEnv, nil
	}

	// 2. Check keyring
	if key, err := GetKeyringAPIKey(); err == nil && key != "" {
		return key, SourceKeyring, nil
	}

	// 3. Check config file
	var cfg *config.Config
	var err error
	if configPath != "" {
		cfg, err = config.LoadFromPath(configPath)
	} else {
		cfg, err = config.Load()
	}
	if err == nil && cfg.APIKey != "" {
		return cfg.APIKey, SourceConfig, nil
	}

	return "", "", ErrNoAPIKey
}
