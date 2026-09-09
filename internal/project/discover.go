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

// Entrypoint filenames. The name of the file declares what a directory is,
// which keeps discovery to one rule and lets a connector's two roles live
// beside each other without a second reserved directory name.
const (
	// EntrypointName marks a plain function.
	//
	// Not function.py, which is redundant inside functions/<name>/; not
	// index.py, which is a JavaScript import; not route.py, since a Notte
	// function has exactly one run() and there is no route/handler split.
	EntrypointName = "main.py"
	// LoginEntrypoint and VerifierEntrypoint mark a managed-auth connector.
	// Both must be present: a connector is the pair, and the API deploys them
	// transactionally.
	LoginEntrypoint    = "login.py"
	VerifierEntrypoint = "verifier.py"
)

// Role is which half of a connector an entrypoint is.
type Role string

const (
	RoleLogin    Role = "login"
	RoleVerifier Role = "verifier"
)

// Connector is a managed-auth connector: two functions plus catalog metadata,
// deployed together.
type Connector struct {
	// Name is the directory name, and the catalog slug. Never the path: a
	// grouping directory such as functions/auth/ must not leak into a slug
	// that is globally unique across the catalog.
	Name string
	// Dir is slash-separated and relative to the functions directory.
	Dir string
	// Login and Verifier are entrypoints, in the form the bundler takes.
	Login    string
	Verifier string
}

// Entrypoints returns the connector's two roles in a stable order.
func (c Connector) Entrypoints() []struct {
	Role       Role
	Entrypoint string
} {
	return []struct {
		Role       Role
		Entrypoint string
	}{
		{RoleLogin, c.Login},
		{RoleVerifier, c.Verifier},
	}
}

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Discover finds every function and connector under the functions directory.
//
// The rule is one sentence, and it is depth-independent: any directory whose
// name does not start with an underscore is a function if it contains main.py,
// or a connector if it contains login.py and verifier.py. A bare <name>.py is
// a single-file function.
//
// Depth-independence is what lets grouping be a convention rather than a rule.
// functions/bluesky/ and functions/auth/bluesky/ both work, so a project can
// stay flat while it is small and group when it is not, without the CLI
// reserving a directory name or anyone migrating.
func Discover(cfg *Config) ([]Function, error) {
	functions, _, err := DiscoverAll(cfg)
	return functions, err
}

// DiscoverAll returns functions and connectors together.
func DiscoverAll(cfg *Config) ([]Function, []Connector, error) {
	root := cfg.FunctionsPath()
	var functions []Function
	var connectors []Connector

	if err := walkUnits(root, "", cfg, &functions, &connectors); err != nil {
		return nil, nil, err
	}

	sort.Slice(functions, func(i, j int) bool { return functions[i].Name < functions[j].Name })
	sort.Slice(connectors, func(i, j int) bool { return connectors[i].Name < connectors[j].Name })

	if err := checkDuplicateNames(functions, connectors); err != nil {
		return nil, nil, err
	}
	if err := checkUnknownConfig(cfg, functions, connectors); err != nil {
		return nil, nil, err
	}
	return functions, connectors, nil
}

// walkUnits descends until it finds a unit, then stops. A directory that is a
// function or a connector is not searched further: its subdirectories are its
// own helpers, not more units.
func walkUnits(root, rel string, cfg *Config, functions *[]Function, connectors *[]Connector) error {
	entries, err := os.ReadDir(path.Join(root, rel))
	if err != nil {
		return fmt.Errorf("read %s: %w", path.Join(cfg.Project.FunctionsDir, rel), err)
	}

	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		child := path.Join(rel, name)

		if !e.IsDir() {
			// A bare .py is a single-file function only at the top level.
			// Deeper down it belongs to whatever contains it — a helper inside
			// a unit, or loose Python inside a directory that has not been
			// finished. Reading those as functions would make an unfinished
			// directory look populated and deploy its helpers.
			if rel != "" || !strings.HasSuffix(name, ".py") || IsTestFile(name) {
				continue
			}
			stem := strings.TrimSuffix(name, ".py")
			if err := validateName(stem); err != nil {
				return err
			}
			*functions = append(*functions, Function{Name: stem, Entrypoint: child})
			continue
		}

		switch kind, err := classify(root, child); {
		case err != nil:
			return err
		case kind == unitFunction:
			if err := validateName(name); err != nil {
				return err
			}
			*functions = append(*functions, Function{
				Name: name, Entrypoint: path.Join(child, EntrypointName), Dir: true,
			})
		case kind == unitConnector:
			if err := validateName(name); err != nil {
				return err
			}
			*connectors = append(*connectors, Connector{
				Name:     name,
				Dir:      child,
				Login:    path.Join(child, LoginEntrypoint),
				Verifier: path.Join(child, VerifierEntrypoint),
			})
		default:
			// Not a unit itself, so it may be a grouping directory. Depth
			// independence means such a directory has to be searched rather
			// than rejected — but a grouping directory holds units, not loose
			// Python. One with .py files and nothing beneath it is an
			// unfinished unit, and skipping it silently would deploy nothing
			// and say nothing.
			before := len(*functions) + len(*connectors)
			if err := walkUnits(root, child, cfg, functions, connectors); err != nil {
				return err
			}
			if len(*functions)+len(*connectors) == before && containsPython(root, child) {
				return fmt.Errorf(
					"%s has Python files but no %s, and no %s/%s pair — "+
						"add an entrypoint, or rename it to _%s if it is shared code",
					child, EntrypointName, LoginEntrypoint, VerifierEntrypoint, name)
			}
		}
	}
	return nil
}

// containsPython reports whether a directory holds .py files directly,
// which is what separates an unfinished unit from a grouping directory.
func containsPython(root, rel string) bool {
	entries, err := os.ReadDir(path.Join(root, rel))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".py") && e.Name() != "__init__.py" {
			return true
		}
	}
	return false
}

type unitKind int

const (
	unitNone unitKind = iota
	unitFunction
	unitConnector
)

// classify decides what a directory is from the entrypoints it contains.
//
// Half a connector is an error rather than a grouping directory: someone who
// wrote login.py and not verifier.py has an unfinished connector, and silently
// treating it as a folder deploys neither and says nothing.
func classify(root, rel string) (unitKind, error) {
	has := func(name string) bool {
		_, err := os.Stat(path.Join(root, rel, name))
		return err == nil
	}

	main, login, verifier := has(EntrypointName), has(LoginEntrypoint), has(VerifierEntrypoint)

	switch {
	case main && (login || verifier):
		return unitNone, fmt.Errorf("%s has both %s and a connector entrypoint; it must be one or the other", rel, EntrypointName)
	case main:
		return unitFunction, nil
	case login && verifier:
		return unitConnector, nil
	case login:
		return unitNone, fmt.Errorf("%s has %s but no %s — a connector needs both, and they deploy together",
			rel, LoginEntrypoint, VerifierEntrypoint)
	case verifier:
		return unitNone, fmt.Errorf("%s has %s but no %s — a connector needs both, and they deploy together",
			rel, VerifierEntrypoint, LoginEntrypoint)
	}
	return unitNone, nil
}

// checkDuplicateNames rejects two units claiming one name.
//
// Grouping directories make this reachable: functions/a/report/ and
// functions/b/report/ both deploy as "report", and for connectors the name is
// a catalog slug that is unique across every workspace.
func checkDuplicateNames(functions []Function, connectors []Connector) error {
	seen := map[string]string{}
	claim := func(name, where string) error {
		if prev, dup := seen[name]; dup {
			return fmt.Errorf("%s and %s both deploy as %q; rename one", prev, where, name)
		}
		seen[name] = where
		return nil
	}
	for _, f := range functions {
		if err := claim(f.Name, f.Entrypoint); err != nil {
			return err
		}
	}
	for _, c := range connectors {
		if err := claim(c.Name, c.Dir); err != nil {
			return err
		}
	}
	return nil
}

func validateName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("function name %q must be lowercase alphanumeric with underscores or dashes", name)
	}
	return nil
}

// checkUnknownConfig rejects a [functions.x] or [connectors.x] block with no
// unit x.
//
// Almost always a typo or a rename, and the symptom otherwise is a cron or a
// catalog description that silently never applies.
func checkUnknownConfig(cfg *Config, functions []Function, connectors []Connector) error {
	haveFn := make(map[string]bool, len(functions))
	for _, f := range functions {
		haveFn[f.Name] = true
	}
	haveConn := make(map[string]bool, len(connectors))
	for _, c := range connectors {
		haveConn[c.Name] = true
	}

	var problems []string
	var unknown []string
	for name := range cfg.Functions {
		if !haveFn[name] {
			unknown = append(unknown, name)
		}
	}
	// Sidecars need no such check: one lives inside the unit it configures, so
	// it cannot name a unit that does not exist.
	sort.Strings(unknown)
	if len(unknown) > 0 {
		problems = append(problems, fmt.Sprintf("configures function(s) that do not exist: %s", strings.Join(unknown, ", ")))
	}

	unknown = nil
	for name := range cfg.Connectors {
		if !haveConn[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		problems = append(problems, fmt.Sprintf("configures connector(s) that do not exist: %s", strings.Join(unknown, ", ")))
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s %s", ConfigName, strings.Join(problems, "; "))
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
