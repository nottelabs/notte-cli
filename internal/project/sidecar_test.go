package project

import (
	"strings"
	"testing"
)

func loadCfg(t *testing.T, files map[string]string) *Config {
	t.Helper()
	if _, ok := files["notte.toml"]; !ok {
		files["notte.toml"] = "[project]\nname = \"demo\"\n"
	}
	cfg, err := Load(write(t, files))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// A sidecar beside the code is what makes a tree of thousands workable:
// marketplace carries 2,524 of them, and a central block per function would be
// one file everyone edits at once.
func TestSidecarConfiguresADirectoryFunction(t *testing.T) {
	cfg := loadCfg(t, map[string]string{
		"functions/amazon/main.py": "def run():\n    return 1\n",
		"functions/amazon/function.toml": `name = "Amazon search"
description = "Searches"
domain = "amazon.com"
cron = "cron(0 9 * * ? *)"
`,
	})
	fns, _, err := DiscoverAll(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fc, err := cfg.FunctionConfigFor(fns[0])
	if err != nil {
		t.Fatal(err)
	}
	if fc.Name != "Amazon search" || fc.Domain != "amazon.com" || fc.Cron == "" {
		t.Fatalf("got %+v", fc)
	}
}

// A single-file function has no directory, so its sidecar sits beside it —
// the convention marketplace already uses.
func TestSidecarConfiguresASingleFileFunction(t *testing.T) {
	cfg := loadCfg(t, map[string]string{
		"functions/quick.py":   "def run():\n    return 1\n",
		"functions/quick.toml": "description = \"A quick one\"\n",
	})
	fns, _, err := DiscoverAll(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fc, err := cfg.FunctionConfigFor(fns[0])
	if err != nil {
		t.Fatal(err)
	}
	if fc.Description != "A quick one" {
		t.Fatalf("got %+v", fc)
	}
}

// Central still works, so a small project needs no sidecars at all.
func TestCentralConfigStillApplies(t *testing.T) {
	cfg := loadCfg(t, map[string]string{
		"notte.toml":               "[project]\nname=\"d\"\n\n[functions.amazon]\ndescription = \"central\"\n",
		"functions/amazon/main.py": "def run():\n    return 1\n",
	})
	fns, _, err := DiscoverAll(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fc, err := cfg.FunctionConfigFor(fns[0])
	if err != nil {
		t.Fatal(err)
	}
	if fc.Description != "central" {
		t.Fatalf("got %+v", fc)
	}
}

// Configuring a unit in both places is an error rather than a precedence rule.
// Picking a winner means the loser's edits silently do nothing, which is the
// failure this design refuses everywhere else.
func TestConfiguringAUnitTwiceIsAnError(t *testing.T) {
	cfg := loadCfg(t, map[string]string{
		"notte.toml":                     "[project]\nname=\"d\"\n\n[functions.amazon]\ndescription = \"central\"\n",
		"functions/amazon/main.py":       "def run():\n    return 1\n",
		"functions/amazon/function.toml": "description = \"sidecar\"\n",
	})
	fns, _, err := DiscoverAll(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cfg.FunctionConfigFor(fns[0])
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"amazon", "function.toml", ConfigName} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name %q: %v", want, err)
		}
	}
}

// TOML's failure mode for a typo is silence, so a sidecar gets the same
// unknown-key treatment the project file does.
func TestSidecarUnknownKeyIsRejected(t *testing.T) {
	cfg := loadCfg(t, map[string]string{
		"functions/amazon/main.py":       "def run():\n    return 1\n",
		"functions/amazon/function.toml": "descriptoin = \"typo\"\n",
	})
	fns, _, err := DiscoverAll(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.FunctionConfigFor(fns[0]); err == nil || !strings.Contains(err.Error(), "descriptoin") {
		t.Fatalf("expected the key to be named, got %v", err)
	}
}

func TestConnectorSidecar(t *testing.T) {
	cfg := loadCfg(t, map[string]string{
		"functions/bluesky/login.py":    "def run(session_id: str):\n    return 1\n",
		"functions/bluesky/verifier.py": "def run(session_id: str):\n    return 1\n",
		"functions/bluesky/connector.toml": `name = "Bluesky"
domain = "bsky.app"
category = "Social"
color = "#0085ff"
`,
	})
	_, conns, err := DiscoverAll(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := cfg.ConnectorConfigFor(conns[0])
	if err != nil {
		t.Fatal(err)
	}
	if cc.Name != "Bluesky" || cc.Domain != "bsky.app" {
		t.Fatalf("got %+v", cc)
	}
	if p := cc.Validate("bluesky"); len(p) != 0 {
		t.Fatalf("valid connector reported %v", p)
	}
}

// A sidecar must not be named notte.toml: Find walks up looking for that name,
// so any command run from inside a function would treat the function's
// directory as the project root.
func TestSidecarNameCannotShadowTheProjectFile(t *testing.T) {
	if FunctionSidecar == ConfigName || ConnectorSidecar == ConfigName {
		t.Fatal("a sidecar sharing the project filename would break upward discovery")
	}
}
