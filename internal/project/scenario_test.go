package project

import (
	"strings"
	"testing"
)

func TestScenariosLoadFromTheTestsDirectory(t *testing.T) {
	cfg := loadCfg(t, map[string]string{
		"functions/amazon/main.py":               "def run(query: str = \"x\"):\n    return 1\n",
		"functions/amazon/tests/search.json":     `{"variables": {"query": "laptop"}}`,
		"functions/amazon/tests/no-results.json": `{"variables": {"query": "zzz"}, "expect_empty": true}`,
	})
	got, err := LoadScenarios(cfg, "amazon")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenarios", len(got))
	}
	// Sorted by name, and the filename is the name.
	if got[0].Name != "no-results" || got[1].Name != "search" {
		t.Fatalf("names = %q %q", got[0].Name, got[1].Name)
	}
	if got[1].Variables["query"] != "laptop" {
		t.Fatalf("variables = %v", got[1].Variables)
	}
	// "No results is not an error" is a legitimate scenario, so the
	// expectation is per-scenario rather than per-function.
	if got[0].ExpectEmpty == nil || !*got[0].ExpectEmpty {
		t.Fatalf("expect_empty = %v", got[0].ExpectEmpty)
	}
	if got[1].ExpectEmpty != nil {
		t.Fatal("an unset expect_empty must stay nil, not default to false")
	}
}

// Scenarios are opt-in: no directory, no execution consent.
func TestNoTestsDirectoryIsNotAnError(t *testing.T) {
	cfg := loadCfg(t, map[string]string{
		"functions/amazon/main.py": "def run():\n    return 1\n",
	})
	got, err := LoadScenarios(cfg, "amazon")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

// A payload naming a parameter run() does not take would fail as a live
// invocation. run()'s parameters are already known, so it is caught for free.
func TestScenarioVariablesAreCheckedAgainstRun(t *testing.T) {
	s := Scenario{Path: "amazon/tests/search.json", Variables: map[string]any{"queyr": "laptop"}}
	problems := s.Problems([]Param{{Name: "query", HasDefault: true}})
	if len(problems) != 1 {
		t.Fatalf("got %v", problems)
	}
	if !strings.Contains(problems[0], "queyr") || !strings.Contains(problems[0], "query") {
		t.Fatalf("error should name both the typo and the real parameter: %v", problems[0])
	}
}

func TestScenarioMustSupplyParametersWithoutDefaults(t *testing.T) {
	s := Scenario{Path: "amazon/tests/search.json", Variables: map[string]any{}}
	if problems := s.Problems([]Param{{Name: "url"}}); len(problems) != 1 {
		t.Fatalf("got %v", problems)
	}
	s.Variables["url"] = "https://x.test"
	if problems := s.Problems([]Param{{Name: "url"}}); len(problems) != 0 {
		t.Fatalf("supplying it should satisfy the check: %v", problems)
	}
}

// A scenario that silently ignores a misspelled key runs different inputs than
// its author wrote.
func TestUnknownKeyInAScenarioIsRejected(t *testing.T) {
	cfg := loadCfg(t, map[string]string{
		"functions/amazon/main.py":           "def run():\n    return 1\n",
		"functions/amazon/tests/search.json": `{"variables": {}, "expects_empty": true}`,
	})
	_, err := LoadScenarios(cfg, "amazon")
	if err == nil || !strings.Contains(err.Error(), "expects_empty") {
		t.Fatalf("expected the key to be named, got %v", err)
	}
}

func TestScenarioNeedsAVariablesObject(t *testing.T) {
	cfg := loadCfg(t, map[string]string{
		"functions/amazon/main.py":           "def run():\n    return 1\n",
		"functions/amazon/tests/search.json": `{"expect_empty": true}`,
	})
	_, err := LoadScenarios(cfg, "amazon")
	if err == nil || !strings.Contains(err.Error(), "variables") {
		t.Fatalf("got %v", err)
	}
}

// Complex payloads are the reason these live in JSON rather than in TOML.
func TestNestedPayloadsSurvive(t *testing.T) {
	cfg := loadCfg(t, map[string]string{
		"functions/amazon/main.py": "def run(filters: dict = {}):\n    return 1\n",
		"functions/amazon/tests/nested.json": `{"variables": {"filters": {"price": {"max": 500},
		  "tags": ["a", "b"]}}}`,
	})
	got, err := LoadScenarios(cfg, "amazon")
	if err != nil {
		t.Fatal(err)
	}
	filters, ok := got[0].Variables["filters"].(map[string]any)
	if !ok {
		t.Fatalf("nested object lost: %#v", got[0].Variables)
	}
	if price, ok := filters["price"].(map[string]any); !ok || price["max"] != float64(500) {
		t.Fatalf("nesting flattened: %#v", filters)
	}
}
