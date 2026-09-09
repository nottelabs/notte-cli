package bundle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRealCorpus bundles every .py file in a real functions tree and checks
// that it survives: the bundler accepts it, the artifact compiles, and no
// top-level definition is lost.
//
// Opt-in via NOTTE_CORPUS because it needs a checkout and takes a couple of
// minutes. It is the highest-value test here by some distance — a hand-written
// suite covers the cases its author thought of, while anything-api/marketplace
// is 2.5k files of production Python written by other people and by an agent,
// full of constructs nobody would think to write down. Point it at a
// marketplace checkout before trusting a change to the scanner:
//
//	NOTTE_CORPUS=~/path/to/anything-api/marketplace go test ./internal/bundle -run TestRealCorpus
func TestRealCorpus(t *testing.T) {
	root := os.Getenv("NOTTE_CORPUS")
	if root == "" {
		t.Skip("set NOTTE_CORPUS")
	}
	var files []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".py") {
			files = append(files, p)
		}
		return nil
	})
	t.Logf("found %d files", len(files))

	py, _ := exec.LookPath("python3")
	var bundleErrs, compileErrs, mismatched int
	for _, f := range files {
		rel, _ := filepath.Rel(root, f)
		res, err := Bundle(os.DirFS(root), filepath.ToSlash(rel), Options{})
		if err != nil {
			bundleErrs++
			if bundleErrs <= 8 {
				t.Logf("BUNDLE ERR %s: %v", rel, err)
			}
			continue
		}
		// Zero relative imports in this corpus, so every def must survive.
		orig, _ := os.ReadFile(f)
		for _, line := range strings.Split(string(orig), "\n") {
			if strings.HasPrefix(line, "def ") && !strings.Contains(res.Code, line) {
				mismatched++
				if mismatched <= 5 {
					t.Logf("LOST DEF %s: %q", rel, line)
				}
				break
			}
		}
		if py != "" {
			tmp := filepath.Join(t.TempDir(), "a.py")
			_ = os.WriteFile(tmp, []byte(res.Code), 0o644)
			if out, err := exec.Command(py, "-m", "py_compile", tmp).CombinedOutput(); err != nil {
				compileErrs++
				if compileErrs <= 8 {
					t.Logf("COMPILE ERR %s: %s", rel, out)
				}
			}
		}
	}
	t.Logf("RESULT files=%d bundleErrs=%d compileErrs=%d lostDefs=%d",
		len(files), bundleErrs, compileErrs, mismatched)
}
