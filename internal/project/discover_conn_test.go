package project

import (
	"strings"
	"testing"
)

func discoverAll(t *testing.T, files map[string]string) ([]Function, []Connector, error) {
	t.Helper()
	if _, ok := files["notte.toml"]; !ok {
		files["notte.toml"] = "[project]\nname = \"demo\"\n"
	}
	dir := write(t, files)
	cfg, err := Load(dir)
	if err != nil {
		return nil, nil, err
	}
	return DiscoverAll(cfg)
}

// A directory with login.py and verifier.py is a connector. The filename
// declares the kind, so the two roles sit beside each other without needing a
// second reserved directory name.
func TestConnectorIsADirectoryWithBothRoles(t *testing.T) {
	fns, conns, err := discoverAll(t, map[string]string{
		"functions/bluesky/login.py":    "def run(session_id: str):\n    return 1\n",
		"functions/bluesky/verifier.py": "def run(session_id: str):\n    return 1\n",
		"functions/bluesky/helpers.py":  "def h():\n    return 1\n",
		"functions/scraper/main.py":     "def run():\n    return 1\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fns) != 1 || fns[0].Name != "scraper" {
		t.Fatalf("functions = %+v", fns)
	}
	if len(conns) != 1 || conns[0].Name != "bluesky" {
		t.Fatalf("connectors = %+v", conns)
	}
	if conns[0].Login != "bluesky/login.py" || conns[0].Verifier != "bluesky/verifier.py" {
		t.Fatalf("entrypoints = %q %q", conns[0].Login, conns[0].Verifier)
	}
}

// Half a connector is an unfinished connector, not a folder. Treating it as a
// grouping directory would deploy neither role and say nothing.
func TestHalfAConnectorIsAnError(t *testing.T) {
	for _, missing := range []struct{ have, want string }{
		{"login.py", "verifier.py"},
		{"verifier.py", "login.py"},
	} {
		_, _, err := discoverAll(t, map[string]string{
			"functions/bluesky/" + missing.have: "def run(session_id: str):\n    return 1\n",
		})
		if err == nil {
			t.Fatalf("%s alone should be an error", missing.have)
		}
		if !strings.Contains(err.Error(), missing.want) {
			t.Errorf("error should name the missing half %q: %v", missing.want, err)
		}
	}
}

// Grouping is a convention, not a rule: both depths work, so a project can
// stay flat while small and group when it is not.
func TestDiscoveryIsDepthIndependent(t *testing.T) {
	fns, conns, err := discoverAll(t, map[string]string{
		"functions/flat/main.py":             "def run():\n    return 1\n",
		"functions/group/nested/main.py":     "def run():\n    return 1\n",
		"functions/auth/bluesky/login.py":    "def run(session_id: str):\n    return 1\n",
		"functions/auth/bluesky/verifier.py": "def run(session_id: str):\n    return 1\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, f := range fns {
		names[f.Name] = f.Entrypoint
	}
	if names["flat"] != "flat/main.py" || names["nested"] != "group/nested/main.py" {
		t.Fatalf("functions = %+v", fns)
	}
	// The slug is the directory name, never the path: a grouping directory
	// must not leak into a globally unique catalog slug.
	if len(conns) != 1 || conns[0].Name != "bluesky" {
		t.Fatalf("connector name should be the directory, not the path: %+v", conns)
	}
	if conns[0].Login != "auth/bluesky/login.py" {
		t.Fatalf("entrypoint = %q", conns[0].Login)
	}
}

// A unit's subdirectories are its own helpers, not more units.
func TestAUnitIsNotSearchedFurther(t *testing.T) {
	fns, _, err := discoverAll(t, map[string]string{
		"functions/outer/main.py":       "def run():\n    return 1\n",
		"functions/outer/inner/main.py": "def run():\n    return 1\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fns) != 1 || fns[0].Name != "outer" {
		t.Fatalf("a nested main.py inside a function is a helper, not a unit: %+v", fns)
	}
}

// Grouping makes this reachable, and for connectors the name is a catalog slug
// unique across every workspace.
func TestTwoUnitsCannotClaimOneName(t *testing.T) {
	_, _, err := discoverAll(t, map[string]string{
		"functions/a/report/main.py": "def run():\n    return 1\n",
		"functions/b/report/main.py": "def run():\n    return 1\n",
	})
	if err == nil {
		t.Fatal("two functions deploying as the same name must be an error")
	}
	if !strings.Contains(err.Error(), "report") {
		t.Errorf("error should name the collision: %v", err)
	}
}

func TestDirectoryWithBothKindsIsAnError(t *testing.T) {
	_, _, err := discoverAll(t, map[string]string{
		"functions/confused/main.py":  "def run():\n    return 1\n",
		"functions/confused/login.py": "def run(session_id: str):\n    return 1\n",
	})
	if err == nil || !strings.Contains(err.Error(), "one or the other") {
		t.Fatalf("got %v", err)
	}
}

func TestUnknownConnectorConfigIsRejected(t *testing.T) {
	_, _, err := discoverAll(t, map[string]string{
		"notte.toml":                    "[project]\nname=\"d\"\n\n[connectors.typo]\nname = \"Typo\"\ndomain = \"x.test\"\n",
		"functions/bluesky/login.py":    "def run(session_id: str):\n    return 1\n",
		"functions/bluesky/verifier.py": "def run(session_id: str):\n    return 1\n",
	})
	if err == nil || !strings.Contains(err.Error(), "typo") {
		t.Fatalf("a [connectors.x] block with no connector x should be rejected: %v", err)
	}
}

func TestConnectorConfigValidation(t *testing.T) {
	ok := ConnectorConfig{Name: "Bluesky", Domain: "bsky.app", Color: "#0085ff", ProxyCountry: "us"}
	if p := ok.Validate("bluesky"); len(p) != 0 {
		t.Fatalf("valid config reported %v", p)
	}
	bad := ConnectorConfig{Color: "0085ff", ProxyCountry: "usa"}
	p := bad.Validate("bluesky")
	if len(p) != 4 {
		t.Fatalf("expected name, domain, color and proxy_country problems, got %v", p)
	}
}

// A bare .py is a single-file function only at the top level. Deeper down it
// belongs to whatever contains it, or an unfinished directory looks populated
// and its helpers get deployed.
func TestBarePythonIsAFunctionOnlyAtTheTopLevel(t *testing.T) {
	fns, _, err := discoverAll(t, map[string]string{
		"functions/quick.py":           "def run():\n    return 1\n",
		"functions/group/deep/main.py": "def run():\n    return 1\n",
		"functions/group/helper.py":    "def h():\n    return 1\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range fns {
		got[f.Name] = true
	}
	if !got["quick"] {
		t.Error("a top-level .py should be a function")
	}
	if got["helper"] {
		t.Error("a .py inside a grouping directory is a helper, not a function")
	}
	if !got["deep"] {
		t.Error("the nested directory function should still be found")
	}
}
