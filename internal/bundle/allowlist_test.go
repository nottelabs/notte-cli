package bundle

import (
	"strings"
	"testing"
)

// checkSource bundles a single-file function and runs the import check.
func checkSource(t *testing.T, src string) []Issue {
	t.Helper()
	res, err := Bundle(mapFS(map[string]string{"fn/main.py": src}), "fn/main.py", Options{})
	if err != nil {
		t.Fatalf("bundle failed: %v", err)
	}
	return CheckImports(res, nil)
}

func TestAllowedImportsPass(t *testing.T) {
	src := `import json
import re
import requests
import httpx
from pydantic import BaseModel
from notte_sdk import NotteClient
from notte_sdk.types import os
from bs4 import BeautifulSoup


def run():
    return 1
`
	if issues := checkSource(t, src); len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
}

// The denylist has to beat the stdlib set, or every blocked module passes
// simply by being part of the standard library.
func TestDeniedStdlibImportsAreReported(t *testing.T) {
	for _, module := range []string{"os", "sys", "subprocess", "pathlib", "socket", "importlib", "shutil", "tempfile"} {
		t.Run(module, func(t *testing.T) {
			issues := checkSource(t, "import "+module+"\n\n\ndef run():\n    return 1\n")
			if len(issues) != 1 {
				t.Fatalf("got %d issues, want 1: %v", len(issues), issues)
			}
			if issues[0].Module != module {
				t.Fatalf("module = %q, want %q", issues[0].Module, module)
			}
		})
	}
}

// os is the one every author reaches for, and the substitute is not guessable.
func TestOsImportCarriesTheNotteSdkHint(t *testing.T) {
	issues := checkSource(t, "import os\n\n\ndef run():\n    return 1\n")
	if len(issues) != 1 {
		t.Fatalf("got %v", issues)
	}
	if !strings.Contains(issues[0].Hint, "notte_sdk.types") {
		t.Fatalf("hint = %q, want the notte_sdk.types substitution", issues[0].Hint)
	}
	if !strings.Contains(issues[0].Error(), "not allowed") {
		t.Fatalf("message = %q", issues[0].Error())
	}
}

// `from notte_sdk.types import os` is the sanctioned form and must not be
// confused with importing os itself.
func TestSanctionedOsImportIsAllowed(t *testing.T) {
	issues := checkSource(t, "from notte_sdk.types import os\n\n\ndef run():\n    return os.environ.get(\"X\")\n")
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
}

func TestSubmoduleOfDeniedRootIsDenied(t *testing.T) {
	issues := checkSource(t, "import os.path\n\n\ndef run():\n    return 1\n")
	if len(issues) != 1 {
		t.Fatalf("got %v", issues)
	}
}

func TestSubmoduleOfAllowedRootIsAllowed(t *testing.T) {
	if issues := checkSource(t, "from notte_sdk.client import X\n\n\ndef run():\n    return 1\n"); len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	if issues := checkSource(t, "import xml.etree\n\n\ndef run():\n    return 1\n"); len(issues) == 0 {
		t.Fatal("xml is denied, so xml.etree must be too")
	}
}

func TestUnknownThirdPartyIsReported(t *testing.T) {
	issues := checkSource(t, "import pandas\n\n\ndef run():\n    return 1\n")
	if len(issues) != 1 || issues[0].Module != "pandas" {
		t.Fatalf("got %v", issues)
	}
}

// An import inside a helper module must be attributed to that file, not to the
// artifact, or the author is sent to the wrong place.
func TestIssueIsAttributedToTheSourceFile(t *testing.T) {
	res, err := Bundle(mapFS(map[string]string{
		"fn/main.py":   "from .helper import helper\n\n\ndef run():\n    return helper()\n",
		"fn/helper.py": "def helper():\n    import subprocess\n    return subprocess\n",
	}), "fn/main.py", Options{})
	if err != nil {
		t.Fatal(err)
	}
	issues := CheckImports(res, nil)
	if len(issues) != 1 {
		t.Fatalf("got %d issues: %v", len(issues), issues)
	}
	if issues[0].Path != "fn/helper.py" {
		t.Fatalf("attributed to %q, want fn/helper.py", issues[0].Path)
	}
	if issues[0].Line != 2 {
		t.Fatalf("line = %d, want 2", issues[0].Line)
	}
}

func TestIssuesAreSortedForStableOutput(t *testing.T) {
	issues := checkSource(t, "import sys\nimport os\nimport pandas\n\n\ndef run():\n    return 1\n")
	if len(issues) != 3 {
		t.Fatalf("got %v", issues)
	}
	for i := 1; i < len(issues); i++ {
		if issues[i-1].Module > issues[i].Module {
			t.Fatalf("not sorted: %v", issues)
		}
	}
}

// The denylist must win over the generated stdlib set for every name it covers,
// otherwise regenerating stdlib.go on a new Python silently opens a hole.
func TestDenylistAlwaysBeatsStdlib(t *testing.T) {
	for module := range deniedStdlib {
		if allowed(module, DefaultStdlib()) {
			t.Errorf("%q is denied but allowed() accepted it", module)
		}
	}
}
