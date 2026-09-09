package bundle

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -update rewrites the golden files. Review the diff it produces: these files
// are the specification of what the bundler emits.
var update = flag.Bool("update", false, "rewrite golden files")

// goldenCases are the fixtures under testdata/<name>/in, bundled from
// fn/main.py and compared against testdata/<name>/want.py.
var goldenCases = []string{
	"single-file",
	"alias-preserved",
	"topo-order",
	"diamond",
	"import-hoist-dedup",
	"future-annotations",
	"shared-parent",
	"docstring-not-an-import",
}

func bundleCase(t *testing.T, name string) *Result {
	t.Helper()
	res, err := Bundle(os.DirFS(filepath.Join("testdata", name, "in")), "fn/main.py", Options{})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}

func TestGolden(t *testing.T) {
	for _, name := range goldenCases {
		t.Run(name, func(t *testing.T) {
			res := bundleCase(t, name)
			goldenPath := filepath.Join("testdata", name, "want.py")

			if *update {
				if err := os.WriteFile(goldenPath, []byte(res.Code), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("missing golden file (run: go test ./internal/bundle -update): %v", err)
			}
			if res.Code != string(want) {
				t.Errorf("artifact differs from golden\n--- got ---\n%s\n--- want ---\n%s", res.Code, want)
			}
		})
	}
}

// The artifact hash drives the remote diff, so identical inputs must produce
// identical bytes. Map iteration order is the obvious way for that to break.
func TestBundleIsDeterministic(t *testing.T) {
	for _, name := range goldenCases {
		t.Run(name, func(t *testing.T) {
			first := bundleCase(t, name)
			for i := 0; i < 8; i++ {
				again := bundleCase(t, name)
				if again.Code != first.Code {
					t.Fatalf("run %d differs from run 0", i+1)
				}
				if again.ArtifactSHA256 != first.ArtifactSHA256 {
					t.Fatalf("ArtifactSHA256 unstable: %s vs %s", again.ArtifactSHA256, first.ArtifactSHA256)
				}
				if again.SourceSHA256 != first.SourceSHA256 {
					t.Fatalf("SourceSHA256 unstable")
				}
			}
		})
	}
}

// The whole point of the alias rule: the artifact must still bind pr.
func TestAliasBindingSurvivesInArtifact(t *testing.T) {
	res := bundleCase(t, "alias-preserved")
	if !strings.Contains(res.Code, "pr = parse_rows") {
		t.Fatalf("alias assignment missing:\n%s", res.Code)
	}
	if strings.Contains(res.Code, "from .parse import") {
		t.Fatalf("relative import survived into the artifact:\n%s", res.Code)
	}
	// clean is unaliased, so it needs no assignment — the definition carries it.
	if strings.Contains(res.Code, "clean = clean") {
		t.Fatalf("emitted a redundant self-assignment:\n%s", res.Code)
	}
}

func TestTopologicalOrder(t *testing.T) {
	res := bundleCase(t, "topo-order")
	if want := []string{"fn/base.py", "fn/mid.py", "fn/main.py"}; !equal(res.Sources, want) {
		t.Fatalf("sources = %v, want %v", res.Sources, want)
	}
	base := strings.Index(res.Code, "def base()")
	middle := strings.Index(res.Code, "def middle()")
	run := strings.Index(res.Code, "def run()")
	if base >= middle || middle >= run {
		t.Fatalf("definitions out of dependency order: base=%d middle=%d run=%d", base, middle, run)
	}
}

// A module reached by two paths must appear once; twice would be a redefinition
// and, for a class, a different object than the one already captured.
func TestDiamondEmitsSharedModuleOnce(t *testing.T) {
	res := bundleCase(t, "diamond")
	if n := strings.Count(res.Code, "def shared()"); n != 1 {
		t.Fatalf("shared emitted %d times, want 1:\n%s", n, res.Code)
	}
	if n := countString(res.Sources, "fn/shared.py"); n != 1 {
		t.Fatalf("shared listed %d times in Sources", n)
	}
}

func TestHoistedImportsAreDeduplicated(t *testing.T) {
	res := bundleCase(t, "import-hoist-dedup")
	if n := strings.Count(res.Code, "import requests"); n != 1 {
		t.Fatalf("`import requests` appears %d times, want 1:\n%s", n, res.Code)
	}
	if n := strings.Count(res.Code, "from pydantic import BaseModel"); n != 1 {
		t.Fatalf("pydantic import appears %d times, want 1:\n%s", n, res.Code)
	}
}

// Only one __future__ import, and it has to be the first statement or Python
// raises SyntaxError.
func TestFutureAnnotationsEmittedOnceAndFirst(t *testing.T) {
	res := bundleCase(t, "future-annotations")
	if n := strings.Count(res.Code, "from __future__ import annotations"); n != 1 {
		t.Fatalf("appears %d times, want 1:\n%s", n, res.Code)
	}
	for _, line := range strings.Split(res.Code, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line != "from __future__ import annotations" {
			t.Fatalf("first real statement is %q, want the __future__ import", line)
		}
		break
	}
}

func TestParentPackageImportResolves(t *testing.T) {
	res := bundleCase(t, "shared-parent")
	if want := []string{"_shared/http.py", "fn/main.py"}; !equal(res.Sources, want) {
		t.Fatalf("sources = %v, want %v", res.Sources, want)
	}
	if !strings.Contains(res.Code, "def fetch_json(q):") {
		t.Fatalf("shared helper not inlined:\n%s", res.Code)
	}
}

// A single-file function has nothing to flatten; it must survive intact.
func TestSingleFileFunctionIsUnchangedApartFromHoisting(t *testing.T) {
	res := bundleCase(t, "single-file")
	if len(res.Sources) != 1 {
		t.Fatalf("sources = %v", res.Sources)
	}
	if !strings.Contains(res.Code, `return requests.get("https://x.test").text`) {
		t.Fatalf("body altered:\n%s", res.Code)
	}
}

// Import-looking text inside a docstring must not be followed.
func TestDocstringImportsAreNotResolved(t *testing.T) {
	res := bundleCase(t, "docstring-not-an-import")
	if want := []string{"fn/real.py", "fn/main.py"}; !equal(res.Sources, want) {
		t.Fatalf("sources = %v, want %v — a docstring import was followed", res.Sources, want)
	}
	if !strings.Contains(res.Code, "from .ghost import missing") {
		t.Fatalf("docstring content was stripped; it should be copied verbatim:\n%s", res.Code)
	}
}

func countString(xs []string, want string) int {
	n := 0
	for _, x := range xs {
		if x == want {
			n++
		}
	}
	return n
}
