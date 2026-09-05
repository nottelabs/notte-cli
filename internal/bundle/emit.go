package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// SourceMap maps 1-based artifact lines back to where they came from.
//
// Without it a traceback points into a concatenated file and the reader has to
// guess which module line 612 belonged to. Entries are sorted by ArtifactLine
// and cover every emitted source line; generated lines (the header, hoisted
// imports, alias assignments) have an empty Path.
type SourceMap struct {
	Entries []MapEntry `json:"entries"`
}

// MapEntry is one contiguous run of artifact lines from one source file.
type MapEntry struct {
	ArtifactLine int    `json:"artifact_line"`
	Path         string `json:"path"`
	SourceLine   int    `json:"source_line"`
	Count        int    `json:"count"`
}

// Lookup resolves an artifact line to its source location. ok is false for
// generated lines, which have no source.
func (sm *SourceMap) Lookup(artifactLine int) (path string, line int, ok bool) {
	i := sort.Search(len(sm.Entries), func(i int) bool {
		return sm.Entries[i].ArtifactLine > artifactLine
	}) - 1
	if i < 0 {
		return "", 0, false
	}
	e := sm.Entries[i]
	if e.Path == "" || artifactLine >= e.ArtifactLine+e.Count {
		return "", 0, false
	}
	return e.Path, e.SourceLine + (artifactLine - e.ArtifactLine), true
}

// writer accumulates artifact lines and the map alongside them.
type writer struct {
	lines   []string
	entries []MapEntry
}

// generated appends lines with no source counterpart.
func (w *writer) generated(lines ...string) {
	w.lines = append(w.lines, lines...)
}

// fromSource appends one source line and records where it came from, extending
// the previous run when it is contiguous.
func (w *writer) fromSource(path string, srcLine int, text string) {
	artifactLine := len(w.lines) + 1
	if n := len(w.entries); n > 0 {
		last := &w.entries[n-1]
		if last.Path == path && last.ArtifactLine+last.Count == artifactLine && last.SourceLine+last.Count == srcLine {
			last.Count++
			w.lines = append(w.lines, text)
			return
		}
	}
	w.entries = append(w.entries, MapEntry{
		ArtifactLine: artifactLine,
		Path:         path,
		SourceLine:   srcLine,
		Count:        1,
	})
	w.lines = append(w.lines, text)
}

// emit concatenates the modules in dependency order.
//
// Per module: relative imports become alias assignments or vanish, absolute
// imports are lifted to a single deduplicated block at the top, and every other
// line is copied verbatim so the artifact still reads like the sources.
func emit(order []string, mods map[string]*module, opts Options) (*Result, error) {
	w := &writer{}

	if opts.Header != "" {
		for _, line := range strings.Split(strings.TrimRight(opts.Header, "\n"), "\n") {
			w.generated(line)
		}
		w.generated("")
	}

	// `from __future__` must precede every other statement, so it is emitted
	// once here no matter which module asked for it.
	if anyFutureAnnotations(order, mods) {
		w.generated("from __future__ import annotations", "")
	}

	if hoisted := hoistImports(order, mods); len(hoisted) > 0 {
		w.generated(hoisted...)
		w.generated("")
	}

	for _, p := range order {
		m := mods[p]
		drop, replace := rewritePlan(m)

		w.generated("# ── " + p + " ──")
		started := false
		for i, text := range m.lines {
			lineNo := i + 1
			if lineNo == len(m.lines) && text == "" {
				continue // trailing newline artefact of the split
			}
			if drop[lineNo] {
				continue
			}
			if aliases, ok := replace[lineNo]; ok {
				w.generated(aliases...)
				started = true
				continue
			}
			// Removing a module's imports strands the blank lines that
			// separated them from the first definition, under the header
			// comment. Skip forward to real content.
			if !started && strings.TrimSpace(text) == "" {
				continue
			}
			started = true
			w.fromSource(p, lineNo, text)
		}
		w.generated("")
	}

	code := strings.Join(w.lines, "\n")
	if !strings.HasSuffix(code, "\n") {
		code += "\n"
	}

	return &Result{
		Code:           code,
		Sources:        order,
		SourceSHA256:   sourceHash(order, mods),
		ArtifactSHA256: sha256Hex(code),
		Map:            &SourceMap{Entries: w.entries},
	}, nil
}

// rewritePlan decides, per physical line, what emission does with it.
//
// drop covers lines that leave entirely (hoisted absolute imports, __future__,
// unaliased relative imports). replace maps the first line of an aliased
// relative import to the assignments that preserve its bindings.
func rewritePlan(m *module) (drop map[int]bool, replace map[int][]string) {
	drop = map[int]bool{}
	replace = map[int][]string{}

	for _, im := range m.imports {
		switch im.Kind {
		case ImportAbsolute, ImportFrom, ImportFuture:
			for l := im.Stmt.StartLine; l <= im.Stmt.EndLine; l++ {
				drop[l] = true
			}
		case ImportRelative:
			// The definition arrives by concatenation, so the unaliased name is
			// already bound. An alias is not, and deleting the line without
			// recreating it is a NameError at run time that nothing before
			// production would catch.
			var assigns []string
			for _, n := range im.Names {
				if n.Alias != "" {
					assigns = append(assigns, fmt.Sprintf("%s = %s", n.Alias, n.Name))
				}
			}
			for l := im.Stmt.StartLine; l <= im.Stmt.EndLine; l++ {
				drop[l] = true
			}
			if len(assigns) > 0 {
				delete(drop, im.Stmt.StartLine)
				replace[im.Stmt.StartLine] = assigns
			}
		}
	}
	return drop, replace
}

func anyFutureAnnotations(order []string, mods map[string]*module) bool {
	for _, p := range order {
		for _, im := range mods[p].imports {
			if im.Kind == ImportFuture {
				return true
			}
		}
	}
	return false
}

// hoistImports collects every absolute import, deduplicated and sorted.
//
// Sorting is what makes the artifact reproducible: the same sources must yield
// the same bytes, because ArtifactSHA256 drives the remote diff.
func hoistImports(order []string, mods map[string]*module) []string {
	seen := map[string]bool{}
	var plain, from []string
	for _, p := range order {
		for _, im := range mods[p].imports {
			if im.Kind != ImportAbsolute && im.Kind != ImportFrom {
				continue
			}
			text := normalizeImport(im)
			if seen[text] {
				continue
			}
			seen[text] = true
			if im.Kind == ImportAbsolute {
				plain = append(plain, text)
			} else {
				from = append(from, text)
			}
		}
	}
	sort.Strings(plain)
	sort.Strings(from)
	return append(plain, from...)
}

// normalizeImport rebuilds an import from its parsed form so that two spellings
// of the same import deduplicate.
func normalizeImport(im Import) string {
	names := make([]string, 0, len(im.Names))
	for _, n := range im.Names {
		if n.Alias != "" {
			names = append(names, n.Name+" as "+n.Alias)
		} else {
			names = append(names, n.Name)
		}
	}
	sort.Strings(names)
	if im.Kind == ImportAbsolute {
		return "import " + strings.Join(names, ", ")
	}
	if im.Star {
		return "from " + im.Module + " import *"
	}
	return "from " + im.Module + " import " + strings.Join(names, ", ")
}

// sourceHash covers the inputs rather than the output, so it is stable when the
// bundler changes but the sources do not.
func sourceHash(order []string, mods map[string]*module) string {
	paths := append([]string(nil), order...)
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		// hash.Hash.Write is documented never to return an error.
		_, _ = fmt.Fprintf(h, "%s:%s\n", p, sha256Hex(mods[p].src))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
