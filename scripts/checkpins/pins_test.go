// Package checkpins guards the workflow files against unpinned third-party
// actions. A mutable tag such as `actions/checkout@v6` can be moved to any
// commit by the action's maintainers, so every `uses:` must reference a full
// commit SHA and carry the resolved version as a trailing comment.
package checkpins

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// usesRef extracts the action reference from any line carrying a `uses:` key,
// whether written as a block entry or inside an inline mapping such as
// `- {uses: actions/checkout@v6}`. The trailing comment, if any, is kept.
var usesRef = regexp.MustCompile(`\buses:\s*["']?([^"',}\n]+)`)

// pinned matches `owner/repo@<40 hex chars> # vX.Y.Z`, with an optional
// subpath after the repo.
var pinned = regexp.MustCompile(`^[\w.-]+/[\w.-]+(/[\w./-]+)?@[0-9a-f]{40}\s+#\s*v\S+$`)

func TestWorkflowActionsArePinnedToCommitSHAs(t *testing.T) {
	dir := filepath.Join("..", "..", ".github", "workflows")
	files, err := filepath.Glob(filepath.Join(dir, "*.y*ml"))
	if err != nil {
		t.Fatalf("listing workflows: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no workflow files found under %s", dir)
	}

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			t.Fatalf("opening %s: %v", file, err)
		}
		scanner := bufio.NewScanner(f)
		line := 0
		for scanner.Scan() {
			line++
			m := usesRef.FindStringSubmatch(scanner.Text())
			if m == nil {
				continue
			}
			ref := strings.TrimSpace(m[1])
			// Local actions and container images are not fetched from a
			// third-party git ref, so there is no tag to pin.
			if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "docker://") {
				continue
			}
			if !pinned.MatchString(ref) {
				t.Errorf("%s:%d: %q must be pinned to a full commit SHA with a version comment", filepath.Base(file), line, ref)
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		_ = f.Close()
	}
}
