package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Sidecar filenames. A unit may carry its configuration beside its code
// instead of in the project file, which is what makes a tree of thousands
// workable: adding one touches a single directory rather than appending to a
// file everyone else is also editing.
//
// Deliberately not named notte.toml. Find walks up looking for that name, so a
// sidecar sharing it would make any command run from inside a function treat
// that function's directory as the project root.
const (
	// FunctionSidecar sits inside a directory function, beside main.py.
	FunctionSidecar = "function.toml"
	// ConnectorSidecar sits inside a connector, beside login.py.
	ConnectorSidecar = "connector.toml"
)

// SingleFileSidecar is the sidecar for functions/<name>.py, which has no
// directory to put one in. Matches the convention marketplace already uses,
// where list_event_exhibitors.toml sits beside list_event_exhibitors.py.
func SingleFileSidecar(entrypoint string) string {
	return strings.TrimSuffix(entrypoint, ".py") + ".toml"
}

// FunctionConfigFor resolves one function's configuration.
//
// Central [functions.<name>] and a sidecar are both supported, and having both
// is an error rather than a precedence rule. Picking a winner would mean the
// loser's edits silently do nothing, which is the failure this design refuses
// everywhere else — an unknown key, a stale [functions.x] block, a cron
// variable that names nothing.
func (c *Config) FunctionConfigFor(fn Function) (FunctionConfig, error) {
	central, hasCentral := c.Functions[fn.Name]

	path := c.sidecarPath(fn)
	sidecar, hasSidecar, err := loadSidecar[FunctionConfig](path)
	if err != nil {
		return FunctionConfig{}, err
	}

	if hasCentral && hasSidecar {
		return FunctionConfig{}, fmt.Errorf(
			"%s is configured twice: [functions.%s] in %s and %s.\n"+
				"  keep one — whichever loses would silently stop applying",
			fn.Name, fn.Name, ConfigName, c.relative(path))
	}
	if hasSidecar {
		return sidecar, nil
	}
	return central, nil
}

// ConnectorConfigFor resolves one connector's configuration.
func (c *Config) ConnectorConfigFor(conn Connector) (ConnectorConfig, error) {
	central, hasCentral := c.Connectors[conn.Name]

	path := filepath.Join(c.FunctionsPath(), filepath.FromSlash(conn.Dir), ConnectorSidecar)
	sidecar, hasSidecar, err := loadSidecar[ConnectorConfig](path)
	if err != nil {
		return ConnectorConfig{}, err
	}

	if hasCentral && hasSidecar {
		return ConnectorConfig{}, fmt.Errorf(
			"%s is configured twice: [connectors.%s] in %s and %s.\n"+
				"  keep one — whichever loses would silently stop applying",
			conn.Name, conn.Name, ConfigName, c.relative(path))
	}
	if hasSidecar {
		return sidecar, nil
	}
	return central, nil
}

// sidecarPath is where a function's sidecar would live.
func (c *Config) sidecarPath(fn Function) string {
	root := c.FunctionsPath()
	if fn.Dir {
		return filepath.Join(root, filepath.FromSlash(filepath.Dir(fn.Entrypoint)), FunctionSidecar)
	}
	return filepath.Join(root, filepath.FromSlash(SingleFileSidecar(fn.Entrypoint)))
}

// relative renders a path for an error message, relative to the project root.
func (c *Config) relative(path string) string {
	if rel, err := filepath.Rel(c.Root, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}

// loadSidecar decodes a sidecar, reporting whether one exists.
//
// An unknown key is an error here for the same reason it is in the project
// file: TOML's failure mode for a misspelling is silence, and a setting that
// quietly does nothing is worse than one that refuses.
func loadSidecar[T any](path string) (T, bool, error) {
	var out T
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return out, false, nil
	}
	if err != nil {
		return out, false, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}

	md, err := toml.Decode(string(raw), &out)
	if err != nil {
		return out, false, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, 0, len(undecoded))
		for _, k := range undecoded {
			keys = append(keys, k.String())
		}
		return out, false, fmt.Errorf("%s: unknown key(s): %s", filepath.Base(path), strings.Join(keys, ", "))
	}
	return out, true, nil
}
