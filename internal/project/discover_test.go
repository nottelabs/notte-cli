package project

import (
	"strings"
	"testing"
)

func loadAndDiscover(t *testing.T, files map[string]string) ([]Function, error) {
	t.Helper()
	if _, ok := files["notte.toml"]; !ok {
		files["notte.toml"] = "[project]\nname = \"demo\"\n"
	}
	dir := write(t, files)
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return Discover(cfg)
}

func names(fns []Function) []string {
	out := make([]string, len(fns))
	for i, f := range fns {
		out[i] = f.Name
	}
	return out
}

func TestDiscoverDirectoryAndSingleFileForms(t *testing.T) {
	fns, err := loadAndDiscover(t, map[string]string{
		"functions/amazon_search/main.py":  "def run():\n    return 1\n",
		"functions/amazon_search/parse.py": "def clean(s):\n    return s\n",
		"functions/quick_check.py":         "def run():\n    return 1\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(fns); len(got) != 2 || got[0] != "amazon_search" || got[1] != "quick_check" {
		t.Fatalf("got %v", got)
	}
	if !fns[0].Dir || fns[1].Dir {
		t.Fatalf("directory/file forms not distinguished: %+v", fns)
	}
	if fns[0].Entrypoint != "amazon_search/main.py" || fns[1].Entrypoint != "quick_check.py" {
		t.Fatalf("entrypoints: %q, %q", fns[0].Entrypoint, fns[1].Entrypoint)
	}
}

// The underscore prefix is the whole configuration story for shared code.
func TestUnderscoreIsNotAFunction(t *testing.T) {
	fns, err := loadAndDiscover(t, map[string]string{
		"functions/_shared/http.py":     "def fetch():\n    return 1\n",
		"functions/_shared/__init__.py": "",
		"functions/_helper.py":          "x = 1\n",
		"functions/real/main.py":        "def run():\n    return 1\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(fns); len(got) != 1 || got[0] != "real" {
		t.Fatalf("got %v, want [real]", got)
	}
}

// A directory with no main.py is a mistake, and skipping it silently would
// deploy nothing and say nothing.
func TestDirectoryWithoutEntrypointIsAnError(t *testing.T) {
	_, err := loadAndDiscover(t, map[string]string{
		"functions/halfdone/parse.py": "def clean(s):\n    return s\n",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "main.py") || !strings.Contains(err.Error(), "_halfdone") {
		t.Fatalf("error should suggest both fixes, got %v", err)
	}
}

func TestInvalidFunctionNameIsRejected(t *testing.T) {
	_, err := loadAndDiscover(t, map[string]string{
		"functions/Amazon Search/main.py": "def run():\n    return 1\n",
	})
	if err == nil {
		t.Fatal("expected an error for a name with a space and capitals")
	}
}

// A [functions.x] block with no function x is almost always a typo or a
// rename, and the symptom is otherwise a cron that silently never applies.
func TestConfigForMissingFunctionIsRejected(t *testing.T) {
	_, err := loadAndDiscover(t, map[string]string{
		"notte.toml":             "[project]\nname=\"d\"\n\n[functions.typo_name]\ncron = \"cron(0 9 * * ? *)\"\n",
		"functions/real/main.py": "def run():\n    return 1\n",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "typo_name") {
		t.Fatalf("error should name the stale block, got %v", err)
	}
}

func TestDiscoverIsSorted(t *testing.T) {
	fns, err := loadAndDiscover(t, map[string]string{
		"functions/zebra.py": "def run():\n    return 1\n",
		"functions/alpha.py": "def run():\n    return 1\n",
		"functions/mid.py":   "def run():\n    return 1\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(fns); got[0] != "alpha" || got[1] != "mid" || got[2] != "zebra" {
		t.Fatalf("not sorted: %v", got)
	}
}

func TestSelectByNameGlobAndPath(t *testing.T) {
	fns := []Function{
		{Name: "amazon_search", Entrypoint: "amazon_search/main.py", Dir: true},
		{Name: "amazon_deals", Entrypoint: "amazon_deals/main.py", Dir: true},
		{Name: "other", Entrypoint: "other.py"},
	}
	cases := []struct {
		target string
		want   int
	}{
		{"", 3},
		{"all", 3},
		{"other", 1},
		{"amazon_*", 2},
		{"functions/amazon_search", 1},
		{"functions/amazon_search/main.py", 1},
		{"functions/other.py", 1},
	}
	for _, tc := range cases {
		got, err := Select(fns, tc.target)
		if err != nil {
			t.Fatalf("%q: %v", tc.target, err)
		}
		if len(got) != tc.want {
			t.Fatalf("%q selected %d, want %d", tc.target, len(got), tc.want)
		}
	}
}

func TestSelectUnknownListsWhatExists(t *testing.T) {
	_, err := Select([]Function{{Name: "alpha"}}, "nope")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "alpha") {
		t.Fatalf("error should list available functions, got %v", err)
	}
}

func TestIsTestFile(t *testing.T) {
	for _, p := range []string{"fn/test_main.py", "fn/main_test.py"} {
		if !IsTestFile(p) {
			t.Errorf("%q should be a test file", p)
		}
	}
	for _, p := range []string{"fn/main.py", "fn/latest.py", "fn/contest.py"} {
		if IsTestFile(p) {
			t.Errorf("%q should not be a test file", p)
		}
	}
}
