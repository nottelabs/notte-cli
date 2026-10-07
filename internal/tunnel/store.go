package tunnel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/config"
)

// FileName holds saved tunnels inside the CLI config directory. OAuth client
// secrets are not written here: they go to the keyring.
const FileName = "tunnels.json"

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// ErrNotFound means no tunnel is saved under the requested name.
var ErrNotFound = errors.New("tunnel not found")

// Tunnel is a saved exit node that sessions can route through.
type Tunnel struct {
	Name          string `json:"name"`
	ExitNode      string `json:"exit_node"`
	Tailnet       string `json:"tailnet,omitempty"`
	OAuthClientID string `json:"oauth_client_id"`
}

// ValidateName checks a tunnel name is usable as a file-safe identifier.
func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid tunnel name %q: use lowercase letters, digits, '.', '_' or '-' (max 63 characters)", name)
	}
	return nil
}

func secretKey(name string) string {
	return "tunnel:" + name + ":oauth_client_secret"
}

func storePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// List returns saved tunnels sorted by name.
func List() ([]Tunnel, error) {
	byName, err := load()
	if err != nil {
		return nil, err
	}
	tunnels := make([]Tunnel, 0, len(byName))
	for _, t := range byName {
		tunnels = append(tunnels, t)
	}
	sort.Slice(tunnels, func(i, j int) bool { return tunnels[i].Name < tunnels[j].Name })
	return tunnels, nil
}

// Get returns the tunnel saved under name.
func Get(name string) (*Tunnel, error) {
	byName, err := load()
	if err != nil {
		return nil, err
	}
	t, ok := byName[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q (see `notte tunnel list`)", ErrNotFound, name)
	}
	return &t, nil
}

// Save stores a tunnel, replacing any tunnel with the same name. An empty
// secret keeps the previously stored one.
func Save(t Tunnel, oauthClientSecret string) error {
	if err := ValidateName(t.Name); err != nil {
		return err
	}
	byName, err := load()
	if err != nil {
		return err
	}
	byName[t.Name] = t
	if oauthClientSecret == "" {
		return write(byName)
	}

	// The record and the secret live in two stores. Swap the secret first and
	// put the old one back if the record cannot be written, so a failed save
	// never pairs one OAuth client's ID with another client's secret.
	previous, getErr := auth.GetKeyringSecret(secretKey(t.Name))
	if err := auth.SetKeyringSecret(secretKey(t.Name), oauthClientSecret); err != nil {
		return fmt.Errorf("failed to store OAuth client secret: %w", err)
	}
	if err := write(byName); err != nil {
		if getErr == nil {
			_ = auth.SetKeyringSecret(secretKey(t.Name), previous)
		} else {
			_ = auth.DeleteKeyringSecret(secretKey(t.Name))
		}
		return err
	}
	return nil
}

// Remove deletes a saved tunnel and its stored secret.
func Remove(name string) error {
	byName, err := load()
	if err != nil {
		return err
	}
	if _, ok := byName[name]; !ok {
		return fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	// Delete the secret before the record so a keyring failure leaves the
	// tunnel listed and the removal retryable, rather than orphaning a secret.
	// If the record then cannot be written, put the secret back so the tunnel
	// that is still listed keeps working.
	previous, getErr := auth.GetKeyringSecret(secretKey(name))
	if err := auth.DeleteKeyringSecret(secretKey(name)); err != nil {
		return fmt.Errorf("failed to delete OAuth client secret for tunnel %q: %w", name, err)
	}
	delete(byName, name)
	if err := write(byName); err != nil {
		if getErr == nil {
			_ = auth.SetKeyringSecret(secretKey(name), previous)
		}
		return err
	}
	return nil
}

// Secret returns the OAuth client secret stored for a tunnel.
func Secret(name string) (string, error) {
	secret, err := auth.GetKeyringSecret(secretKey(name))
	if err != nil || secret == "" {
		return "", fmt.Errorf("no OAuth client secret stored for tunnel %q; run `notte tunnel up --name %s --oauth-client-secret ...` again", name, name)
	}
	return secret, nil
}

func load() (map[string]Tunnel, error) {
	path, err := storePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Tunnel{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var tunnels []Tunnel
	if err := json.Unmarshal(data, &tunnels); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	byName := make(map[string]Tunnel, len(tunnels))
	for _, t := range tunnels {
		byName[t.Name] = t
	}
	return byName, nil
}

func write(byName map[string]Tunnel) error {
	path, err := storePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	tunnels := make([]Tunnel, 0, len(byName))
	for _, t := range byName {
		tunnels = append(tunnels, t)
	}
	sort.Slice(tunnels, func(i, j int) bool { return tunnels[i].Name < tunnels[j].Name })
	data, err := json.MarshalIndent(tunnels, "", "  ")
	if err != nil {
		return err
	}
	// Write a sibling temp file and rename it into place, so an interrupted or
	// failed write never leaves a truncated tunnels.json behind.
	tmp, err := os.CreateTemp(filepath.Dir(path), FileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}
