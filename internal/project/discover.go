package project

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Function is one deployable unit found on disk.
type Function struct {
	// Name is the directory or file stem, and the identity used in
	// notte.toml. It never contains a function id: ids are per-environment,
	// so one in a filename would tie the tree to a single environment.
	Name string
	// Entrypoint is slash-separated and relative to the functions directory,
	// which is what the bundler takes.
	Entrypoint string
	// Dir reports the directory form, which can carry helpers and tests.
	Dir bool
}

// EntrypointName is the file a function directory must contain.
//
// Not function.py, which is redundant inside functions/<name>/; not index.py,
// which is a JavaScript import; not route.py, since a Notte function has
// exactly one run() and there is no route/handler split to encode.
const EntrypointName = "main.py"

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Discover finds every function under the functions directory.
//
// The rule is one sentence: anything directly under it whose name does not
// start with an underscore is a function — either <name>/main.py or <name>.py.
// The underscore prefix is Supabase's _shared convention, and it doubles as
// the marker for "library, not a unit", so shared code needs no configuration
// to be excluded.
func Discover(cfg *Config) ([]Function, error) {
	root := cfg.FunctionsPath()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", cfg.Project.FunctionsDir, err)
	}

	var out []Function
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}

		switch {
		case e.IsDir():
			if _, err := os.Stat(path.Join(root, name, EntrypointName)); err != nil {
				// A directory without an entrypoint is a package the author
				// has not finished, or a helper they forgot to underscore.
				// Skipping silently would deploy neither and say nothing.
				return nil, fmt.Errorf("%s/%s has no %s — add one, or rename it to _%s if it is shared code",
					cfg.Project.FunctionsDir, name, EntrypointName, name)
			}
			if err := validateName(name); err != nil {
				return nil, err
			}
			out = append(out, Function{Name: name, Entrypoint: path.Join(name, EntrypointName), Dir: true})

		case strings.HasSuffix(name, ".py"):
			stem := strings.TrimSuffix(name, ".py")
			if err := validateName(stem); err != nil {
				return nil, err
			}
			out = append(out, Function{Name: stem, Entrypoint: name})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if err := checkUnknownConfig(cfg, out); err != nil {
		return nil, err
	}
	return out, nil
}

func validateName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("function name %q must be lowercase alphanumeric with underscores or dashes", name)
	}
	return nil
}

// checkUnknownConfig rejects a [functions.x] block with no function x.
//
// Almost always a typo or a rename, and the symptom otherwise is a cron or a
// description that silently never applies.
func checkUnknownConfig(cfg *Config, found []Function) error {
	have := make(map[string]bool, len(found))
	for _, f := range found {
		have[f.Name] = true
	}
	var unknown []string
	for name := range cfg.Functions {
		if !have[name] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("%s configures function(s) that do not exist: %s", ConfigName, strings.Join(unknown, ", "))
}

// IsTestFile reports whether a path is a test, which is never bundled.
func IsTestFile(p string) bool {
	base := path.Base(p)
	return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")
}

// Select filters functions by a target: a name, a glob, a path, or "all".
func Select(functions []Function, target string) ([]Function, error) {
	if target == "" || target == "all" {
		return functions, nil
	}
	// A path is accepted so shell completion on the tree works.
	target = strings.TrimSuffix(strings.TrimSuffix(strings.Trim(target, "/"), "/"+EntrypointName), ".py")
	if i := strings.LastIndex(target, "/"); i >= 0 {
		target = target[i+1:]
	}

	var out []Function
	for _, f := range functions {
		ok, err := path.Match(target, f.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", target, err)
		}
		if ok {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		names := make([]string, 0, len(functions))
		for _, f := range functions {
			names = append(names, f.Name)
		}
		return nil, fmt.Errorf("no function matches %q (have: %s)", target, strings.Join(names, ", "))
	}
	return out, nil
}

// Sources lists every .py file the functions directory contributes, excluding
// tests. Used to report what a shared-module edit would touch.
func Sources(fsys fs.FS) ([]string, error) {
	var out []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".py") || IsTestFile(p) {
			return nil
		}
		out = append(out, p)
		return nil
	})
	sort.Strings(out)
	return out, err
}
