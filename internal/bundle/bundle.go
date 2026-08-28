// Package bundle flattens a Python package into the single file the Notte
// functions API accepts.
//
// The API rejects every form of local import: `from . import x` is refused
// outright by the upload validator, and `from .util import x` then fails the
// import allowlist by name — at run time as well, because the Lambda runner
// keeps __import__ bound to safe_import even though it disables the
// RestrictedPython AST policy. So a multi-module function has to become one
// module before it is uploaded, and it has to do so statically: the sys.modules
// prelude every off-the-shelf bundler emits would need imports that are
// themselves blocked.
//
// Flattening is concatenation in dependency order with relative imports
// removed. Names that would collide are an error rather than something to
// mangle, which is what keeps a full Python parser out of this package: nothing
// is ever rewritten, so nothing has to be understood well enough to rewrite.
// It also keeps the artifact readable, which matters because the artifact is
// what the console shows and what tracebacks point at.
package bundle

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// Result is a successful bundle.
type Result struct {
	// Code is the artifact: valid Python defining exactly the names the
	// entrypoint's module would have had.
	Code string
	// Sources are the contributing files in emission order, entrypoint last.
	Sources []string
	// SourceSHA256 covers the set of inputs. It answers "does this need
	// deploying?" and is stable across bundler changes that do not change the
	// sources.
	SourceSHA256 string
	// ArtifactSHA256 covers Code. It answers "what changed upstream?" and is
	// what a remote diff compares against.
	ArtifactSHA256 string
	// Map resolves an artifact line back to the file and line it came from.
	Map *SourceMap
}

// Error is a bundling failure carrying the location that caused it.
type Error struct {
	Path string
	Line int
	Msg  string
	Hint string
}

func (e *Error) Error() string {
	loc := e.Path
	if e.Line > 0 {
		loc = fmt.Sprintf("%s:%d", e.Path, e.Line)
	}
	if e.Hint != "" {
		return fmt.Sprintf("%s: %s — %s", loc, e.Msg, e.Hint)
	}
	return fmt.Sprintf("%s: %s", loc, e.Msg)
}

func errAt(p string, line int, msg, hint string) *Error {
	return &Error{Path: p, Line: line, Msg: msg, Hint: hint}
}

// Options tunes the emitted artifact.
type Options struct {
	// Header is prepended verbatim. Callers pass provenance here; the bundler
	// does not invent one so that output stays a pure function of the input.
	Header string
}

// module is one parsed source file.
type module struct {
	path    string // slash-separated, relative to the package root
	src     string
	lines   []string
	stmts   []Stmt
	imports []Import
	deps    []string // resolved paths of relative imports, in source order
}

// Bundle flattens the package reachable from entrypoint into one file.
//
// fsys is rooted at the functions directory, and entrypoint is a path within
// it such as "amazon_search/main.py". Only relative imports are followed;
// absolute ones are hoisted and left for the allowlist check.
func Bundle(fsys fs.FS, entrypoint string, opts Options) (*Result, error) {
	mods := map[string]*module{}
	order, err := collect(fsys, entrypoint, mods, nil)
	if err != nil {
		return nil, err
	}
	if err := checkCollisions(order, mods); err != nil {
		return nil, err
	}
	return emit(order, mods, opts)
}

// collect loads the entrypoint and everything it reaches, returning modules in
// dependency-first order. stack carries the current resolution path so a cycle
// can be reported as the cycle it is rather than as a stack overflow.
func collect(fsys fs.FS, p string, mods map[string]*module, stack []string) ([]string, error) {
	for i, s := range stack {
		if s == p {
			return nil, errAt(p, 0, "import cycle: "+strings.Join(append(stack[i:], p), " -> "),
				"break the cycle by moving the shared names into their own module")
		}
	}
	if _, done := mods[p]; done {
		return nil, nil
	}

	m, err := load(fsys, p)
	if err != nil {
		return nil, err
	}
	mods[p] = m

	var order []string
	for _, dep := range m.deps {
		sub, err := collect(fsys, dep, mods, append(stack, p))
		if err != nil {
			return nil, err
		}
		order = append(order, sub...)
	}
	return append(order, p), nil
}

func load(fsys fs.FS, p string) (*module, error) {
	raw, err := fs.ReadFile(fsys, p)
	if err != nil {
		return nil, errAt(p, 0, "cannot read module", "")
	}
	src := string(raw)
	m := &module{
		path:  p,
		src:   src,
		lines: strings.Split(src, "\n"),
		stmts: Scan(src),
	}

	for _, s := range m.stmts {
		im, ok := ParseImport(s)
		if !ok {
			continue
		}
		if !s.TopLevel() {
			// A relative import inside a function body would have to be
			// rewritten in place, and rewriting is the thing this design
			// avoids. Absolute ones are harmless where they are.
			if im.Kind == ImportRelative {
				return nil, errAt(p, s.StartLine, "relative import inside an indented block",
					"move it to the top of the file")
			}
			continue
		}
		m.imports = append(m.imports, im)

		switch im.Kind {
		case ImportFuture:
			if len(im.Names) != 1 || im.Names[0].Name != "annotations" {
				return nil, errAt(p, s.StartLine, "only 'from __future__ import annotations' is allowed", "")
			}
		case ImportRelative:
			if im.Star {
				return nil, errAt(p, s.StartLine, "star imports cannot be flattened",
					"import the names explicitly")
			}
			if im.Module == "" {
				return nil, errAt(p, s.StartLine, "'from . import <module>' cannot be flattened",
					fmt.Sprintf("use 'from .%s import <name>' instead", firstName(im)))
			}
			dep, err := resolve(p, im)
			if err != nil {
				return nil, err
			}
			m.deps = append(m.deps, dep)
		}
	}
	return m, nil
}

func firstName(im Import) string {
	if len(im.Names) > 0 {
		return im.Names[0].Name
	}
	return "mod"
}

// resolve turns a relative import into a path within the package root.
//
// Level 1 is the importing module's own package, level 2 its parent, and so
// on — the same rule Python uses, so a layout that resolves here resolves in
// the editor too.
func resolve(from string, im Import) (string, error) {
	pkg := path.Dir(from)
	if pkg == "." {
		pkg = ""
	}
	for i := 1; i < im.Level; i++ {
		if pkg == "" {
			return "", errAt(from, im.Stmt.StartLine,
				"relative import goes above the functions directory", "")
		}
		pkg = path.Dir(pkg)
		if pkg == "." {
			pkg = ""
		}
	}
	rel := strings.ReplaceAll(im.Module, ".", "/") + ".py"
	if pkg == "" {
		return rel, nil
	}
	return pkg + "/" + rel, nil
}

// checkCollisions rejects two modules defining the same module-level name.
//
// Concatenation makes the later definition win silently, so this is reported
// rather than resolved. Aliases count: `from .parse import clean as fetch`
// introduces `fetch` exactly as a def would.
func checkCollisions(order []string, mods map[string]*module) error {
	type owner struct {
		path string
		line int
	}
	seen := map[string]owner{}

	for _, p := range order {
		m := mods[p]
		for _, s := range m.stmts {
			if !s.TopLevel() {
				continue
			}
			for _, name := range bindingsOwnedBy(s) {
				if prev, dup := seen[name]; dup && prev.path != p {
					return errAt(p, s.StartLine,
						fmt.Sprintf("%s:%d and %s:%d both define %q", prev.path, prev.line, p, s.StartLine, name),
						"rename one of them")
				}
				seen[name] = owner{path: p, line: s.StartLine}
			}
		}
	}
	return nil
}

// bindingsOwnedBy are the names a statement introduces into the flattened
// namespace.
//
// An unaliased relative import is excluded: `from .parse import clean` refers
// to the very definition that will be concatenated in, so counting it would
// report every shared helper as colliding with itself. Absolute imports are
// excluded because they are hoisted and deduplicated, so four modules importing
// requests produce one binding rather than four.
func bindingsOwnedBy(s Stmt) []string {
	im, isImport := ParseImport(s)
	if !isImport {
		return TopLevelBindings(s)
	}
	if im.Kind != ImportRelative {
		return nil
	}
	var out []string
	for _, n := range im.Names {
		if n.Alias != "" {
			out = append(out, n.Alias)
		}
	}
	return out
}
