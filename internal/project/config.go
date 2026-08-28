// Package project reads a notte stack: the notte.toml a human writes, the
// notte.lock.json the CLI maintains, and the functions on disk between them.
//
// The split matters. notte.toml is hand-owned and never rewritten by the CLI,
// because every Go TOML library either drops comments on write or exposes them
// read-only — and the comments are where the reasoning lives. Everything the
// CLI needs to remember goes in the lock, which is JSON and machine-owned.
// Mixing the two is what left marketplace/manifest.json at 2.2 MB and dirty
// after every sync.
package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// ConfigName is the hand-written project file.
	ConfigName = "notte.toml"
	// LockName is the machine-written companion.
	LockName = "notte.lock.json"
	// StateDir holds build output, the venv and caches. Gitignored.
	StateDir = ".notte"
	// DefaultFunctionsDir is where functions live unless notte.toml says
	// otherwise. Deliberately not "notte", which would shadow the real notte
	// package the moment the repo root lands on sys.path — which pytest does.
	DefaultFunctionsDir = "functions"
	// DefaultEnv is the environment every command uses when none is named.
	// Almost every project has exactly this one.
	DefaultEnv = "prod"
)

// Config is notte.toml.
type Config struct {
	Project   ProjectSection            `toml:"project"`
	Envs      map[string]EnvConfig      `toml:"env"`
	Functions map[string]FunctionConfig `toml:"functions"`

	// Root is the directory containing notte.toml. Not from the file.
	Root string `toml:"-"`
}

type ProjectSection struct {
	Name         string `toml:"name"`
	FunctionsDir string `toml:"functions_dir"`
}

// EnvConfig is one entry under [env.*]. A project with a single environment
// has none of these at all; prod is implied.
type EnvConfig struct {
	APIURL  string            `toml:"api_url"`
	APIKey  string            `toml:"api_key"`
	Extends string            `toml:"extends"`
	Headers map[string]string `toml:"headers"`
}

// FunctionConfig is one entry under [functions.*], keyed by function name.
type FunctionConfig struct {
	Name        string   `toml:"name"`
	Description string   `toml:"description"`
	Shared      bool     `toml:"shared"`
	Cron        string   `toml:"cron"`
	Secrets     []string `toml:"secrets"`
}

// Load reads notte.toml from dir.
func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, ConfigName)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ConfigName, err)
	}

	var cfg Config
	md, err := toml.Decode(string(raw), &cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ConfigName, err)
	}
	// TOML's failure mode for a misspelled key is silence, so an unknown key is
	// an error rather than a setting that quietly does nothing.
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, 0, len(undecoded))
		for _, k := range undecoded {
			keys = append(keys, k.String())
		}
		return nil, fmt.Errorf("%s: unknown key(s): %s", ConfigName, strings.Join(keys, ", "))
	}

	cfg.Root = dir
	if cfg.Project.FunctionsDir == "" {
		cfg.Project.FunctionsDir = DefaultFunctionsDir
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", ConfigName, err)
	}
	return &cfg, nil
}

// Find walks up from dir looking for notte.toml, the way git finds .git. Any
// command can then run from a subdirectory of the stack.
func Find(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, ConfigName)); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no %s found in %s or any parent directory", ConfigName, dir)
		}
		abs = parent
	}
}

var envNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func (c *Config) validate() error {
	if strings.HasPrefix(c.Project.FunctionsDir, "/") || strings.Contains(c.Project.FunctionsDir, "..") {
		return fmt.Errorf("functions_dir must be a relative path inside the project, got %q", c.Project.FunctionsDir)
	}
	for name, env := range c.Envs {
		if !envNameRe.MatchString(name) {
			return fmt.Errorf("env name %q must be lowercase alphanumeric with dashes", name)
		}
		if env.Extends != "" {
			if _, ok := c.Envs[env.Extends]; !ok {
				return fmt.Errorf("env %q extends %q, which is not defined", name, env.Extends)
			}
			if env.Extends == name {
				return fmt.Errorf("env %q extends itself", name)
			}
		}
	}
	// A cycle would otherwise be an infinite loop at resolution time.
	for name := range c.Envs {
		seen := map[string]bool{name: true}
		for cur := c.Envs[name].Extends; cur != ""; cur = c.Envs[cur].Extends {
			if seen[cur] {
				return fmt.Errorf("env %q has a circular extends chain", name)
			}
			seen[cur] = true
		}
	}
	return nil
}

// ResolveEnv flattens an environment, following extends, and expands
// interpolations. An environment that is not declared is not an error when it
// is the default: a single-environment project has no [env.*] block at all.
func (c *Config) ResolveEnv(name string) (EnvConfig, error) {
	if name == "" {
		name = DefaultEnv
	}
	env, declared := c.Envs[name]
	if !declared {
		if name != DefaultEnv {
			return EnvConfig{}, fmt.Errorf("env %q is not defined in %s", name, ConfigName)
		}
		env = EnvConfig{}
	}

	// Inherit from the base, nearest wins.
	for base := env.Extends; base != ""; {
		parent := c.Envs[base]
		if env.APIURL == "" {
			env.APIURL = parent.APIURL
		}
		if env.APIKey == "" {
			env.APIKey = parent.APIKey
		}
		merged := map[string]string{}
		for k, v := range parent.Headers {
			merged[k] = v
		}
		for k, v := range env.Headers {
			merged[k] = v
		}
		env.Headers = merged
		base = parent.Extends
	}

	var err error
	if env.APIURL, err = expand(env.APIURL); err != nil {
		return EnvConfig{}, fmt.Errorf("env %q api_url: %w", name, err)
	}
	if env.APIKey, err = expand(env.APIKey); err != nil {
		return EnvConfig{}, fmt.Errorf("env %q api_key: %w", name, err)
	}
	for k, v := range env.Headers {
		if env.Headers[k], err = expand(v); err != nil {
			return EnvConfig{}, fmt.Errorf("env %q header %q: %w", name, k, err)
		}
	}
	return env, nil
}

var interpolationRe = regexp.MustCompile(`\$\{([a-z]+):([^}]+)\}`)

// expand resolves ${env:VAR} and ${git:branch} style references.
//
// An unresolved reference is an error rather than an empty string. managed-auth
// records why: a header silently ignored meant "a silent wrong write", and
// expanding to "" produces exactly that class of failure — an api_url of "" or
// a credential of "" that fails somewhere far from the cause.
func expand(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	var firstErr error
	out := interpolationRe.ReplaceAllStringFunc(s, func(match string) string {
		parts := interpolationRe.FindStringSubmatch(match)
		ns, key := parts[1], parts[2]
		value, err := lookup(ns, key)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		return value
	})
	return out, firstErr
}

func lookup(ns, key string) (string, error) {
	switch ns {
	case "env":
		v, ok := os.LookupEnv(key)
		if !ok {
			return "", fmt.Errorf("environment variable %s is not set", key)
		}
		return v, nil
	case "git":
		return gitValue(key)
	default:
		return "", fmt.Errorf("unknown interpolation namespace %q (want env or git)", ns)
	}
}

func gitValue(key string) (string, error) {
	var args []string
	switch key {
	case "branch":
		args = []string{"rev-parse", "--abbrev-ref", "HEAD"}
	case "sha":
		args = []string{"rev-parse", "HEAD"}
	case "short_sha":
		args = []string{"rev-parse", "--short", "HEAD"}
	default:
		return "", fmt.Errorf("unknown git value %q (want branch, sha or short_sha)", key)
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	value := strings.TrimSpace(string(out))
	if value == "" || value == "HEAD" {
		return "", fmt.Errorf("git %s returned no usable value (detached HEAD?)", key)
	}
	return value, nil
}

// FunctionsPath is the absolute path to the functions package.
func (c *Config) FunctionsPath() string {
	return filepath.Join(c.Root, c.Project.FunctionsDir)
}

// StatePath is a path inside the gitignored .notte directory.
func (c *Config) StatePath(parts ...string) string {
	return filepath.Join(append([]string{c.Root, StateDir}, parts...)...)
}
