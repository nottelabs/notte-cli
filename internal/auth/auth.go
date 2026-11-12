package auth

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/nottelabs/notte-cli/internal/config"
)

const (
	EnvAPIKey = "NOTTE_API_KEY"
	// EnvAPIKeyFile names a file holding the credential. It is read on every
	// invocation, so whatever writes the file can replace the credential (for
	// example a short-lived access token) without restarting the caller.
	EnvAPIKeyFile = "NOTTE_API_KEY_FILE"

	// AccessTokenPrefix starts short-lived access tokens, as opposed to API keys.
	AccessTokenPrefix = "nat_"
)

// Source indicates where the API key was found
type Source string

const (
	SourceFile    Source = "file"
	SourceEnv     Source = "environment"
	SourceKeyring Source = "keyring"
	SourceConfig  Source = "config"
)

var ErrNoAPIKey = errors.New("no API key found. Run 'notte login' or set NOTTE_API_KEY")

// APIKeyFileError means NOTTE_API_KEY_FILE is set but no credential could be
// read from it. No other source is used in that case.
type APIKeyFileError struct {
	Path string
	Err  error // nil when the file is empty
}

func (e *APIKeyFileError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("%s is set to %s, but the file is empty", EnvAPIKeyFile, e.Path)
	}
	return fmt.Sprintf("%s is set to %s, but the file could not be read: %v", EnvAPIKeyFile, e.Path, e.Err)
}

func (e *APIKeyFileError) Unwrap() error { return e.Err }

// APIKeyFile returns the path NOTTE_API_KEY_FILE names, or "" when it is unset.
func APIKeyFile() string {
	return os.Getenv(EnvAPIKeyFile)
}

// IsAccessToken reports whether key is a short-lived access token.
func IsAccessToken(key string) bool {
	return strings.HasPrefix(key, AccessTokenPrefix)
}

// GetAPIKey resolves API key from NOTTE_API_KEY_FILE → env → keyring → config
// configPath can be empty to use default
func GetAPIKey(configPath string) (string, Source, error) {
	// 1. Check the key file. When it is named, it is the only source: falling
	// back could pick up a long-lived key the caller meant to keep out.
	if path := APIKeyFile(); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", "", &APIKeyFileError{Path: path, Err: err}
		}
		key := strings.TrimSpace(string(data))
		if key == "" {
			return "", "", &APIKeyFileError{Path: path}
		}
		return key, SourceFile, nil
	}

	// 2. Check environment variable
	if key := os.Getenv(EnvAPIKey); key != "" {
		return key, SourceEnv, nil
	}

	// 3. Check keyring
	if key, err := GetKeyringAPIKey(); err == nil && key != "" {
		return key, SourceKeyring, nil
	}

	// 4. Check config file
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
