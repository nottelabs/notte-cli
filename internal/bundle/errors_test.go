package bundle

import (
	"strings"
	"testing"
	"testing/fstest"
)

// mapFS builds an in-memory package. Keys are paths under the functions
// directory; values are file bodies.
func mapFS(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for p, body := range files {
		fsys[p] = &fstest.MapFile{Data: []byte(body)}
	}
	return fsys
}

// wantErr bundles and requires failure, returning the message.
func wantErr(t *testing.T, files map[string]string) string {
	t.Helper()
	res, err := Bundle(mapFS(files), "fn/main.py", Options{})
	if err == nil {
		t.Fatalf("expected an error, got a bundle:\n%s", res.Code)
	}
	return err.Error()
}

func mustContain(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Fatalf("error %q does not mention %q", got, w)
		}
	}
}

// Two modules defining the same name: concatenation would silently let the
// later one win.
func TestCollisionIsReportedWithBothLocations(t *testing.T) {
	msg := wantErr(t, map[string]string{
		"fn/main.py": "from .a import a\nfrom .b import b\n\n\ndef run():\n    return a() + b()\n",
		"fn/a.py":    "def clean(s):\n    return s\n\n\ndef a():\n    return clean(1)\n",
		"fn/b.py":    "def clean(s):\n    return s\n\n\ndef b():\n    return clean(2)\n",
	})
	mustContain(t, msg, "clean", "fn/a.py", "fn/b.py", "rename")
}

// An alias occupies a name exactly as a definition does.
func TestAliasCollidesWithDefinition(t *testing.T) {
	msg := wantErr(t, map[string]string{
		"fn/main.py": "from .a import helper as fetch\nfrom .b import fetch as other\n\n\ndef run():\n    return fetch, other\n",
		"fn/a.py":    "def helper():\n    return 1\n",
		"fn/b.py":    "def fetch():\n    return 2\n",
	})
	mustContain(t, msg, "fetch")
}

// The same name imported unaliased by two modules is one definition, not a
// collision. A naive binding count reports every shared helper as conflicting.
func TestSharedHelperImportedTwiceIsNotACollision(t *testing.T) {
	res, err := Bundle(mapFS(map[string]string{
		"fn/main.py":   "from .a import a\nfrom .b import b\n\n\ndef run():\n    return a() + b()\n",
		"fn/a.py":      "from .shared import shared\n\n\ndef a():\n    return shared()\n",
		"fn/b.py":      "from .shared import shared\n\n\ndef b():\n    return shared()\n",
		"fn/shared.py": "def shared():\n    return 1\n",
	}), "fn/main.py", Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := strings.Count(res.Code, "def shared()"); n != 1 {
		t.Fatalf("shared emitted %d times, want 1", n)
	}
}

func TestImportCycleIsReported(t *testing.T) {
	msg := wantErr(t, map[string]string{
		"fn/main.py": "from .a import a\n\n\ndef run():\n    return a()\n",
		"fn/a.py":    "from .b import b\n\n\ndef a():\n    return b()\n",
		"fn/b.py":    "from .a import a\n\n\ndef b():\n    return a()\n",
	})
	mustContain(t, msg, "cycle", "fn/a.py", "fn/b.py")
}

func TestSelfImportIsReportedAsCycle(t *testing.T) {
	msg := wantErr(t, map[string]string{
		"fn/main.py": "from .main import run\n\n\ndef run():\n    return 1\n",
	})
	mustContain(t, msg, "cycle")
}

func TestStarImportIsRejected(t *testing.T) {
	msg := wantErr(t, map[string]string{
		"fn/main.py":    "from .helpers import *\n\n\ndef run():\n    return 1\n",
		"fn/helpers.py": "def h():\n    return 1\n",
	})
	mustContain(t, msg, "star", "explicitly")
}

// `from . import mod` then `mod.f()` needs the module to survive as an object,
// which flattening cannot provide. The message has to name the alternative.
func TestFromDotImportIsRejectedWithAFix(t *testing.T) {
	msg := wantErr(t, map[string]string{
		"fn/main.py":  "from . import parse\n\n\ndef run():\n    return parse.clean(1)\n",
		"fn/parse.py": "def clean(s):\n    return s\n",
	})
	mustContain(t, msg, "from .parse import")
}

func TestRelativeImportInsideFunctionIsRejected(t *testing.T) {
	msg := wantErr(t, map[string]string{
		"fn/main.py":  "def run():\n    from .parse import clean\n    return clean(1)\n",
		"fn/parse.py": "def clean(s):\n    return s\n",
	})
	mustContain(t, msg, "indented", "top of the file")
}

// An absolute import inside a function is harmless where it is: it needs no
// rewriting, so there is no reason to reject it.
func TestAbsoluteImportInsideFunctionIsAllowed(t *testing.T) {
	if _, err := Bundle(mapFS(map[string]string{
		"fn/main.py": "def run():\n    import json\n    return json.dumps({})\n",
	}), "fn/main.py", Options{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNonAnnotationsFutureImportIsRejected(t *testing.T) {
	msg := wantErr(t, map[string]string{
		"fn/main.py": "from __future__ import division\n\n\ndef run():\n    return 1\n",
	})
	mustContain(t, msg, "__future__")
}

func TestMissingModuleIsReported(t *testing.T) {
	msg := wantErr(t, map[string]string{
		"fn/main.py": "from .nope import x\n\n\ndef run():\n    return x\n",
	})
	mustContain(t, msg, "fn/nope.py")
}

// Climbing above the functions directory has no meaning; it must not silently
// resolve to something outside the tree.
func TestImportAboveRootIsReported(t *testing.T) {
	msg := wantErr(t, map[string]string{
		"fn/main.py": "from ...outside import x\n\n\ndef run():\n    return x\n",
	})
	mustContain(t, msg, "above the functions directory")
}

func TestMissingEntrypointIsReported(t *testing.T) {
	_, err := Bundle(mapFS(map[string]string{"fn/other.py": "x = 1\n"}), "fn/main.py", Options{})
	if err == nil {
		t.Fatal("expected an error")
	}
	mustContain(t, err.Error(), "fn/main.py")
}

// Errors carry a location so the message can be acted on without searching.
func TestErrorCarriesPathAndLine(t *testing.T) {
	_, err := Bundle(mapFS(map[string]string{
		"fn/main.py":    "import requests\n\nfrom .helpers import *\n",
		"fn/helpers.py": "def h():\n    return 1\n",
	}), "fn/main.py", Options{})
	if err == nil {
		t.Fatal("expected an error")
	}
	be, ok := err.(*Error)
	if !ok {
		t.Fatalf("error is %T, want *bundle.Error", err)
	}
	if be.Path != "fn/main.py" || be.Line != 3 {
		t.Fatalf("location = %s:%d, want fn/main.py:3", be.Path, be.Line)
	}
}

// `import json; import re` used to parse as a single import of a module named
// "json;", drop the whole line, and never hoist re — a NameError from an
// artifact that compiled and passed upload validation.
func TestSemicolonSeparatedImportsAreRejected(t *testing.T) {
	msg := wantErr(t, map[string]string{
		"fn/main.py": "import json; import re\n\n\ndef run():\n    return json, re\n",
	})
	mustContain(t, msg, "own line")
}

func TestImportSharingALineWithCodeIsRejected(t *testing.T) {
	msg := wantErr(t, map[string]string{
		"fn/main.py": "import json; x = 1\n\n\ndef run():\n    return x\n",
	})
	mustContain(t, msg, "own line")
}

// Semicolons elsewhere are fine; only imports constrain the rewriter.
func TestSemicolonInOrdinaryCodeIsAllowed(t *testing.T) {
	res, err := Bundle(mapFS(map[string]string{
		"fn/main.py": "A = 1; B = 2\n\n\ndef run():\n    return A + B\n",
	}), "fn/main.py", Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Code, "A = 1; B = 2") {
		t.Fatalf("line altered:\n%s", res.Code)
	}
}

// Collisions must be caught for names bound without an '=' too.
func TestClauseBoundNamesCollide(t *testing.T) {
	for _, b := range []struct{ name, src string }{
		{"for", "for item in [1]:\n    pass\n"},
		{"with", "with open(\"f\") as item:\n    pass\n"},
		{"walrus", "if (item := 1):\n    pass\n"},
	} {
		t.Run(b.name, func(t *testing.T) {
			msg := wantErr(t, map[string]string{
				"fn/main.py": "from .a import a\nfrom .b import b\n\n\ndef run():\n    return a() + b()\n",
				"fn/a.py":    b.src + "\n\ndef a():\n    return 1\n",
				"fn/b.py":    "item = 2\n\n\ndef b():\n    return item\n",
			})
			mustContain(t, msg, "item")
		})
	}
}
