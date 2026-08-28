package bundle

import (
	"fmt"
	"sort"
	"strings"
)

// The Notte runtime gates imports twice, and this package mirrors the stricter
// of the two so that a rejection happens locally in milliseconds rather than
// after a multipart upload.
//
//   - Upload: ScriptValidator.parse_script(source, restricted=True) applies the
//     full RestrictedPython policy plus notte_api.ast.ALLOWED_IMPORTS.
//   - Runtime: the Lambda runner executes with restricted=False, so the AST
//     policy is off — but __import__ stays bound to safe_import, which name
//     checks every import against its own list. tempfile in particular is
//     allowed at upload and removed at run time.
//
// These lists are a vendored copy and will drift. The intended fix is a
// capabilities endpoint the CLI can fetch and cache; until that exists, a name
// wrong in one direction rejects code that would have run, and wrong in the
// other accepts code that fails inside the sandbox.
var (
	// deniedStdlib is the union of both denylists: process control, filesystem
	// access, raw sockets, dynamic import and native memory.
	deniedStdlib = words(`
		_ctypes _elementtree _imp _io _multiprocessing _pickle _posixshmem
		_posixsubprocess _signal _socket _sqlite3 _thread asyncio.subprocess
		builtins code codeop compileall configparser ctypes dbm fcntl filecmp
		fileinput gc glob grp importlib inspect linecache marshal mmap
		modulefinder multiprocessing nt os pathlib pickle pickletools pkgutil
		posix pty pwd py_compile resource runpy shelve shutil signal socket
		socketserver sqlite3 subprocess sys sysconfig tarfile tempfile termios
		threading tty venv winreg xml zipapp zipfile zipimport
	`)

	// allowedThirdParty is everything outside the standard library that the
	// runner image provides.
	allowedThirdParty = words(`
		notte notte_sdk notte_core notte_browser notte_agent notte_llm
		pydantic loguru requests httpx httpcloak playwright gspread google
		litellm bs4 pipedream tqdm typing_extensions
	`)
)

func words(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// fixes are the substitutions worth naming, because the alternative is
// discoverable only by reading the runner.
var fixes = map[string]string{
	"os":       "use `from notte_sdk.types import os` to read environment variables",
	"pathlib":  "the runtime has no writable filesystem outside /tmp",
	"tempfile": "removed from the runtime allowlist; write to /tmp directly if you must",
	"sys":      "not available at run time",
}

// Issue is one import the runtime will reject.
type Issue struct {
	Path   string
	Line   int
	Module string
	Hint   string
}

func (i Issue) Error() string {
	msg := fmt.Sprintf("%s:%d: import of %q is not allowed", i.Path, i.Line, i.Module)
	if i.Hint != "" {
		msg += " — " + i.Hint
	}
	return msg
}

// CheckImports reports absolute imports the Notte runtime will refuse.
//
// Relative imports are absent by construction: Bundle has already inlined them,
// so anything left is a real module name the runner will look up.
func CheckImports(res *Result, stdlib map[string]bool) []Issue {
	if stdlib == nil {
		stdlib = DefaultStdlib()
	}
	var issues []Issue
	for _, stmt := range Scan(res.Code) {
		im, ok := ParseImport(stmt)
		if !ok || (im.Kind != ImportAbsolute && im.Kind != ImportFrom) {
			continue
		}
		for _, module := range importedModules(im) {
			if allowed(module, stdlib) {
				continue
			}
			path, line, mapped := res.Map.Lookup(stmt.StartLine)
			if !mapped {
				// Hoisted imports are generated lines with no source; report
				// them against the artifact rather than inventing a location.
				path, line = "<bundle>", stmt.StartLine
			}
			issues = append(issues, Issue{Path: path, Line: line, Module: module, Hint: fixes[root(module)]})
		}
	}
	sort.Slice(issues, func(a, b int) bool { return issues[a].Module < issues[b].Module })
	return issues
}

// importedModules is the module names a statement actually loads. `import a.b`
// loads a.b even though it binds a, and both halves must be checked.
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

// allowed applies the runtime's rule: a denied root denies its submodules, and
// an allowed root allows them.
func allowed(module string, stdlib map[string]bool) bool {
	if deniedStdlib[module] {
		return false
	}
	r := root(module)
	if deniedStdlib[r] {
		return false
	}
	return allowedThirdParty[r] || stdlib[r]
}

func root(module string) string {
	if i := strings.IndexByte(module, '.'); i >= 0 {
		return module[:i]
	}
	return module
}
