package bundle

import (
	"sort"
	"strings"
)

// ImportKind classifies a top-level import statement. The distinction that
// matters is Relative versus everything else: relative imports are resolved
// and inlined, absolute ones are hoisted verbatim and checked against the
// runtime allowlist.
type ImportKind int

const (
	NotImport ImportKind = iota
	ImportAbsolute
	ImportFrom
	ImportRelative
	ImportFuture
)

// Name is one imported name, with the alias it was bound under if any.
type Name struct {
	Name  string
	Alias string
}

// Binding is the module-level name this import actually defines.
//
// `import a.b` binds a, not a.b — the submodule is reached through the parent.
// Aliasing changes that to the alias in every form.
func (n Name) Binding() string {
	if n.Alias != "" {
		return n.Alias
	}
	if i := strings.IndexByte(n.Name, '.'); i >= 0 {
		return n.Name[:i]
	}
	return n.Name
}

// Import is a parsed top-level import statement.
type Import struct {
	Kind   ImportKind
	Level  int    // leading dots; relative imports only
	Module string // "" for `import x` and for `from . import x`
	Names  []Name
	Star   bool
	Stmt   Stmt
}

// Bindings are the module-level names this statement introduces.
func (im Import) Bindings() []string {
	out := make([]string, 0, len(im.Names))
	for _, n := range im.Names {
		out = append(out, n.Binding())
	}
	return out
}

// ParseImport reads an import statement, or reports false if the statement is
// not one. Input is Stmt.Text, so continuations are already joined.
func ParseImport(s Stmt) (Import, bool) {
	text := s.Text
	switch {
	case strings.HasPrefix(text, "import "):
		names, star := parseNameList(text[len("import "):])
		return Import{Kind: ImportAbsolute, Names: names, Star: star, Stmt: s}, true
	case strings.HasPrefix(text, "from "):
		rest := text[len("from "):]
		idx := strings.Index(rest, " import ")
		if idx < 0 {
			return Import{}, false
		}
		spec := strings.TrimSpace(rest[:idx])
		names, star := parseNameList(rest[idx+len(" import "):])

		level := 0
		for level < len(spec) && spec[level] == '.' {
			level++
		}
		module := strings.TrimSpace(spec[level:])

		im := Import{Level: level, Module: module, Names: names, Star: star, Stmt: s}
		switch {
		case module == "__future__" && level == 0:
			im.Kind = ImportFuture
		case level > 0:
			im.Kind = ImportRelative
		default:
			im.Kind = ImportFrom
		}
		return im, true
	}
	return Import{}, false
}

// parseNameList reads the comma-separated tail of an import statement,
// tolerating the parenthesised multi-line form the scanner has already joined.
func parseNameList(s string) ([]Name, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "(")
	s = strings.TrimSuffix(strings.TrimSpace(s), ")")

	var names []Name
	star := false
	for _, part := range strings.Split(s, ",") {
		fields := strings.Fields(part)
		switch {
		case len(fields) == 0:
			continue
		case fields[0] == "*":
			star = true
		case len(fields) >= 3 && fields[1] == "as":
			names = append(names, Name{Name: fields[0], Alias: fields[2]})
		default:
			names = append(names, Name{Name: fields[0]})
		}
	}
	return names, star
}

// TopLevelBindings are the module-level names a statement defines, for
// collision detection across concatenated modules.
//
// Attribute and subscript targets (a.b = ..., a[0] = ...) mutate an existing
// object rather than binding a module name, so they are deliberately absent.
func TopLevelBindings(s Stmt) []string {
	text := s.Text
	switch {
	case strings.HasPrefix(text, "def "):
		return []string{identAfter(text, "def ")}
	case strings.HasPrefix(text, "async def "):
		return []string{identAfter(text, "async def ")}
	case strings.HasPrefix(text, "class "):
		return []string{identAfter(text, "class ")}
	}
	if im, ok := ParseImport(s); ok {
		if im.Kind == ImportFuture {
			return nil
		}
		return im.Bindings()
	}
	if names := clauseTargets(text); len(names) > 0 {
		return names
	}
	if names := walrusTargets(text); len(names) > 0 {
		return names
	}
	return assignTargets(text)
}

// clauseTargets covers the top-level statements that bind a name without an
// '=': a for loop's variable, a with block's alias, and an except clause's
// exception. They are rare at module level but bind exactly as a def does, so
// omitting them means a real collision goes unreported.
func clauseTargets(text string) []string {
	switch {
	case strings.HasPrefix(text, "for "):
		rest := text[len("for "):]
		idx := strings.Index(rest, " in ")
		if idx < 0 {
			return nil
		}
		return splitTargets(rest[:idx])
	case strings.HasPrefix(text, "with "), strings.HasPrefix(text, "async with "),
		strings.HasPrefix(text, "except "), strings.HasPrefix(text, "except* "):
		// Every `as NAME` in the clause; `with` may carry several.
		var out []string
		for _, part := range strings.Split(text, " as ")[1:] {
			name := strings.TrimSpace(part)
			end := 0
			for end < len(name) && isIdentByte(name[end]) {
				end++
			}
			if n := name[:end]; isIdentifier(n) {
				out = append(out, n)
			}
		}
		return out
	}
	return nil
}

// walrusTargets covers `if (found := f()):` and friends at module level.
func walrusTargets(text string) []string {
	var out []string
	for i := 0; i+1 < len(text); i++ {
		if text[i] != ':' || text[i+1] != '=' {
			continue
		}
		end := i
		for end > 0 && text[end-1] == ' ' {
			end--
		}
		start := end
		for start > 0 && isIdentByte(text[start-1]) {
			start--
		}
		if n := text[start:end]; isIdentifier(n) {
			out = append(out, n)
		}
	}
	return out
}

// splitTargets pulls plain identifiers out of a possibly-tupled target list.
func splitTargets(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		name := strings.TrimSpace(strings.Trim(strings.TrimSpace(part), "()[]"))
		if isIdentifier(name) {
			out = append(out, name)
		}
	}
	return out
}

// identAfter reads the identifier following a keyword.
func identAfter(text, keyword string) string {
	rest := strings.TrimSpace(text[len(keyword):])
	end := 0
	for end < len(rest) && isIdentByte(rest[end]) {
		end++
	}
	return rest[:end]
}

// assignTargets extracts names bound by a top-level assignment, covering the
// plain, annotated, tuple and chained forms.
func assignTargets(text string) []string {
	eq := topLevelAssign(text)
	if eq < 0 {
		return nil
	}
	lhs := text[:eq]

	// Chained assignment: every segment left of the final value is a target.
	var out []string
	for _, segment := range strings.Split(lhs, "=") {
		// Annotated form: the type follows a colon and binds nothing.
		if colon := strings.IndexByte(segment, ':'); colon >= 0 {
			segment = segment[:colon]
		}
		for _, target := range strings.Split(segment, ",") {
			name := strings.TrimSpace(strings.Trim(strings.TrimSpace(target), "()[]"))
			if name == "" || !isIdentifier(name) {
				continue
			}
			out = append(out, name)
		}
	}
	return out
}

// topLevelAssign returns the index of the assignment '=' that separates
// targets from the value, or -1 if the statement assigns nothing.
//
// It returns the *last* such '=' so that chained assignment (A = B = 1) keeps
// every target on the left; taking the first silently drops all but one.
// Comparisons are stepped over rather than rejected, because `A = B == C` is a
// perfectly good binding, while an augmented operator rebinds an existing name
// and introduces none.
func topLevelAssign(text string) int {
	depth := 0
	last := -1
	var quote byte
	for i := 0; i < len(text); i++ {
		c := text[i]
		if quote != 0 {
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '=':
			if depth != 0 {
				continue
			}
			if i+1 < len(text) && text[i+1] == '=' {
				i++ // comparison; step over both characters
				continue
			}
			if i > 0 && strings.IndexByte("=!<>+-*/%&|^@", text[i-1]) >= 0 {
				return -1 // augmented assignment rebinds, it does not bind
			}
			last = i
		}
	}
	return last
}

func isIdentifier(s string) bool {
	if s == "" || (s[0] >= '0' && s[0] <= '9') {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isIdentByte(s[i]) {
			return false
		}
	}
	return true
}

func isIdentByte(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

// ExternalImports are the non-relative module names a bundled artifact
// imports, deduplicated and sorted by root.
//
// After flattening there are no relative imports left, so everything returned
// is a real module the runtime will have to resolve. Roots only: `os.path` is
// reported as `os`, because that is the granularity the allowlist works at.
func ExternalImports(code string) []string {
	seen := map[string]bool{}
	var out []string
	for _, stmt := range Scan(code) {
		if !stmt.TopLevel() {
			continue
		}
		im, ok := ParseImport(stmt)
		if !ok || (im.Kind != ImportAbsolute && im.Kind != ImportFrom) {
			continue
		}
		for _, module := range importedModules(im) {
			root := module
			if i := strings.IndexByte(root, '.'); i >= 0 {
				root = root[:i]
			}
			if root == "" || seen[root] {
				continue
			}
			seen[root] = true
			out = append(out, root)
		}
	}
	sort.Strings(out)
	return out
}

// importedModules is the module names a statement loads. `import a.b` loads
// a.b even though it binds a, and `from a.b import c` loads a.b.
func importedModules(im Import) []string {
	if im.Kind == ImportFrom {
		return []string{im.Module}
	}
	out := make([]string, 0, len(im.Names))
	for _, n := range im.Names {
		out = append(out, n.Name)
	}
	return out
}
