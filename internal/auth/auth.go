package auth

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/nottelabs/notte-cli/internal/config"
)

const (
	EnvAPIKey = "NOTTE_API_KEY"
	// EnvAPIKeyFile names a file holding the credential. It is read on every
	// invocation, so whatever writes the file can replace the credential (for
	// example a short-lived access token) without restarting the caller.
	EnvAPIKeyFile = "NOTTE_API_KEY_FILE"

	// MaxAPIKeyFileSize caps how much of NOTTE_API_KEY_FILE is read. A
	// credential is far smaller; a larger file is the wrong file.
	MaxAPIKeyFileSize = 64 << 10
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
	Path    string
	Err     error // nil when the file is empty or too large
	TooLong bool  // the file is larger than MaxAPIKeyFileSize
}

func (e *APIKeyFileError) Error() string {
	if e.TooLong {
		return fmt.Sprintf("%s is set to %s, but the file is larger than %d KiB, too large to hold a credential",
			EnvAPIKeyFile, e.Path, MaxAPIKeyFileSize>>10)
	}
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

// warnings receives the notes GetAPIKey prints; each is printed once per
// process, since a command may resolve the key more than once.
var (
	warnings   io.Writer = os.Stderr
	warnedMu   sync.Mutex
	warnedOnce = map[string]bool{}
)

// SetWarningOutput sends the notes GetAPIKey prints to w and forgets which
// were printed. It returns a function that restores the previous writer.
// For tests.
func SetWarningOutput(w io.Writer) func() {
	warnedMu.Lock()
	defer warnedMu.Unlock()
	prev := warnings
	warnings = w
	warnedOnce = map[string]bool{}
	return func() {
		warnedMu.Lock()
		defer warnedMu.Unlock()
		warnings = prev
		warnedOnce = map[string]bool{}
	}
}

func warn(msg string) {
	warnedMu.Lock()
	defer warnedMu.Unlock()
	if warnedOnce[msg] {
		return
	}
	warnedOnce[msg] = true
	_, _ = fmt.Fprintln(warnings, msg)
}

// readKeyFile reads the credential from path, at most MaxAPIKeyFileSize
// bytes of it. Its permissions are not checked: Kubernetes and Docker mount
// secrets readable by others (0644, 0444) by default, so a warning would be
// printed on every command in the setups this file is meant for.
func readKeyFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", &APIKeyFileError{Path: path, Err: err}
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, MaxAPIKeyFileSize+1))
	if err != nil {
		return "", &APIKeyFileError{Path: path, Err: err}
	}
	if len(data) > MaxAPIKeyFileSize {
		return "", &APIKeyFileError{Path: path, TooLong: true}
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", &APIKeyFileError{Path: path}
	}
	return key, nil
}

// GetAPIKey resolves API key from NOTTE_API_KEY_FILE → env → keyring → config
// configPath can be empty to use default
func GetAPIKey(configPath string) (string, Source, error) {
	// 1. Check the key file. When it is named, it is the only source: falling
	// back could pick up a long-lived key the caller meant to keep out.
	if path := APIKeyFile(); path != "" {
		if os.Getenv(EnvAPIKey) != "" {
			warn(fmt.Sprintf("note: %s is ignored because %s is set", EnvAPIKey, EnvAPIKeyFile))
		}
		key, err := readKeyFile(path)
		if err != nil {
			return "", "", err
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
