package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every line the bundler copied from a source file must map back to the exact
// text it came from. This is the invariant the whole source map exists for; if
// it drifts by even one line, a traceback points at the wrong statement, which
// is worse than pointing at nothing.
func TestSourceMapResolvesEveryCopiedLine(t *testing.T) {
	for _, name := range goldenCases {
		t.Run(name, func(t *testing.T) {
			res := bundleCase(t, name)
			root := filepath.Join("testdata", name, "in")

			artifactLines := strings.Split(res.Code, "\n")
			checked := 0
			for i, text := range artifactLines {
				path, srcLine, ok := res.Map.Lookup(i + 1)
				if !ok {
					continue
				}
				raw, err := os.ReadFile(filepath.Join(root, path))
				if err != nil {
					t.Fatalf("mapped to unreadable file %q: %v", path, err)
				}
				srcLines := strings.Split(string(raw), "\n")
				if srcLine < 1 || srcLine > len(srcLines) {
					t.Fatalf("artifact line %d maps to %s:%d, out of range", i+1, path, srcLine)
				}
				if got := srcLines[srcLine-1]; got != text {
					t.Fatalf("artifact line %d = %q but %s:%d = %q", i+1, text, path, srcLine, got)
				}
				checked++
			}
			if checked == 0 {
				t.Fatal("no artifact line mapped to a source; the map is empty")
			}
		})
	}
}

// Generated lines — the header, hoisted imports, alias assignments — have no
// source. Reporting a location for them would be a fabricated traceback.
func TestSourceMapReturnsNotOkForGeneratedLines(t *testing.T) {
	res := bundleCase(t, "alias-preserved")
	lines := strings.Split(res.Code, "\n")

	var aliasLine int
	for i, l := range lines {
		if strings.TrimSpace(l) == "pr = parse_rows" {
			aliasLine = i + 1
			break
		}
	}
	if aliasLine == 0 {
		t.Fatalf("alias assignment not found:\n%s", res.Code)
	}
	if path, line, ok := res.Map.Lookup(aliasLine); ok {
		t.Fatalf("generated alias line mapped to %s:%d, want no mapping", path, line)
	}
}

func TestSourceMapHeaderCommentIsNotMapped(t *testing.T) {
	res := bundleCase(t, "topo-order")
	for i, l := range strings.Split(res.Code, "\n") {
		if strings.HasPrefix(l, "# ── ") {
			if _, _, ok := res.Map.Lookup(i + 1); ok {
				t.Fatalf("module header at line %d claims a source location", i+1)
			}
		}
	}
}

func TestSourceMapLookupOutOfRange(t *testing.T) {
	res := bundleCase(t, "single-file")
	if _, _, ok := res.Map.Lookup(0); ok {
		t.Fatal("line 0 should not resolve")
	}
	if _, _, ok := res.Map.Lookup(1 << 20); ok {
		t.Fatal("a line past the artifact should not resolve")
	}
}

// Contiguous runs are coalesced, so the map stays small on a large bundle.
func TestSourceMapCoalescesContiguousRuns(t *testing.T) {
	res := bundleCase(t, "topo-order")
	if len(res.Map.Entries) > len(res.Sources)*2 {
		t.Fatalf("map has %d entries for %d sources; runs are not being merged",
			len(res.Map.Entries), len(res.Sources))
	}
}

// A run's Count must not overstate its extent, or lines belonging to the next
// module resolve to the previous one.
func TestSourceMapRunsDoNotOverlap(t *testing.T) {
	for _, name := range goldenCases {
		t.Run(name, func(t *testing.T) {
			res := bundleCase(t, name)
			prevEnd := 0
			for _, e := range res.Map.Entries {
				if e.ArtifactLine <= prevEnd {
					t.Fatalf("entry at artifact line %d overlaps a run ending at %d", e.ArtifactLine, prevEnd)
				}
				prevEnd = e.ArtifactLine + e.Count - 1
			}
		})
	}
}
