package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScenarioDir holds a unit's test payloads, one JSON file per scenario.
//
// Payloads live in files rather than in the config because they are the thing
// most likely to be pasted from somewhere else — a network tab, an API
// response — and those arrive as JSON. Retyping one as a TOML inline table is
// friction that discourages writing the test at all.
//
// The directory is also the opt-in. A unit with scenarios may be executed by a
// gate; one without may not. That property is worth more than a boolean flag:
// a flag can be set across a whole tree with one substitution, and payloads
// cannot.
const ScenarioDir = "tests"

// Scenario is one recorded invocation.
type Scenario struct {
	// Name is the filename without its extension.
	Name string `json:"-"`
	// Path is where it came from, for error messages.
	Path string `json:"-"`
	// Variables are passed to run().
	Variables map[string]any `json:"variables"`
	// ExpectEmpty records whether an empty result is the point.
	//
	// A pointer because the three states differ: unset means "no opinion",
	// false means a result is required, true means emptiness is the expected
	// outcome. "No results is not an error" is a legitimate scenario, so this
	// cannot be a per-function setting.
	ExpectEmpty *bool `json:"expect_empty,omitempty"`
}

// LoadScenarios reads a unit's scenarios, sorted by name.
//
// A unit with no tests directory has none, which is not an error: scenarios
// are opt-in. A single-file function has no directory of its own, so it has
// nowhere to put them — promote it to a directory to add scenarios.
func LoadScenarios(cfg *Config, unitDir string) ([]Scenario, error) {
	dir := filepath.Join(cfg.FunctionsPath(), filepath.FromSlash(unitDir), ScenarioDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Join(unitDir, ScenarioDir), err)
	}

	var out []Scenario
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		s := Scenario{
			Name: strings.TrimSuffix(e.Name(), ".json"),
			Path: filepath.ToSlash(filepath.Join(unitDir, ScenarioDir, e.Name())),
		}
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		// An unknown key is a typo, and a scenario that silently ignores one
		// runs different inputs than its author wrote.
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&s); err != nil {
			return nil, fmt.Errorf("%s: %w", s.Path, err)
		}
		if s.Variables == nil {
			return nil, fmt.Errorf("%s: needs a \"variables\" object, even if empty", s.Path)
		}
		out = append(out, s)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Problems reports scenario variables that run() does not accept.
//
// The same check cron_variables gets, and for the same reason: run()'s
// parameters are already known from validation, so a typo is caught here
// rather than by an invocation that fails against a live site.
func (s Scenario) Problems(params []Param) []string {
	known := make(map[string]bool, len(params))
	for _, p := range params {
		known[p.Name] = true
	}

	var unexpected []string
	for key := range s.Variables {
		if !known[key] {
			unexpected = append(unexpected, key)
		}
	}
	sort.Strings(unexpected)

	var problems []string
	for _, key := range unexpected {
		problems = append(problems, fmt.Sprintf("%s: %q is not a parameter of run(%s)",
			s.Path, key, paramList(params)))
	}
	for _, p := range params {
		if p.HasDefault {
			continue
		}
		if _, ok := s.Variables[p.Name]; !ok {
			problems = append(problems, fmt.Sprintf("%s: run() requires %q and it has no default",
				s.Path, p.Name))
		}
	}
	return problems
}
